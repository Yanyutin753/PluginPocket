package demo

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Upstream uses the official protocol implementation; only tool results are fake.
func Upstream() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "loadout-demo-upstream", Version: "1"}, nil)
	for _, name := range []string{"success", "failure"} {
		server.AddTool(&mcp.Tool{Name: name, Description: "DEMO: deterministic " + name, InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{IsError: name == "failure", Content: []mcp.Content{&mcp.TextContent{Text: "DEMO: " + name}}}, nil
		})
	}
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true})
}
