package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
)

func sqliteTestStore(t testing.TB) *SQLiteStore {
	t.Helper()
	s, err := OpenSQLite(context.Background(), t.TempDir()+"/ledger.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sqliteFixture(t testing.TB, s *SQLiteStore) (int64, int64, int64) {
	t.Helper()
	ctx := context.Background()
	var u, w, tok int64
	if err := s.DB.QueryRowContext(ctx, "INSERT INTO users(username,password_hash) VALUES ('tester','hash') RETURNING id").Scan(&u); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRowContext(ctx, "INSERT INTO wallets(user_id,balance) VALUES (?,10) RETURNING id", u).Scan(&w); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRowContext(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES (?,?,'test','ppt_x','hash') RETURNING id", u, w).Scan(&tok); err != nil {
		t.Fatal(err)
	}
	return u, w, tok
}

func TestSQLiteMigrationsIdempotentReopen(t *testing.T) {
	path := t.TempDir() + "/ledger.db"
	s, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	_, w, _ := sqliteFixture(t, s)
	ctx := context.Background()
	if _, err = s.DB.ExecContext(ctx, "UPDATE wallets SET balance=7 WHERE id=?", w); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	var balance int64
	if err = reopened.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 7 {
		t.Fatalf("balance after reopen=%d", balance)
	}
	entries, err := migrationsSQLite.ReadDir("migrations_sqlite")
	if err != nil {
		t.Fatal(err)
	}
	var applied int
	if err = reopened.DB.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != len(entries) {
		t.Fatalf("applied=%d want %d", applied, len(entries))
	}
}

func TestSQLiteConcurrentReservationsNeverOverdrawAndRefundOnce(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	var successes atomic.Int64
	var wg sync.WaitGroup
	ids := make(chan int64, 40)
	for i := range 40 {
		wg.Go(func() {
			call, err := s.Reserve(ctx, u, tok, w, "echo", 1, fmt.Sprint(i))
			if err == nil {
				successes.Add(1)
				ids <- call.ID
			} else if err != ErrInsufficientBalance {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	close(ids)
	if got := successes.Load(); got != 10 {
		t.Fatalf("want 10 admitted calls, got %d", got)
	}
	var balance int64
	if err := s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 0 {
		t.Fatalf("balance=%d", balance)
	}
	id := <-ids
	for range 2 {
		if err := s.Finish(ctx, id, false, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 1 {
		t.Fatalf("refund must apply once: %d", balance)
	}
	var delta int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(sum(delta),0) FROM ledger WHERE wallet_id=?", w).Scan(&delta); err != nil {
		t.Fatal(err)
	}
	if delta != -9 {
		t.Fatalf("ledger=%d", delta)
	}
}

func TestSQLiteReservationReplayRecoveryAndOwnership(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	first, e := s.Reserve(ctx, u, tok, w, "echo", 3, "same")
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Reserve(ctx, u, tok, w, "echo", 3, "same")
	if e != nil || again.ID != first.ID {
		t.Fatalf("duplicate reserve: %#v %v", again, e)
	}
	if _, e = s.Reserve(ctx, u, tok, w, "echo", 4, "same"); e != ErrConflict {
		t.Fatalf("different payload replay: %v", e)
	}
	if _, e = s.Reserve(ctx, u+1, tok, w, "echo", 1, "other"); e != ErrUnauthorized {
		t.Fatalf("wallet ownership bypass: %v", e)
	}
	n, e := s.RecoverPending(ctx, time.Now().Add(time.Minute))
	if e != nil || n != 1 {
		t.Fatalf("recover=%d %v", n, e)
	}
	n, e = s.RecoverPending(ctx, time.Now().Add(time.Minute))
	if e != nil || n != 0 {
		t.Fatalf("recover replay=%d %v", n, e)
	}
	if e = s.Finish(ctx, first.ID, true, time.Second); e != nil {
		t.Fatal(e)
	}
	var balance int64
	_ = s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance)
	if balance != 10 {
		t.Fatalf("recovered balance=%d", balance)
	}
}

func TestSQLiteOnlySuccessfulCallsReportChargedCost(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	failed, e := s.Reserve(ctx, u, tok, w, "echo", 2, "failed")
	if e != nil {
		t.Fatal(e)
	}
	if failed.Cost != 0 {
		t.Fatalf("pending cost must be zero: %d", failed.Cost)
	}
	if e = s.Finish(ctx, failed.ID, false, time.Second); e != nil {
		t.Fatal(e)
	}
	denied, e := s.Reserve(ctx, u, tok, w, "echo", 11, "denied")
	if e != ErrInsufficientBalance || denied.Cost != 0 {
		t.Fatalf("denied cost: %#v %v", denied, e)
	}
	ok, e := s.Reserve(ctx, u, tok, w, "echo", 2, "ok")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, ok.ID, true, time.Second); e != nil {
		t.Fatal(e)
	}
	var cost int64
	_ = s.DB.QueryRowContext(ctx, "SELECT cost FROM usage_logs WHERE id=?", ok.ID).Scan(&cost)
	if cost != 2 {
		t.Fatalf("successful charged cost: %d", cost)
	}
	_ = s.DB.QueryRowContext(ctx, "SELECT cost FROM usage_logs WHERE id=?", failed.ID).Scan(&cost)
	if cost != 0 {
		t.Fatalf("failed charged cost: %d", cost)
	}
}

func TestSQLiteReservationAndRefundLedgerCapturePostBalance(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	call, e := s.Reserve(ctx, u, tok, w, "echo", 3, "balances")
	if e != nil {
		t.Fatal(e)
	}
	var balance *int64
	e = s.DB.QueryRowContext(ctx, "SELECT balance_after FROM ledger WHERE call_id=? AND kind='reservation'", call.ID).Scan(&balance)
	if e != nil || balance == nil || *balance != 7 {
		t.Fatalf("reservation balance_after=%v err=%v", balance, e)
	}
	if e = s.Finish(ctx, call.ID, false, 0); e != nil {
		t.Fatal(e)
	}
	e = s.DB.QueryRowContext(ctx, "SELECT balance_after FROM ledger WHERE call_id=? AND kind='refund'", call.ID).Scan(&balance)
	if e != nil || balance == nil || *balance != 10 {
		t.Fatalf("refund balance_after=%v err=%v", balance, e)
	}
}

func TestSQLiteTeamWalletMembershipAuthorizesReserve(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	var team int64
	if e := s.DB.QueryRowContext(ctx, "INSERT INTO teams(name) VALUES('Shared') RETURNING id").Scan(&team); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.ExecContext(ctx, "UPDATE wallets SET user_id=NULL,team_id=? WHERE id=?", team, w); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.ExecContext(ctx, "INSERT INTO team_members(team_id,user_id,role) VALUES(?,?,'member')", team, u); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Reserve(ctx, u, tok, w, "echo", 1, "team-reserve"); e != nil {
		t.Fatalf("team member reserve: %v", e)
	}
	if _, e := s.DB.ExecContext(ctx, "DELETE FROM team_members WHERE team_id=? AND user_id=?", team, u); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Reserve(ctx, u, tok, w, "echo", 1, "team-revoked"); e != ErrUnauthorized {
		t.Fatalf("membership removal must revoke reserve: %v", e)
	}
}

func TestSQLiteReserveToolUsesToolPricing(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	var toolID int64
	if e := s.DB.QueryRowContext(ctx, "INSERT INTO tools(key,name,kind,cost) VALUES('pricey','Pricey','http',5) RETURNING id").Scan(&toolID); e != nil {
		t.Fatal(e)
	}
	call, e := s.ReserveTool(ctx, u, tok, w, toolID, "pricey", "tool-key")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, call.ID, true, time.Second); e != nil {
		t.Fatal(e)
	}
	var balance int64
	_ = s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance)
	if balance != 5 {
		t.Fatalf("tool pricing balance=%d", balance)
	}
	if _, e = s.ReserveTool(ctx, u, tok, w, 0, "pricey", "zero"); e != ErrNotFound {
		t.Fatalf("missing tool id: %v", e)
	}
	if _, e = s.DB.ExecContext(ctx, "UPDATE tools SET enabled=0 WHERE id=?", toolID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReserveTool(ctx, u, tok, w, toolID, "pricey", "disabled"); e != ErrNotFound {
		t.Fatalf("disabled tool: %v", e)
	}
}

func TestSQLiteAuthTokenPrincipalAndRevocation(t *testing.T) {
	s := sqliteTestStore(t)
	ctx := context.Background()
	var u, w, tok int64
	if e := s.DB.QueryRowContext(ctx, "INSERT INTO users(username,password_hash) VALUES ('authusr','hash') RETURNING id").Scan(&u); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.QueryRowContext(ctx, "INSERT INTO wallets(user_id,balance) VALUES (?,3) RETURNING id", u).Scan(&w); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.QueryRowContext(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES (?,?,'auth','ppt_s',?) RETURNING id", u, w, auth.Digest("ppt_secret")).Scan(&tok); e != nil {
		t.Fatal(e)
	}
	p, e := s.AuthToken(ctx, "ppt_secret")
	if e != nil {
		t.Fatal(e)
	}
	if p.UserID != u || p.TokenID != tok || p.WalletID != w || p.Username != "authusr" || p.Role != "user" {
		t.Fatalf("principal=%#v", p)
	}
	var lastUsed *string
	if e = s.DB.QueryRowContext(ctx, "SELECT last_used_at FROM tokens WHERE id=?", tok).Scan(&lastUsed); e != nil || lastUsed == nil {
		t.Fatalf("last_used_at not recorded: %v %v", lastUsed, e)
	}
	if _, e = s.DB.ExecContext(ctx, "UPDATE tokens SET revoked_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?", tok); e != nil {
		t.Fatal(e)
	}
	if _, e = s.AuthToken(ctx, "ppt_secret"); e != ErrUnauthorized {
		t.Fatalf("revoked token: %v", e)
	}
	var tok2 int64
	if e = s.DB.QueryRowContext(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES (?,?,'auth2','ppt_t',?) RETURNING id", u, w, auth.Digest("ppt_second")).Scan(&tok2); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.ExecContext(ctx, "UPDATE users SET enabled=0 WHERE id=?", u); e != nil {
		t.Fatal(e)
	}
	if _, e = s.AuthToken(ctx, "ppt_second"); e != ErrUnauthorized {
		t.Fatalf("disabled user: %v", e)
	}
	if _, e = s.AuthToken(ctx, "not_a_token"); e != ErrUnauthorized {
		t.Fatalf("malformed token: %v", e)
	}
}

func BenchmarkSQLiteReserveFinish(b *testing.B) {
	s := sqliteTestStore(b)
	u, w, tok := sqliteFixture(b, s)
	ctx := context.Background()
	if _, e := s.DB.ExecContext(ctx, "UPDATE wallets SET balance=1000000000000 WHERE id=?", w); e != nil {
		b.Fatal(e)
	}
	var seq atomic.Int64
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		call, e := s.Reserve(ctx, u, tok, w, "echo", 1, fmt.Sprint(seq.Add(1)))
		if e != nil {
			b.Fatal(e)
		}
		if e = s.Finish(ctx, call.ID, true, 0); e != nil {
			b.Fatal(e)
		}
	}
}

func TestSQLiteBillingRoleMultiplierAppliesAtReserve(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	if _, e := s.DB.ExecContext(ctx, "UPDATE users SET billing_role='vip' WHERE id=?", u); e != nil {
		t.Fatal(e)
	}
	var toolID int64
	if e := s.DB.QueryRowContext(ctx, "INSERT INTO tools(key,name,kind,cost) VALUES('vip_tool','VT','http',5) RETURNING id").Scan(&toolID); e != nil {
		t.Fatal(e)
	}
	call, e := s.ReserveTool(ctx, u, tok, w, toolID, "vip_tool", "role-key")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, call.ID, true, time.Second); e != nil {
		t.Fatal(e)
	}
	var balance, cost, bp int64
	var role string
	if e = s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	if balance != 7 {
		t.Fatalf("vip 0.5x of price 5 must charge 3, balance=%d", balance)
	}
	if e = s.DB.QueryRowContext(ctx, "SELECT cost,billing_role,multiplier_bp FROM usage_logs WHERE id=?", call.ID).Scan(&cost, &role, &bp); e != nil {
		t.Fatal(e)
	}
	if cost != 3 || role != "vip" || bp != 5000 {
		t.Fatalf("usage row cost=%d role=%s bp=%d", cost, role, bp)
	}
	if _, e = s.DB.ExecContext(ctx, "UPDATE billing_roles SET multiplier_bp=0 WHERE name='vip'"); e != nil {
		t.Fatal(e)
	}
	free, e := s.ReserveTool(ctx, u, tok, w, toolID, "vip_tool", "free-key")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, free.ID, true, 0); e != nil {
		t.Fatal(e)
	}
	if e = s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	if balance != 7 {
		t.Fatalf("zero multiplier must be free, balance=%d", balance)
	}
	if e = s.DB.QueryRowContext(ctx, "SELECT cost FROM usage_logs WHERE id=?", free.ID).Scan(&cost); e != nil {
		t.Fatal(e)
	}
	if cost != 0 {
		t.Fatalf("free call cost=%d", cost)
	}
}

