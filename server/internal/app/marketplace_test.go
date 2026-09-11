package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Yanyutin753/loadout/server/internal/gateway"
	"github.com/Yanyutin753/loadout/server/internal/marketplace"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type marketplaceFixture struct {
	store    *store.Store
	gateway  *gateway.Gateway
	key      []byte
	handler  http.Handler
	admin    *http.Cookie
	user     *http.Cookie
	upstream string
}

// remount 以新的 GitHub API 地址重建 handler（技能解析走 contents API）。
func (f *marketplaceFixture) remount(githubBase string) {
	f.handler = New(f.store, Options{Origin: "http://example.com", Gateway: f.gateway, EncryptionKey: f.key, Marketplace: marketplace.Options{BaseURL: githubBase}})
}

func marketplaceApp(t *testing.T, github http.HandlerFunc) marketplaceFixture {
	t.Helper()
	s, h := setup(t)
	admin := register(t, h, "operator")
	user := register(t, h, "alice")
	if _, e := s.Pool.Exec(context.Background(), "UPDATE users SET role='admin' WHERE username='operator'"); e != nil {
		t.Fatal(e)
	}
	remote := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "ping", Description: "original ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "pong"}}}, nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	t.Cleanup(upstream.Close)
	if github == nil {
		github = func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"total_count":1,"items":[{"full_name":"github/github-mcp-server","name":"github-mcp-server","description":"official","html_url":"https://github.com/github/github-mcp-server","homepage":"","stargazers_count":32852}]}`))
		}
	}
	githubServer := httptest.NewServer(github)
	t.Cleanup(githubServer.Close)
	key := []byte("01234567890123456789012345678901")
	g := gateway.New(s, gateway.Options{AllowPrivate: true, EncryptionKey: key})
	t.Cleanup(g.Close)
	return marketplaceFixture{
		store: s, gateway: g, key: key, admin: admin, user: user, upstream: upstream.URL,
		handler: New(s, Options{Origin: "http://example.com", Gateway: g, EncryptionKey: key, Marketplace: marketplace.Options{BaseURL: githubServer.URL}}),
	}
}

func TestMarketplaceListSyncInstallUninstall(t *testing.T) {
	f := marketplaceApp(t, nil)
	if w := request(f.handler, "GET", "/api/v1/admin/marketplace", "", f.user); w.Code != 403 {
		t.Fatalf("user marketplace access %d", w.Code)
	}
	w := request(f.handler, "GET", "/api/v1/admin/marketplace", "", f.admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"slug":"deepwiki"`) || !strings.Contains(w.Body.String(), "mcp.deepwiki.com") {
		t.Fatalf("curated list %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/sync", "", f.admin); w.Code != 200 || !strings.Contains(w.Body.String(), `"synced":1`) {
		t.Fatalf("sync %d %s", w.Code, w.Body)
	}
	w = request(f.handler, "GET", "/api/v1/admin/marketplace", "", f.admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "github-github-mcp-server") {
		t.Fatalf("synced item missing %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/install", `{"slug":"github-github-mcp-server"}`, f.admin); w.Code != 400 {
		t.Fatalf("unknown transport without config %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/install", `{"slug":"deepwiki"}`, f.admin); w.Code != 201 {
		t.Fatalf("curated install %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "GET", "/api/v1/tools", "", f.user); w.Code != 200 || !strings.Contains(w.Body.String(), `"key":"deepwiki"`) {
		t.Fatalf("installed tool not listed %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/install", `{"slug":"deepwiki"}`, f.admin); w.Code != 409 {
		t.Fatalf("double install %d", w.Code)
	}
	install := `{"slug":"github-github-mcp-server","transport":"http","config":{"url":"` + f.upstream + `"}}`
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/install", install, f.admin); w.Code != 201 {
		t.Fatalf("explicit install %d %s", w.Code, w.Body)
	}
	var raw []byte
	if e := f.store.Pool.QueryRow(context.Background(), "SELECT config FROM tools WHERE key='github-github-mcp-server'").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), "127.0.0.1") {
		t.Fatal("install config must be sealed")
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/uninstall", `{"slug":"deepwiki"}`, f.admin); w.Code != 200 {
		t.Fatalf("uninstall %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/uninstall", `{"slug":"deepwiki"}`, f.admin); w.Code != 409 {
		t.Fatalf("double uninstall %d", w.Code)
	}
	if w = request(f.handler, "GET", "/api/v1/admin/tools", "", f.admin); w.Code != 200 || !strings.Contains(w.Body.String(), `"key":"deepwiki"`) {
		t.Fatalf("uninstalled tool row must remain visible to admin %d %s", w.Code, w.Body)
	}
}

func TestMarketplaceSyncFailureKeepsCatalog(t *testing.T) {
	f := marketplaceApp(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	})
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/sync", "", f.admin); w.Code != 502 {
		t.Fatalf("refused sync must 502, got %d", w.Code)
	}
	w := request(f.handler, "GET", "/api/v1/admin/marketplace", "", f.admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"slug":"deepwiki"`) {
		t.Fatalf("catalog must survive failed sync %d %s", w.Code, w.Body)
	}
}

func TestToolMetadataOverrideEndpoints(t *testing.T) {
	f := marketplaceApp(t, nil)
	body := `{"key":"remote","name":"Remote","description":"test","kind":"http","enabled":true,"units_per_call":1,"input_schema":{"type":"object"},"config":{"url":"` + f.upstream + `"}}`
	w := request(f.handler, "POST", "/api/v1/admin/tools", body, f.admin)
	if w.Code != 201 {
		t.Fatalf("create upstream tool %d %s", w.Code, w.Body)
	}
	var created struct {
		Item struct{ ID int64 }
	}
	if e := json.Unmarshal(w.Body.Bytes(), &created); e != nil {
		t.Fatal(e)
	}
	path := "/api/v1/admin/tools/" + strconv.FormatInt(created.Item.ID, 10)
	w = request(f.handler, "GET", path+"/upstream", "", f.admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"ping"`) || !strings.Contains(w.Body.String(), "original ping") {
		t.Fatalf("upstream discovery %d %s", w.Code, w.Body)
	}
	override := `{"remote_name":"ping","description":"自定义说明","input_schema":{"type":"object","properties":{"q":{"type":"string","description":"参数说明"}},"additionalProperties":false}}`
	if w = request(f.handler, "PUT", path+"/metadata", override, f.admin); w.Code != 200 {
		t.Fatalf("save override %d %s", w.Code, w.Body)
	}
	w = request(f.handler, "GET", path+"/upstream", "", f.admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "自定义说明") || !strings.Contains(w.Body.String(), "original ping") {
		t.Fatalf("override merge view %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "PUT", path+"/metadata", `{"remote_name":"ping","description":"","input_schema":{"type":"string"}}`, f.admin); w.Code != 400 {
		t.Fatalf("invalid override must 400, got %d", w.Code)
	}
	if w = request(f.handler, "PUT", path+"/metadata", `{"remote_name":"ping","description":""}`, f.admin); w.Code != 400 {
		t.Fatalf("empty override must 400, got %d", w.Code)
	}
	if w = request(f.handler, "PUT", path+"/metadata", override, f.user); w.Code != 403 {
		t.Fatalf("user override %d", w.Code)
	}
	if w = request(f.handler, "DELETE", path+"/metadata/ping", "", f.admin); w.Code != 204 {
		t.Fatalf("delete override %d", w.Code)
	}
	w = request(f.handler, "GET", path+"/upstream", "", f.admin)
	if w.Code != 200 || strings.Contains(w.Body.String(), "自定义说明") {
		t.Fatalf("override must be gone %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "GET", "/api/v1/admin/tools/404404/upstream", "", f.admin); w.Code != 404 {
		t.Fatalf("missing tool %d", w.Code)
	}
}

func TestPublicMarketplaceListWithToken(t *testing.T) {
	f := marketplaceApp(t, nil)
	created_ := request(f.handler, "POST", "/api/v1/account/tokens", `{"name":"cli"}`, f.user)
	if created_.Code != 201 {
		t.Fatalf("token create %d %s", created_.Code, created_.Body)
	}
	var created struct {
		Token string `json:"token"`
	}
	if e := json.Unmarshal(created_.Body.Bytes(), &created); e != nil || created.Token == "" {
		t.Fatalf("token value missing: %v", e)
	}
	token := created.Token
	r := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/marketplace", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"slug":"deepwiki"`) || !strings.Contains(w.Body.String(), "mcp.deepwiki.com") {
		t.Fatalf("token marketplace list %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), `"transport":"stdio"`) {
		t.Fatalf("marketplace must be http-only %s", w.Body)
	}
	if strings.Contains(w.Body.String(), "ciphertext") || strings.Contains(w.Body.String(), "installed_tool_id") {
		t.Fatalf("marketplace list leaked internal fields %s", w.Body)
	}
	r2 := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/marketplace", nil)
	w2 := httptest.NewRecorder()
	f.handler.ServeHTTP(w2, r2)
	if w2.Code != 401 {
		t.Fatalf("unauthenticated marketplace list %d", w2.Code)
	}
}

func TestMarketplaceInstallRejectsNonHTTP(t *testing.T) {
	f := marketplaceApp(t, nil)
	body := `{"slug":"deepwiki","transport":"stdio","config":{"command":"fixture"}}`
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/install", body, f.admin); w.Code != 400 {
		t.Fatalf("stdio marketplace install must be rejected, got %d %s", w.Code, w.Body)
	}
}

func base64Of(raw string) string {
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func githubContentsFixture(t *testing.T, entries string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(entries))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestMarketplaceSkillsBundlesAndFiles(t *testing.T) {
	githubEntries := fmt.Sprintf(`[{"name":"SKILL.md","path":"skills/pro/SKILL.md","type":"file","content":%q,"encoding":"base64"}]`, base64Of("---\nname: pro\n---\nbody"))
	f := marketplaceApp(t, nil)
	// github 技能导入需要可注入的 GitHub API 地址：重建 handler 指向 fixture。
	f.remount(githubContentsFixture(t, githubEntries).URL)
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", `{"name":"Pro skill","description":"imported","source":"github","repo":"expert/skills","path":"skills/pro","slug":"pro"}`, f.admin); w.Code != 201 {
		t.Fatalf("github skill import %d %s", w.Code, w.Body)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", `{"name":"Inline","description":"inline skill","source":"inline","slug":"inline-one","files":{"SKILL.md":"---\nname: inline-one\n---\nhello"}}`, f.admin); w.Code != 201 {
		t.Fatalf("inline skill create %d %s", w.Code, w.Body)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", `{"name":"Bad","source":"inline","slug":"bad","files":{"../evil.md":"x"}}`, f.admin); w.Code != 400 {
		t.Fatalf("traversal file must be rejected, got %d", w.Code)
	}
	// 同 slug 再发 = 更新（内容迭代），而不是唯一键冲突 500。
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", "{\"name\":\"Inline\",\"description\":\"v2\",\"source\":\"inline\",\"slug\":\"inline-one\",\"files\":{\"SKILL.md\":\"---\\nname: inline-one\\n---\\nhello v2\"}}", f.admin); w.Code != 200 {
		t.Fatalf("skill update must 200, got %d %s", w.Code, w.Body)
	}
	var content string
	if e := f.store.Pool.QueryRow(context.Background(), "SELECT spec->'files'->>'SKILL.md' FROM marketplace_items WHERE slug='inline-one'").Scan(&content); e != nil || !strings.Contains(content, "v2") {
		t.Fatalf("skill update not persisted: %q err=%v", content, e)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/bundles", `{"name":"Starter","description":"专家入门装备","slug":"starter","includes":["inline-one","pro","deepwiki"]}`, f.admin); w.Code != 201 {
		t.Fatalf("bundle create %d %s", w.Code, w.Body)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/bundles", `{"name":"Bad","slug":"bad-bundle","includes":["missing-slug"]}`, f.admin); w.Code != 400 {
		t.Fatalf("unknown include must be rejected, got %d", w.Code)
	}
	w := request(f.handler, "GET", "/api/v1/marketplace", "", f.user)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"kind":"skill"`) || !strings.Contains(w.Body.String(), `"kind":"bundle"`) {
		t.Fatalf("public list kinds %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "GET", "/api/v1/marketplace/inline-one/files", "", f.user); w.Code != 200 || !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("inline skill files %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "GET", "/api/v1/marketplace/pro/files", "", f.user); w.Code != 200 || !strings.Contains(w.Body.String(), "name: pro") {
		t.Fatalf("github skill files %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "GET", "/api/v1/marketplace/starter/files", "", f.user); w.Code != 400 {
		t.Fatalf("bundle has no files endpoint, got %d", w.Code)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/skills", `{"name":"User","source":"inline","slug":"user-made","files":{"SKILL.md":"x"}}`, f.user); w.Code != 403 {
		t.Fatalf("user skill publish must be forbidden, got %d", w.Code)
	}
}

func TestSaveToolSettlementPolicy(t *testing.T) {
	f := marketplaceApp(t, nil)
	body := func(settlement string) string {
		return `{"key":"biz","name":"Biz","description":"","kind":"http","enabled":true,"units_per_call":1,"input_schema":{"type":"object"},"config":{"url":"` + f.upstream + `"},"settlement":` + settlement + `}`
	}
	scriptBody := `{"key":"bizjs","name":"BizJS","description":"","kind":"http","enabled":true,"units_per_call":1,"input_schema":{"type":"object"},"config":{"url":"` + f.upstream + `"},"settlement":{"script":"const body = JSON.parse(result.text);\nreturn body.code === 0;"}}`
	valid := `{"content":{"path":"code","equals":0}}`
	if w := request(f.handler, "POST", "/api/v1/admin/tools", body(valid), f.admin); w.Code != 201 {
		t.Fatalf("create with settlement %d %s", w.Code, w.Body)
	}
	if w := request(f.handler, "GET", "/api/v1/tools", "", f.user); w.Code != 200 || !strings.Contains(w.Body.String(), `"settlement":{"content"`) {
		t.Fatalf("settlement not listed %d %s", w.Code, w.Body)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/tools", body(`{"content":{"path":"code"}}`), f.admin); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_settlement") {
		t.Fatalf("path without equals must 400 invalid_settlement, got %d %s", w.Code, w.Body)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/tools", body(`{"content":{"pattern":"["}}`), f.admin); w.Code != 400 {
		t.Fatalf("invalid regex must 400, got %d", w.Code)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/tools", body(`{"unknown":true}`), f.admin); w.Code != 400 {
		t.Fatalf("unknown policy must 400, got %d", w.Code)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/tools", scriptBody, f.admin); w.Code != 201 {
		t.Fatalf("script policy must save, got %d %s", w.Code, w.Body)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/tools", body(`{"script":"return ??? syntax"}`), f.admin); w.Code != 400 {
		t.Fatalf("syntax error script must 400, got %d", w.Code)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/tools", body(`{"script":"`+strings.Repeat("a", 9000)+`"}`), f.admin); w.Code != 400 {
		t.Fatalf("oversized script must 400, got %d", w.Code)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/tools", body(`{"script":"return true","content":{"pattern":"x"}}`), f.admin); w.Code != 400 {
		t.Fatalf("script+content mix must 400, got %d", w.Code)
	}
}

func TestPublishPoolToolAsPluginComponent(t *testing.T) {
	f := marketplaceApp(t, nil)
	created := request(f.handler, "POST", "/api/v1/admin/tools", `{"key":"seedance","name":"Seedance 视频","description":"文生视频","kind":"http","enabled":true,"units_per_call":10,"input_schema":{"type":"object"},"config":{"url":"`+f.upstream+`"}}`, f.admin)
	if created.Code != 201 {
		t.Fatalf("create pool tool %d %s", created.Code, created.Body)
	}
	var tool struct {
		Item struct{ ID int64 }
	}
	_ = json.Unmarshal(created.Body.Bytes(), &tool)
	body := fmt.Sprintf(`{"tool_id":%d,"slug":"seedance","name":"Seedance 视频生成","description":"服务端独享，零配置直接用"}`, tool.Item.ID)
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/tools", body, f.admin); w.Code != 201 {
		t.Fatalf("publish pool tool %d %s", w.Code, w.Body)
	}
	w := request(f.handler, "GET", "/api/v1/marketplace", "", f.user)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"transport":"gateway"`) || !strings.Contains(w.Body.String(), "服务端独享") {
		t.Fatalf("gateway component not listed %d %s", w.Code, w.Body)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/tools", `{"tool_id":404404}`, f.admin); w.Code != 404 {
		t.Fatalf("missing tool must 404, got %d", w.Code)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/tools", body, f.admin); w.Code != 409 {
		t.Fatalf("duplicate publish must 409, got %d", w.Code)
	}
	if w = request(f.handler, "POST", "/api/v1/admin/marketplace/tools", body, f.user); w.Code != 403 {
		t.Fatalf("user publish must 403, got %d", w.Code)
	}
}

func TestPublicPluginDirectoryJSON(t *testing.T) {
	f := marketplaceApp(t, nil)
	w := request(f.handler, "GET", "/api/v1/plugins", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "\"description\"") || !strings.Contains(w.Body.String(), "DeepWiki") {
		t.Fatalf("directory page %d missing SEO meta or content", w.Code)
	}
	if !strings.Contains(w.Body.String(), "\"origin\"") {
		t.Fatalf("directory must show install command")
	}
	w = request(f.handler, "GET", "/api/v1/plugins/deepwiki", "", nil)
	if w.Code != 200 {
		t.Fatalf("plugin detail %d body=%s", w.Code, w.Body.String()[:min(200, w.Body.Len())])
	}
	if !strings.Contains(w.Body.String(), "DeepWiki") || !strings.Contains(w.Body.String(), "\"name\"") {
		t.Fatalf("plugin detail content wrong: %s", w.Body.String()[:min(300, w.Body.Len())])
	}
	if w = request(f.handler, "GET", "/api/v1/plugins/nope", "", nil); w.Code != 404 {
		t.Fatalf("unknown plugin %d", w.Code)
	}
}

func TestMarketplaceFormalVersioning(t *testing.T) {
	f := marketplaceApp(t, nil)
	create := func(body string) int {
		return request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin).Code
	}
	// 首发：默认 1.0.0。
	if code := create(`{"name":"演示技能","slug":"demo","source":"inline","version":"1.0.0","files":{"SKILL.md":"v1"}}`); code != 201 {
		t.Fatalf("first publish %d", code)
	}
	var version string
	query := func() string {
		if e := f.store.Pool.QueryRow(context.Background(), "SELECT version FROM marketplace_items WHERE slug='demo'").Scan(&version); e != nil {
			t.Fatal(e)
		}
		return version
	}
	if query() != "1.0.0" {
		t.Fatalf("initial version=%s", version)
	}
	// 内容变了、未指定版本 → 自动 patch +1。
	if code := create(`{"name":"演示技能","slug":"demo","source":"inline","files":{"SKILL.md":"v2"}}`); code != 200 {
		t.Fatalf("silent update %d", code)
	}
	if query() != "1.0.1" {
		t.Fatalf("auto patch bump=%s want 1.0.1", version)
	}
	// 内容再变 + 显式发版 → 用指定版本。
	if code := create(`{"name":"演示技能","slug":"demo","source":"inline","version":"2.0.0","files":{"SKILL.md":"v3"}}`); code != 200 {
		t.Fatalf("explicit release %d", code)
	}
	if query() != "2.0.0" {
		t.Fatalf("explicit version=%s want 2.0.0", version)
	}
	// 内容不变重复提交 → 版本不动。
	if code := create(`{"name":"演示技能","slug":"demo","source":"inline","files":{"SKILL.md":"v3"}}`); code != 200 {
		t.Fatalf("no-op update %d", code)
	}
	if query() != "2.0.0" {
		t.Fatalf("no-op must keep version, got %s", version)
	}
	// 非法版本号拒绝。
	if code := create(`{"name":"演示技能","slug":"demo","source":"inline","version":"latest","files":{"SKILL.md":"v4"}}`); code != 400 {
		t.Fatalf("invalid version must 400, got %d", code)
	}
	// 公开列表与 SEO 详情展示正式版本。
	w := request(f.handler, "GET", "/api/v1/plugins/demo", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "2.0.0") {
		t.Fatalf("plugin page must show formal version %d", w.Code)
	}
}
