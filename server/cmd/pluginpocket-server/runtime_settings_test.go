package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/Yanyutin753/PluginPocket/server/internal/config"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
)

func TestApplicationReloadsPersistedSettingsWithoutRestart(t *testing.T) {
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("runtime_application_%d", time.Now().UnixNano())
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
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err = s.Pool.Exec(ctx, "INSERT INTO users(username,password_hash,role) VALUES('settings-admin','unused','admin'); INSERT INTO wallets(user_id) VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{1}, 32)
	cfg := config.Config{DatabaseURL: u.String(), PublicURL: "https://pluginpocket.test", InitialCredits: 17, GitHubClientID: "environment-client", GitHubClientSecret: "environment-secret", EncryptionKey: key}
	h, closeHandler, err := applicationHandler(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeHandler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/meta", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"github":true`) {
		t.Fatalf("environment default metadata status=%d", w.Code)
	}
	if _, err = s.Pool.Exec(ctx, "INSERT INTO sessions(user_id,session_hash,expires_at) VALUES(1,$1,now()+interval '1 hour')", auth.Digest("admin-session")); err != nil {
		t.Fatal(err)
	}
	patch := httptest.NewRequest("PATCH", cfg.PublicURL+"/api/v1/admin/settings", strings.NewReader(`{"revision":0,"initial_credits":73,"github_enabled":false,"github_client_id":"","github_org":"","smtp_enabled":false,"smtp_address":"","smtp_from":"","smtp_username":""}`))
	patch.Header.Set("Origin", cfg.PublicURL)
	patch.AddCookie(&http.Cookie{Name: "pluginpocket_session", Value: "admin-session"})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, patch)
	if w.Code != 200 || strings.Contains(w.Body.String(), cfg.GitHubClientSecret) {
		t.Fatalf("administrator settings save failed or exposed a secret: status=%d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/meta", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"github":false`) {
		t.Fatalf("application retained environment settings after database save: status=%d body=%s", w.Code, w.Body)
	}
	r := httptest.NewRequest("POST", cfg.PublicURL+"/api/v1/auth/register", strings.NewReader(`{"username":"runtime-user","password":"correct horse battery"}`))
	r.Header.Set("Origin", cfg.PublicURL)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"balance":73`) {
		t.Fatalf("application registration retained old grant: status=%d body=%s", w.Code, w.Body)
	}
}
