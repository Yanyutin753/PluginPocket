package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProductJourneyMCPHotReload(t *testing.T) {
	f := newProductFixture(t)
	a, b := f.server(), f.server()
	admin := a.admin()
	user, id := b.register("hot-reload-user")
	token := b.token(user)
	bridge := b.bridge(token, os.Getenv("PLUGINPOCKET_CLI_BINARY"))
	direct := a.mcp(token)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	remote := func(name, marker string, block bool) *httptest.Server {
		server := mcp.NewServer(&mcp.Implementation{Name: "reload-fixture", Version: marker}, nil)
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: marker, InputSchema: map[string]any{"type": "object", "properties": map[string]any{marker: map[string]any{"type": "string"}}, "required": []string{marker}}}, func(ctx context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {
			if block {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: marker}}}, nil, nil
		})
		h := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true}))
		t.Cleanup(h.Close)
		return h
	}
	old, newUpstream := remote("old", "first", true), remote("new", "second", false)
	// Ensure the blocked handler is released before httptest cleanup, even on failure.
	t.Cleanup(unblock)
	names := func(s *mcp.ClientSession) []string {
		t.Helper()
		result, err := s.ListTools(f.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range result.Tools {
			names = append(names, tool.Name)
		}
		return names
	}
	if slices.Contains(names(bridge), "reload__old") {
		t.Fatal("fixture exists before administrator adds it")
	}
	tool := map[string]any{"key": "reload", "name": "Hot reload fixture", "description": "live replacement", "kind": "http", "enabled": true, "units_per_call": 7, "input_schema": map[string]any{"type": "object"}, "config": map[string]any{"url": old.URL}}
	saved := a.api(admin, "POST", "/admin/tools", tool, 201)
	path := fmt.Sprintf("/admin/tools/%.0f", saved["item"].(map[string]any)["id"])
	for _, s := range []*mcp.ClientSession{direct, bridge} {
		if !slices.Contains(names(s), "reload__old") {
			t.Fatal("added upstream missing from live session")
		}
	}
	type answer struct {
		result *mcp.CallToolResult
		err    error
	}
	pending := make(chan answer, 1)
	go func() {
		r, e := bridge.CallTool(f.ctx, &mcp.CallToolParams{Name: "reload__old", Arguments: map[string]any{"first": "value"}})
		pending <- answer{r, e}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("old call did not enter upstream")
	}
	tool["config"] = map[string]any{"url": newUpstream.URL}
	tool["units_per_call"] = 11
	a.api(admin, "PATCH", path, tool, 200)
	for _, s := range []*mcp.ClientSession{direct, bridge} {
		list, err := s.ListTools(f.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, definition := range list.Tools {
			if definition.Name == "reload__old" {
				t.Fatal("old definition remained after replacement")
			}
			if definition.Name == "reload__new" {
				found = true
				raw, err := json.Marshal(definition.InputSchema)
				if err != nil {
					t.Fatal(err)
				}
				var schema struct {
					Required []string `json:"required"`
				}
				if json.Unmarshal(raw, &schema) != nil || !slices.Equal(schema.Required, []string{"second"}) {
					t.Fatal("replacement advertised stale required input")
				}
				if definition.Description != "second" {
					t.Fatal("stale description")
				}
			}
		}
		if !found {
			t.Fatal("new definition missing after replacement")
		}
	}
	// Required input has changed. The validating upstream must reject old arguments without a charge.
	bad, err := bridge.CallTool(f.ctx, &mcp.CallToolParams{Name: "reload__new", Arguments: map[string]any{"first": "value"}})
	if err == nil && !bad.IsError {
		t.Fatal("replacement did not update required input schema")
	}
	result, err := bridge.CallTool(f.ctx, &mcp.CallToolParams{Name: "reload__new", Arguments: map[string]any{"second": "value"}})
	if err != nil || result.IsError || result.Content[0].(*mcp.TextContent).Text != "second" {
		t.Fatal("new call did not reach replacement upstream")
	}
	unblock()
	select {
	case done := <-pending:
		if done.err != nil || done.result.IsError || done.result.Content[0].(*mcp.TextContent).Text != "first" {
			t.Fatal("hot reload interrupted in-flight call")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("old call did not finish")
	}
	if got := f.balance(id); got != 982 {
		t.Fatalf("old/new price accounting=%d want=982", got)
	}
	delete(tool, "config")
	tool["enabled"] = false
	a.api(admin, "PATCH", path, tool, 200)
	if slices.Contains(names(bridge), "reload__new") {
		t.Fatal("disabled upstream remained in live bridge list")
	}
	denied, err := bridge.CallTool(f.ctx, &mcp.CallToolParams{Name: "reload__new", Arguments: map[string]any{"second": "value"}})
	if err == nil && !denied.IsError {
		t.Fatal("disabled upstream still callable")
	}
	if f.balance(id) != 982 {
		t.Fatal("disabled call charged wallet")
	}
	tool["enabled"] = true
	tool["units_per_call"] = 3
	a.api(admin, "PATCH", path, tool, 200)
	if !slices.Contains(names(bridge), "reload__new") {
		t.Fatal("reenabled upstream missing")
	}
	result, err = bridge.CallTool(f.ctx, &mcp.CallToolParams{Name: "reload__new", Arguments: map[string]any{"second": "value"}})
	if err != nil || result.IsError || f.balance(id) != 979 {
		t.Fatal("reenabled upstream did not use new price")
	}
	if a.command.ProcessState != nil || b.command.ProcessState != nil {
		t.Fatal("server restarted during reload")
	}
}
