package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPUpstreamSchemaNamespaceAndReplacement(t *testing.T) {
	s := gatewayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	remote := mcp.NewServer(&mcp.Implementation{Name: "upstream", Version: "1"}, nil)
	schema := map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}
	remote.AddTool(&mcp.Tool{Name: "search", InputSchema: schema}, func(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return toolText("found"), nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	defer upstream.Close()
	key := bytes.Repeat([]byte{9}, 32)
	g := New(s, Options{AllowPrivate: true, EncryptionKey: key})
	defer g.Close()
	config, err := SealConfig(key, []byte(`{"url":"`+upstream.URL+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err = s.Pool.QueryRow(ctx, "INSERT INTO tools(key,name,kind,config) VALUES ('remote','Remote','http',$1) RETURNING id", config).Scan(&id); err != nil {
		t.Fatal(err)
	}
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found *toolBinding
	for _, b := range bindings {
		if b.definition.Name == "remote__search" {
			found = &b
		}
	}
	if found == nil {
		t.Fatal("upstream tool was not namespaced")
	}
	expected, _ := json.Marshal(schema)
	actual, _ := json.Marshal(found.definition.InputSchema)
	if !bytes.Equal(expected, actual) {
		t.Fatalf("schema changed: %s", actual)
	}
	result := g.execute(ctx, *found, json.RawMessage(`{"query":"q"}`))
	if result.IsError {
		t.Fatal(result)
	}
	// Re-encrypting changes the connection signature and must release the old session.
	config, err = SealConfig(key, []byte(`{"url":"`+upstream.URL+`","headers":{"X-Revision":"2"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE tools SET config=$1 WHERE id=$2", config, id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	if _, err = g.tools(ctx); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	count := len(g.httpPools)
	g.mu.Unlock()
	if count != 1 {
		t.Fatalf("old upstream connection retained after config change: %d", count)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE tools SET enabled=false WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	if _, err = g.tools(ctx); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	count = len(g.httpPools)
	g.mu.Unlock()
	if count != 0 {
		t.Fatalf("disabled upstream retained %d sessions", count)
	}
}

func TestRateLimitIsSharedAndAtomic(t *testing.T) {
	s := gatewayDB(t)
	a := New(s, Options{})
	b := New(s, Options{})
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			g := a
			if i%2 == 0 {
				g = b
			}
			if g.admit(context.Background(), store.Principal{UserID: 8, TokenID: 9}) {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	if n := accepted.Load(); n != 60 {
		t.Fatalf("allowed %d calls across instances; want 60", n)
	}
}

func TestSlowUpstreamDoesNotHideHealthyTools(t *testing.T) {
	s := gatewayDB(t)
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer slow.Close()
	remote := mcp.NewServer(&mcp.Implementation{Name: "healthy", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText("pong"), nil })
	healthy := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	defer healthy.Close()
	key := bytes.Repeat([]byte{8}, 32)
	g := New(s, Options{AllowPrivate: true, EncryptionKey: key})
	defer g.Close()
	for i, endpoint := range []string{slow.URL, healthy.URL} {
		config, err := SealConfig(key, []byte(`{"url":"`+endpoint+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		name := []string{"slow", "healthy"}[i]
		if _, err = s.Pool.Exec(context.Background(), "INSERT INTO tools(key,name,kind,config) VALUES($1,$1,'http',$2)", name, config); err != nil {
			t.Fatal(err)
		}
	}
	// The caller stays alive beyond the independent 2s upstream budget. Caller
	// cancellation itself is covered separately and must return a context error.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bindings {
		if b.definition.Name == "healthy__ping" {
			return
		}
	}
	t.Fatal("a slow upstream hid healthy tools")
}

// Integration coverage: execute an actual HTTP MCP failure/timeout and check
// the committed wallet/usage rows, rather than mocking the accounting calls.
func TestUpstreamFailuresRefundWithoutReplay(t *testing.T) {
	s := gatewayDB(t)
	ctx := context.Background()
	var user, wallet, token int64
	if err := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES('refund','hash') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES($1,9) RETURNING id", user).Scan(&wallet); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES($1,$2,'refund','ldt_refund','refundhash') RETURNING id", user, wallet).Scan(&token); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "failures", Version: "1"}, nil)
	for _, name := range []string{"fail", "slow"} {
		remote.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			calls.Add(1)
			if r.Params.Name == "slow" {
				select {
				case <-ctx.Done():
				case <-time.After(200 * time.Millisecond):
				}
			}
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "private provider credential"}}}, nil
		})
	}
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true}))
	defer upstream.Close()
	key := bytes.Repeat([]byte{4}, 32)
	g := New(s, Options{AllowPrivate: true, EncryptionKey: key, Timeout: 50 * time.Millisecond})
	defer g.Close()
	config, err := SealConfig(key, []byte(`{"url":"`+upstream.URL+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "INSERT INTO tools(key,name,kind,cost,config) VALUES('failures','Failures','http',3,$1)", config); err != nil {
		t.Fatal(err)
	}
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tested := 0
	for _, b := range bindings {
		if b.row.Kind == "builtin" {
			continue
		}
		tested++
		result := g.call(ctx, store.Principal{UserID: user, WalletID: wallet, TokenID: token}, b, json.RawMessage(`{}`))
		data, _ := json.Marshal(result)
		if !result.IsError || bytes.Contains(data, []byte("private provider")) {
			t.Fatalf("unsafe result %s", data)
		}
	}
	if tested != 2 || calls.Load() != 2 {
		t.Fatalf("upstream calls=%d tested=%d", calls.Load(), tested)
	}
	var balance, cost, rows int64
	if err = s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", wallet).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err = s.Pool.QueryRow(ctx, "SELECT count(*),sum(cost) FROM usage_logs WHERE user_id=$1 AND status='error'", user).Scan(&rows, &cost); err != nil {
		t.Fatal(err)
	}
	if balance != 9 || cost != 0 || rows != 2 {
		t.Fatalf("balance=%d cost=%d rows=%d", balance, cost, rows)
	}
}

func TestDisabledCallIsAudited(t *testing.T) {
	s := gatewayDB(t)
	ctx := context.Background()
	var uid, wid, tid int64
	if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES('auditor','hash') RETURNING id").Scan(&uid); e != nil {
		t.Fatal(e)
	}
	if e := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES($1,10) RETURNING id", uid).Scan(&wid); e != nil {
		t.Fatal(e)
	}
	if e := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES($1,$2,'token','ldt_','hash') RETURNING id", uid, wid).Scan(&tid); e != nil {
		t.Fatal(e)
	}
	g := New(s, Options{})
	defer g.Close()
	bindings, e := g.tools(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var echo toolBinding
	for _, b := range bindings {
		if b.row.Key == "echo" {
			echo = b
		}
	}
	if _, e = s.Pool.Exec(ctx, "UPDATE tools SET enabled=false WHERE id=$1", echo.row.ID); e != nil {
		t.Fatal(e)
	}
	result := g.call(ctx, store.Principal{UserID: uid, TokenID: tid, WalletID: wid}, echo, json.RawMessage(`{"message":"test"}`))
	if !result.IsError {
		t.Fatal("disabled call unexpectedly succeeded")
	}
	var count int
	if e = s.Pool.QueryRow(ctx, "SELECT count(*) FROM usage_logs WHERE user_id=$1", uid).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatalf("disabled call audit rows=%d want1", count)
	}
}
