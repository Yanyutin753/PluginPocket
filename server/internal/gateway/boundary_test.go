package gateway

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func boundaryPrincipal(t *testing.T, s *store.Store) store.Principal {
	t.Helper()
	var p store.Principal
	ctx := t.Context()
	if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES('boundaryreview','hash') RETURNING id").Scan(&p.UserID); e != nil {
		t.Fatal(e)
	}
	if e := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES($1,10) RETURNING id", p.UserID).Scan(&p.WalletID); e != nil {
		t.Fatal(e)
	}
	if e := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES($1,$2,'boundary','ldt_',$3) RETURNING id", p.UserID, p.WalletID, auth.Digest("ldt_boundary_review")).Scan(&p.TokenID); e != nil {
		t.Fatal(e)
	}
	return p
}
func boundaryUpstream(t *testing.T, name, text string) *httptest.Server {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: text, Version: "1"}, nil)
	s.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText(text), nil })
	h := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
	t.Cleanup(h.Close)
	return h
}
func TestBoundaryNamespaceCollisionRoutesToWrongProvider(t *testing.T) {
	s := gatewayDB(t)
	p := boundaryPrincipal(t, s)
	first := boundaryUpstream(t, "bar__baz", "provider-A")
	second := boundaryUpstream(t, "baz", "provider-B")
	key := bytes.Repeat([]byte{7}, 32)
	g := New(s, Options{AllowPrivate: true, EncryptionKey: key})
	t.Cleanup(g.Close)
	var addSecond func()
	for i, fixture := range []struct {
		key, url string
		cost     int64
	}{{"foo", first.URL, 1}, {"foo__bar", second.URL, 7}} {
		config, e := SealConfig(key, []byte(`{"url":"`+fixture.url+`"}`))
		if e != nil {
			t.Fatal(e)
		}
		if i == 1 {
			addSecond = func() {
				if _, err := s.Pool.Exec(t.Context(), "INSERT INTO tools(key,name,kind,config,cost) VALUES($1,$2,'http',$3,$4)", fixture.key, "Provider B", config, fixture.cost); err != nil {
					t.Fatal(err)
				}
			}
			continue
		}
		if _, e = s.Pool.Exec(t.Context(), "INSERT INTO tools(key,name,kind,config,cost) VALUES($1,$2,'http',$3,$4)", fixture.key, fmt.Sprintf("Provider %d", i), config, fixture.cost); e != nil {
			t.Fatal(e)
		}
	}
	gatewayServer := httptest.NewServer(g)
	defer gatewayServer.Close()
	hc, e := upstreamClient(gatewayServer.URL, map[string]string{"Authorization": "Bearer ldt_boundary_review"}, true)
	if e != nil {
		t.Fatal(e)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "boundary", Version: "1"}, nil)
	session, e := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: gatewayServer.URL, HTTPClient: hc, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = session.Close() }()
	baseline, e := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "foo__bar__baz", Arguments: map[string]any{}})
	if e != nil {
		t.Fatal(e)
	}
	if baseline.Content[0].(*mcp.TextContent).Text != "provider-A" {
		t.Fatal("first provider baseline failed")
	}
	addSecond()
	list, e := session.ListTools(t.Context(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(list.Tools) != 4 {
		t.Errorf("two upstreams plus two builtins should expose 4 tools; got %d", len(list.Tools))
	}
	result, e := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "foo__bar__baz", Arguments: map[string]any{}})
	if e != nil {
		t.Fatal(e)
	}
	var balance int64
	if e = s.Pool.QueryRow(t.Context(), "SELECT balance FROM wallets WHERE id=$1", p.WalletID).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if text != "provider-A" || balance != 8 {
		t.Errorf("existing foo/bar__baz route changed after configuring foo__bar/baz: result=%q balance=%d; expected provider-A and balance 8", text, balance)
	}
}
func TestBoundaryAll129EnabledUpstreamsVisible(t *testing.T) {
	s := gatewayDB(t)
	remote := boundaryUpstream(t, "ping", "pong")
	key := bytes.Repeat([]byte{7}, 32)
	g := New(s, Options{AllowPrivate: true, EncryptionKey: key})
	t.Cleanup(g.Close)
	config, e := SealConfig(key, []byte(`{"url":"`+remote.URL+`"}`))
	if e != nil {
		t.Fatal(e)
	}
	for i := range 129 {
		name := fmt.Sprintf("provider%d", i)
		if _, e = s.Pool.Exec(t.Context(), "INSERT INTO tools(key,name,kind,config) VALUES($1,$1,'http',$2)", name, config); e != nil {
			t.Fatal(e)
		}
	}
	bindings, e := g.tools(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, b := range bindings {
		if b.definition.Name == "provider128__ping" {
			found = true
		}
	}
	if !found || len(bindings) != 131 {
		t.Fatalf("129 enabled upstreams plus builtins should expose 131 bindings including provider128; actual bindings=%d final_provider_present=%v", len(bindings), found)
	}
}
