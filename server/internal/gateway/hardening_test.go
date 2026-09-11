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
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func catalogFixture(t *testing.T, remote *mcp.Server) (*Gateway, int64) {
	t.Helper()
	return catalogFixtureWithMode(t, remote, false)
}

func catalogFixtureWithMode(t *testing.T, remote *mcp.Server, stateless bool) (*Gateway, int64) {
	t.Helper()
	s := gatewayDB(t)
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return remote }, &mcp.StreamableHTTPOptions{Stateless: stateless, JSONResponse: true, PropagateRequestCancellation: true}))
	t.Cleanup(upstream.Close)
	key := bytes.Repeat([]byte{5}, 32)
	g := New(s, Options{AllowPrivate: true, EncryptionKey: key})
	t.Cleanup(g.Close)
	raw, _ := json.Marshal(UpstreamConfig{URL: upstream.URL})
	config, err := SealConfig(key, raw)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := s.Pool.QueryRow(t.Context(), "INSERT INTO tools(key,name,kind,config) VALUES ('remote','Remote','http',$1) RETURNING id", config).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return g, id
}

func TestConcurrentCatalogRequestsBoundDiscoveryAndShareConnections(t *testing.T) {
	var active, maximum, discoveries atomic.Int64
	entered := make(chan struct{}, 12)
	release := make(chan struct{})
	remote := mcp.NewServer(&mcp.Implementation{Name: "bounded", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText("pong"), nil })
	remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "server/discover" {
				discoveries.Add(1)
				n := active.Add(1)
				defer active.Add(-1)
				for old := maximum.Load(); n > old; old = maximum.Load() {
					if maximum.CompareAndSwap(old, n) {
						break
					}
				}
				entered <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return next(ctx, method, req)
		}
	})
	g, id := catalogFixture(t, remote)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := g.store.Pool.Exec(ctx, "INSERT INTO tools(key,name,kind,config) SELECT 'remote_'||n,'Remote','http',config FROM tools CROSS JOIN generate_series(1,11) n WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 24)
	for range 24 {
		go func() {
			bindings, err := g.tools(ctx)
			if err == nil && len(bindings) != 17 {
				err = errors.New("catalog missing builtin or upstream tools")
			}
			completed <- err
		}()
	}
	for range 8 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("bounded discovery workers did not start")
		}
	}
	select {
	case <-entered:
		t.Fatal("discovery exceeded eight simultaneous upstream handshakes")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	for range 24 {
		if err := <-completed; err != nil {
			t.Fatal(err)
		}
	}
	if maximum.Load() > 8 || discoveries.Load() != 12 {
		t.Fatalf("discovery max concurrency=%d handshakes=%d, want <=8 and 12", maximum.Load(), discoveries.Load())
	}
}

func blockedUpstream(t *testing.T, blockedMethod string) (*mcp.Server, <-chan struct{}, func()) {
	t.Helper()
	remote := mcp.NewServer(&mcp.Implementation{Name: "blocked", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText("pong"), nil })
	entered := make(chan struct{})
	release := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == blockedMethod {
				enterOnce.Do(func() { close(entered) })
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return next(ctx, method, req)
		}
	})
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	return remote, entered, unblock
}

func TestCatalogWaiterCancellationDoesNotWaitForDiscovery(t *testing.T) {
	remote, entered, release := blockedUpstream(t, "tools/list")
	g, _ := catalogFixture(t, remote)
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	leader := make(chan error, 1)
	go func() { _, err := g.tools(ctx); leader <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("upstream discovery did not start")
	}
	waitCtx, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	waiter := make(chan error, 1)
	go func() { _, err := g.tools(waitCtx); waiter <- err }()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("canceled catalog waiter returned %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("canceled catalog waiter remained blocked by another request's discovery")
	}
	release()
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
}

