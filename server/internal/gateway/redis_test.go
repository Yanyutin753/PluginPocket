package gateway

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/cache"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redis/go-redis/v9"
)

func redisGatewayFixture(t *testing.T) (*Gateway, *Gateway, *mcp.Server, *atomic.Int64, *redis.Client, string) {
	t.Helper()
	raw := os.Getenv("LOADOUT_TEST_REDIS_URL")
	if raw == "" {
		t.Skip("real Redis URL required")
	}
	namespace := "gateway_" + rand.Text()
	var lists atomic.Int64
	remote := mcp.NewServer(&mcp.Implementation{Name: "redis-metadata", Version: "1"}, nil)
	remote.AddTool(&mcp.Tool{Name: "ping", Description: "Remote ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText("pong"), nil })
	remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "tools/list" {
				lists.Add(1)
			}
			return next(ctx, method, req)
		}
	})
	template, _ := catalogFixtureWithMode(t, remote, true)
	var gateways []*Gateway
	for range 2 {
		client, err := cache.Open(raw, namespace)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if err := client.Ping(t.Context()); err != nil {
			t.Fatal(err)
		}
		options := template.options
		options.Cache = client
		g := New(template.store, options)
		t.Cleanup(g.Close)
		gateways = append(gateways, g)
	}
	options, err := redis.ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	inspector := redis.NewClient(options)
	t.Cleanup(func() { _ = inspector.Close() })
	return gateways[0], gateways[1], remote, &lists, inspector, namespace
}

func TestRedisSharesHTTPMetadataAcrossGateways(t *testing.T) {
	a, b, _, lists, inspector, namespace := redisGatewayFixture(t)
	poolTool(t, a, t.Context(), "remote__ping")
	binding := poolTool(t, b, t.Context(), "remote__ping")
	if lists.Load() != 1 {
		t.Fatalf("two replicas performed %d upstream discoveries; want one shared metadata fetch", lists.Load())
	}
	if got := b.execute(t.Context(), store.Principal{}, binding, json.RawMessage(`{}`)); got.IsError {
		t.Fatal("cached metadata did not preserve callable remote binding")
	}
	keys, err := inspector.Keys(t.Context(), namespace+":data:*").Result()
	if err != nil || len(keys) != 1 {
		t.Fatalf("expected one metadata cache key, got %d: %v", len(keys), err)
	}
	ttl, err := inspector.PTTL(t.Context(), keys[0]).Result()
	if err != nil || ttl <= 0 || ttl > 5*time.Second {
		t.Fatalf("metadata TTL=%s error=%v", ttl, err)
	}
	value, err := inspector.Get(t.Context(), keys[0]).Bytes()
	var definitions []map[string]any
	if err != nil || json.Unmarshal(value, &definitions) != nil || len(definitions) != 1 || definitions[0]["name"] != "ping" {
		t.Fatal("Redis did not store only original remote tool definitions")
	}
	if _, ok := definitions[0]["config"]; ok {
		t.Fatal("upstream configuration leaked into metadata cache")
	}
}

func waitGatewayRedis(t *testing.T, description string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(description)
}

func TestRedisInvalidationRefreshesPeerAndClosesSubscription(t *testing.T) {
	a, b, _, _, inspector, namespace := redisGatewayFixture(t)
	subscribers := func() int64 {
		counts, err := inspector.PubSubNumSub(t.Context(), namespace+":invalidate").Result()
		if err != nil {
			t.Fatal(err)
		}
		return counts[namespace+":invalidate"]
	}
	waitGatewayRedis(t, "gateways did not subscribe to shared invalidation", func() bool { return subscribers() == 2 })
	poolTool(t, a, t.Context(), "remote__ping")
	poolTool(t, b, t.Context(), "remote__ping")
	a.mu.Lock()
	before := a.catalogVersion
	a.mu.Unlock()
	a.Invalidate()
	waitGatewayRedis(t, "peer kept its local catalog after published invalidation", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.catalogUntil.IsZero()
	})
	// The origin receives the same broadcast; it must not publish/refresh again.
	time.Sleep(30 * time.Millisecond)
	a.mu.Lock()
	after := a.catalogVersion
	a.mu.Unlock()
	if after != before+1 {
		t.Fatalf("origin reapplied its own invalidation: version delta %d", after-before)
	}
	closed := make(chan struct{})
	go func() { b.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Close waited on Redis subscription")
	}
	waitGatewayRedis(t, "closed gateway left a Redis subscription", func() bool { return subscribers() == 1 })
	if err := b.options.Cache.Ping(t.Context()); err != nil {
		t.Fatal("Gateway.Close closed externally owned cache client")
	}
}