func TestSQLiteToolRoleGating(t *testing.T) {
	s := sqliteTestStore(t)
	u, w, tok := sqliteFixture(t, s)
	ctx := context.Background()
	var toolID int64
	if e := s.DB.QueryRowContext(ctx, "INSERT INTO tools(key,name,kind,cost,allowed_roles) VALUES('gated','G','http',5,'[\"vip\"]') RETURNING id").Scan(&toolID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ReserveTool(ctx, u, tok, w, toolID, "gated", "denied-key"); e != ErrRoleNotAllowed {
		t.Fatalf("default role must be gated: %v", e)
	}
	if _, e := s.DB.ExecContext(ctx, "UPDATE users SET billing_role='vip' WHERE id=?", u); e != nil {
		t.Fatal(e)
	}
	call, e := s.ReserveTool(ctx, u, tok, w, toolID, "gated", "allowed-key")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, call.ID, true, 0); e != nil {
		t.Fatal(e)
	}
	var balance int64
	if e = s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	if balance != 7 {
		t.Fatalf("vip gate+multiplier balance=%d", balance)
	}
	if _, e = s.DB.ExecContext(ctx, "UPDATE users SET billing_role='default' WHERE id=?", u); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.ExecContext(ctx, "UPDATE tools SET allowed_roles='[]' WHERE id=?", toolID); e != nil {
		t.Fatal(e)
	}
	open, e := s.ReserveTool(ctx, u, tok, w, toolID, "gated", "open-key")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, open.ID, true, 0); e != nil {
		t.Fatal(e)
	}
	if e = s.DB.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?", w).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	if balance != 2 {
		t.Fatalf("empty allowlist must be open at 1.0x, balance=%d", balance)
	}
}
