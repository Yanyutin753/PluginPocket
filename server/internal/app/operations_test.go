package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Yanyutin753/PluginPocket/server/internal/gateway"
)

func adminFixture(t *testing.T) (*http.Cookie, *http.Cookie, http.Handler, func(string, ...any)) {
	t.Helper()
	s, h := setup(t)
	user := register(t, h, "alice")
	admin := register(t, h, "operator")
	_, e := s.Pool.Exec(context.Background(), "UPDATE users SET role='admin' WHERE username='operator'")
	if e != nil {
		t.Fatal(e)
	}
	return user, admin, h, func(sql string, args ...any) {
		t.Helper()
		if _, e := s.Pool.Exec(context.Background(), sql, args...); e != nil {
			t.Fatal(e)
		}
	}
}
func TestAdminCanDisableUserButNotSelf(t *testing.T) {
	user, admin, h, _ := adminFixture(t)
	if w := request(h, "PATCH", "/api/v1/admin/users/1", `{"enabled":false}`, user); w.Code != 403 {
		t.Fatalf("user admin mutation %d", w.Code)
	}
	if w := request(h, "PATCH", "/api/v1/admin/users/1", `{"enabled":false}`, admin); w.Code != 200 {
		t.Fatalf("disable %d %s", w.Code, w.Body)
	}
	if w := request(h, "GET", "/api/v1/account/me", "", user); w.Code != 401 {
		t.Fatalf("disabled session %d", w.Code)
	}
	if w := request(h, "PATCH", "/api/v1/admin/users/2", `{"enabled":false}`, admin); w.Code != 409 {
		t.Fatalf("self disable %d", w.Code)
	}
}
func TestAdminToolsEncryptSecretsAndProtectConfiguration(t *testing.T) {
	s, h := setup(t)
	admin := register(t, h, "operator")
	user := register(t, h, "alice")
	_, _ = s.Pool.Exec(context.Background(), "UPDATE users SET role='admin' WHERE username='operator'")
	key := []byte("01234567890123456789012345678901")
	g := gateway.New(s, gateway.Options{EncryptionKey: key})
	t.Cleanup(g.Close)
	h = New(s, Options{Origin: "http://example.com", Gateway: g, EncryptionKey: key})
	body := `{"key":"remote","name":"Remote tools","description":"test","kind":"http","enabled":true,"units_per_call":2,"input_schema":{"type":"object"},"config":{"url":"https://example.com/mcp","headers":{"Authorization":"Bearer secret-value"}}}`
	if w := request(h, "POST", "/api/v1/admin/tools", body, user); w.Code != 403 {
		t.Fatalf("user tool edit %d", w.Code)
	}
	w := request(h, "POST", "/api/v1/admin/tools", body, admin)
	if w.Code != 201 {
		t.Fatalf("create tool %d %s", w.Code, w.Body)
	}
	var response struct{ Item struct{ ID int64 } }
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	for _, path := range []string{"/api/v1/admin/tools", "/api/v1/tools"} {
		w = request(h, "GET", path, "", admin)
		if w.Code != 200 || strings.Contains(w.Body.String(), "secret-value") || strings.Contains(w.Body.String(), "ciphertext") {
			t.Fatalf("secret leak %d %s", w.Code, w.Body)
		}
	}
	var raw []byte
	e := s.Pool.QueryRow(context.Background(), "SELECT config FROM tools WHERE id=$1", response.Item.ID).Scan(&raw)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), "secret-value") {
		t.Fatal("plaintext persisted")
	}
	plain, e := gateway.OpenConfig(key, raw)
	if e != nil || !strings.Contains(string(plain), "secret-value") {
		t.Fatalf("encrypted config round trip %v", e)
	}
	body = `{"key":"remote","name":"Remote tools","description":"test","kind":"http","enabled":false,"units_per_call":3,"input_schema":{"type":"object"}}`
	if w = request(h, "PATCH", fmt.Sprintf("/api/v1/admin/tools/%d", response.Item.ID), body, admin); w.Code != 200 {
		t.Fatalf("patch tool %d %s", w.Code, w.Body)
	}
	body = `{"key":"evil","name":"bad","kind":"stdio","enabled":true,"units_per_call":1,"input_schema":{"type":"object"},"config":{"command":"sh"}}`
	if w = request(h, "POST", "/api/v1/admin/tools", body, admin); w.Code != 400 {
		t.Fatalf("arbitrary command %d", w.Code)
	}
}
func TestGlobalUsageFilterAndSafeCSV(t *testing.T) {
	user, admin, h, exec := adminFixture(t)
	if w := request(h, "GET", "/api/v1/admin/usage", "", user); w.Code != 403 {
		t.Fatalf("global usage access %d", w.Code)
	}
	request(h, "POST", "/api/v1/account/tokens", `{"name":"client"}`, user)
	exec("INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key) SELECT user_id,id,wallet_id,'=FORMULA()',0,'denied','csv' FROM tokens")
	w := request(h, "GET", "/api/v1/admin/usage?status=denied", "", admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "=FORMULA()") {
		t.Fatalf("global usage %d %s", w.Code, w.Body)
	}
	w = request(h, "GET", "/api/v1/admin/usage/export", "", admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "'=FORMULA()") {
		t.Fatalf("unsafe CSV %d %s", w.Code, w.Body)
	}
	w = request(h, "GET", "/api/v1/account/usage?status=ok", "", user)
	if w.Code != 200 || strings.Contains(w.Body.String(), "FORMULA") {
		t.Fatalf("usage filter ignored %d %s", w.Code, w.Body)
	}
}
func TestUsageSummariesAggregateCostsAndErrors(t *testing.T) {
	s, h := setup(t)
	owner := register(t, h, "alice")
	admin := register(t, h, "operator")
	ctx := context.Background()
	_, _ = s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='operator'")
	id := newTeam(t, h, owner)
	w := request(h, "POST", "/api/v1/account/tokens", fmt.Sprintf(`{"name":"Team","team_id":%d}`, id), owner)
	if w.Code != 201 {
		t.Fatal(w.Code)
	}
	_, e := s.Pool.Exec(ctx, "INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,requested_cost,status,request_key) SELECT user_id,id,wallet_id,'echo',3,3,'ok','one' FROM tokens")
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Pool.Exec(ctx, "INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key) SELECT user_id,id,wallet_id,'echo',0,'error','two' FROM tokens")
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/v1/admin/usage/summary?days=7", fmt.Sprintf("/api/v1/account/teams/%d/usage/summary?days=1", id)} {
		w = request(h, "GET", path, "", admin)
		if strings.Contains(path, "/teams/") {
			w = request(h, "GET", path, "", owner)
		}
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"calls": 2`) && !strings.Contains(w.Body.String(), `"calls":2`) || !strings.Contains(w.Body.String(), `"cost": 3`) && !strings.Contains(w.Body.String(), `"cost":3`) {
			t.Fatalf("summary %s %d %s", path, w.Code, w.Body)
		}
		var response struct {
			Items []struct{ Calls, Cost, Errors int64 }
		}
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		if len(response.Items) != 1 || response.Items[0].Calls != 2 || response.Items[0].Cost != 3 || response.Items[0].Errors != 1 {
			t.Fatalf("summary values %s", w.Body)
		}
	}
	if w = request(h, "GET", "/api/v1/admin/usage/summary?days=999", "", admin); w.Code != 400 {
		t.Fatalf("invalid days %d", w.Code)
	}
}
func TestManagementWritesEnforceAuthenticationAndValidation(t *testing.T) {
	user, admin, h, _ := adminFixture(t)
	cases := []struct{ method, path, invalid string }{
		{"PATCH", "/api/v1/admin/users/1", `{}`},
		{"POST", "/api/v1/admin/tools", `{"kind":"builtin","key":"invented"}`},
		{"PATCH", "/api/v1/admin/tools/1", `{"kind":"builtin","key":"invented"}`},
		{"POST", "/api/v1/admin/plans", `{"name":"Empty","credits":0}`},
		{"PATCH", "/api/v1/admin/plans/1", `{"name":"Empty","credits":0}`},
		{"POST", "/api/v1/admin/redemption-codes", `{"credits":-1,"note":"invalid"}`},
	}
	for _, c := range cases {
		t.Run(c.method+c.path, func(t *testing.T) {
			if w := request(h, c.method, c.path, c.invalid, nil); w.Code != 401 {
				t.Fatalf("anonymous %d", w.Code)
			}
			if w := request(h, c.method, c.path, c.invalid, user); w.Code != 403 {
				t.Fatalf("nonadmin %d", w.Code)
			}
			if w := request(h, c.method, c.path, c.invalid, admin); w.Code != 400 {
				t.Fatalf("invalid input %d %s", w.Code, w.Body)
			}
		})
	}
	for _, path := range []string{"/api/v1/account/redeem", "/api/v1/account/orders", "/api/v1/account/teams", "/api/v1/account/team-invites/accept", "/api/v1/account/devices/approve"} {
		if w := request(h, "POST", path, `{}`, nil); w.Code != 401 {
			t.Fatalf("anonymous %s %d", path, w.Code)
		}
		if w := request(h, "POST", path, `{}`, user); w.Code != 400 {
			t.Fatalf("invalid input %s %d %s", path, w.Code, w.Body)
		}
	}
}