func TestRedisMetadataRevisionAndLocalSecurityIsolation(t *testing.T) {
	a, b, remote, lists, _, _ := redisGatewayFixture(t)
	poolTool(t, a, t.Context(), "remote__ping")
	for _, mode := range []string{"private-network-denied", "wrong-decryption-key"} {
		t.Run(mode, func(t *testing.T) {
			options := b.options
			if mode == "private-network-denied" {
				options.AllowPrivate = false
			} else {
				options.EncryptionKey = make([]byte, 32)
			}
			restricted := New(b.store, options)
			defer restricted.Close()
			bindings, err := restricted.tools(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			for _, binding := range bindings {
				if binding.row.Kind == "http" {
					t.Fatal("shared metadata bypassed local upstream security")
				}
			}
		})
	}
	remote.AddTool(&mcp.Tool{Name: "new", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { return toolText("new"), nil })
	if _, err := a.store.Pool.Exec(t.Context(), "UPDATE tools SET cost=cost+1 WHERE key='remote'"); err != nil {
		t.Fatal(err)
	}
	// No local invalidation or Pub/Sub is required to observe a committed revision.
	poolTool(t, b, t.Context(), "remote__new")
	if lists.Load() != 2 {
		t.Fatalf("new database revision reused stale shared metadata: discoveries=%d", lists.Load())
	}
}

func TestRedisMetadataFailureFallsBackToUpstream(t *testing.T) {
	for _, mode := range []string{"disabled", "unavailable", "closed", "malformed", "null-tool", "invalid-schema", "invalid-header"} {
		t.Run(mode, func(t *testing.T) {
			a, b, _, lists, inspector, namespace := redisGatewayFixture(t)
			poolTool(t, a, t.Context(), "remote__ping")
			options := b.options
			switch mode {
			case "disabled":
				options.Cache = nil
			case "unavailable":
				client, err := cache.Open("redis://127.0.0.1:1/0", namespace)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = client.Close() })
				options.Cache = client
			case "closed":
				_ = options.Cache.Close()
			default:
				keys, err := inspector.Keys(t.Context(), namespace+":data:*").Result()
				if err != nil || len(keys) != 1 {
					t.Fatal("missing initial metadata")
				}
				value := "invalid-json"
				if mode == "null-tool" {
					value = "[null]"
				}
				if mode == "invalid-schema" {
					value = `[{"name":"ping","inputSchema":{"type":"string"}}]`
				}
				if mode == "invalid-header" {
					value = `[{"name":"ping","inputSchema":{"type":"object","properties":{"id":{"type":"string","x-mcp-header":42}}}}]`
				}
				if err := inspector.Set(t.Context(), keys[0], value, 5*time.Second).Err(); err != nil {
					t.Fatal(err)
				}
			}
			fallback := New(a.store, options)
			defer fallback.Close()
			started := time.Now()
			poolTool(t, fallback, t.Context(), "remote__ping")
			if time.Since(started) > time.Second {
				t.Fatal("cache failure consumed upstream discovery budget")
			}
			if lists.Load() != 2 {
				t.Fatalf("cache failure did not discover directly: %d fetches", lists.Load())
			}
		})
	}
}

func TestRedisDoesNotShareMachineLocalStdioMetadata(t *testing.T) {
	a, b, _, _, inspector, namespace := redisGatewayFixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(UpstreamConfig{Command: "fixture", Args: []string{"-test.run=^TestStdioUpstreamHelper$"}, Env: map[string]string{"LOADOUT_STDIO_TEST_HELPER": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	config, err := SealConfig(a.options.EncryptionKey, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.store.Pool.Exec(t.Context(), "UPDATE tools SET kind='stdio',config=$1 WHERE key='remote'", config); err != nil {
		t.Fatal(err)
	}
	// Replica A has this command; replica B deliberately has no local allowlist.
	options := a.options
	options.StdioCommands = map[string]string{"fixture": executable}
	allowed := New(a.store, options)
	defer allowed.Close()
	poolTool(t, allowed, t.Context(), "remote__ping")
	bindings, err := b.tools(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range bindings {
		if binding.row.Kind == "stdio" {
			t.Fatal("stdio metadata crossed the machine allowlist boundary")
		}
	}
	keys, err := inspector.Keys(t.Context(), namespace+":data:*").Result()
	if err != nil || len(keys) != 0 {
		t.Fatalf("stdio metadata entered shared Redis: %d keys, %v", len(keys), err)
	}
}

func TestInvalidUpstreamMetadataDoesNotHideHealthyTools(t *testing.T) {
	for _, schema := range []string{`{"type":"string"}`, `{"type":"object","properties":{"id":{"type":"string","x-mcp-header":42}}}`} {
		t.Run(schema, func(t *testing.T) {
			remote := mcp.NewServer(&mcp.Implementation{Name: "invalid-metadata", Version: "1"}, nil)
			remote.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == "tools/list" {
						return &mcp.ListToolsResult{Tools: []*mcp.Tool{{Name: "broken", InputSchema: json.RawMessage(schema)}}}, nil
					}
					return next(ctx, method, req)
				}
			})
			g, _ := catalogFixtureWithMode(t, remote, true)
			bindings, err := g.tools(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			for _, binding := range bindings {
				if binding.row.Kind != "builtin" {
					t.Fatal("SDK-invalid upstream definition entered the serving catalog")
				}
			}
			names := map[string]bool{}
			for _, binding := range bindings {
				names[binding.definition.Name] = true
			}
			if !names["echo"] || !names["time_now"] {
				t.Fatal("invalid upstream hid healthy builtin tools")
			}
		})
	}
}
