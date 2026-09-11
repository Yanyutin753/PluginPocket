package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yanyutin753/PluginPocket/server/internal/gateway"
)

func TestDistributedAPIRouteMatrix(t *testing.T) {
	f := marketplaceApp(t, nil)
	other := replicaStore(t, f.store)
	g := gateway.New(other, gateway.Options{AllowPrivate: true, EncryptionKey: f.key})
	t.Cleanup(g.Close)
	a, b := f.handler, New(other, Options{Origin: "http://example.com", Gateway: g, EncryptionKey: f.key})
	call := func(h http.Handler, method, path, body string, cookie *http.Cookie, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := request(h, method, "/api/v1"+path, body, cookie)
		if w.Code != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, status, w.Body)
		}
		return w
	}
	itemID := func(w *httptest.ResponseRecorder) int64 {
		t.Helper()
		var out struct{ Item struct{ ID int64 } }
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Item.ID
	}
	teamID := newTeam(t, a, f.user)
	teamPath := fmt.Sprintf("/account/teams/%d", teamID)
	call(b, "PATCH", teamPath, `{"name":"Cross replica team"}`, f.user, 200)
	if !strings.Contains(call(a, "GET", teamPath, "", f.user, 200).Body.String(), "Cross replica team") {
		t.Fatal("team update not shared")
	}
	call(a, "POST", "/admin/marketplace/sync", "", f.admin, 200)
	if !strings.Contains(call(b, "GET", "/admin/marketplace", "", f.admin, 200).Body.String(), "github-github-mcp-server") {
		t.Fatal("sync not shared")
	}
	skill := `{"slug":"distributed-api","name":"Distributed API","source":"inline","files":{"SKILL.md":"Shared skill"}}`
	call(a, "POST", "/admin/marketplace/skills", skill, f.admin, 201)
	if !strings.Contains(call(b, "GET", "/marketplace/distributed-api/files", "", f.user, 200).Body.String(), "Shared skill") {
		t.Fatal("skill files not shared")
	}
	call(b, "POST", "/admin/marketplace/bundles", `{"slug":"distributed-pack","name":"Distributed pack","includes":["distributed-api"]}`, f.admin, 201)
	call(a, "GET", "/plugins/distributed-pack", "", nil, 200)
	remote := fmt.Sprintf(`{"key":"distributed-remote","name":"Remote","description":"test","kind":"http","enabled":true,"units_per_call":1,"input_schema":{"type":"object"},"config":{"url":%q}}`, f.upstream)
	toolID := itemID(call(a, "POST", "/admin/tools", remote, f.admin, 201))
	toolPath := fmt.Sprintf("/admin/tools/%d", toolID)
	call(b, "GET", toolPath+"/upstream", "", f.admin, 200)
	call(a, "PUT", toolPath+"/metadata", `{"remote_name":"ping","description":"Shared override","input_schema":{"type":"object"}}`, f.admin, 200)
	if !strings.Contains(call(b, "GET", toolPath+"/upstream", "", f.admin, 200).Body.String(), "Shared override") {
		t.Fatal("metadata override not shared")
	}
	call(b, "DELETE", toolPath+"/metadata/ping", "", f.admin, 204)
	if strings.Contains(call(a, "GET", toolPath+"/upstream", "", f.admin, 200).Body.String(), "Shared override") {
		t.Fatal("metadata deletion not shared")
	}
	call(b, "PATCH", toolPath, strings.Replace(remote, `"name":"Remote"`, `"name":"Updated remote"`, 1), f.admin, 200)
	if !strings.Contains(call(a, "GET", "/tools", "", f.user, 200).Body.String(), "Updated remote") {
		t.Fatal("tool update not shared")
	}
	install := fmt.Sprintf(`{"slug":"github-github-mcp-server","transport":"http","config":{"url":%q}}`, f.upstream)
	call(b, "POST", "/admin/marketplace/install", install, f.admin, 201)
	call(a, "POST", "/admin/marketplace/install", install, f.admin, 409)
	call(a, "POST", "/admin/marketplace/uninstall", `{"slug":"github-github-mcp-server"}`, f.admin, 200)
	call(b, "POST", "/admin/marketplace/uninstall", `{"slug":"github-github-mcp-server"}`, f.admin, 409)
	planID := itemID(call(a, "POST", "/admin/plans", `{"name":"Shared plan","credits":100,"price_cents":1000,"currency":"CNY","enabled":true}`, f.admin, 201))
	call(b, "PATCH", fmt.Sprintf("/admin/plans/%d", planID), `{"name":"Updated plan","credits":200,"price_cents":1000,"currency":"CNY","enabled":true}`, f.admin, 200)
	if !strings.Contains(call(a, "GET", "/plans", "", f.user, 200).Body.String(), "Updated plan") {
		t.Fatal("plan update not shared")
	}
	for _, h := range []http.Handler{a, b} {
		call(h, "POST", "/account/orders", fmt.Sprintf(`{"plan_id":%d}`, planID), f.user, 503)
	}
	var code struct{ Code string }
	if err := json.Unmarshal(call(a, "POST", "/admin/redemption-codes", `{"credits":25,"note":"cross-replica"}`, f.admin, 201).Body.Bytes(), &code); err != nil {
		t.Fatal(err)
	}
	redeem := fmt.Sprintf(`{"code":%q}`, code.Code)
	call(b, "POST", "/account/redeem", redeem, f.user, 200)
	call(a, "POST", "/account/redeem", redeem, f.user, 409)
	var userID int64
	if err := f.store.Pool.QueryRow(t.Context(), "SELECT id FROM users WHERE username='alice'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	adjust := fmt.Sprintf("/admin/users/%d/balance", userID)
	for _, h := range []http.Handler{a, b} {
		call(h, "POST", adjust, `{"delta":10,"note":"shared adjustment","idempotency_key":"distributed-api"}`, f.admin, 200)
	}
	if !strings.Contains(call(a, "GET", "/account/me", "", f.user, 200).Body.String(), `"balance":35`) {
		t.Fatal("adjustment replay charged twice")
	}
	tokenResponse := call(b, "POST", "/account/tokens", `{"name":"Shared token"}`, f.user, 201)
	tokenID := itemID(tokenResponse)
	call(a, "DELETE", fmt.Sprintf("/account/tokens/%d", tokenID), "", f.user, 204)
	reads := []struct {
		path   string
		cookie *http.Cookie
	}{
		{"/account/me", f.user}, {"/account/tokens", f.user}, {"/account/usage", f.user}, {"/tools", f.user}, {"/plans", f.user}, {"/account/orders", f.user}, {"/account/ledger", f.user}, {"/account/teams", f.user},
		{teamPath, f.user}, {teamPath + "/members", f.user}, {teamPath + "/usage", f.user}, {teamPath + "/usage/export", f.user}, {teamPath + "/usage/summary", f.user},
		{"/admin/users", f.admin}, {"/admin/tools", f.admin}, {"/admin/marketplace", f.admin}, {"/admin/plans", f.admin}, {"/admin/redemption-codes", f.admin}, {"/admin/usage", f.admin}, {"/admin/usage/export", f.admin}, {"/admin/usage/summary", f.admin}, {"/admin/ledger", f.admin},
		{"/marketplace", f.user}, {"/marketplace/commit-style/files", f.user}, {"/plugins", nil}, {"/plugins/commit-style", nil},
	}
	for _, read := range reads {
		left, right := call(a, "GET", read.path, "", read.cookie, 200), call(b, "GET", read.path, "", read.cookie, 200)
		if left.Body.String() != right.Body.String() {
			t.Fatalf("replica-dependent API: %s", read.path)
		}
		if left.Header().Get("Cache-Control") != "no-store" || right.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("cacheable API: %s", read.path)
		}
		if strings.HasPrefix(read.path, "/admin/") {
			call(b, "GET", read.path, "", f.user, 403)
		}
	}
	userPath := fmt.Sprintf("/admin/users/%d", userID)
	call(b, "PATCH", userPath, `{"enabled":false}`, f.admin, 200)
	call(a, "GET", "/account/me", "", f.user, 401)
}
