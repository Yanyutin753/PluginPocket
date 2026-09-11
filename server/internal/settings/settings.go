package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Yanyutin753/loadout/server/internal/gateway"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const LockID int64 = 817392107

var (
	ErrConflict              = errors.New("settings revision conflict")
	ErrInvalid               = errors.New("invalid settings")
	ErrEncryptionUnavailable = errors.New("settings encryption unavailable")
)

type Values struct {
	AccessTokenSeconds  int    `json:"access_token_seconds"`
	RefreshTokenSeconds int    `json:"refresh_token_seconds"`
	InitialCredits      int64  `json:"initial_credits"`
	GitHubEnabled       bool   `json:"github_enabled"`
	GitHubClientID      string `json:"github_client_id"`
	GitHubClientSecret  string `json:"-"`
	GitHubOrg           string `json:"github_org"`
	SMTPEnabled         bool   `json:"smtp_enabled"`
	SMTPAddress         string `json:"smtp_address"`
	SMTPFrom            string `json:"smtp_from"`
	SMTPUsername        string `json:"smtp_username"`
	SMTPPassword        string `json:"-"`
}

type Snapshot struct {
	Values
	Revision int64 `json:"revision"`
}

type Querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Manager struct {
	Pool     *pgxpool.Pool
	Defaults Values
	Key      []byte
	Origin   string
}

var organizationPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,39}$`)

func (v Values) Validate(origin string) error {
	v = v.WithBrowserDefaults()
	if v.AccessTokenSeconds < 60 || v.AccessTokenSeconds > 86400 || v.RefreshTokenSeconds < v.AccessTokenSeconds || v.RefreshTokenSeconds > 31536000 {
		return ErrInvalid
	}
	if v.InitialCredits < 0 || v.InitialCredits > 1_000_000_000_000 || len(v.GitHubClientID) > 256 || len(v.GitHubClientSecret) > 4096 || len(v.SMTPAddress) > 320 || len(v.SMTPFrom) > 320 || len(v.SMTPUsername) > 320 || len(v.SMTPPassword) > 4096 {
		return ErrInvalid
	}
	if v.GitHubOrg != "" && !organizationPattern.MatchString(v.GitHubOrg) {
		return ErrInvalid
	}
	if v.GitHubEnabled || v.SMTPEnabled {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return ErrInvalid
		}
	}
	if v.GitHubEnabled && (strings.TrimSpace(v.GitHubClientID) == "" || strings.TrimSpace(v.GitHubClientSecret) == "") {
		return ErrInvalid
	}
	if v.SMTPEnabled {
		host, port, err := net.SplitHostPort(v.SMTPAddress)
		number, portErr := strconv.Atoi(port)
		if err != nil || host == "" || strings.ContainsAny(host, " /\\\t\r\n") || portErr != nil || number < 1 || number > 65535 {
			return ErrInvalid
		}
		if _, err = mail.ParseAddress(v.SMTPFrom); err != nil || strings.ContainsAny(v.SMTPFrom, "\r\n") {
			return ErrInvalid
		}
		if (v.SMTPUsername == "") != (v.SMTPPassword == "") {
			return ErrInvalid
		}
	}
	return nil
}

func (v Values) WithBrowserDefaults() Values {
	if v.AccessTokenSeconds == 0 {
		v.AccessTokenSeconds = 900
	}
	if v.RefreshTokenSeconds == 0 {
		v.RefreshTokenSeconds = 604800
	}
	return v
}

type secrets struct {
	GitHubClientSecret string `json:"github_client_secret"`
	SMTPPassword       string `json:"smtp_password"`
}

func (m *Manager) Read(ctx context.Context, q Querier) (Snapshot, error) {
	if q == nil {
		q = m.Pool
	}
	var snapshot Snapshot
	var public, encrypted []byte
	err := q.QueryRow(ctx, "SELECT revision,public_values,encrypted_secrets FROM runtime_settings WHERE id=1").Scan(&snapshot.Revision, &public, &encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{Values: m.Defaults.WithBrowserDefaults()}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	if json.Unmarshal(public, &snapshot.Values) != nil {
		return Snapshot{}, ErrInvalid
	}
	if len(encrypted) > 0 {
		plain, err := gateway.OpenConfig(m.Key, encrypted)
		if err != nil {
			return Snapshot{}, ErrEncryptionUnavailable
		}
		var private secrets
		if json.Unmarshal(plain, &private) != nil {
			return Snapshot{}, ErrEncryptionUnavailable
		}
		snapshot.GitHubClientSecret = private.GitHubClientSecret
		snapshot.SMTPPassword = private.SMTPPassword
	}
	snapshot.Values = snapshot.WithBrowserDefaults()
	return snapshot, nil
}

func (m *Manager) Write(ctx context.Context, tx pgx.Tx, v Values, expectedRevision, actorID int64) error {
	if err := v.Validate(m.Origin); err != nil {
		return err
	}
	if expectedRevision < 0 {
		return ErrConflict
	}
	public, err := json.Marshal(v)
	if err != nil {
		return ErrInvalid
	}
	var encrypted []byte
	if v.GitHubClientSecret != "" || v.SMTPPassword != "" {
		plain, err := json.Marshal(secrets{GitHubClientSecret: v.GitHubClientSecret, SMTPPassword: v.SMTPPassword})
		if err != nil {
			return ErrInvalid
		}
		encrypted, err = gateway.SealConfig(m.Key, plain)
		if err != nil {
			return ErrEncryptionUnavailable
		}
	}
	var revision int64
	if expectedRevision == 0 {
		err = tx.QueryRow(ctx, `INSERT INTO runtime_settings(id,revision,public_values,encrypted_secrets,updated_by) VALUES(1,1,$1,$2,$3) ON CONFLICT(id) DO NOTHING RETURNING revision`, public, encrypted, actorID).Scan(&revision)
	} else {
		err = tx.QueryRow(ctx, `UPDATE runtime_settings SET revision=revision+1,public_values=$1,encrypted_secrets=$2,updated_by=$3,updated_at=statement_timestamp() WHERE id=1 AND revision=$4 RETURNING revision`, public, encrypted, actorID, expectedRevision).Scan(&revision)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	return err
}
