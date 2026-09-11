package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func overrideFixture(t *testing.T) (*Gateway, int64) {
	t.Helper()
	remote := mcp.NewServer(&mcp.Implementation{Name: "override", Version: "1"}, nil)
	handler := func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return toolText("pong"), nil
	}
	remote.AddTool(&mcp.Tool{Name: "ping", Description: "original ping", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string", "description": "query terms"}},
	}}, handler)
	remote.AddTool(&mcp.Tool{Name: "status", Description: "original status", InputSchema: map[string]any{"type": "object"}}, handler)
	remote.AddTool(&mcp.Tool{Name: "deep", Description: "original deep", InputSchema: map[string]any{"type": "object"}}, handler)
	return catalogFixture(t, remote)
}

func TestCatalogAppliesMetadataOverrides(t *testing.T) {
	g, id := overrideFixture(t)
	ctx := t.Context()
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before := map[string]string{}
	for _, binding := range bindings {
		before[binding.definition.Name] = binding.definition.Description
	}
	if before["remote__ping"] != "original ping" || before["remote__status"] != "original status" {
		t.Fatalf("upstream descriptions not discovered: %v", before)
	}
	schema := `{"type":"object","properties":{"q":{"type":"string","description":"自定义查询词"}},"required":["q"],"additionalProperties":false}`
	if _, err = g.store.Pool.Exec(ctx, "INSERT INTO tool_metadata_overrides(tool_id,remote_name,description,input_schema) VALUES ($1,'ping','自定义说明',$2::jsonb),($1,'status','状态查询',NULL)", id, schema); err != nil {
		t.Fatal(err)
	}
	if _, err = g.store.Pool.Exec(ctx, "INSERT INTO tool_metadata_overrides(tool_id,remote_name,description,input_schema) VALUES ($1,'deep','坏schema','{\"type\":\"string\"}'::jsonb)", id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	bindings, err = g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	after := map[string]*toolBinding{}
	for _, binding := range bindings {
		after[binding.definition.Name] = &binding
	}
	if ping := after["remote__ping"]; ping == nil || ping.definition.Description != "自定义说明" {
		t.Fatalf("ping override not applied: %+v", after["remote__ping"])
	}
	var applied map[string]any
	if ping := after["remote__ping"]; ping != nil {
		raw, _ := json.Marshal(ping.definition.InputSchema)
		if err := json.Unmarshal(raw, &applied); err != nil || applied["required"] == nil {
			t.Fatalf("ping schema override not applied: %s", raw)
		}
		properties := applied["properties"].(map[string]any)["q"].(map[string]any)
		if properties["description"] != "自定义查询词" {
			t.Fatalf("parameter description override not applied: %v", properties)
		}
		if g.execute(ctx, store.Principal{}, *ping, json.RawMessage(`{}`)).IsError {
			t.Fatal("overridden tool stopped routing to its upstream")
		}
	}
	if status := after["remote__status"]; status == nil || status.definition.Description != "状态查询" {
		t.Fatalf("description-only override not applied: %+v", after["remote__status"])
	}
	var statusSchema map[string]any
	if status := after["remote__status"]; status != nil {
		raw, _ := json.Marshal(status.definition.InputSchema)
		if err := json.Unmarshal(raw, &statusSchema); err != nil || statusSchema["properties"] != nil {
			t.Fatalf("NULL schema override must keep the upstream schema: %s", raw)
		}
	}
	if deep := after["remote__deep"]; deep == nil || deep.definition.Description != "original deep" {
		t.Fatalf("invalid schema override must be ignored entirely: %+v", after["remote__deep"])
	}
}

func TestUpstreamToolsReturnsRawDefinitions(t *testing.T) {
	g, id := overrideFixture(t)
	if _, err := g.store.Pool.Exec(t.Context(), "INSERT INTO tool_metadata_overrides(tool_id,remote_name,description) VALUES ($1,'ping','自定义说明')", id); err != nil {
		t.Fatal(err)
	}
	tools, err := g.UpstreamTools(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, tool := range tools {
		names[tool.Name] = tool.Description
	}
	if names["ping"] != "original ping" || names["status"] != "original status" {
		t.Fatalf("UpstreamTools must return raw upstream definitions: %v", names)
	}
	if _, err := g.UpstreamTools(t.Context(), 404404); err == nil {
		t.Fatal("missing tool must return an error")
	}
}
