package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProductJourneyRestartPreservesSessionsAndAccounting(t *testing.T) {
	f := newProductFixture(t)
	p := f.server()
	client, id := p.register("restartuser")
	admin := p.admin()
	token := p.token(client)
	session := p.mcp(token)
	result, err := session.CallTool(f.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "durable"}})
	if err != nil || result.IsError {
		t.Fatal("initial call failed")
	}
	path := fmt.Sprintf("/admin/users/%d/balance", id)
	adjustment := map[string]any{"delta": 10, "note": "durable topup", "idempotency_key": "restart-topup"}
	p.api(admin, "POST", path, adjustment, 200)
	if f.balance(id) != 1009 {
		t.Fatal("unexpected balance before restart")
	}
	tokens := p.api(client, "GET", "/account/tokens", nil, 200)["items"].([]any)
	tokenID := int64(tokens[0].(map[string]any)["id"].(float64))
	p.api(client, "DELETE", fmt.Sprintf("/account/tokens/%d", tokenID), nil, 204)
	p.stop(true)
	p.start()
	p.api(client, "GET", "/account/me", nil, 200)
	p.api(admin, "POST", path, adjustment, 200)
	if f.balance(id) != 1009 {
		t.Fatal("restart replay duplicated balance adjustment")
	}
	if _, err = session.ListTools(f.ctx, nil); err == nil {
		t.Fatal("revoked token revived after process restart")
	}
	ledger := p.api(client, "GET", "/account/ledger", nil, 200)["items"].([]any)
	var net float64
	for _, row := range ledger {
		net += row.(map[string]any)["delta"].(float64)
	}
	if net != 1009 {
		t.Fatalf("durable ledger does not reconcile: %.0f", net)
	}
	origin, _ := url.Parse(p.origin)
	staleCookies := client.Jar.Cookies(origin)
	if len(staleCookies) == 0 {
		t.Fatal("logout test requires an authenticated session cookie")
	}
	p.api(client, "POST", "/auth/logout", nil, 204)
	p.stop(true)
	p.start()
	client.Jar.SetCookies(origin, staleCookies)
	p.api(client, "GET", "/account/me", nil, 401)
}

func TestProductJourneyTwoInstancesNeverOverspend(t *testing.T) {
	f := newProductFixture(t)
	a := f.server()
	b := f.server()
	client, id := a.register("shareduser")
	token := a.token(client)
	admin := a.admin()
	a.api(admin, "POST", fmt.Sprintf("/admin/users/%d/balance", id), map[string]any{"delta": -990, "note": "limited test wallet", "idempotency_key": "limit"}, 200)
	sessions := []*mcp.ClientSession{a.mcp(token), b.mcp(token)}
	var wg sync.WaitGroup
	var successful, denied atomic.Int64
	for i := range 24 {
		wg.Go(func() {
			result, err := sessions[i%2].CallTool(f.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "concurrent"}})
			if err != nil {
				t.Errorf("concurrent protocol call: %v", err)
				return
			}
			if result.IsError {
				denied.Add(1)
			} else {
				successful.Add(1)
			}
		})
	}
	wg.Wait()
	if successful.Load() != 10 || denied.Load() != 14 || f.balance(id) != 0 {
		t.Fatalf("cross-process accounting success=%d denied=%d balance=%d", successful.Load(), denied.Load(), f.balance(id))
	}
	var calls, cost int64
	if err := f.conn.QueryRow(f.ctx, "SELECT count(*),sum(cost) FROM usage_logs WHERE user_id=$1", id).Scan(&calls, &cost); err != nil {
		t.Fatal(err)
	}
	if calls != 24 || cost != 10 {
		t.Fatalf("usage calls=%d cost=%d", calls, cost)
	}
	a.api(admin, "PATCH", fmt.Sprintf("/admin/users/%d", id), map[string]bool{"enabled": false}, 200)
	if _, err := sessions[1].ListTools(f.ctx, nil); err == nil {
		t.Fatal("disabled user remains authorized through second instance")
	}
}

