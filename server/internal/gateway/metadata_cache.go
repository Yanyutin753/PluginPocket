package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (g *Gateway) remoteTools(ctx context.Context, row toolRow, revision int64) (definitions []*mcp.Tool, err error) {
	key := ""
	if row.Kind == "http" && g.options.Cache != nil {
		// A deployment namespace identifies the database. Revision and encrypted
		// configuration isolate updates; local decryption/SSRF policy must agree too.
		key = fmt.Sprintf("tools:v1:%d:%s:%x:%t", revision, sessionKey(row), sha256.Sum256(g.options.EncryptionKey), g.options.AllowPrivate)
		if value, cacheErr := g.options.Cache.Get(ctx, key); cacheErr == nil && json.Unmarshal(value, &definitions) == nil && validToolDefinitions(definitions) {
			return definitions, nil
		}
	}
	session, err := g.upstream(ctx, row)
	if err != nil {
		return nil, err
	}
	if session.ctx != nil {
		ctx = session.ctx
	}
	defer func() { session.release(err) }()
	definitions = make([]*mcp.Tool, 0)
	for tool, failure := range session.session.Tools(ctx, nil) {
		if failure != nil {
			g.dropSession(ctx, row, failure)
			return nil, failure
		}
		definitions = append(definitions, tool)
	}
	if !validToolDefinitions(definitions) {
		return nil, errors.New("invalid upstream tool metadata")
	}
	if key != "" {
		if value, marshalErr := json.Marshal(definitions); marshalErr == nil {
			_ = g.options.Cache.Set(ctx, key, value, 5*time.Second)
		}
	}
	return definitions, nil
}

// AddTool is the official SDK's validation boundary, including protocol header
// annotations. It reports invalid definitions by panic; reject those before
// publishing a serving catalog. This validation server never serves requests.
func validToolDefinitions(definitions []*mcp.Tool) (valid bool) {
	defer func() {
		if recover() != nil {
			valid = false
		}
	}()
	if definitions == nil {
		return false
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "loadout-metadata", Version: "0.2.0"}, nil)
	names := make(map[string]bool, len(definitions))
	for _, tool := range definitions {
		if tool == nil || tool.Name == "" || names[tool.Name] {
			return false
		}
		names[tool.Name] = true
		server.AddTool(tool, nil)
	}
	return true
}

// ValidateToolSchema uses the same official SDK checks used before serving tools.
func ValidateToolSchema(raw json.RawMessage) error {
	if !validToolDefinitions([]*mcp.Tool{{Name: "validation", InputSchema: raw}}) {
		return errors.New("invalid tool schema")
	}
	return nil
}
