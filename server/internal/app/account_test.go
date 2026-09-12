package app

import (
	"bytes"
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

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
)

func setup(t *testing.T) (*store.Store, http.Handler) {
	t.Helper()
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("PLUGINPOCKET_TEST_DATABASE_URL required for real PostgreSQL integration")
	}
	ctx := context.Background()
	c, e := pgx.Connect(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	schema := fmt.Sprintf("app_%d", time.Now().UnixNano())
	if _, e = c.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := store.Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = c.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = c.Close(context.Background())
	})
	zero := int64(0)
	return s, New(s, Options{Origin: "http://example.com", InitialCredits: &zero})
}
func request(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://example.com")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func register(t *testing.T, h http.Handler, name string) *http.Cookie {
	t.Helper()
	w := request(h, "POST", "/api/v1/auth/register", fmt.Sprintf(`{"username":%q,"password":"correct horse battery"}`, name), nil)
	if w.Code != 201 {
		t.Fatalf("register expected 201, got %d: %s", w.Code, w.Body)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 2 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("missing secure session attributes")
	}
	return cookies[0]
}
func TestRegisterValidationReportsSpecificCodes(t *testing.T) {
	_, h := setup(t)
	if w := request(h, "POST", "/api/v1/auth/register", `{"username":"valid_name","password":"short"}`, nil); w.Code != 400 || w.Body.String() != "{\"error\":\"invalid_password\"}\n" {
		t.Fatalf("short password: %d %s", w.Code, w.Body)
	}
	if w := request(h, "POST", "/api/v1/auth/register", `{"username":"bad name!","password":"correct horse battery"}`, nil); w.Code != 400 || w.Body.String() != "{\"error\":\"invalid_username\"}\n" {
		t.Fatalf("invalid username: %d %s", w.Code, w.Body)
	}
	if w := request(h, "POST", "/api/v1/auth/register", fmt.Sprintf(`{"username":"valid_name","password":%q}`, strings.Repeat("a", 1025)), nil); w.Code != 400 || w.Body.String() != "{\"error\":\"invalid_password\"}\n" {
		t.Fatalf("oversized password: %d %s", w.Code, w.Body)
	}
}

