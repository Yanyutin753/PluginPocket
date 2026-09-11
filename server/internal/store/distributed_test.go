package store

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func secondStore(t testing.TB, first *Store) *Store {
	t.Helper()
	s, err := Open(context.Background(), first.Pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestDistributedRateBudget(t *testing.T) {
	s := testStore(t)
	other := secondStore(t, s)
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for i := range 80 {
		instance := []*Store{s, other}[i%2]
		wg.Go(func() {
			ok, err := instance.AllowRequest(context.Background(), "test", 0, 86400, 20)
			if err != nil {
				t.Error(err)
			}
			if ok {
				admitted.Add(1)
			}
		})
	}
	wg.Wait()
	if got := admitted.Load(); got != 20 {
		t.Fatalf("shared budget admitted %d requests, want 20", got)
	}
	restart := secondStore(t, s)
	if ok, err := restart.AllowRequest(context.Background(), "test", 0, 86400, 20); err != nil || ok {
		t.Fatalf("restart reset budget: allowed=%v err=%v", ok, err)
	}
	if ok, err := other.AllowRequest(context.Background(), "other", 0, 86400, 20); err != nil || !ok {
		t.Fatalf("scope isolation: allowed=%v err=%v", ok, err)
	}
}

func TestRateWindowOnlyAdvances(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO rate_limits(scope,subject,window_id,used) VALUES('future',0,floor(extract(epoch FROM statement_timestamp())/60)::bigint+1,2),('past',0,0,2)`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.AllowRequest(ctx, "future", 0, 60, 2); err != nil || ok {
		t.Fatalf("older request reopened full window: allowed=%v err=%v", ok, err)
	}
	if ok, err := s.AllowRequest(ctx, "past", 0, 60, 2); err != nil || !ok {
		t.Fatalf("new window not available: allowed=%v err=%v", ok, err)
	}
	var used int
	if err := s.Pool.QueryRow(ctx, "SELECT used FROM rate_limits WHERE scope='past'").Scan(&used); err != nil || used != 1 {
		t.Fatalf("new window used=%d err=%v", used, err)
	}
}

func TestDistributedReservationsAndRecoveryRefundOnce(t *testing.T) {
	s := testStore(t)
	other := secondStore(t, s)
	u, w, tok := fixture(t, s)
	ctx := context.Background()
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := range 40 {
		replica := []*Store{s, other}[i%2]
		wg.Go(func() {
			_, err := replica.Reserve(ctx, u, tok, w, "echo", 1, fmt.Sprint(i))
			if err == nil {
				accepted.Add(1)
			} else if err != ErrInsufficientBalance {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 10 {
		t.Fatalf("accepted=%d", accepted.Load())
	}
	rows, err := s.Pool.Query(ctx, "SELECT id FROM usage_logs WHERE status='pending'")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, replica := range []*Store{s, other} {
		wg.Go(func() {
			if _, err := replica.RecoverPending(ctx, time.Now().Add(time.Minute)); err != nil {
				t.Error(err)
			}
		})
	}
	for _, id := range ids {
		wg.Go(func() {
			if err := other.Finish(ctx, id, false, 0); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var balance, delta, refunds, pending int64
	err = s.Pool.QueryRow(ctx, `SELECT balance,(SELECT coalesce(sum(delta),0) FROM ledger WHERE wallet_id=$1),(SELECT count(*) FROM ledger WHERE wallet_id=$1 AND kind IN ('refund','recovery')),(SELECT count(*) FROM usage_logs WHERE status='pending') FROM wallets WHERE id=$1`, w).Scan(&balance, &delta, &refunds, &pending)
	if err != nil || balance != 10 || delta != 0 || refunds != 10 || pending != 0 {
		t.Fatalf("balance=%d delta=%d refunds=%d pending=%d err=%v", balance, delta, refunds, pending, err)
	}
	if n, err := secondStore(t, s).RecoverPending(ctx, time.Now().Add(time.Minute)); err != nil || n != 0 {
		t.Fatalf("restarted recovery n=%d err=%v", n, err)
	}
}

func TestDistributedConcurrentFirstMigrations(t *testing.T) {
	parent := testStore(t)
	ctx := context.Background()
	schema := fmt.Sprintf("migration_%d", time.Now().UnixNano())
	if _, err := parent.Pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = parent.Pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	})
	raw, err := url.Parse(parent.Pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	query := raw.Query()
	query.Set("search_path", schema)
	raw.RawQuery = query.Encode()
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			s, err := Open(ctx, raw.String())
			if err != nil {
				t.Error(err)
				return
			}
			defer s.Close()
			var tools int
			if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM tools").Scan(&tools); err != nil || tools != 5 {
				t.Errorf("seed tools=%d err=%v", tools, err)
			}
		})
	}
	wg.Wait()
}
