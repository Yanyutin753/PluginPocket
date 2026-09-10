package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func reviewPrincipal(t *testing.T, s *store.Store) store.Principal {
	t.Helper()
	ctx := t.Context()
	var p store.Principal
	if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES('reviewer','hash') RETURNING id").Scan(&p.UserID); e != nil {
		t.Fatal(e)
	}
	if e := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES($1,10) RETURNING id", p.UserID).Scan(&p.WalletID); e != nil {
		t.Fatal(e)
	}
	if e := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES($1,$2,'review','ldt_',$3) RETURNING id", p.UserID, p.WalletID, auth.Digest("ldt_review")).Scan(&p.TokenID); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestReviewDisableDuringWalletWait(t *testing.T) {
	s := gatewayDB(t)
	p := reviewPrincipal(t, s)
	g := New(s, Options{})
	defer g.Close()
	ctx := t.Context()
	bindings, e := g.tools(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var b toolBinding
	for _, v := range bindings {
		if v.row.Key == "echo" {
			b = v
		}
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM wallets WHERE id=$1 FOR UPDATE", p.WalletID).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan *mcp.CallToolResult, 1)
	go func() { done <- g.call(ctx, p, b, json.RawMessage(`{"message":"executed after disable"}`)) }()
	waited := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if e = s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&waited); e != nil {
			t.Fatal(e)
		}
		if waited {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waited {
		t.Fatal("did not block on wallet")
	}
	if _, e = s.Pool.Exec(ctx, "UPDATE tools SET enabled=false WHERE id=$1", b.row.ID); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	result := <-done
	var balance int64
	if e = s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", p.WalletID).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	if !result.IsError || balance != 10 {
		t.Fatalf("disabled tool executed and charged after wallet wait: balance=%d result=%+v", balance, result)
	}
}
func TestReviewMalformedBuiltinSchema(t *testing.T) {
	s := gatewayDB(t)
	reviewPrincipal(t, s)
	g := New(s, Options{})
	defer g.Close()
	if _, e := s.Pool.Exec(t.Context(), `UPDATE tools SET input_schema='{"type":"object","properties":{"message":{"type":"object","x-mcp-header":"bad"}}}' WHERE key='echo'`); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("authenticated MCP request panics from API-accepted builtin schema: %v", r)
		}
	}()
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"review","version":"1"}}}`))
	req.Header.Set("Authorization", "Bearer ldt_review")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthy catalog unavailable: %d", w.Code)
	}
}

func TestGatewayAuthenticationDatabaseFailureIsRetryable(t *testing.T) {
	s := gatewayDB(t)
	reviewPrincipal(t, s)
	g := New(s, Options{})
	defer g.Close()
	tx, err := s.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(t.Context(), "LOCK TABLE tokens IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer ldt_review")
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("database timeout returned %d, want 503", w.Code)
	}
	if strings.Contains(w.Body.String(), "tokens") {
		t.Fatal("database details exposed")
	}
	if err = tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer ldt_review")
	w = httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("valid token did not recover: %d", w.Code)
	}
}

func TestPriceChangeDuringWalletWait(t *testing.T) {
	s := gatewayDB(t)
	p := reviewPrincipal(t, s)
	g := New(s, Options{})
	defer g.Close()
	ctx := t.Context()
	bindings, e := g.tools(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var b toolBinding
	for _, v := range bindings {
		if v.row.Key == "echo" {
			b = v
		}
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM wallets WHERE id=$1 FOR UPDATE", p.WalletID).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan *mcp.CallToolResult, 1)
	go func() { done <- g.call(ctx, p, b, json.RawMessage(`{"message":"executed after disable"}`)) }()
	waited := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if e = s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&waited); e != nil {
			t.Fatal(e)
		}
		if waited {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waited {
		t.Fatal("did not block on wallet")
	}
	if _, e = s.Pool.Exec(ctx, "UPDATE tools SET cost=4 WHERE id=$1", b.row.ID); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	result := <-done
	var balance int64
	if e = s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", p.WalletID).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	var charged, reserved int64
	if e = s.Pool.QueryRow(ctx, "SELECT cost FROM usage_logs WHERE user_id=$1 AND status='ok'", p.UserID).Scan(&charged); e != nil {
		t.Fatal(e)
	}
	if e = s.Pool.QueryRow(ctx, "SELECT delta FROM ledger WHERE wallet_id=$1 AND kind='reservation'", p.WalletID).Scan(&reserved); e != nil {
		t.Fatal(e)
	}
	if charged != 4 || reserved != -4 {
		t.Fatalf("usage cost=%d reservation=%d", charged, reserved)
	}
	if result.IsError || balance != 6 {
		t.Fatalf("tool must use the price committed before wallet acquisition: balance=%d result=%+v", balance, result)
	}
}

func TestTokenRevokedDuringToolLockWait(t *testing.T) {
	s := gatewayDB(t)
	p := reviewPrincipal(t, s)
	g := New(s, Options{})
	defer g.Close()
	ctx := t.Context()
	bindings, e := g.tools(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var b toolBinding
	for _, v := range bindings {
		if v.row.Key == "echo" {
			b = v
		}
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM tools WHERE id=$1 FOR UPDATE", b.row.ID).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan *mcp.CallToolResult, 1)
	go func() { done <- g.call(ctx, p, b, json.RawMessage(`{"message":"executed after disable"}`)) }()
	waited := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if e = s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&waited); e != nil {
			t.Fatal(e)
		}
		if waited {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waited {
		t.Fatal("did not block on tool")
	}
	if _, e = s.Pool.Exec(ctx, "UPDATE tokens SET revoked_at=now() WHERE id=$1", p.TokenID); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	result := <-done
	var balance int64
	if e = s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", p.WalletID).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	if !result.IsError || balance != 10 {
		t.Fatalf("revoked token executed and charged after tool wait: balance=%d result=%+v", balance, result)
	}
}

func TestAuthorizationChangedDuringReservationForeignKeyWait(t *testing.T) {
	for _, target := range []string{"user", "token"} {
		t.Run(target, func(t *testing.T) {
			s := gatewayDB(t)
			p := reviewPrincipal(t, s)
			g := New(s, Options{})
			defer g.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			bindings, err := g.tools(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var binding toolBinding
			for _, candidate := range bindings {
				if candidate.row.Key == "echo" {
					binding = candidate
				}
			}
			tx, err := s.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			lockSQL := "SELECT pg_backend_pid() FROM users WHERE id=$1 FOR UPDATE"
			updateSQL := "UPDATE users SET enabled=false WHERE id=$1"
			id := p.UserID
			if target == "token" {
				lockSQL = "SELECT pg_backend_pid() FROM tokens WHERE id=$1 FOR UPDATE"
				updateSQL = "UPDATE tokens SET revoked_at=now() WHERE id=$1"
				id = p.TokenID
			}
			var pid int
			if err = tx.QueryRow(ctx, lockSQL, id).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			done := make(chan *mcp.CallToolResult, 1)
			go func() { done <- g.call(ctx, p, binding, json.RawMessage(`{"message":"must not execute"}`)) }()
			for {
				var waiting bool
				if err = s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE 'INSERT INTO usage_logs%')", pid).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("reservation did not reach its foreign-key lock wait")
				case <-time.After(10 * time.Millisecond):
				}
			}
			if _, err = tx.Exec(ctx, updateSQL, id); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			result := <-done
			var balance, usage, ledger int64
			if err = s.Pool.QueryRow(ctx, "SELECT balance,(SELECT count(*) FROM usage_logs WHERE user_id=$2),(SELECT count(*) FROM ledger WHERE wallet_id=$1) FROM wallets WHERE id=$1", p.WalletID, p.UserID).Scan(&balance, &usage, &ledger); err != nil {
				t.Fatal(err)
			}
			if !result.IsError || balance != 10 || usage != 0 || ledger != 0 {
				t.Fatalf("authorization change during FK wait must roll back before execution: error=%v balance=%d usage=%d ledger=%d", result.IsError, balance, usage, ledger)
			}
		})
	}
}
