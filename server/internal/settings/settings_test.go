package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
)

func settingsDB(t *testing.T) *store.Store {
	t.Helper()
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("settings_%d", time.Now().UnixNano())
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = conn.Close(ctx)
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err = s.Pool.Exec(ctx, "INSERT INTO users(username,password_hash,role) VALUES('admin','unused','admin')"); err != nil {
		t.Fatal(err)
	}
	return s
}

func writeSettings(t *testing.T, m *Manager, v Values, revision int64) error {
	t.Helper()
	tx, err := m.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = m.Write(t.Context(), tx, v, revision, 1); err != nil {
		return err
	}
	return tx.Commit(t.Context())
}

func TestSettingsPersistAcrossManagersEncryptSecretsAndRejectStaleWrites(t *testing.T) {
	s := settingsDB(t)
	m := &Manager{Pool: s.Pool, Defaults: Values{InitialCredits: 42}, Key: bytes.Repeat([]byte{1}, 32), Origin: "https://pluginpocket.test"}
	initial, err := m.Read(t.Context(), nil)
	if err != nil || initial.Revision != 0 || initial.InitialCredits != 42 {
		t.Fatalf("defaults: revision=%d credits=%d error=%v", initial.Revision, initial.InitialCredits, err)
	}
	v := Values{InitialCredits: 73, GitHubEnabled: true, GitHubClientID: "client", GitHubClientSecret: "private-github-secret", SMTPEnabled: true, SMTPAddress: "smtp.example.com:587", SMTPFrom: "pluginpocket@example.com", SMTPUsername: "mailer", SMTPPassword: "private-mail-password"}
	if err = writeSettings(t, m, v, 0); err != nil {
		t.Fatal(err)
	}
	peer := &Manager{Pool: s.Pool, Defaults: Values{InitialCredits: 999}, Key: m.Key, Origin: m.Origin}
	saved, err := peer.Read(t.Context(), nil)
	if err != nil || saved.Revision != 1 || saved.Values != v.WithBrowserDefaults() {
		t.Fatalf("saved settings not visible on peer: revision=%d credits=%d error=%v", saved.Revision, saved.InitialCredits, err)
	}
	encoded, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	var public, secrets []byte
	var actor int64
	if err = s.Pool.QueryRow(t.Context(), "SELECT public_values,encrypted_secrets,updated_by FROM runtime_settings WHERE id=1").Scan(&public, &secrets, &actor); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{encoded, public, secrets} {
		if bytes.Contains(raw, []byte(v.GitHubClientSecret)) || bytes.Contains(raw, []byte(v.SMTPPassword)) {
			t.Fatal("secret exposed in serialized settings or database plaintext")
		}
	}
	if len(secrets) == 0 || actor != 1 {
		t.Fatal("missing encryption or audit actor")
	}
	v.InitialCredits = 101
	if err = writeSettings(t, peer, v, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if err = writeSettings(t, peer, v, 1); err != nil {
		t.Fatal(err)
	}
	saved, err = m.Read(t.Context(), nil)
	if err != nil || saved.Revision != 2 || saved.InitialCredits != 101 {
		t.Fatalf("live reload revision=%d credits=%d error=%v", saved.Revision, saved.InitialCredits, err)
	}
	peer.Key = bytes.Repeat([]byte{2}, 32)
	if _, err = peer.Read(t.Context(), nil); !errors.Is(err, ErrEncryptionUnavailable) {
		t.Fatalf("wrong key silently accepted or leaked crypto internals: %v", err)
	}
}

func TestSettingsWithoutEncryptionKeyOnlySaveValuesWithoutSecrets(t *testing.T) {
	s := settingsDB(t)
	m := &Manager{Pool: s.Pool, Origin: "https://pluginpocket.test"}
	v := Values{InitialCredits: 19}
	if err := writeSettings(t, m, v, 0); err != nil {
		t.Fatal(err)
	}
	saved, err := m.Read(t.Context(), nil)
	if err != nil || saved.Revision != 1 || saved.InitialCredits != 19 {
		t.Fatalf("public settings not persisted: revision=%d credits=%d error=%v", saved.Revision, saved.InitialCredits, err)
	}
	v.GitHubClientSecret = "new-secret"
	if err = writeSettings(t, m, v, 1); !errors.Is(err, ErrEncryptionUnavailable) {
		t.Fatalf("secret accepted without key: %v", err)
	}
	v.GitHubClientSecret = ""
	if err = writeSettings(t, m, v, 1); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsValidation(t *testing.T) {
	for name, change := range map[string]func(*Values){
		"negative credits":     func(v *Values) { v.InitialCredits = -1 },
		"excessive credits":    func(v *Values) { v.InitialCredits = 1_000_000_000_001 },
		"incomplete github":    func(v *Values) { v.GitHubEnabled = true },
		"invalid organization": func(v *Values) { v.GitHubOrg = "../bad" },
		"incomplete smtp":      func(v *Values) { v.SMTPEnabled = true },
		"invalid smtp port":    func(v *Values) { v.SMTPEnabled = true; v.SMTPAddress = "mail.example:0"; v.SMTPFrom = "a@example.com" },
		"invalid smtp sender": func(v *Values) {
			v.SMTPEnabled = true
			v.SMTPAddress = "mail.example:587"
			v.SMTPFrom = "bad\r\nX: injected"
		},
		"incomplete smtp auth": func(v *Values) {
			v.SMTPEnabled = true
			v.SMTPAddress = "mail.example:587"
			v.SMTPFrom = "a@example.com"
			v.SMTPUsername = "user"
		},
		"oversized secret": func(v *Values) { v.SMTPPassword = strings.Repeat("s", 4097) },
	} {
		t.Run(name, func(t *testing.T) {
			v := Values{}
			change(&v)
			if err := v.Validate("https://pluginpocket.test"); !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid values accepted: %v", err)
			}
		})
	}
	v := Values{GitHubEnabled: true, GitHubClientID: "client", GitHubClientSecret: "secret"}
	if err := v.Validate(""); !errors.Is(err, ErrInvalid) {
		t.Fatal("enabled integration accepted without public origin")
	}
	if err := (Values{}).Validate(""); err != nil {
		t.Fatal("disabled integrations require an origin")
	}
}
