package store

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func testStore(t testing.TB) *Store {
	t.Helper()
	raw := os.Getenv("LOADOUT_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("LOADOUT_TEST_DATABASE_URL required for real PostgreSQL integration")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
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
	s, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = conn.Close(context.Background())
	})
	return s
}
func TestMigrationsCreateDurableSchema(t *testing.T) {
	s := testStore(t)
	var found bool
	err := s.Pool.QueryRow(context.Background(), "SELECT to_regclass('users') IS NOT NULL").Scan(&found)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("migration must create persistent users table")
	}
}
func fixture(t testing.TB, s *Store) (int64, int64, int64) {
	t.Helper()
	ctx := context.Background()
	var u, w, tok int64
	if err := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES ('tester','hash') RETURNING id").Scan(&u); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES ($1,10) RETURNING id", u).Scan(&w); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES ($1,$2,'test','ldt_x','hash') RETURNING id", u, w).Scan(&tok); err != nil {
		t.Fatal(err)
	}
	return u, w, tok
}
func TestConcurrentReservationsNeverOverdrawAndRefundOnce(t *testing.T) {
	s := testStore(t)
	u, w, tok := fixture(t, s)
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
	if err := s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", w).Scan(&balance); err != nil {
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
	if err := s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", w).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 1 {
		t.Fatalf("refund must apply once: %d", balance)
	}
	var delta int64
	if err := s.Pool.QueryRow(ctx, "SELECT COALESCE(sum(delta),0) FROM ledger WHERE wallet_id=$1", w).Scan(&delta); err != nil {
		t.Fatal(err)
	}
	if delta != -9 {
		t.Fatalf("ledger=%d", delta)
	}
}
func TestReservationReplayRecoveryAndOwnership(t *testing.T) {
	s := testStore(t)
	u, w, tok := fixture(t, s)
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
	_ = s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", w).Scan(&balance)
	if balance != 10 {
		t.Fatalf("recovered balance=%d", balance)
	}
}
func TestLedgerIsAppendOnly(t *testing.T) {
	s := testStore(t)
	u, w, tok := fixture(t, s)
	ctx := context.Background()
	if _, e := s.Reserve(ctx, u, tok, w, "echo", 1, "immutable"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Pool.Exec(ctx, "UPDATE ledger SET delta=100 WHERE wallet_id=$1", w); e == nil {
		t.Fatal("ledger history must not be editable")
	}
	if _, e := s.Pool.Exec(ctx, "DELETE FROM ledger WHERE wallet_id=$1", w); e == nil {
		t.Fatal("ledger history must not be deletable")
	}
}
func TestOnlySuccessfulCallsReportChargedCost(t *testing.T) {
	s := testStore(t)
	u, w, tok := fixture(t, s)
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
	_ = s.Pool.QueryRow(ctx, "SELECT cost FROM usage_logs WHERE id=$1", ok.ID).Scan(&cost)
	if cost != 2 {
		t.Fatalf("successful charged cost: %d", cost)
	}
	_ = s.Pool.QueryRow(ctx, "SELECT cost FROM usage_logs WHERE id=$1", failed.ID).Scan(&cost)
	if cost != 0 {
		t.Fatalf("failed charged cost: %d", cost)
	}
}

func BenchmarkReserveFinish(b *testing.B) {
	for _, parallel := range []bool{false, true} {
		name := "serial"
		if parallel {
			name = "parallel"
		}
		b.Run(name, func(b *testing.B) {
			s := testStore(b)
			u, w, tok := fixture(b, s)
			ctx := context.Background()
			if _, e := s.Pool.Exec(ctx, "UPDATE wallets SET balance=1000000000000 WHERE id=$1", w); e != nil {
				b.Fatal(e)
			}
			var seq atomic.Int64
			callOnce := func() {
				call, e := s.Reserve(ctx, u, tok, w, "echo", 1, fmt.Sprint(seq.Add(1)))
				if e != nil {
					b.Error(e)
					return
				}
				if e = s.Finish(ctx, call.ID, true, 0); e != nil {
					b.Error(e)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			if parallel {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						callOnce()
					}
				})
			} else {
				for b.Loop() {
					callOnce()
				}
			}
		})
	}
}