func TestProductJourneyCrashRecoveryDoesNotReplayUpstream(t *testing.T) {
	f := newProductFixture(t)
	p := f.server()
	client, id := p.register("crashuser")
	admin := p.admin()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var executions atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "crash-fixture", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "work", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		executions.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "completed"}}}, nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true}))
	defer upstream.Close()
	defer close(release)
	p.api(admin, "POST", "/admin/tools", map[string]any{"key": "crash", "name": "Crash fixture", "description": "local controlled upstream", "kind": "http", "enabled": true, "units_per_call": 7, "input_schema": map[string]any{"type": "object"}, "config": map[string]any{"url": upstream.URL}}, 201)
	session := p.mcp(p.token(client))
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = session.CallTool(f.ctx, &mcp.CallToolParams{Name: "crash__work", Arguments: map[string]any{}})
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("upstream was not executed")
	}
	if f.balance(id) != 993 {
		t.Fatal("upstream started without a durable reservation")
	}
	p.stop(false)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("client did not observe process death")
	}
	// Simulate elapsed time in persisted state, without a five-minute test sleep.
	if _, err := f.conn.Exec(f.ctx, "UPDATE usage_logs SET created_at=now()-interval '6 minutes' WHERE user_id=$1 AND status='pending'", id); err != nil {
		t.Fatal(err)
	}
	p.start()
	deadline := time.Now().Add(5 * time.Second)
	for f.balance(id) != 1000 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if f.balance(id) != 1000 {
		t.Fatal("expired crashed reservation was not refunded on restart")
	}
	var recovered, refunds int
	if err := f.conn.QueryRow(f.ctx, "SELECT count(*) FROM usage_logs WHERE user_id=$1 AND status='recovered' AND cost=0", id).Scan(&recovered); err != nil {
		t.Fatal(err)
	}
	if err := f.conn.QueryRow(f.ctx, "SELECT count(*) FROM ledger WHERE user_id=$1 AND kind='recovery' AND delta=7", id).Scan(&refunds); err != nil {
		t.Fatal(err)
	}
	p.stop(true)
	p.start()
	if recovered != 1 || refunds != 1 || f.balance(id) != 1000 || executions.Load() != 1 {
		t.Fatalf("recovery duplicated work or credit: recovered=%d refunds=%d upstream=%d balance=%d", recovered, refunds, executions.Load(), f.balance(id))
	}
}

func TestProductJourneyAdminUpstreamPricingAndRefund(t *testing.T) {
	f := newProductFixture(t)
	p := f.server()
	client, id := p.register("upstreamuser")
	admin := p.admin()
	var executions atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "billing-fixture", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "work", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		n := executions.Add(1)
		if n == 1 {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "private upstream credential must never escape"}}}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "completed"}}}, nil
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	defer upstream.Close()
	tool := map[string]any{"key": "billing", "name": "Billing fixture", "description": "test", "kind": "http", "enabled": true, "units_per_call": 7, "input_schema": map[string]any{"type": "object"}, "config": map[string]any{"url": upstream.URL}}
	saved := p.api(admin, "POST", "/admin/tools", tool, 201)
	toolID := int64(saved["item"].(map[string]any)["id"].(float64))
	session := p.bridge(p.token(client), os.Getenv("PLUGINPOCKET_CLI_BINARY"))
	failed, err := session.CallTool(f.ctx, &mcp.CallToolParams{Name: "billing__work", Arguments: map[string]any{}})
	if err != nil || !failed.IsError {
		t.Fatal("upstream failure did not reach bridge as a tool error")
	}
	raw, _ := json.Marshal(failed)
	if bytes.Contains(raw, []byte("private upstream")) {
		t.Fatal("upstream credential was exposed to client")
	}
	if f.balance(id) != 1000 || executions.Load() != 1 {
		t.Fatal("failed upstream was charged or replayed")
	}
	tool["units_per_call"] = 5
	delete(tool, "config")
	p.api(admin, "PATCH", fmt.Sprintf("/admin/tools/%d", toolID), tool, 200)
	result, err := session.CallTool(f.ctx, &mcp.CallToolParams{Name: "billing__work", Arguments: map[string]any{}})
	if err != nil || result.IsError || f.balance(id) != 995 || executions.Load() != 2 {
		t.Fatalf("updated pricing did not apply through bridge: %v balance=%d calls=%d", err, f.balance(id), executions.Load())
	}
	tool["enabled"] = false
	p.api(admin, "PATCH", fmt.Sprintf("/admin/tools/%d", toolID), tool, 200)
	result, err = session.CallTool(f.ctx, &mcp.CallToolParams{Name: "billing__work", Arguments: map[string]any{}})
	if err == nil && !result.IsError {
		t.Fatal("disabled upstream remained callable")
	}
	if executions.Load() != 2 || f.balance(id) != 995 {
		t.Fatal("disabled upstream executed or changed balance")
	}
}

func TestProductJourneyDesktopBridgeUsesRealServer(t *testing.T) {
	executable := os.Getenv("PLUGINPOCKET_DESKTOP_BINARY")
	if executable == "" {
		t.Skip("desktop E2E requires built native desktop")
	}
	f := newProductFixture(t)
	p := f.server()
	client, id := p.register("desktopuser")
	session := p.bridge(p.token(client), executable)
	result, err := session.CallTool(f.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "desktop to PostgreSQL"}})
	if err != nil || result.IsError || f.balance(id) != 999 {
		t.Fatalf("native desktop bridge billing: %v balance=%d", err, f.balance(id))
	}
}
