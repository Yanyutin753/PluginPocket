package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// settlementFixture 返回网关与已登录主体；上游工具按 mode 参数返回不同业务体。
func settlementFixture(t *testing.T, settlement string) (*Gateway, store.Principal, toolBinding, *store.Store, int64) {
	t.Helper()
	s := gatewayDB(t)
	ctx := context.Background()
	var user, wallet, token int64
	if err := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES ('settle','hash') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES ($1,5) RETURNING id", user).Scan(&wallet); err != nil {
		t.Fatal(err)
	}
	raw := "ppt_settlement_token"
	sum := sha256.Sum256([]byte(raw))
	if err := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES ($1,$2,'t','ppt_t',$3) RETURNING id", user, wallet, hex.EncodeToString(sum[:])).Scan(&token); err != nil {
		t.Fatal(err)
	}
	remote := mcp.NewServer(&mcp.Implementation{Name: "settle", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "biz", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"mode": map[string]any{"type": "string"}}}}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var arguments struct {
			Mode string `json:"mode"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &arguments)
		mode := arguments.Mode
		body := map[string]any{"code": 0, "data": "fine"}
		if mode == "fail" {
			body = map[string]any{"code": 500, "message": "boom"}
		}
		if mode == "notjson" {
			return toolText("plain text body"), nil
		}
		encoded, _ := json.Marshal(body)
		return toolText(string(encoded)), nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	t.Cleanup(upstream.Close)
	key := []byte{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7}
	g := New(s, Options{AllowPrivate: true, EncryptionKey: key})
	t.Cleanup(g.Close)
	sealed, err := SealConfig(key, fmt.Appendf(nil, `{"url":%q}`, upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if _, err = s.Pool.Exec(ctx, "INSERT INTO tools(key,name,kind,config,cost,settlement) VALUES ('biz','Biz','http',$1,1,$2::jsonb) RETURNING id", sealed, []byte(settlement)); err != nil {
		t.Fatal(err)
	}
	_ = id
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range bindings {
		if binding.definition.Name == "biz__biz" {
			return g, store.Principal{UserID: user, TokenID: token, WalletID: wallet, Username: "settle"}, binding, s, wallet
		}
	}
	t.Fatal("biz tool missing from catalog")
	return nil, store.Principal{}, toolBinding{}, nil, 0
}

func assertBalance(t *testing.T, s *store.Store, wallet, want int64) {
	t.Helper()
	var balance int64
	if err := s.Pool.QueryRow(context.Background(), "SELECT balance FROM wallets WHERE id=$1", wallet).Scan(&balance); err != nil || balance != want {
		t.Fatalf("balance=%d want=%d err=%v", balance, want, err)
	}
}

func TestSettlementContentPathChargesOnlyBusinessSuccess(t *testing.T) {
	g, principal, binding, s, wallet := settlementFixture(t, `{"content":{"path":"code","equals":0}}`)
	ctx := context.Background()
	assertBalance(t, s, wallet, 5)
	call := func(mode string) *mcp.CallToolResult {
		return g.call(ctx, principal, binding, json.RawMessage(fmt.Sprintf(`{"mode":%q}`, mode)))
	}
	if result := call("ok"); result.IsError {
		t.Fatalf("business success flagged: %+v", result.Content)
	}
	assertBalance(t, s, wallet, 4)
	if result := call("fail"); !result.IsError {
		t.Fatal("business failure must surface as error")
	}
	assertBalance(t, s, wallet, 4)
	var status string
	var cost int64
	if err := s.Pool.QueryRow(ctx, "SELECT status,cost FROM usage_logs WHERE tool='biz__biz' ORDER BY id DESC LIMIT 1").Scan(&status, &cost); err != nil || status != "error" || cost != 0 {
		t.Fatalf("failed settlement usage=%s cost=%d err=%v", status, cost, err)
	}
	if result := call("notjson"); !result.IsError {
		t.Fatal("unparseable body under a path policy must settle as failure")
	}
	assertBalance(t, s, wallet, 4)
}

func TestSettlementContentPattern(t *testing.T) {
	g, principal, binding, s, wallet := settlementFixture(t, `{"content":{"pattern":"\"code\":0"}}`)
	ctx := context.Background()
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"ok"}`)); result.IsError {
		t.Fatal("pattern match must succeed")
	}
	assertBalance(t, s, wallet, 4)
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"fail"}`)); !result.IsError {
		t.Fatal("pattern mismatch must fail")
	}
	assertBalance(t, s, wallet, 4)
}

func TestSettlementDefaultKeepsProtocolBehavior(t *testing.T) {
	g, principal, binding, s, wallet := settlementFixture(t, `{}`)
	ctx := context.Background()
	// 默认策略：上游 isError=false 的业务失败（code:500）照常扣费。
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"fail"}`)); result.IsError {
		t.Fatal("default policy must trust upstream success")
	}
	assertBalance(t, s, wallet, 4)
}

func TestSettlementEqualsAnyOf(t *testing.T) {
	// 不同上游成功码规范不一：equals 支持数组任一匹配。
	g, principal, binding, s, wallet := settlementFixture(t, `{"content":{"path":"code","equals":[0,200]}}`)
	ctx := context.Background()
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"ok"}`)); result.IsError {
		t.Fatal("code 0 must match anyof")
	}
	assertBalance(t, s, wallet, 4)
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"fail"}`)); !result.IsError {
		t.Fatal("code 500 must fail anyof")
	}
	assertBalance(t, s, wallet, 4)
}

func TestSettlementScript(t *testing.T) {
	g, principal, binding, s, wallet := settlementFixture(t, `{"script":"return !result.isError && JSON.parse(result.text).code === 0"}`)
	ctx := context.Background()
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"ok"}`)); result.IsError {
		t.Fatal("script success must settle ok")
	}
	assertBalance(t, s, wallet, 4)
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"fail"}`)); !result.IsError {
		t.Fatal("script business failure must refund")
	}
	assertBalance(t, s, wallet, 4)
	if result := g.call(ctx, principal, binding, json.RawMessage(`{"mode":"notjson"}`)); !result.IsError {
		t.Fatal("script exception must settle as failure")
	}
	assertBalance(t, s, wallet, 4)
}

func TestSettlementScriptTruthyAndInfiniteLoop(t *testing.T) {
	truthy, principal2, binding2, s2, wallet2 := settlementFixture(t, `{"script":"return 1"}`)
	if result := truthy.call(context.Background(), principal2, binding2, json.RawMessage(`{"mode":"ok"}`)); result.IsError {
		t.Fatal("truthy return must settle ok")
	}
	assertBalance(t, s2, wallet2, 4)
	loop, principal3, binding3, s3, wallet3 := settlementFixture(t, `{"script":"while (true) {}"}`)
	begin := time.Now()
	if result := loop.call(context.Background(), principal3, binding3, json.RawMessage(`{"mode":"ok"}`)); !result.IsError {
		t.Fatal("runaway script must settle as failure")
	}
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Fatalf("script interrupt too slow: %s", elapsed)
	}
	assertBalance(t, s3, wallet3, 5)
}