func TestReservationAndRefundLedgerCapturePostBalance(t *testing.T) {
	s := testStore(t)
	u, w, tok := fixture(t, s)
	ctx := context.Background()
	call, e := s.Reserve(ctx, u, tok, w, "echo", 3, "balances")
	if e != nil {
		t.Fatal(e)
	}
	var balance *int64
	e = s.Pool.QueryRow(ctx, "SELECT balance_after FROM ledger WHERE call_id=$1 AND kind='reservation'", call.ID).Scan(&balance)
	if e != nil || balance == nil || *balance != 7 {
		t.Fatalf("reservation balance_after=%v err=%v", balance, e)
	}
	if e = s.Finish(ctx, call.ID, false, 0); e != nil {
		t.Fatal(e)
	}
	e = s.Pool.QueryRow(ctx, "SELECT balance_after FROM ledger WHERE call_id=$1 AND kind='refund'", call.ID).Scan(&balance)
	if e != nil || balance == nil || *balance != 10 {
		t.Fatalf("refund balance_after=%v err=%v", balance, e)
	}
}
func TestReservationRechecksMembershipAfterWalletLockWait(t *testing.T) {
	s := testStore(t)
	u, w, tok := fixture(t, s)
	ctx := context.Background()
	var team int64
	if e := s.Pool.QueryRow(ctx, "INSERT INTO teams(name) VALUES('Shared') RETURNING id").Scan(&team); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Pool.Exec(ctx, "UPDATE wallets SET user_id=NULL,team_id=$1 WHERE id=$2", team, w); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Pool.Exec(ctx, "INSERT INTO team_members(team_id,user_id,role) VALUES($1,$2,'member')", team, u); e != nil {
		t.Fatal(e)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM wallets WHERE id=$1 FOR UPDATE", w).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, err := s.Reserve(ctx, u, tok, w, "echo", 1, "removed-during-wait"); done <- err }()
	waiting := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if e = s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("reservation did not wait for wallet lock")
	}
	if _, e = tx.Exec(ctx, "DELETE FROM team_members WHERE team_id=$1 AND user_id=$2", team, u); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "UPDATE wallets SET balance=balance WHERE id=$1", w); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-done:
		if e != ErrUnauthorized {
			t.Fatalf("removed member reservation after lock wait: %v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reservation stuck")
	}
}

// BenchmarkManyWallets 测网关真实负载面：不同钱包并发 Reserve→Finish，
// 无热点行争用时的事务吞吐（池上限 20 连接）。
func BenchmarkManyWallets(b *testing.B) {
	s := testStore(b)
	ctx := context.Background()
	type account struct{ user, wallet, token int64 }
	accounts := make([]account, 64)
	for i := range accounts {
		var a account
		if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES ($1,'hash') RETURNING id", fmt.Sprintf("bench%d", i)).Scan(&a.user); e != nil {
			b.Fatal(e)
		}
		if e := s.Pool.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES ($1,1000000000000) RETURNING id", a.user).Scan(&a.wallet); e != nil {
			b.Fatal(e)
		}
		if e := s.Pool.QueryRow(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES ($1,$2,'t','ldt_b',$3) RETURNING id", a.user, a.wallet, fmt.Sprintf("h%d", i)).Scan(&a.token); e != nil {
			b.Fatal(e)
		}
		accounts[i] = a
	}
	var seq atomic.Int64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		a := accounts[int(seq.Add(1))%len(accounts)]
		var n int64
		for pb.Next() {
			n++
			call, e := s.Reserve(ctx, a.user, a.token, a.wallet, "echo", 1, fmt.Sprintf("k%d-%d", a.token, n))
			if e != nil {
				b.Error(e)
				return
			}
			if e = s.Finish(ctx, call.ID, true, 0); e != nil {
				b.Error(e)
			}
		}
	})
}
