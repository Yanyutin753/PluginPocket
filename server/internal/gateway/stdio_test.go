package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioUpstreamHelper(t *testing.T) {
	if os.Getenv("PLUGINPOCKET_STDIO_TEST_HELPER") != "1" {
		return
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "stdio-fixture", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return toolText("stdio pong"), nil
	})
	if server.Run(context.Background(), &mcp.StdioTransport{}) != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestAllowlistedStdioUpstreamUsesOfficialProtocol(t *testing.T) {
	s := gatewayDB(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{3}, 32)
	raw, err := json.Marshal(UpstreamConfig{Command: "fixture", Args: []string{"-test.run=^TestStdioUpstreamHelper$"}, Env: map[string]string{"PLUGINPOCKET_STDIO_TEST_HELPER": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	denied := New(s, Options{EncryptionKey: key})
	if denied.ValidateConfig("stdio", raw) == nil {
		t.Fatal("stdio enabled without deployment allowlist")
	}
	g := New(s, Options{EncryptionKey: key, StdioCommands: map[string]string{"fixture": executable}})
	defer g.Close()
	if err = g.ValidateConfig("stdio", raw); err != nil {
		t.Fatal(err)
	}
	config, err := SealConfig(key, raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = s.Pool.Exec(ctx, "INSERT INTO tools(key,name,kind,config) VALUES('local','Local','stdio',$1)", config); err != nil {
		t.Fatal(err)
	}
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range bindings {
		if binding.definition.Name == "local__ping" {
			result := g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{}`))
			if result.IsError || len(result.Content) != 1 {
				t.Fatal("stdio call failed")
			}
			if result.Content[0].(*mcp.TextContent).Text != "stdio pong" {
				t.Fatal("unexpected stdio response")
			}
			return
		}
	}
	t.Fatal("allowlisted stdio tools not discovered")
}
