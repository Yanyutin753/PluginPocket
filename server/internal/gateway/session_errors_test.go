package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRPCErrorDoesNotCloseConcurrentCall(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		legacy bool
	}{
		{"modern handler error", errors.New("private upstream detail"), false},
		{"modern invalid params", &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "private upstream detail"}, false},
		{"modern method not found", &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "private upstream detail"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			var connections atomic.Int64
			remote := mcp.NewServer(&mcp.Implementation{Name: "rpc-errors", Version: "1"}, nil)
			remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == "server/discover" {
						connections.Add(1)
						if test.legacy {
							return nil, errors.New("legacy upstream uses initialize")
						}
					}
					return next(ctx, method, req)
				}
			})
			remote.AddTool(&mcp.Tool{Name: "wait", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				entered <- struct{}{}
				select {
				case <-release:
					return toolText("finished"), nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
			remote.AddTool(&mcp.Tool{Name: "fail", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return nil, test.err })
			g, _ := catalogFixtureWithMode(t, remote, !test.legacy)
			defer unblock()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bindings, err := g.tools(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var wait, fail toolBinding
			for _, b := range bindings {
				switch b.definition.Name {
				case "remote__wait":
					wait = b
				case "remote__fail":
					fail = b
				}
			}
			if wait.definition == nil || fail.definition == nil {
				t.Fatal("missing fixture tools")
			}
			slow := make(chan *mcp.CallToolResult, 1)
			go func() { slow <- g.execute(ctx, store.Principal{}, wait, json.RawMessage(`{}`)) }()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("concurrent upstream call did not start")
			}
			failed := make(chan *mcp.CallToolResult, 1)
			go func() { failed <- g.execute(ctx, store.Principal{}, fail, json.RawMessage(`{}`)) }()
			select {
			case result := <-failed:
				if !result.IsError {
					t.Error("RPC error returned success")
				}
			case <-time.After(300 * time.Millisecond):
				t.Error("RPC error blocked while closing an unrelated in-flight call")
			}
			unblock()
			select {
			case result := <-slow:
				if result.IsError {
					t.Error("ordinary RPC error interrupted another call")
				}
			case <-ctx.Done():
				t.Fatal("concurrent call did not finish")
			}
			if result := g.execute(ctx, store.Principal{}, wait, json.RawMessage(`{}`)); result.IsError {
				t.Error("subsequent call failed after ordinary RPC error")
			}
			if connections.Load() != 2 {
				t.Errorf("concurrent calls need two isolated sessions and then reuse: %d handshakes", connections.Load())
			}
		})
	}
}

func TestHTTPDisconnectRebuildsOnlyTheFailedSession(t *testing.T) {
	var disconnected atomic.Bool
	var connections, calls atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "disconnect", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		return toolText("pong"), nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Mcp-Method") == "server/discover" {
			connections.Add(1)
		}
		if r.Header.Get("Mcp-Method") == "tools/call" && disconnected.CompareAndSwap(false, true) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer upstream.Close()
	key := bytes.Repeat([]byte{6}, 32)
	config, err := SealConfig(key, []byte(`{"url":"`+upstream.URL+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	g := New(nil, Options{AllowPrivate: true, EncryptionKey: key})
	defer g.Close()
	binding := toolBinding{row: toolRow{ID: 1, Kind: "http", Config: config}, remoteName: "ping"}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if !g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{}`)).IsError {
		t.Fatal("disconnected call returned success or was replayed")
	}
	if g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{}`)).IsError {
		t.Fatal("next request did not recover after a real HTTP disconnect")
	}
	if connections.Load() != 2 || calls.Load() != 1 {
		t.Fatalf("connections=%d executed calls=%d; want 2 and 1 without replay", connections.Load(), calls.Load())
	}
}

func TestCloseCancelsActiveHTTPCall(t *testing.T) {
	remote, entered, release := blockedUpstream(t, "tools/call")
	g, _ := catalogFixture(t, remote)
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var binding toolBinding
	for _, b := range bindings {
		if b.definition.Name == "remote__ping" {
			binding = b
		}
	}
	if binding.definition == nil {
		t.Fatal("missing remote tool")
	}
	result := make(chan *mcp.CallToolResult, 1)
	go func() { result <- g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{}`)) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("upstream call did not start")
	}
	closed := make(chan struct{})
	go func() { g.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(300 * time.Millisecond):
		t.Error("Close waited for the original upstream call deadline")
	}
	release()
	select {
	case got := <-result:
		if !got.IsError {
			t.Error("active call completed successfully after gateway shutdown")
		}
	case <-ctx.Done():
		t.Fatal("active request did not finish on shutdown")
	}
}

func TestHTTPProtocolFailureRebuildsConnection(t *testing.T) {
	for _, test := range []struct {
		name string
		code int64
	}{
		{"HTTP 400", jsonrpc.CodeInvalidParams},
		{"HTTP 404", jsonrpc.CodeMethodNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls, connections atomic.Int64
			remote := mcp.NewServer(&mcp.Implementation{Name: "http-errors", Version: "1"}, nil)
			remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == "server/discover" {
						connections.Add(1)
					}
					return next(ctx, method, req)
				}
			})
			remote.AddTool(&mcp.Tool{Name: "flaky", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				if calls.Add(1) == 1 {
					return nil, &jsonrpc.Error{Code: test.code, Message: "upstream HTTP rejection"}
				}
				return toolText("recovered"), nil
			})
			g, _ := catalogFixtureWithMode(t, remote, true)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bindings, err := g.tools(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var binding toolBinding
			for _, b := range bindings {
				if b.definition.Name == "remote__flaky" {
					binding = b
				}
			}
			if binding.definition == nil {
				t.Fatal("missing flaky tool")
			}
			if result := g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{}`)); !result.IsError {
				t.Fatal("first HTTP error returned success")
			}
			if result := g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{}`)); result.IsError {
				t.Error("HTTP error left a dead connection cached for the next call")
			}
			if calls.Load() != 2 || connections.Load() != 2 {
				t.Errorf("recovery calls=%d connections=%d, want 2 each without replay", calls.Load(), connections.Load())
			}
		})
	}
}
