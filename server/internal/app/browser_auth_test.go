package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/settings"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBrowserRefreshAcrossReplicas(t *testing.T) {
	s, a := setup(t)
	peer, err := pgxpool.NewWithConfig(t.Context(), s.Pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	b := New(&store.Store{Pool: peer}, Options{Origin: "http://example.com"})
	w := request(a, "POST", "/api/v1/auth/register", `{"username":"browser","password":"correct horse battery"}`, nil)
	var access, refresh *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "loadout_session" {
			access = c
		}
		if c.Name == "loadout_refresh" {
			refresh = c
		}
	}
	if access == nil || refresh == nil || access.MaxAge != 900 || refresh.MaxAge != 604800 {
		t.Fatalf("expected short access and long refresh cookies, status=%d cookie count=%d", w.Code, len(w.Result().Cookies()))
	}
	if !refresh.HttpOnly || refresh.SameSite != http.SameSiteLaxMode {
		t.Fatal("refresh cookie attributes")
	}
	if _, err := s.Pool.Exec(t.Context(), "UPDATE session_access SET expires_at=statement_timestamp()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if w := request(b, "GET", "/api/v1/account/me", "", access); w.Code != 401 {
		t.Fatalf("expired AT: %d", w.Code)
	}
	forged := *refresh
	forged.Name = "loadout_session"
	if w := request(b, "GET", "/api/v1/account/me", "", &forged); w.Code != 401 {
		t.Fatalf("RT cannot act as AT: %d", w.Code)
	}
	const n = 4
	results := make(chan *httptest.ResponseRecorder, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { results <- request(b, "POST", "/api/v1/auth/refresh", "", refresh) })
	}
	wg.Wait()
	close(results)
	var tokens []*http.Cookie
	for result := range results {
		if result.Code != 204 {
			t.Fatalf("refresh: %d %s", result.Code, result.Body)
		}
		for _, c := range result.Result().Cookies() {
			if c.Name == "loadout_session" {
				tokens = append(tokens, c)
			}
		}
	}
	if len(tokens) != n {
		t.Fatal("missing concurrent access cookies")
	}
	for _, c := range tokens {
		if w := request(a, "GET", "/api/v1/account/me", "", c); w.Code != 200 {
			t.Fatalf("concurrent AT invalidated: %d", w.Code)
		}
	}
	if w := request(a, "POST", "/api/v1/auth/logout", "", tokens[0]); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := request(b, "POST", "/api/v1/auth/refresh", "", refresh); w.Code != 401 {
		t.Fatalf("revoked RT: %d", w.Code)
	}
	for _, c := range tokens {
		if w := request(b, "GET", "/api/v1/account/me", "", c); w.Code != 401 {
			t.Fatalf("revoked AT: %d", w.Code)
		}
	}
}

