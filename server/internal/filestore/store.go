package filestore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxFileBytes = 16 << 20
const DefaultInlineMaxBytes = 256 << 10

var ErrUnavailable = errors.New("file storage unavailable")

type Ref struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Options struct {
	Endpoint, Region, Bucket, AccessKeyID, SecretAccessKey, SessionToken, Prefix string
	PathStyle, AllowHTTP                                                         bool
	InlineMaxBytes                                                               int64
}
type Store struct {
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
		Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	}
	options Options
	client  *s3.Client
}

// WithTx binds file metadata reads and writes to the caller's transaction.
func (s *Store) WithTx(tx pgx.Tx) *Store { bound := *s; bound.db = tx; return &bound }

func New(pool *pgxpool.Pool, options Options) (*Store, error) {
	if options.InlineMaxBytes == 0 {
		options.InlineMaxBytes = DefaultInlineMaxBytes
	}
	if options.InlineMaxBytes < 0 || options.InlineMaxBytes > MaxFileBytes {
		return nil, errors.New("invalid inline file limit")
	}
	options.Endpoint = strings.TrimRight(options.Endpoint, "/")
	if options.Endpoint != "" {
		u, err := url.Parse(options.Endpoint)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "https" && (u.Scheme != "http" || !options.AllowHTTP)) {
			return nil, errors.New("file storage endpoint must be an HTTPS origin (HTTP requires explicit opt-in)")
		}
	}
	s := &Store{db: pool, options: options}
	if options.Bucket == "" {
		if options.Endpoint != "" || options.AccessKeyID != "" || options.SecretAccessKey != "" || options.SessionToken != "" {
			return nil, errors.New("file storage requires bucket and credentials")
		}
		return s, nil
	}
	if options.Region == "" || options.AccessKeyID == "" || options.SecretAccessKey == "" {
		return nil, errors.New("file storage requires region and credentials")
	}
	s.client = s3.New(s3.Options{Region: options.Region, BaseEndpoint: optionalString(options.Endpoint), UsePathStyle: options.PathStyle,
		Credentials: credentials.NewStaticCredentialsProvider(options.AccessKeyID, options.SecretAccessKey, options.SessionToken),
		HTTPClient:  &http.Client{Timeout: 30 * time.Second}, RetryMaxAttempts: 2,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired, ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired})
	return s, nil
}
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func reference(data []byte) Ref {
	hash := sha256.Sum256(data)
	return Ref{hex.EncodeToString(hash[:]), int64(len(data))}
}
func (s *Store) Put(ctx context.Context, data []byte) (Ref, error) {
	if len(data) > MaxFileBytes {
		return Ref{}, errors.New("file exceeds 16 MiB")
	}
	ref := reference(data)
	var exists bool
	if err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM file_objects WHERE sha256=$1)", ref.SHA256).Scan(&exists); err != nil {
		return Ref{}, err
	}
	if exists {
		_, err := s.Get(ctx, ref)
		return ref, err
	}
	var content []byte
	endpoint, region, bucket, key := "", "", "", ""
	if int64(len(data)) <= s.options.InlineMaxBytes {
		content = append([]byte{}, data...)
	} else {
		if s.client == nil {
			return Ref{}, ErrUnavailable
		}
		endpoint, region, bucket = s.options.Endpoint, s.options.Region, s.options.Bucket
		key = strings.Trim(s.options.Prefix, "/")
		if key != "" {
			key += "/"
		}
		key += ref.SHA256
		if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &bucket, Key: &key, Body: bytes.NewReader(data), ContentLength: aws.Int64(ref.Size), ContentType: aws.String("application/octet-stream")}); err != nil {
			return Ref{}, fmt.Errorf("%w: upload failed", ErrUnavailable)
		}
	}
	_, err := s.db.Exec(ctx, `INSERT INTO file_objects(sha256,size,content,endpoint,region,bucket,object_key) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(sha256) DO NOTHING`, ref.SHA256, ref.Size, content, endpoint, region, bucket, key)
	if err != nil {
		return Ref{}, err
	}
	return ref, nil
}
func (s *Store) Get(ctx context.Context, ref Ref) ([]byte, error) {
	digest, err := hex.DecodeString(ref.SHA256)
	if err != nil || len(digest) != sha256.Size || ref.SHA256 != strings.ToLower(ref.SHA256) || ref.Size < 0 || ref.Size > MaxFileBytes {
		return nil, errors.New("invalid file reference")
	}
	var size int64
	var data []byte
	var endpoint, region, bucket, key string
	err = s.db.QueryRow(ctx, `SELECT size,content,endpoint,region,bucket,object_key FROM file_objects WHERE sha256=$1`, ref.SHA256).Scan(&size, &data, &endpoint, &region, &bucket, &key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: object missing", ErrUnavailable)
	}
	if err != nil {
		return nil, err
	}
	if size != ref.Size {
		return nil, errors.New("file size mismatch")
	}
	if bucket != "" {
		if s.client == nil || endpoint != s.options.Endpoint || region != s.options.Region || bucket != s.options.Bucket {
			return nil, fmt.Errorf("%w: object belongs to a different storage target", ErrUnavailable)
		}
		out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
		if err != nil {
			return nil, fmt.Errorf("%w: download failed", ErrUnavailable)
		}
		defer func() { _ = out.Body.Close() }()
		data, err = io.ReadAll(io.LimitReader(out.Body, size+1))
		if err != nil {
			return nil, fmt.Errorf("%w: download failed", ErrUnavailable)
		}
	}
	if reference(data) != ref {
		return nil, errors.New("file integrity mismatch")
	}
	return data, nil
}