func TestRegistrationLoginAndRevocableSession(t *testing.T) {
	_, h := setup(t)
	cookie := register(t, h, "alice")
	if w := request(h, "GET", "/api/v1/account/me", "", cookie); w.Code != 200 || !strings.Contains(w.Body.String(), `"balance":0`) {
		t.Fatalf("me: %d %s", w.Code, w.Body)
	}
	if w := request(h, "POST", "/api/v1/auth/register", `{"username":"alice","password":"correct horse battery"}`, nil); w.Code != 409 {
		t.Fatalf("duplicate: %d", w.Code)
	}
	if w := request(h, "POST", "/api/v1/auth/login", `{"username":"alice","password":"wrong password"}`, nil); w.Code != 401 {
		t.Fatalf("wrong password: %d", w.Code)
	}
	if w := request(h, "POST", "/api/v1/auth/logout", "{}", cookie); w.Code != 204 {
		t.Fatalf("logout: %d", w.Code)
	}
	if w := request(h, "GET", "/api/v1/account/me", "", cookie); w.Code != 401 {
		t.Fatalf("revoked session: %d", w.Code)
	}
	if w := request(h, "POST", "/api/v1/auth/login", `{"username":"alice","password":"correct horse battery"}`, nil); w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
}
func TestTokenIsolationRevocationAndOrigin(t *testing.T) {
	_, h := setup(t)
	alice := register(t, h, "alice")
	bob := register(t, h, "bob")
	w := request(h, "POST", "/api/v1/account/tokens", `{"name":"Laptop"}`, alice)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var created struct {
		Token string
		Item  struct{ ID int64 }
	}
	if e := json.Unmarshal(w.Body.Bytes(), &created); e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(created.Token, "ppt_") {
		t.Fatal("missing gateway secret")
	}
	w = request(h, "GET", "/api/v1/account/tokens", "", alice)
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(created.Token)) || bytes.Contains(w.Body.Bytes(), []byte("token_hash")) {
		t.Fatalf("list leaks secret: %s", w.Body)
	}
	path := fmt.Sprintf("/api/v1/account/tokens/%d", created.Item.ID)
	if w = request(h, "DELETE", path, "", bob); w.Code != 404 {
		t.Fatalf("cross user revoke: %d", w.Code)
	}
	r := httptest.NewRequest("GET", "http://example.com/api/v1/account/verify", nil)
	r.Header.Set("Authorization", "Bearer "+created.Token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	if w = request(h, "DELETE", path, "", alice); w.Code != 204 {
		t.Fatalf("revoke: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("revoked token: %d", w.Code)
	}
	r = httptest.NewRequest("POST", "http://example.com/api/v1/account/tokens", strings.NewReader(`{"name":"evil"}`))
	r.AddCookie(alice)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross origin write: %d", w.Code)
	}
}
func TestAdminAdjustmentIdempotencyAndAuthorization(t *testing.T) {
	s, h := setup(t)
	alice := register(t, h, "alice")
	admin := register(t, h, "admin")
	ctx := context.Background()
	_, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='admin'")
	if e != nil {
		t.Fatal(e)
	}
	var id int64
	_ = s.Pool.QueryRow(ctx, "SELECT id FROM users WHERE username='alice'").Scan(&id)
	if w := request(h, "GET", "/api/v1/admin/users", "", alice); w.Code != 403 {
		t.Fatalf("user admin access: %d", w.Code)
	}
	path := fmt.Sprintf("/api/v1/admin/users/%d/balance", id)
	body := `{"delta":10,"note":"test credit","idempotency_key":"unique"}`
	for range 2 {
		if w := request(h, "POST", path, body, admin); w.Code != 200 {
			t.Fatalf("adjust: %d %s", w.Code, w.Body)
		}
	}
	if w := request(h, "POST", path, `{"delta":11,"note":"test credit","idempotency_key":"unique"}`, admin); w.Code != 409 {
		t.Fatalf("conflict: %d", w.Code)
	}
	if w := request(h, "GET", "/api/v1/account/me", "", alice); w.Code != 200 || !strings.Contains(w.Body.String(), `"balance":10`) {
		t.Fatalf("balance %d %s", w.Code, w.Body)
	}
	var count int
	_ = s.Pool.QueryRow(ctx, "SELECT count(*) FROM ledger WHERE kind='adjustment'").Scan(&count)
	if count != 1 {
		t.Fatalf("ledger entries: %d", count)
	}
}
func TestUnauthenticatedAndCrossOriginRequestsRejected(t *testing.T) {
	h := New(nil, Options{Origin: "http://example.com"})
	if w := request(h, "GET", "/api/v1/account/me", "", nil); w.Code != 401 {
		t.Fatalf("unauthenticated me: want 401 got %d", w.Code)
	}
	r := httptest.NewRequest("POST", "http://example.com/api/v1/auth/register", strings.NewReader(`{"username":"alice","password":"correct horse battery"}`))
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross origin register: want 403 got %d", w.Code)
	}
}
func TestDisabledUserSessionExpiryAndInputValidation(t *testing.T) {
	s, h := setup(t)
	cookie := register(t, h, "alice")
	ctx := context.Background()
	for _, body := range []string{`{"username":"ab","password":"correct horse battery"}`, `{"username":"a bad name","password":"correct horse battery"}`, `{"username":"valid","password":"short"}`, `{"username":"valid","password":"correct horse battery","role":"admin"}`} {
		if w := request(h, "POST", "/api/v1/auth/register", body, nil); w.Code != 400 {
			t.Fatalf("invalid register: %d %s", w.Code, w.Body)
		}
	}
	_, e := s.Pool.Exec(ctx, "UPDATE sessions SET expires_at=now()-interval '1 minute'")
	if e != nil {
		t.Fatal(e)
	}
	if w := request(h, "GET", "/api/v1/account/me", "", cookie); w.Code != 401 {
		t.Fatalf("expired session: %d", w.Code)
	}
	w := request(h, "POST", "/api/v1/auth/login", `{"username":"alice","password":"correct horse battery"}`, nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	cookie = w.Result().Cookies()[0]
	_, e = s.Pool.Exec(ctx, "UPDATE users SET enabled=false WHERE username='alice'")
	if e != nil {
		t.Fatal(e)
	}
	if w = request(h, "GET", "/api/v1/account/me", "", cookie); w.Code != 401 {
		t.Fatalf("disabled user: %d", w.Code)
	}
	if w = request(h, "POST", "/api/v1/auth/login", `{"username":"alice","password":"correct horse battery"}`, nil); w.Code != 401 {
		t.Fatalf("disabled login: %d", w.Code)
	}
}
func TestTokenCursorAndUsageAreUserScoped(t *testing.T) {
	s, h := setup(t)
	alice := register(t, h, "alice")
	bob := register(t, h, "bob")
	for _, cookie := range []*http.Cookie{alice, alice, bob} {
		if w := request(h, "POST", "/api/v1/account/tokens", `{"name":"Laptop"}`, cookie); w.Code != 201 {
			t.Fatalf("create token %d", w.Code)
		}
	}
	w := request(h, "GET", "/api/v1/account/tokens?limit=1", "", alice)
	var page struct {
		Items []struct{ ID int64 }
		Next  string `json:"next_cursor"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &page); e != nil {
		t.Fatal(e)
	}
	if w.Code != 200 || len(page.Items) != 1 || page.Next == "" {
		t.Fatalf("first page %d %s", w.Code, w.Body)
	}
	first := page.Items[0].ID
	w = request(h, "GET", "/api/v1/account/tokens?limit=1&cursor="+page.Next, "", alice)
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if w.Code != 200 || len(page.Items) != 1 || page.Items[0].ID == first || page.Next != "" {
		t.Fatalf("second page %d %s", w.Code, w.Body)
	}
	for _, path := range []string{"/api/v1/account/tokens?limit=101", "/api/v1/account/usage?cursor=invalid"} {
		if w = request(h, "GET", path, "", alice); w.Code != 400 {
			t.Fatalf("invalid pagination %d", w.Code)
		}
	}
	_, e := s.Pool.Exec(context.Background(), "INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key) SELECT user_id,id,wallet_id,'private',0,'denied','test' FROM tokens WHERE user_id=(SELECT id FROM users WHERE username='bob')")
	if e != nil {
		t.Fatal(e)
	}
	w = request(h, "GET", "/api/v1/account/usage", "", alice)
	if w.Code != 200 || strings.Contains(w.Body.String(), "private") {
		t.Fatalf("usage isolation %d %s", w.Code, w.Body)
	}
}
func TestUsageAndLedgerAcceptScopedBearerToken(t *testing.T) {
	alice, admin, h, exec := adminFixture(t)
	bob := register(t, h, "bob")
	w := request(h, "POST", "/api/v1/account/tokens", `{"name":"Desktop"}`, alice)
	if w.Code != 201 {
		t.Fatalf("create token %d %s", w.Code, w.Body)
	}
	var created struct{ Token string }
	if e := json.Unmarshal(w.Body.Bytes(), &created); e != nil {
		t.Fatal(e)
	}
	w = request(h, "POST", "/api/v1/admin/redemption-codes", `{"credits":5,"note":"alice grant"}`, admin)
	if w.Code != 201 {
		t.Fatalf("code for alice %d %s", w.Code, w.Body)
	}
	var aliceCode struct{ Code string }
	_ = json.Unmarshal(w.Body.Bytes(), &aliceCode)
	if w = request(h, "POST", "/api/v1/account/redeem", fmt.Sprintf(`{"code":%q}`, aliceCode.Code), alice); w.Code != 200 {
		t.Fatalf("alice redeem %d %s", w.Code, w.Body)
	}
	w = request(h, "POST", "/api/v1/admin/redemption-codes", `{"credits":6,"note":"bob grant"}`, admin)
	var bobCode struct{ Code string }
	_ = json.Unmarshal(w.Body.Bytes(), &bobCode)
	if w = request(h, "POST", "/api/v1/account/redeem", fmt.Sprintf(`{"code":%q}`, bobCode.Code), bob); w.Code != 200 {
		t.Fatalf("bob redeem %d %s", w.Code, w.Body)
	}
	exec("INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key) SELECT user_id,id,wallet_id,'her-tool',0,'denied','test' FROM tokens WHERE user_id=(SELECT id FROM users WHERE username='alice')")
	exec("INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key) SELECT user_id,id,wallet_id,'private',0,'denied','test' FROM tokens WHERE user_id=(SELECT id FROM users WHERE username='bob')")
	bearer := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://example.com"+path, nil)
		r.Header.Set("Authorization", "Bearer "+created.Token)
		wr := httptest.NewRecorder()
		h.ServeHTTP(wr, r)
		return wr
	}
	if w = bearer("/api/v1/account/usage"); w.Code != 200 || !strings.Contains(w.Body.String(), "her-tool") || strings.Contains(w.Body.String(), "private") {
		t.Fatalf("bearer usage %d %s", w.Code, w.Body)
	}
	if w = bearer("/api/v1/account/ledger"); w.Code != 200 || !strings.Contains(w.Body.String(), `"delta":5`) || strings.Contains(w.Body.String(), `"delta":6`) {
		t.Fatalf("bearer ledger %d %s", w.Code, w.Body)
	}
	if w = bearer("/api/v1/account/usage?limit=101"); w.Code != 400 {
		t.Fatalf("bearer invalid pagination %d", w.Code)
	}
	r := httptest.NewRequest("GET", "http://example.com/api/v1/account/usage", nil)
	r.Header.Set("Authorization", "Bearer ppt_forged")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("forged bearer %d", w.Code)
	}
}
func TestAdjustmentReplayReturnsOriginalBalanceAndRecordsActor(t *testing.T) {
	s, h := setup(t)
	register(t, h, "alice")
	admin := register(t, h, "admin")
	ctx := context.Background()
	_, _ = s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='admin'")
	var id int64
	_ = s.Pool.QueryRow(ctx, "SELECT id FROM users WHERE username='alice'").Scan(&id)
	path := fmt.Sprintf("/api/v1/admin/users/%d/balance", id)
	first := request(h, "POST", path, `{"delta":10,"note":"first","idempotency_key":"first"}`, admin)
	if first.Code != 200 {
		t.Fatal(first.Code)
	}
	if w := request(h, "POST", path, `{"delta":5,"note":"second","idempotency_key":"second"}`, admin); w.Code != 200 {
		t.Fatal(w.Code)
	}
	replay := request(h, "POST", path, `{"delta":10,"note":"first","idempotency_key":"first"}`, admin)
	if replay.Code != 200 || replay.Body.String() != first.Body.String() {
		t.Fatalf("replay must retain original balance: first=%s replay=%s", first.Body, replay.Body)
	}
	var actor string
	e := s.Pool.QueryRow(ctx, "SELECT u.username FROM ledger l JOIN users u ON u.id=l.actor_id WHERE l.idempotency_key='first'").Scan(&actor)
	if e != nil || actor != "admin" {
		t.Fatalf("actor=%s %v", actor, e)
	}
}
func TestBootstrapAdminIsIdempotentAndCannotPromoteExistingUser(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	for range 2 {
		if e := BootstrapAdmin(ctx, s, "operator", "correct horse battery"); e != nil {
			t.Fatal(e)
		}
	}
	w := request(h, "POST", "/api/v1/auth/login", `{"username":"operator","password":"correct horse battery"}`, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"role":"admin"`) {
		t.Fatalf("bootstrap login %d %s", w.Code, w.Body)
	}
	register(t, h, "ordinary")
	if e := BootstrapAdmin(ctx, s, "ordinary", "correct horse battery"); e == nil {
		t.Fatal("bootstrap must not promote an existing ordinary user")
	}
}
func TestAuthenticationEndpointsRejectExcessiveAttempts(t *testing.T) {
	s, h := setup(t)
	for i := range 120 {
		w := request(h, "POST", "/api/v1/auth/register", `{}`, nil)
		if w.Code != 400 {
			t.Fatalf("invalid request expected400 got%d", w.Code)
		}
		if i == 0 {
			pinAuthenticationWindow(t, s)
		}
	}
	if w := request(h, "POST", "/api/v1/auth/register", `{}`, nil); w.Code != 429 {
		t.Fatalf("auth request budget unbounded: %d", w.Code)
	}
}

func TestRegistrationCreditsHaveAtomicLedgerAndCanBeDisabled(t *testing.T) {
	s, _ := setup(t)
	h := New(s, Options{Origin: "http://example.com"})
	cookie := register(t, h, "welcome")
	w := request(h, "GET", "/api/v1/account/me", "", cookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"balance":1000`) {
		t.Fatalf("default registration credits %d %s", w.Code, w.Body)
	}
	var delta, balance int64
	e := s.Pool.QueryRow(context.Background(), "SELECT delta,balance_after FROM ledger WHERE kind='registration'").Scan(&delta, &balance)
	if e != nil || delta != 1000 || balance != 1000 {
		t.Fatalf("registration ledger delta=%d balance=%d err=%v", delta, balance, e)
	}
	zero := int64(0)
	h = New(s, Options{Origin: "http://example.com", InitialCredits: &zero})
	cookie = register(t, h, "no_grant")
	w = request(h, "GET", "/api/v1/account/me", "", cookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"balance":0`) {
		t.Fatalf("disabled registration credits %d %s", w.Code, w.Body)
	}
}