func TestBrowserLoginWithOneDatabaseConnection(t *testing.T) {
	s, runtime, _, _, _ := runtimeFixture(t)
	config := s.Pool.Config()
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	runtime.Pool = pool
	h := New(&store.Store{Pool: pool}, Options{Origin: runtime.Origin, Runtime: runtime})
	for _, endpoint := range []string{"login", "register"} {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		body := `{"username":"ordinary","password":"correct horse battery"}`
		if endpoint == "register" {
			body = `{"username":"newbrowser","password":"correct horse battery"}`
		}
		r := httptest.NewRequest("POST", "http://example.com/api/v1/auth/"+endpoint, strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Origin", runtime.Origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		cancel()
		if w.Code != 200 && w.Code != 201 {
			t.Fatalf("%s with one connection: %d %s", endpoint, w.Code, w.Body)
		}
	}
}

func TestBrowserLifetimeHotReloadAcrossReplicas(t *testing.T) {
	s, runtime, a, admin, _ := runtimeFixture(t)
	b := New(s, Options{Origin: runtime.Origin, Runtime: &settings.Manager{Pool: s.Pool, Origin: runtime.Origin, Key: runtime.Key}})
	body := runtimeBody(t, 0, 0, map[string]any{"access_token_seconds": 120, "refresh_token_seconds": 3600})
	runtimeResponse(t, request(a, "PATCH", settingsPath, body, admin))
	login := request(b, "POST", "/api/v1/auth/login", `{"username":"ordinary","password":"correct horse battery"}`, nil)
	var refresh *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == "loadout_session" && c.MaxAge != 120 {
			t.Fatalf("AT TTL not hot: %d", c.MaxAge)
		}
		if c.Name == "loadout_refresh" {
			refresh = c
			if c.MaxAge != 3600 {
				t.Fatalf("RT TTL not hot: %d", c.MaxAge)
			}
		}
	}
	if refresh == nil {
		t.Fatal("missing RT")
	}
	body = runtimeBody(t, 1, 0, map[string]any{"access_token_seconds": 60, "refresh_token_seconds": 7200})
	runtimeResponse(t, request(a, "PATCH", settingsPath, body, admin))
	renewed := request(b, "POST", "/api/v1/auth/refresh", "", refresh)
	if renewed.Code != 204 {
		t.Fatal(renewed.Code)
	}
	for _, c := range renewed.Result().Cookies() {
		if c.Name == "loadout_session" && c.MaxAge != 60 {
			t.Fatalf("refreshed AT TTL not hot: %d", c.MaxAge)
		}
	}
	var unchanged bool
	if err := s.Pool.QueryRow(t.Context(), "SELECT bool_and(expires_at-created_at<=interval '3601 seconds') FROM sessions WHERE user_id=(SELECT id FROM users WHERE username='ordinary') AND session_hash<>'' AND created_at>(SELECT min(created_at) FROM sessions WHERE user_id=(SELECT id FROM users WHERE username='ordinary'))").Scan(&unchanged); err != nil || !unchanged {
		t.Fatalf("existing RT extended: %v", err)
	}
	for _, invalid := range []map[string]any{{"access_token_seconds": 0}, {"access_token_seconds": 86401}, {"refresh_token_seconds": 31536001}, {"access_token_seconds": 7200, "refresh_token_seconds": 3600}} {
		raw := runtimeBody(t, 2, 0, invalid)
		if w := request(b, "PATCH", settingsPath, raw, admin); w.Code != 400 {
			var result any
			_ = json.Unmarshal(w.Body.Bytes(), &result)
			t.Fatalf("invalid lifetime accepted: %d %v", w.Code, result)
		}
	}
}

func TestBrowserRefreshBoundaries(t *testing.T) {
	s, h := setup(t)
	cookie := register(t, h, "boundaries")
	// Access alone must not extend its own lifetime.
	if w := request(h, "POST", "/api/v1/auth/refresh", "", cookie); w.Code != 401 {
		t.Fatalf("AT used as RT: %d", w.Code)
	}
	login := request(h, "POST", "/api/v1/auth/login", `{"username":"boundaries","password":"correct horse battery"}`, nil)
	var refresh *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == "loadout_refresh" {
			refresh = c
		}
	}
	if refresh == nil {
		t.Fatal("missing RT")
	}
	r := httptest.NewRequest("POST", "http://example.com/api/v1/auth/refresh", strings.NewReader(""))
	r.AddCookie(refresh)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross-origin refresh: %d", w.Code)
	}
	if _, err := s.Pool.Exec(t.Context(), "UPDATE users SET enabled=false"); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "POST", "/api/v1/auth/refresh", "", refresh); w.Code != 401 {
		t.Fatalf("disabled RT: %d", w.Code)
	}
	if _, err := s.Pool.Exec(t.Context(), "UPDATE users SET enabled=true; UPDATE sessions SET expires_at=statement_timestamp()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "POST", "/api/v1/auth/refresh", "", refresh); w.Code != 401 {
		t.Fatalf("expired RT: %d", w.Code)
	}
}
