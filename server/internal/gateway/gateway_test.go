package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayRequiresToken(t *testing.T) {
	g := New(nil, Options{})
	response := httptest.NewRecorder()
	g.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if response.Code != 401 {
		t.Fatalf("unauthenticated gateway returned %d", response.Code)
	}
	if body := response.Body.String(); body != "{\"error\":\"unauthorized\"}\n" {
		t.Fatalf("unauthenticated gateway body %q is not the JSON error envelope", body)
	}
	if ct := response.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unauthenticated gateway Content-Type %q is not JSON", ct)
	}
}

func gatewayDB(t *testing.T) *store.Store {
	t.Helper()
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("real PostgreSQL URL required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("gateway_%d", time.Now().UnixNano())
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = conn.Close(ctx)
	})
	return s
}

// hostProxyTransport rewrites every request's Host header, simulating a
// same-host reverse proxy that forwards to the loopback listener while
// preserving the public domain clients actually dial.
type hostProxyTransport struct {
	http.RoundTripper
	host string
}

func (t hostProxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	cloned := r.Clone(r.Context())
	cloned.Host = t.host
	return t.RoundTripper.RoundTrip(cloned)
}

func TestGatewayServesPublicHostBehindLoopbackListener(t *testing.T) {
	s := gatewayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var user, wallet int64
	if err := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES ('proxyhost','hash') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES ($1,5) RETURNING id", user).Scan(&wallet); err != nil {
		t.Fatal(err)
	}
	raw := "ppt_loopback_proxy_host"
	sum := sha256.Sum256([]byte(raw))
	if _, err := s.Pool.Exec(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES ($1,$2,'test','ppt_test',$3)", user, wallet, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	g := New(s, Options{})
	defer g.Close()
	server := httptest.NewServer(g)
	defer server.Close()
	hc, err := upstreamClient(server.URL, map[string]string{"Authorization": "Bearer " + raw}, true)
	if err != nil {
		t.Fatal(err)
	}
	hc.Transport = hostProxyTransport{RoundTripper: hc.Transport, host: "pluginpocket.example.com"}
	client := mcp.NewClient(&mcp.Implementation{Name: "proxyhost", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: hc, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatalf("public Host behind loopback listener rejected connection: %v", err)
	}
	defer func() { _ = session.Close() }()
	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatalf("tools/list with public Host behind loopback listener failed: %v", err)
	}
}

func TestOfficialMCPClientAndLedger(t *testing.T) {
	s := gatewayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var user, wallet, token int64
	if err := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES ('mcpuser','hash') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES ($1,2) RETURNING id", user).Scan(&wallet); err != nil {
		t.Fatal(err)
	}
	raw := "ppt_gateway_test_token"
	sum := sha256.Sum256([]byte(raw))
	if err := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES ($1,$2,'test','ppt_test',$3) RETURNING id", user, wallet, hex.EncodeToString(sum[:])).Scan(&token); err != nil {
		t.Fatal(err)
	}
	g := New(s, Options{})
	defer g.Close()
	server := httptest.NewServer(g)
	defer server.Close()
	hc, err := upstreamClient(server.URL, map[string]string{"Authorization": "Bearer " + raw}, true)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: hc, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) < 2 {
		t.Fatal("missing builtin tools")
	}
	for i := 0; i < 3; i++ {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"message": "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != (i == 2) {
			t.Fatalf("call %d isError=%v", i, result.IsError)
		}
	}
	var balance, count, total int64
	if err := s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", wallet).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "SELECT count(*),sum(cost) FROM usage_logs WHERE user_id=$1", user).Scan(&count, &total); err != nil {
		t.Fatal(err)
	}
	if balance != 0 || count != 3 || total != 2 {
		t.Fatalf("balance=%d logs=%d cost=%d", balance, count, total)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE tokens SET revoked_at=now() WHERE id=$1", token); err != nil {
		t.Fatal(err)
	}
	if _, err = session.ListTools(ctx, nil); err == nil {
		t.Fatal("revoked token remained authorized")
	}
}
