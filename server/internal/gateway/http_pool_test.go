package gateway

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func poolTool(t *testing.T, g *Gateway, ctx context.Context, name string) toolBinding {
	t.Helper()
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range bindings {
		if binding.definition.Name == name {
			return binding
		}
	}
	t.Fatalf("missing %s", name)
	return toolBinding{}
}

func TestRetiredHTTPPoolDoesNotBlockReenableOrInterruptActiveCall(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var connections atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "retirement", Version: "1"}, nil)
	remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "server/discover" {
				connections.Add(1)
			}
			return next(ctx, method, req)
		}
	})
	remote.AddTool(&mcp.Tool{Name: "wait", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ Old bool }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		if args.Old {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return toolText("finished"), nil
	})
	g, id := catalogFixtureWithMode(t, remote, true)
	defer unblock()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	binding := poolTool(t, g, ctx, "remote__wait")
	old := make(chan *mcp.CallToolResult, 1)
	go func() { old <- g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{"old":true}`)) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("old call did not start")
	}
	if _, err := g.store.Pool.Exec(ctx, "UPDATE tools SET enabled=false WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	if _, err := g.tools(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-old:
		t.Fatal("retiring a pool interrupted an active call")
	default:
	}
	if _, err := g.store.Pool.Exec(ctx, "UPDATE tools SET enabled=true WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	current := poolTool(t, g, ctx, "remote__wait")
	if g.execute(ctx, store.Principal{}, current, json.RawMessage(`{}`)).IsError {
		t.Fatal("reenabled upstream waited for retired active calls")
	}
	unblock()
	if result := <-old; result.IsError {
		t.Fatal("retired active call did not finish successfully")
	}
	if g.execute(ctx, store.Principal{}, current, json.RawMessage(`{}`)).IsError || connections.Load() != 2 {
		t.Fatalf("retired release replaced the new pool: handshakes=%d", connections.Load())
	}
}

func TestRepeatedRetirementKeepsProviderCapacity(t *testing.T) {
	remote := mcp.NewServer(&mcp.Implementation{Name: "repeated-retirement", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText("pong"), nil })
	g, _ := catalogFixtureWithMode(t, remote, true)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	binding := poolTool(t, g, ctx, "remote__ping")
	first, err := g.upstream(ctx, binding.row)
	if err != nil {
		t.Fatal(err)
	}
	defer first.release(nil)
	first.pool.close(false)
	middle, err := g.httpPool(binding.row)
	if err != nil {
		t.Fatal(err)
	}
	middle.close(false)
	latest, err := g.httpPool(binding.row)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		latest.close(false)
		g.mu.Lock()
		defer g.mu.Unlock()
		if len(g.httpProviders) != 0 {
			t.Fatal("retired provider capacity remained after all leases finished")
		}
	})
	for range 31 {
		lease, err := latest.lease(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.release(nil)
	}
	limited, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	extra, err := latest.acquire(limited)
	if err == nil {
		extra.release(nil)
		t.Fatal("double retirement allowed 33 sessions for the same provider")
	}
}

func TestHTTPPoolBoundsLeasesReusesSessionsAndKeepsOtherUpstreamsAvailable(t *testing.T) {
	entered := make(chan struct{}, 40)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var connections atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "pool-bound", Version: "1"}, nil)
	remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "server/discover" {
				connections.Add(1)
			}
			return next(ctx, method, req)
		}
	})
	remote.AddTool(&mcp.Tool{Name: "wait", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ Block bool }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		if args.Block {
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return toolText("finished"), nil
	})
	g, id := catalogFixtureWithMode(t, remote, true)
	defer unblock()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	binding := poolTool(t, g, ctx, "remote__wait")
	results := make(chan *mcp.CallToolResult, 40)
	for range 40 {
		go func() { results <- g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{"block":true}`)) }()
	}
	for range 32 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("pool serialized independent calls or failed to lease sessions")
		}
	}
	select {
	case <-entered:
		t.Fatal("more than 32 calls entered one upstream")
	case <-time.After(100 * time.Millisecond):
	}
	// Updating credentials/config must not double a provider's concurrency while
	// requests using the previous configuration are still running.
	raw, err := OpenConfig(g.options.EncryptionKey, binding.row.Config)
	if err != nil {
		t.Fatal(err)
	}
	var config UpstreamConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	config.Headers = map[string]string{"X-Revision": "2"}
	raw, err = json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	next := binding
	next.row.Config, err = SealConfig(g.options.EncryptionKey, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.store.Pool.Exec(ctx, "UPDATE tools SET config=$1 WHERE id=$2", next.row.Config, id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	limited, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	if !g.execute(limited, store.Principal{}, next, json.RawMessage(`{}`)).IsError {
		stop()
		t.Fatal("configuration replacement bypassed the provider's 32-call limit")
	}
	stop()
	if _, err := g.store.Pool.Exec(ctx, "UPDATE tools SET config=$1 WHERE id=$2", binding.row.Config, id); err != nil {
		t.Fatal(err)
	}
	if _, err := g.store.Pool.Exec(ctx, "INSERT INTO tools(key,name,kind,config) SELECT 'healthy','Healthy','http',config FROM tools WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	healthy := poolTool(t, g, ctx, "healthy__wait")
	if g.execute(ctx, store.Principal{}, healthy, json.RawMessage(`{}`)).IsError {
		t.Fatal("saturated upstream hid another upstream")
	}
	unblock()
	for range 40 {
		select {
		case result := <-results:
			if result.IsError {
				t.Fatal("bounded concurrent call failed")
			}
		case <-ctx.Done():
			t.Fatal("queued calls did not finish")
		}
	}
	for range 3 {
		if g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{}`)).IsError {
			t.Fatal("idle session reuse failed")
		}
	}
	if connections.Load() != 33 {
		t.Fatalf("expected 32 leased sessions plus one healthy upstream, got %d handshakes", connections.Load())
	}
}
