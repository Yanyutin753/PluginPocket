package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProductJourneyRedisSharedCatalogAndOutage(t *testing.T) {
	if os.Getenv("LOADOUT_TEST_REDIS_URL") == "" {
		t.Skip("real Redis required")
	}
	f := newProductFixture(t)
	a, b := f.server(), f.server()
	user, id := a.register("redisuser")
	token := a.token(user)
	var discoveries atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "redis-fixture", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "work", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "completed"}}}, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			raw, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			var request struct{ Method string }
			_ = json.Unmarshal(raw, &request)
			if request.Method == "tools/list" {
				discoveries.Add(1)
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
		}
		handler.ServeHTTP(w, r)
	}))
	defer upstream.Close()
	a.api(a.admin(), "POST", "/admin/tools", map[string]any{"key": "shared", "name": "Shared discovery", "kind": "http", "enabled": true, "units_per_call": 7, "input_schema": map[string]any{"type": "object"}, "config": map[string]any{"url": upstream.URL}}, 201)
	for _, p := range []*productProcess{a, b} {
		result, err := p.mcp(token).ListTools(f.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, tool := range result.Tools {
			found = found || tool.Name == "shared__work"
		}
		if !found {
			t.Fatal("shared remote tool missing")
		}
	}
	if got := discoveries.Load(); got != 1 {
		t.Fatalf("two processes discovered upstream %d times; want one shared Redis catalog", got)
	}
	assertRedisReady := func(p *productProcess, state string) {
		t.Helper()
		response, err := p.client().Get(p.origin + "/readyz")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := response.Body.Close(); err != nil {
				t.Errorf("fixture cleanup failed: %v", err)
			}
		}()
		var body map[string]string
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil || response.StatusCode != 200 || body["redis"] != state {
			t.Fatalf("cache health want=%s status=%d body=%v decode=%v", state, response.StatusCode, body, err)
		}
	}
	assertRedisReady(b, "ready")
	// Simulate an unavailable cache endpoint for a fresh replica without
	// stopping the shared developer/test Redis or relying on a warm local cache.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.redisURL = "redis://" + listener.Addr().String() + "/0"
	_ = listener.Close()
	b.stop(true)
	b.start()
	assertRedisReady(b, "degraded")
	result, err := b.mcp(token).CallTool(f.ctx, &mcp.CallToolParams{Name: "shared__work", Arguments: map[string]any{}})
	if err != nil || result.IsError || f.balance(id) != 993 {
		t.Fatalf("Redis outage disrupted upstream execution or accounting: err=%v balance=%d", err, f.balance(id))
	}
	if discoveries.Load() != 2 {
		t.Fatalf("fresh replica did not discover directly during outage: %d", discoveries.Load())
	}
}
