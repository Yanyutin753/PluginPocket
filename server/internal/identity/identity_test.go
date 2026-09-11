package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
)

func TestUnconfiguredIdentityDoesNotPretendToWork(t *testing.T) {
	h := New(nil, Options{})
	for _, path := range []string{"/api/v1/auth/github/start", "/api/v1/auth/github/callback?code=fake&state=fake"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 503 {
			t.Fatalf("unconfigured %s returned %d", path, w.Code)
		}
	}
}

func TestGitHubOAuthBindsStateAndCreatesOneAccount(t *testing.T) {
	s := identityDB(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			_ = r.ParseForm()
			if r.Form.Get("code_verifier") == "" {
				t.Error("missing PKCE verifier")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "provider-secret", "token_type": "bearer"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer provider-secret" {
			t.Error("missing provider authentication")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1234, "login": "github-user"})
	}))
	defer provider.Close()
	cfg := &oauth2.Config{ClientID: "client", ClientSecret: "secret", RedirectURL: "http://local.test/api/v1/auth/github/callback", Endpoint: oauth2.Endpoint{AuthURL: provider.URL + "/authorize", TokenURL: provider.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}}
	initial := int64(42)
	h := New(s, Options{InitialCredits: &initial, Origin: "http://local.test", GitHub: cfg, UserURL: provider.URL + "/user"})
	start := httptest.NewRecorder()
	h.ServeHTTP(start, httptest.NewRequest("GET", "/api/v1/auth/github/start", nil))
	if start.Code != 302 {
		t.Fatalf("oauth start %d %s", start.Code, start.Body.String())
	}
	u, e := url.Parse(start.Header().Get("Location"))
	if e != nil {
		t.Fatal(e)
	}
	if u.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("PKCE challenge absent")
	}
	callback := func(state string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/auth/github/callback?code=code&state="+url.QueryEscape(state), nil)
		for _, cookie := range start.Result().Cookies() {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := callback("forged"); w.Code != 400 {
		t.Fatalf("forged state %d", w.Code)
	}
	state := u.Query().Get("state")
	done := callback(state)
	if done.Code != 302 {
		t.Fatalf("callback %d %s", done.Code, done.Body.String())
	}
	session := false
	for _, cookie := range done.Result().Cookies() {
		if cookie.Name == "pluginpocket_session" && cookie.HttpOnly {
			session = true
		}
	}
	if !session {
		t.Fatal("OAuth did not create an HttpOnly session")
	}
	if w := callback(state); w.Code != 400 {
		t.Fatalf("state replay %d", w.Code)
	}
	var users int
	_ = s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM oauth_identities WHERE provider='github' AND subject='1234'").Scan(&users)
	if users != 1 {
		t.Fatalf("OAuth identities=%d", users)
	}
	var balance, delta, after int64
	if e := s.Pool.QueryRow(context.Background(), "SELECT w.balance,l.delta,l.balance_after FROM oauth_identities o JOIN wallets w ON w.user_id=o.user_id JOIN ledger l ON l.wallet_id=w.id WHERE o.subject='1234' AND l.kind='registration'").Scan(&balance, &delta, &after); e != nil || balance != 42 || delta != 42 || after != 42 {
		t.Fatalf("OAuth grant balance=%d delta=%d after=%d err=%v", balance, delta, after, e)
	}
}

func identityDB(t *testing.T) *store.Store {
	t.Helper()
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	conn, e := pgx.Connect(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	schema := fmt.Sprintf("identity_%d", time.Now().UnixNano())
	_, e = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	if e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := store.Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = conn.Close(ctx)
	})
	return s
}

