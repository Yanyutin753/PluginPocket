package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type storeHandle struct {
	store  *store.Store
	wallet int64
}

func accountToolsSession(t *testing.T, balance int64) (*storeHandle, *mcp.ClientSession, context.Context) {
	t.Helper()
	s := gatewayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	var user, wallet int64
	if err := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES ('account_tools','hash') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES ($1,$2) RETURNING id", user, balance).Scan(&wallet); err != nil {
		t.Fatal(err)
	}
	raw := "ldt_account_tools_token"
	sum := sha256.Sum256([]byte(raw))
	if _, err := s.Pool.Exec(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES ($1,$2,'test','ldt_test',$3)", user, wallet, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	g := New(s, Options{})
	t.Cleanup(g.Close)
	server := httptest.NewServer(g)
	t.Cleanup(server.Close)
	hc, err := upstreamClient(server.URL, map[string]string{"Authorization": "Bearer " + raw}, true)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: hc, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return &storeHandle{store: s, wallet: wallet}, session, ctx
}

func callToolText(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("%s returned tool error: %+v", name, result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("%s returned %d content blocks", name, len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s returned non-text content", name)
	}
	return text.Text
}

func TestAccountBuiltinTools(t *testing.T) {
	h, session, ctx := accountToolsSession(t, 7)
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, tool := range list.Tools {
		present[tool.Name] = true
	}
	for _, name := range []string{"account_balance", "account_usage", "tools_catalog"} {
		if !present[name] {
			t.Fatalf("catalog missing %s; have %v", name, present)
		}
	}
	var balance struct {
		Username string `json:"username"`
		Balance  int64  `json:"balance"`
	}
	if err := json.Unmarshal([]byte(callToolText(t, ctx, session, "account_balance", map[string]any{})), &balance); err != nil {
		t.Fatal(err)
	}
	if balance.Username != "account_tools" || balance.Balance != 7 {
		t.Fatalf("account_balance = %+v", balance)
	}
	var usage struct {
		Summary struct {
			TodayCalls int64 `json:"today_calls"`
			MonthCost  int64 `json:"month_cost"`
		} `json:"summary"`
		Recent []struct {
			Tool   string `json:"tool"`
			Cost   int64  `json:"cost"`
			Status string `json:"status"`
		} `json:"recent"`
	}
	if err := json.Unmarshal([]byte(callToolText(t, ctx, session, "account_usage", map[string]any{"limit": 5})), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.Summary.TodayCalls != 1 || usage.Summary.MonthCost != 0 {
		t.Fatalf("usage summary = %+v", usage.Summary)
	}
	if len(usage.Recent) != 1 || usage.Recent[0].Tool != "account_balance" || usage.Recent[0].Cost != 0 || usage.Recent[0].Status != "ok" {
		t.Fatalf("usage recent = %+v", usage.Recent)
	}
	var catalog []struct {
		Key         string `json:"key"`
		CostPerCall int64  `json:"cost_per_call"`
	}
	if err := json.Unmarshal([]byte(callToolText(t, ctx, session, "tools_catalog", map[string]any{})), &catalog); err != nil {
		t.Fatal(err)
	}
	entries := map[string]int64{}
	for _, entry := range catalog {
		entries[entry.Key] = entry.CostPerCall
	}
	if entries["account_balance"] != 0 || entries["echo"] != 1 {
		t.Fatalf("catalog = %+v", entries)
	}
	var final int64
	if err := h.store.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", h.wallet).Scan(&final); err != nil {
		t.Fatal(err)
	}
	if final != 7 {
		t.Fatalf("account tools must not cost credits; balance=%d", final)
	}
	if result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "account_usage", Arguments: map[string]any{"limit": 0}}); err != nil || !result.IsError {
		t.Fatalf("invalid limit must return a tool error: err=%v isError=%v", err, result != nil && result.IsError)
	}
}