func TestInvalidationDuringDiscoveryDoesNotRestoreDisabledTool(t *testing.T) {
	remote, entered, release := blockedUpstream(t, "tools/list")
	g, id := catalogFixture(t, remote)
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	leader := make(chan error, 1)
	go func() { _, err := g.tools(ctx); leader <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("upstream discovery did not start")
	}
	if _, err := g.store.Pool.Exec(ctx, "UPDATE tools SET enabled=false WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	g.Invalidate()
	release()
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bindings {
		if b.definition.Name == "remote__ping" {
			t.Fatal("inflight discovery restored a disabled tool after invalidation")
		}
	}
}

func TestCanceledCatalogLeaderDoesNotPoisonNextRequest(t *testing.T) {
	remote, entered, release := blockedUpstream(t, "tools/list")
	g, _ := catalogFixture(t, remote)
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	leaderCtx, cancelLeader := context.WithCancel(ctx)
	leader := make(chan error, 1)
	go func() { _, err := g.tools(leaderCtx); leader <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("upstream discovery did not start")
	}
	cancelLeader()
	select {
	case err := <-leader:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("canceled catalog leader returned %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("canceled catalog leader remained blocked")
	}
	release()
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bindings {
		if b.definition.Name == "remote__ping" {
			return
		}
	}
	t.Fatal("a canceled request cached an incomplete catalog for the next request")
}

func TestCloseDuringConnectRejectsAndReleasesLateSession(t *testing.T) {
	remote, entered, release := blockedUpstream(t, "server/discover")
	g, id := catalogFixture(t, remote)
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	row := toolRow{ID: id, Kind: "http"}
	if err := g.store.Pool.QueryRow(ctx, "SELECT config FROM tools WHERE id=$1", id).Scan(&row.Config); err != nil {
		t.Fatal(err)
	}
	connected := make(chan error, 1)
	go func() {
		session, err := g.upstream(ctx, row)
		if err == nil {
			session.release(nil)
		}
		connected <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("upstream connection did not start")
	}
	g.Close()
	release()
	select {
	case err := <-connected:
		if err == nil {
			t.Error("connection completed successfully after gateway closed")
		}
	case <-ctx.Done():
		t.Fatal("connection did not stop after gateway closed")
	}
	g.mu.Lock()
	retained := len(g.sessions) + len(g.httpPools)
	g.mu.Unlock()
	if retained != 0 {
		t.Errorf("closed gateway retained %d live upstream sessions", retained)
	}
	if _, err := g.upstream(ctx, row); err == nil {
		t.Error("closed gateway accepted a new upstream connection")
	}
	if _, err := g.tools(ctx); err == nil {
		t.Error("closed gateway continued serving a catalog")
	}
}

func TestConnectionWaiterCancellationDoesNotWaitForHandshake(t *testing.T) {
	remote, entered, release := blockedUpstream(t, "server/discover")
	g, id := catalogFixture(t, remote)
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	row := toolRow{ID: id, Kind: "http"}
	if err := g.store.Pool.QueryRow(ctx, "SELECT config FROM tools WHERE id=$1", id).Scan(&row.Config); err != nil {
		t.Fatal(err)
	}
	leader := make(chan error, 1)
	go func() {
		session, err := g.upstream(ctx, row)
		if err == nil {
			session.release(nil)
		}
		leader <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("upstream connection did not start")
	}
	waitCtx, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	waiter := make(chan error, 1)
	go func() { _, err := g.upstream(waitCtx, row); waiter <- err }()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("canceled connection waiter returned %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("canceled connection waiter remained blocked by another request's handshake")
	}
	release()
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
}

func TestCanceledLegacyHTTPCallDoesNotCloseAnotherCallOnSharedSession(t *testing.T) {
	testCanceledHTTPCall(t, true)
}

func TestCanceledModernHTTPCallStopsUpstreamAndPreservesSharedSession(t *testing.T) {
	testCanceledHTTPCall(t, false)
}

func testCanceledHTTPCall(t *testing.T, legacy bool) {
	t.Helper()
	entered := make(chan string, 2)
	firstStopped := make(chan struct{})
	release := make(chan struct{})
	remote := mcp.NewServer(&mcp.Implementation{Name: "concurrent-calls", Version: "1"}, nil)
	remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if legacy && method == "server/discover" {
				return nil, errors.New("legacy upstream uses initialize")
			}
			return next(ctx, method, req)
		}
	})
	remote.AddTool(&mcp.Tool{Name: "wait", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct{ Name string }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		entered <- args.Name
		select {
		case <-ctx.Done():
			if args.Name == "first" {
				close(firstStopped)
			}
			return nil, ctx.Err()
		case <-release:
			return toolText(args.Name), nil
		}
	})
	g, _ := catalogFixtureWithMode(t, remote, !legacy)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var binding toolBinding
	for _, b := range bindings {
		if b.definition.Name == "remote__wait" {
			binding = b
		}
	}
	if binding.definition == nil {
		t.Fatal("missing upstream wait tool")
	}
	firstCtx, cancelFirst := context.WithCancel(ctx)
	defer cancelFirst()
	first := make(chan *mcp.CallToolResult, 1)
	second := make(chan *mcp.CallToolResult, 1)
	go func() { first <- g.execute(firstCtx, store.Principal{}, binding, json.RawMessage(`{"name":"first"}`)) }()
	go func() { second <- g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{"name":"second"}`)) }()
	for range 2 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("both upstream calls did not start")
		}
	}
	cancelFirst()
	select {
	case result := <-first:
		if !result.IsError {
			t.Error("canceled call returned success")
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("canceled call waited for unrelated work on the shared session")
	}
	select {
	case <-firstStopped:
	case <-time.After(300 * time.Millisecond):
		t.Error("canceled request did not stop its upstream handler")
	}
	select {
	case result := <-second:
		t.Fatalf("unrelated in-flight call ended when another request canceled: isError=%v", result.IsError)
	case <-time.After(100 * time.Millisecond):
	}
	// Release both handlers only after checking isolation; leave deferred cleanup
	// to unblock them if an earlier assertion fails.
	unblock()
	select {
	case result := <-second:
		if result.IsError {
			encoded, _ := json.Marshal(result)
			t.Errorf("unrelated call failed after the other request was canceled: %s", encoded)
		}
	case <-ctx.Done():
		t.Fatal("unrelated call did not finish")
	}
	result := g.execute(ctx, store.Principal{}, binding, json.RawMessage(`{"name":"third"}`))
	if result.IsError {
		t.Error("subsequent call failed after cancellation")
	}
}

func TestCatalogIncludesEveryUpstreamPage(t *testing.T) {
	remote := mcp.NewServer(&mcp.Implementation{Name: "paged", Version: "1"}, &mcp.ServerOptions{PageSize: 1})
	for _, name := range []string{"first", "second", "third"} {
		remote.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText("ok"), nil })
	}
	g, _ := catalogFixture(t, remote)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	bindings, err := g.tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	for _, binding := range bindings {
		names[binding.definition.Name] = true
	}
	for _, name := range []string{"remote__first", "remote__second", "remote__third"} {
		if !names[name] {
			t.Errorf("catalog omitted paginated tool %s", name)
		}
	}
}
