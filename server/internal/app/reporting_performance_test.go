package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Capture the SQL actually executed by the HTTP reporting path, then measure its
// database work. No assertion depends on query spelling or a particular index.
type reportTrace struct {
	mu      sync.Mutex
	queries []pgx.TraceQueryStartData
}

func (t *reportTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(strings.TrimSpace(data.SQL), "SELECT") && strings.Contains(data.SQL, "usage_logs") {
		t.mu.Lock()
		t.queries = append(t.queries, data)
		t.mu.Unlock()
	}
	return ctx
}
func (*reportTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (t *reportTrace) take() []pgx.TraceQueryStartData {
	t.mu.Lock()
	defer t.mu.Unlock()
	q := t.queries
	t.queries = nil
	return q
}

type reportPlan struct {
	NodeType  string       `json:"Node Type"`
	Relation  string       `json:"Relation Name"`
	Rows      float64      `json:"Actual Rows"`
	Loops     float64      `json:"Actual Loops"`
	Removed   float64      `json:"Rows Removed by Filter"`
	Rechecked float64      `json:"Rows Removed by Index Recheck"`
	Hits      int          `json:"Shared Hit Blocks"`
	Reads     int          `json:"Shared Read Blocks"`
	Plans     []reportPlan `json:"Plans"`
}

func (p reportPlan) usageRows() float64 {
	var rows float64
	if p.Relation == "usage_logs" {
		rows = (p.Rows + p.Removed + p.Rechecked) * p.Loops
	}
	for _, child := range p.Plans {
		rows += child.usageRows()
	}
	return rows
}

func TestReportsReadOnlyTheRequestedTimeWindow(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	owner := register(t, h, "reporter")
	admin := register(t, h, "operator")
	if _, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='operator'"); e != nil {
		t.Fatal(e)
	}
	teamID := newTeam(t, h, owner)
	w := request(h, "POST", "/api/v1/account/tokens", fmt.Sprintf(`{"name":"Reports","team_id":%d}`, teamID), owner)
	if w.Code != 201 {
		t.Fatalf("token %d %s", w.Code, w.Body)
	}
	now := time.Now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// Numerous old tool names also prevent a low-cardinality index skip scan from
	// accidentally masking a missing global time index on PostgreSQL 18.
	if _, e := s.Pool.Exec(ctx, `INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key,created_at)
 SELECT t.user_id,t.id,t.wallet_id,'history_'||n,1,'ok','old_'||n,$1 FROM tokens t CROSS JOIN generate_series(1,50000) n`, month.AddDate(0, -3, 0)); e != nil {
		t.Fatal(e)
	}
	for i, row := range []struct {
		at   time.Time
		cost int64
	}{{month.Add(-time.Microsecond), 99}, {day, 2}, {now, 3}} {
		if _, e := s.Pool.Exec(ctx, `INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key,created_at) SELECT user_id,id,wallet_id,'echo',$1,'ok',$2,$3 FROM tokens`, row.cost, fmt.Sprint("boundary_", i), row.at); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.Pool.Exec(ctx, "ANALYZE usage_logs"); e != nil {
		t.Fatal(e)
	}
	trace := &reportTrace{}
	cfg := s.Pool.Config()
	cfg.ConnConfig.Tracer = trace
	cfg.ConnConfig.RuntimeParams["timezone"] = "Pacific/Honolulu"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	s.Pool.Close()
	s.Pool = pool
	for _, tc := range []struct {
		name, path string
		cookie     *http.Cookie
	}{
		{"global_7days", "/api/v1/admin/usage/summary?days=7", admin},
		{"account_me", "/api/v1/account/me", owner},
		{"team_7days", fmt.Sprintf("/api/v1/account/teams/%d/usage/summary?days=7", teamID), owner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "account_me" {
				// Current traffic belonging to another account requires selective user/time
				// and wallet/time access, not merely a global time index.
				if _, e = s.Pool.Exec(ctx, "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) SELECT user_id,id,'Other account','other','report-other' FROM wallets WHERE user_id=2"); e != nil {
					t.Fatal(e)
				}
				if _, e = s.Pool.Exec(ctx, `INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key,created_at)
 SELECT t.user_id,t.id,t.wallet_id,'other_'||n,1,'ok','current_other_'||n,$1 FROM tokens t CROSS JOIN generate_series(1,50000) n WHERE t.user_id=2`, now); e != nil {
					t.Fatal(e)
				}
				if _, e = s.Pool.Exec(ctx, "ANALYZE usage_logs"); e != nil {
					t.Fatal(e)
				}
			}
			trace.take()
			w := request(h, "GET", tc.path, "", tc.cookie)
			if w.Code != 200 {
				t.Fatalf("report %d %s", w.Code, w.Body)
			}
			if tc.name == "account_me" {
				var result struct {
					Summary struct {
						TodayCalls int64 `json:"today_calls"`
						MonthCost  int64 `json:"month_cost"`
					}
				}
				if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil {
					t.Fatal(e)
				}
				if result.Summary.TodayCalls != 2 || result.Summary.MonthCost != 5 {
					t.Fatalf("UTC summary changed: %s", w.Body)
				}
			}
			queries := trace.take()
			if len(queries) != 1 {
				t.Fatalf("expected one measured aggregate query, got %d", len(queries))
			}
			var raw []byte
			if e = s.Pool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+queries[0].SQL, queries[0].Args...).Scan(&raw); e != nil {
				t.Fatal(e)
			}
			var result []struct {
				Plan        reportPlan `json:"Plan"`
				ExecutionMS float64    `json:"Execution Time"`
			}
			if e = json.Unmarshal(raw, &result); e != nil || len(result) != 1 {
				t.Fatalf("explain parse: %v %s", e, raw)
			}
			plan := result[0].Plan
			t.Logf("usage scan rows=%.0f shared hit=%d read=%d execution_ms=%.3f", plan.usageRows(), plan.Hits, plan.Reads, result[0].ExecutionMS)
			if plan.usageRows() > 100 {
				t.Logf("slow execution plan: %s", raw)
				t.Errorf("report processed %.0f usage scan rows for a window with at most 3 rows; old history must not be scanned", plan.usageRows())
			}
			if plan.Hits+plan.Reads > 100 {
				t.Errorf("report touched %d shared buffers for at most 3 current rows", plan.Hits+plan.Reads)
			}
		})
	}
}
