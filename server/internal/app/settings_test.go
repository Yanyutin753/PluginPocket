package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/settings"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

const settingsPath = "/api/v1/admin/settings"

func runtimeFixture(t *testing.T) (*store.Store, *settings.Manager, http.Handler, *http.Cookie, *http.Cookie) {
	t.Helper()
	s, h := setup(t)
	admin := register(t, h, "operator")
	user := register(t, h, "ordinary")
	if _, err := s.Pool.Exec(t.Context(), "UPDATE users SET role='admin' WHERE username='operator'"); err != nil {
		t.Fatal(err)
	}
	runtime := &settings.Manager{Pool: s.Pool, Defaults: settings.Values{InitialCredits: 7}, Key: bytes.Repeat([]byte{9}, 32), Origin: "http://example.com"}
	return s, runtime, New(s, Options{Origin: runtime.Origin, Runtime: runtime}), admin, user
}
func runtimeBody(t *testing.T, revision, credits int64, changes map[string]any) string {
	t.Helper()
	values := map[string]any{"revision": revision, "initial_credits": credits, "github_enabled": false, "github_client_id": "", "github_org": "", "smtp_enabled": false, "smtp_address": "", "smtp_from": "", "smtp_username": ""}
	for k, v := range changes {
		values[k] = v
	}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func runtimeResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("settings response %d: %s", w.Code, w.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestRuntimeSettingsPersistAcrossHandlersAndChangeRegistrationCredits(t *testing.T) {
	s, runtime, h, admin, _ := runtimeFixture(t)
	initial := runtimeResponse(t, request(h, http.MethodGet, settingsPath, "", admin))["item"].(map[string]any)
	if initial["initial_credits"] != float64(7) || initial["revision"] != float64(0) {
		t.Fatalf("unexpected defaults: %v", initial)
	}
	secrets := map[string]any{"github_enabled": true, "github_client_id": "client-fixture", "github_client_secret": "github-fixture-secret", "smtp_password": "smtp-fixture-secret"}
	saved := request(h, http.MethodPatch, settingsPath, runtimeBody(t, 0, 42, secrets), admin)
	item := runtimeResponse(t, saved)["item"].(map[string]any)
	if item["revision"] != float64(1) || item["github_client_secret_set"] != true || item["smtp_password_set"] != true {
		t.Fatalf("save response missing revision or secret status: %v", item)
	}
	if strings.Contains(saved.Body.String(), "fixture-secret") {
		t.Fatal("PATCH echoed secret")
	}
	nextRuntime := &settings.Manager{Pool: s.Pool, Defaults: settings.Values{InitialCredits: 999}, Key: runtime.Key, Origin: runtime.Origin}
	next := New(s, Options{Origin: runtime.Origin, Runtime: nextRuntime})
	loaded := request(next, http.MethodGet, settingsPath, "", admin)
	item = runtimeResponse(t, loaded)["item"].(map[string]any)
	if item["initial_credits"] != float64(42) || item["revision"] != float64(1) || strings.Contains(loaded.Body.String(), "fixture-secret") {
		t.Fatalf("replica did not read safe persisted values: %s", loaded.Body)
	}
	if _, ok := item["github_client_secret"]; ok {
		t.Fatal("raw secret key returned")
	}
	if _, ok := item["smtp_password"]; ok {
		t.Fatal("raw password key returned")
	}
	// Public updates omit both secret fields, preserving their encrypted values.
	body := runtimeBody(t, 1, 43, map[string]any{"github_enabled": true, "github_client_id": "client-fixture"})
	runtimeResponse(t, request(next, http.MethodPatch, settingsPath, body, admin))
	snapshot, err := runtime.Read(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.GitHubClientSecret != "github-fixture-secret" || snapshot.SMTPPassword != "smtp-fixture-secret" {
		t.Fatal("omitted secrets were overwritten")
	}
	if w := request(h, http.MethodPatch, settingsPath, body, admin); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "settings_conflict") {
		t.Fatalf("stale version accepted: %d %s", w.Code, w.Body)
	}
	cookie := register(t, next, "newmember")
	account := request(next, http.MethodGet, "/api/v1/account/me", "", cookie)
	if account.Code != http.StatusOK || !strings.Contains(account.Body.String(), `"balance":43`) {
		t.Fatalf("hot credits not applied: %d %s", account.Code, account.Body)
	}
	runtimeResponse(t, request(h, http.MethodPatch, settingsPath, runtimeBody(t, 2, 0, map[string]any{"github_client_secret": "", "smtp_password": ""}), admin))
	snapshot, err = nextRuntime.Read(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.GitHubClientSecret != "" || snapshot.SMTPPassword != "" {
		t.Fatal("explicit secret clearing was ignored")
	}
}
func TestRegistrationReadsRuntimeDefaults(t *testing.T) {
	_, _, h, _, _ := runtimeFixture(t)
	cookie := register(t, h, "runtimecredits")
	w := request(h, http.MethodGet, "/api/v1/account/me", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"balance":7`) {
		t.Fatalf("runtime default credits ignored: %d %s", w.Code, w.Body)
	}
}
func TestRuntimeSettingsRequireAdministratorAndOrigin(t *testing.T) {
	s, _, h, admin, user := runtimeFixture(t)
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		for _, credential := range []struct {
			cookie *http.Cookie
			want   int
		}{{nil, http.StatusUnauthorized}, {user, http.StatusForbidden}} {
			if w := request(h, method, settingsPath, runtimeBody(t, 0, 10, nil), credential.cookie); w.Code != credential.want {
				t.Fatalf("%s permission got %d want %d", method, w.Code, credential.want)
			}
		}
	}
	r := httptest.NewRequest(http.MethodPatch, "http://example.com"+settingsPath, strings.NewReader(runtimeBody(t, 0, 10, nil)))
	r.AddCookie(admin)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation returned %d", w.Code)
	}
	if _, err := s.Pool.Exec(t.Context(), "UPDATE sessions SET expires_at=statement_timestamp()-interval '1 second' WHERE user_id=1"); err != nil {
		t.Fatal(err)
	}
	if w = request(h, http.MethodGet, settingsPath, "", admin); w.Code != http.StatusUnauthorized {
		t.Fatalf("expired session returned %d", w.Code)
	}
}
func TestRuntimeSettingsRejectInvalidAndUnavailableSecretWrites(t *testing.T) {
	s, runtime, h, admin, _ := runtimeFixture(t)
	for _, body := range []string{
		`{}`, `{"initial_credits":1}`, runtimeBody(t, 0, -1, nil), runtimeBody(t, 0, 1, map[string]any{"github_enabled": true}), runtimeBody(t, 0, 1, map[string]any{"smtp_enabled": true, "smtp_address": "bad", "smtp_from": "invalid"}),
	} {
		if w := request(h, http.MethodPatch, settingsPath, body, admin); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_request") {
			t.Fatalf("invalid settings response: %d %s", w.Code, w.Body)
		}
	}
	runtime.Key = nil
	unavailable := runtimeResponse(t, request(h, http.MethodGet, settingsPath, "", admin))
	if unavailable["secret_writes_available"] != false {
		t.Fatal("missing encryption key advertised as available")
	}
	if w := request(h, http.MethodPatch, settingsPath, runtimeBody(t, 0, 1, map[string]any{"smtp_password": "secret-fixture"}), admin); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "settings_encryption_unavailable") {
		t.Fatalf("secret write without key returned %d %s", w.Code, w.Body)
	}
	runtimeResponse(t, request(h, http.MethodPatch, settingsPath, runtimeBody(t, 0, 9, nil), admin))
	noRuntime := New(s, Options{Origin: runtime.Origin})
	if w := request(noRuntime, http.MethodGet, settingsPath, "", admin); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "settings_unavailable") {
		t.Fatalf("missing runtime handler response: %d %s", w.Code, w.Body)
	}
}
func TestRuntimeSettingsDatabaseFailureDoesNotFallBack(t *testing.T) {
	s, runtime, h, admin, _ := runtimeFixture(t)
	closed, err := pgxpool.NewWithConfig(t.Context(), s.Pool.Config().Copy())
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	runtime.Pool = closed
	w := request(h, http.MethodGet, settingsPath, "", admin)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "temporarily_unavailable") {
		t.Fatalf("settings failure returned stale defaults: %d %s", w.Code, w.Body)
	}
	w = request(h, http.MethodPost, "/api/v1/auth/register", `{"username":"blockednew","password":"correct horse battery"}`, nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("registration ignored settings failure: %d %s", w.Code, w.Body)
	}
	var count int
	if err = s.Pool.QueryRow(t.Context(), "SELECT count(*) FROM users WHERE username='blockednew'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed registration persisted user")
	}
	runtime.Pool = s.Pool
	if _, err = s.Pool.Exec(t.Context(), "DROP TABLE runtime_settings"); err != nil {
		t.Fatal(err)
	}
	w = request(h, http.MethodPatch, settingsPath, runtimeBody(t, 0, 11, nil), admin)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "temporarily_unavailable") {
		t.Fatalf("transactional settings read failure returned %d %s", w.Code, w.Body)
	}

}
func TestRuntimeSettingsRecheckAuthorizationAfterWaits(t *testing.T) {
	for _, scenario := range []struct {
		lockTarget, mutation string
		want                 int
	}{
		{"advisory", "DELETE FROM sessions WHERE user_id=1", http.StatusUnauthorized},
		{"advisory", "UPDATE sessions SET expires_at=statement_timestamp()-interval '1 second' WHERE user_id=1", http.StatusUnauthorized},
		{"actor_foreign_key", "UPDATE users SET enabled=false WHERE id=1", http.StatusUnauthorized},
		{"actor_foreign_key", "UPDATE users SET role='user' WHERE id=1", http.StatusForbidden},
	} {
		lockTarget := scenario.lockTarget
		t.Run(lockTarget+scenario.mutation, func(t *testing.T) {
			s, runtime, h, admin, _ := runtimeFixture(t)
			tx, err := s.Pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			var pid int
			lock := "SELECT pg_backend_pid() FROM pg_advisory_xact_lock($1)"
			id := settings.LockID
			if lockTarget == "actor_foreign_key" {
				lock = "SELECT pg_backend_pid() FROM users WHERE id=$1 FOR UPDATE"
				id = 1
			}
			if err = tx.QueryRow(t.Context(), lock, id).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- request(h, http.MethodPatch, settingsPath, runtimeBody(t, 0, 90, nil), admin) }()
			waitForAppLock(t, s, pid)
			if _, err = tx.Exec(t.Context(), scenario.mutation); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(t.Context()); err != nil {
				t.Fatal(err)
			}
			if w := <-done; w.Code != scenario.want {
				t.Fatalf("unauthorized admin wrote after %s wait: %d %s", lockTarget, w.Code, w.Body)
			}
			snapshot, err := runtime.Read(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Revision != 0 || snapshot.InitialCredits != 7 {
				t.Fatalf("unauthorized write committed: %v", snapshot)
			}
		})
	}
}

func TestRuntimeRegistrationUsesSingleDatabaseConnection(t *testing.T) {
	s, runtime, _, _, _ := runtimeFixture(t)
	config := s.Pool.Config().Copy()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	runtime.Pool = pool
	h := New(&store.Store{Pool: pool}, Options{Origin: runtime.Origin, Runtime: runtime})
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	r := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/auth/register", strings.NewReader(`{"username":"oneconnection","password":"correct horse battery"}`)).WithContext(ctx)
	r.Header.Set("Origin", runtime.Origin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"balance":7`) {
		t.Fatalf("runtime registration needs a second pooled connection: %d %s", w.Code, w.Body)
	}
}