func TestEmailVerificationIsAuthenticatedOneTimeAndExpires(t *testing.T) {
	s := identityDB(t)
	ctx := context.Background()
	var id int64
	if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES ('emailuser','hash') RETURNING id").Scan(&id); e != nil {
		t.Fatal(e)
	}
	_, e := s.Pool.Exec(ctx, "INSERT INTO sessions(user_id,session_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')", id, auth.Digest("session"))
	if e != nil {
		t.Fatal(e)
	}
	var delivered string
	h := New(s, Options{Origin: "http://local.test", Mail: func(ctx context.Context, to, link string) error {
		if to != "owner@example.com" {
			t.Fatal(to)
		}
		delivered = link
		return nil
	}})
	request := func(path, body, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Origin", "http://local.test")
		r.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "pluginpocket_session", Value: cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("/api/v1/account/email/request", `{"email":"owner@example.com"}`, ""); w.Code != 401 {
		t.Fatalf("anonymous email send %d", w.Code)
	}
	if w := request("/api/v1/account/email/request", `{"email":"owner@example.com"}`, "session"); w.Code != 202 {
		t.Fatalf("email request %d %s", w.Code, w.Body.String())
	}
	u, e := url.Parse(delivered)
	if e != nil {
		t.Fatal(e)
	}
	token := u.Query().Get("token")
	if token == "" {
		t.Fatal("no verification token in mail")
	}
	w := request("/api/v1/auth/email/verify", `{"token":"`+token+`"}`, "")
	if w.Code != 200 {
		t.Fatalf("verify %d %s", w.Code, w.Body.String())
	}
	if w = request("/api/v1/auth/email/verify", `{"token":"`+token+`"}`, ""); w.Code != 400 {
		t.Fatalf("token replay %d", w.Code)
	}
	var email string
	if e = s.Pool.QueryRow(ctx, "SELECT email FROM users WHERE id=$1 AND email_verified_at IS NOT NULL", id).Scan(&email); e != nil || email != "owner@example.com" {
		t.Fatalf("email persisted: %s %v", email, e)
	}
}

func pinIdentityWindow(t *testing.T, s *store.Store) {
	t.Helper()
	// Keep budget assertions in one window; store tests cover clock boundaries.
	if _, err := s.Pool.Exec(context.Background(), "UPDATE rate_limits SET window_id=floor(extract(epoch FROM statement_timestamp())/60)::bigint+60 WHERE scope LIKE 'public:%'"); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubStartBudgetAndExpiredStateCleanup(t *testing.T) {
	s := identityDB(t)
	cfg := &oauth2.Config{ClientID: "client", Endpoint: oauth2.Endpoint{AuthURL: "https://example.com/authorize"}}
	h := New(s, Options{Origin: "http://local.test", GitHub: cfg})
	if _, e := s.Pool.Exec(context.Background(), "INSERT INTO oauth_states(state_hash,browser_hash,verifier,expires_at) VALUES('expired','browser','verifier',now()-interval '1 hour')"); e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/auth/github/start", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("first OAuth start: %d", w.Code)
	}
	pinIdentityWindow(t, s)
	for attempt := 2; attempt <= 121; attempt++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/auth/github/start", nil))
		want := http.StatusFound
		if attempt == 121 {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("OAuth start %d: status=%d, want=%d", attempt, w.Code, want)
		}
	}
	var expired int
	if e := s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM oauth_states WHERE expires_at<now()").Scan(&expired); e != nil {
		t.Fatal(e)
	}
	if expired != 0 {
		t.Fatalf("retained %d expired states", expired)
	}
}
func TestDisabledEmailDoesNotReportVerified(t *testing.T) {
	s := identityDB(t)
	ctx := context.Background()
	var id int64
	if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash,enabled) VALUES('disabled','hash',false) RETURNING id").Scan(&id); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Pool.Exec(ctx, "INSERT INTO email_verifications(token_hash,user_id,email,expires_at) VALUES($1,$2,'owner@example.com',now()+interval '1 hour')", auth.Digest("verification"), id); e != nil {
		t.Fatal(e)
	}
	h := New(s, Options{Origin: "http://local.test"})
	r := httptest.NewRequest("POST", "/api/v1/auth/email/verify", strings.NewReader(`{"token":"verification"}`))
	r.Header.Set("Origin", "http://local.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("disabled email verification must be403, got%d %s", w.Code, w.Body)
	}
	var verified bool
	if e := s.Pool.QueryRow(ctx, "SELECT email_verified_at IS NOT NULL FROM users WHERE id=$1", id).Scan(&verified); e != nil || verified {
		t.Fatalf("disabled email changed: %v %v", verified, e)
	}
	var pending int
	if e := s.Pool.QueryRow(ctx, "SELECT count(*) FROM email_verifications WHERE user_id=$1", id).Scan(&pending); e != nil || pending != 1 {
		t.Fatalf("failed verification consumed token: %d %v", pending, e)
	}
}
