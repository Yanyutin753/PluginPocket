package filestore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func poolForTest(t *testing.T) *pgxpool.Pool {
	t.Helper()
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("PLUGINPOCKET_TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("files_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); _, _ = conn.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); _ = conn.Close(ctx) })
	return db.Pool
}
func TestInlineBinaryAcrossInstances(t *testing.T) {
	p := poolForTest(t)
	ctx := context.Background()
	a, _ := New(p, Options{})
	b, _ := New(p, Options{})
	data := []byte{0, 255, 128, 10}
	ref, err := a.Put(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := b.Put(ctx, data)
	if err != nil || ref != ref2 {
		t.Fatalf("dedupe: %v %v", ref2, err)
	}
	got, err := b.Get(ctx, ref)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("binary roundtrip: %x %v", got, err)
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM file_objects").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rows %d %v", count, err)
	}
	ref.Size++
	if _, err = b.Get(ctx, ref); err == nil {
		t.Fatal("wrong size accepted")
	}
	if _, err = a.Put(ctx, make([]byte, MaxFileBytes+1)); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err = a.Put(ctx, make([]byte, DefaultInlineMaxBytes+1)); err == nil {
		t.Fatal("large file without S3 accepted")
	}
}
func TestS3RoundtripAndFailure(t *testing.T) {
	p := poolForTest(t)
	ctx := context.Background()
	data := []byte{0, 255, 128, 1, 2}
	var saved []byte
	fail := false
	corrupt := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("missing SDK signature")
		}
		if !strings.HasPrefix(r.URL.Path, "/private/files/") {
			t.Errorf("path %s", r.URL.Path)
		}
		if fail {
			w.WriteHeader(403)
			return
		}
		if r.Method == "PUT" {
			saved, _ = io.ReadAll(r.Body)
			w.WriteHeader(200)
			return
		}
		if corrupt {
			_, _ = w.Write([]byte("corrupt"))
			return
		}
		_, _ = w.Write(saved)
	}))
	defer server.Close()
	opts := Options{Endpoint: server.URL, Region: "auto", Bucket: "private", AccessKeyID: "test", SecretAccessKey: "test", Prefix: "files", PathStyle: true, AllowHTTP: true, InlineMaxBytes: 1}
	a, err := New(p, opts)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := a.Put(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New(p, opts)
	got, err := b.Get(ctx, ref)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("S3 roundtrip %x %v", got, err)
	}
	corrupt = true
	if _, err = b.Get(ctx, ref); err == nil {
		t.Fatal("corrupt S3 body accepted")
	}
	corrupt = false
	opts.Bucket = "different"
	other, _ := New(p, opts)
	if _, err = other.Get(ctx, ref); err == nil {
		t.Fatal("changed target accepted")
	}
	fail = true
	if _, err = a.Put(ctx, []byte("new object")); err == nil {
		t.Fatal("failed upload accepted")
	}
	var n int
	_ = p.QueryRow(ctx, "SELECT count(*) FROM file_objects").Scan(&n)
	if n != 1 {
		t.Fatalf("failed upload left metadata: %d", n)
	}
}
func TestOptionsRejectUnsafeEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://localhost:9000", "https://user:secret@example.com", "https://example.com?token=x"} {
		if _, err := New(nil, Options{Endpoint: endpoint, Bucket: "files", Region: "auto", AccessKeyID: "a", SecretAccessKey: "b"}); err == nil {
			t.Errorf("accepted %s", endpoint)
		}
	}
}

func TestConcurrentDeduplicationAndEmptyFiles(t *testing.T) {
	p := poolForTest(t)
	ctx := context.Background()
	data := []byte{0, 128, 255}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			s, _ := New(p, Options{})
			ref, err := s.Put(ctx, data)
			if err != nil {
				t.Error(err)
				return
			}
			got, err := s.Get(ctx, ref)
			if err != nil || !bytes.Equal(got, data) {
				t.Errorf("concurrent read: %v", err)
			}
		})
	}
	wg.Wait()
	var count int
	if err := p.QueryRow(ctx, "SELECT count(*) FROM file_objects").Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent rows %d %v", count, err)
	}
	s, _ := New(p, Options{})
	ref, err := s.Put(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, ref)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty file %x %v", got, err)
	}
}

func TestGenericStoreAllowsGitEncodingOverhead(t *testing.T) {
	s, err := New(poolForTest(t), Options{InlineMaxBytes: 16 << 20})
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, (8<<20)+1024)
	ctx := context.Background()
	ref, err := s.Put(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, ref)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("git sized object failed: %v", err)
	}
}

func TestTransactionReusesSingleConnectionAndRollsBack(t *testing.T) {
	original := poolForTest(t)
	cfg := original.Config()
	cfg.MaxConns = 1
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	s, _ := New(p, Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	bound := s.WithTx(tx)
	data := []byte{0, 255, 128}
	ref, err := bound.Put(ctx, data)
	if err != nil {
		t.Fatalf("transaction Put must reuse its connection: %v", err)
	}
	got, err := bound.Get(ctx, ref)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("transaction Get: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, ref); err == nil {
		t.Fatal("rolled-back object is visible")
	}
	if _, err = s.Put(ctx, data); err != nil {
		t.Fatalf("original store must retain pool: %v", err)
	}
}
