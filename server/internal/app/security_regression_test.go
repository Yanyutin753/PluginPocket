package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/Yanyutin753/loadout/server/internal/gateway"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestReviewDisabledAdminCannotAdjustAfterWalletWait(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	register(t, h, "alice")
	admin := register(t, h, "operator")
	other := register(t, h, "supervisor")
	if _, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username IN ('operator','supervisor')"); e != nil {
		t.Fatal(e)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM wallets WHERE user_id=1 FOR UPDATE").Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- request(h, "POST", "/api/v1/admin/users/1/balance", `{"delta":100,"note":"blocked adjustment","idempotency_key":"review"}`, admin)
	}()
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
		t.Fatal("request did not wait for wallet lock")
	}
	if w := request(h, "PATCH", "/api/v1/admin/users/2", `{"enabled":false}`, other); w.Code != 200 {
		t.Fatalf("disable: %d %s", w.Code, w.Body)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w := <-done:
		var balance int64
		if e = s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE user_id=1").Scan(&balance); e != nil {
			t.Fatal(e)
		}
		if w.Code < 400 || balance != 0 {
			t.Fatalf("disabled admin adjustment returned %d; wallet balance=%d; want rejected and balance=0", w.Code, balance)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request stuck")
	}
}

func TestReviewDisabledOwnerCannotFundAfterTeamWait(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	owner := register(t, h, "owner")
	admin := register(t, h, "operator")
	id := newTeam(t, h, owner)
	if _, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='operator'"); e != nil {
		t.Fatal(e)
	}
	if w := request(h, "POST", "/api/v1/admin/users/1/balance", `{"delta":100,"note":"funding","idempotency_key":"seed"}`, admin); w.Code != 200 {
		t.Fatal(w.Code)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM teams WHERE id=$1 FOR UPDATE", id).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- request(h, "POST", fmt.Sprintf("/api/v1/account/teams/%d/fund", id), `{"credits":50,"idempotency_key":"disabled"}`, owner)
	}()
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
		t.Fatal("request did not wait for team lock")
	}
	if w := request(h, "PATCH", "/api/v1/admin/users/1", `{"enabled":false}`, admin); w.Code != 200 {
		t.Fatalf("disable failed %d %s", w.Code, w.Body)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w := <-done:
		var balance int64
		if e = s.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE team_id=$1", id).Scan(&balance); e != nil {
			t.Fatal(e)
		}
		if w.Code < 400 || balance != 0 {
			t.Fatalf("disabled owner funding returned %d; team balance=%d; want rejected and balance=0", w.Code, balance)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request stuck")
	}
}

func TestReviewAdminRejectsUnservableBuiltinSchema(t *testing.T) {
	_, admin, h, _ := adminFixture(t)
	w := request(h, "PATCH", "/api/v1/admin/tools/1", `{"key":"echo","name":"Echo","kind":"builtin","enabled":true,"units_per_call":1,"input_schema":{"type":"object","properties":{"message":{"type":"object","x-mcp-header":"bad"}}}}`, admin)
	if w.Code != 400 {
		t.Fatalf("API accepted unservable builtin schema: status=%d body=%s", w.Code, w.Body)
	}
}

func TestBoundaryOmittedConfigKeepsConcurrentRotation(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	admin := register(t, h, "operator")
	if _, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='operator'"); e != nil {
		t.Fatal(e)
	}
	key := []byte("01234567890123456789012345678901")
	g := gateway.New(s, gateway.Options{EncryptionKey: key})
	defer g.Close()
	h = New(s, Options{Origin: "http://example.com", Gateway: g, EncryptionKey: key})
	w := request(h, "POST", "/api/v1/admin/tools", `{"key":"remote","name":"Remote","kind":"http","enabled":true,"units_per_call":1,"input_schema":{"type":"object"},"config":{"url":"https://example.com/mcp","headers":{"Authorization":"Bearer old-fixture"}}}`, admin)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var out struct{ Item struct{ ID int64 } }
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	fresh := []byte(`{"url":"https://example.com/mcp","headers":{"Authorization":"Bearer new-fixture"}}`)
	sealed, e := gateway.SealConfig(key, fresh)
	if e != nil {
		t.Fatal(e)
	}
	var pid int
	if e = tx.QueryRow(ctx, "UPDATE tools SET config=$1 WHERE id=$2 RETURNING pg_backend_pid()", sealed, out.Item.ID).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- request(h, "PATCH", fmt.Sprintf("/api/v1/admin/tools/%d", out.Item.ID), `{"key":"remote","name":"Renamed","kind":"http","enabled":true,"units_per_call":2,"input_schema":{"type":"object"}}`, admin)
	}()
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
		t.Fatal("metadata update did not block behind rotation")
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w = <-done:
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("update stuck")
	}
	var stored []byte
	if e = s.Pool.QueryRow(ctx, "SELECT config FROM tools WHERE id=$1", out.Item.ID).Scan(&stored); e != nil {
		t.Fatal(e)
	}
	plain, e := gateway.OpenConfig(key, stored)
	if e != nil {
		t.Fatal(e)
	}
	if string(plain) != string(fresh) {
		t.Fatalf("metadata-only PATCH restored old fixture config after rotation committed; got %s", plain)
	}
}

func TestReviewDeviceExpiryIsRecheckedAfterLockWait(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	cookie := register(t, h, "expires_wait")
	w := publicRequest(h, "/api/v1/device/authorize", `{}`)
	var codes struct {
		Device string `json:"device_code"`
		User   string `json:"user_code"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &codes); e != nil || w.Code != 200 {
		t.Fatal(w.Code, e)
	}
	if w = request(h, "POST", "/api/v1/account/devices/approve", fmt.Sprintf(`{"user_code":%q}`, codes.User), cookie); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if _, e := s.Pool.Exec(ctx, "UPDATE device_authorizations SET expires_at=clock_timestamp()+interval '1 second'"); e != nil {
		t.Fatal(e)
	}
	blocker, e := pgx.Connect(ctx, s.Pool.Config().ConnString())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = blocker.Close(ctx) }()
	tx, e := blocker.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e := tx.Exec(ctx, "SELECT id FROM device_authorizations FOR UPDATE"); e != nil {
		t.Fatal(e)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- publicRequest(h, "/api/v1/device/token", fmt.Sprintf(`{"device_code":%q}`, codes.Device))
	}()
	deadline := time.Now().Add(3 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		if e := s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT id,approved_user_id,expires_at%')").Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("device handler never waited on row lock")
	}
	expired := false
	for time.Now().Before(deadline) {
		if e := s.Pool.QueryRow(ctx, "SELECT expires_at<clock_timestamp() FROM device_authorizations").Scan(&expired); e != nil {
			t.Fatal(e)
		}
		if expired {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !expired {
		t.Fatal("fixture failed to expire")
	}
	if e := tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	w = <-done
	if w.Code != 400 || !strings.Contains(w.Body.String(), "expired_token") {
		t.Fatalf("expired code exchanged after lock wait: status=%d", w.Code)
	}
}

func TestReviewAnonymousWrongMethodExhaustsOtherUsersAuthentication(t *testing.T) {
	_, h := setup(t)
	for i := 0; i < 120; i++ {
		r := httptest.NewRequest("GET", "/api/v1/device/token", nil)
		r.RemoteAddr = "198.51.100.1:10000"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 405 {
			t.Fatalf("attack request %d status %d", i, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(`{"username":"legitimate","password":"correct horse battery"}`))
	r.RemoteAddr = "203.0.113.10:20000"
	r.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("legitimate registration from another address blocked: status=%d body=%s", w.Code, w.Body)
	}
}

func TestQueuedWritesRecheckSessionAndRole(t *testing.T) {
	for _, operation := range []string{"adjust", "team", "fund_wallet", "redeem_wallet", "redeem_code", "admin_user", "admin_target", "plan", "accept", "tool", "token", "token_insert", "adjust_ledger", "fund_ledger", "tool_insert", "revoke"} {
		for _, invalidation := range []string{"disabled", "logout", "expired", "demoted"} {
			if invalidation == "demoted" && operation != "adjust" && operation != "admin_user" && operation != "tool" && operation != "admin_target" && operation != "plan" && operation != "adjust_ledger" && operation != "tool_insert" {
				continue
			}
			t.Run(operation+"/"+invalidation, func(t *testing.T) {
				s, h := setup(t)
				ctx := context.Background()
				cookie := register(t, h, "operator")
				register(t, h, "target")
				if _, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE id=1; UPDATE wallets SET balance=100 WHERE user_id=1"); e != nil {
					t.Fatal(e)
				}
				team := newTeam(t, h, cookie)
				method, path, body, lock := "POST", "/api/v1/admin/users/2/balance", `{"delta":10,"note":"queued","idempotency_key":"queued"}`, "SELECT pg_backend_pid() FROM wallets WHERE user_id=2 FOR UPDATE"
				switch operation {
				case "team":
					method, path, body, lock = "PATCH", fmt.Sprintf("/api/v1/account/teams/%d", team), `{"name":"changed"}`, "SELECT pg_backend_pid() FROM teams FOR UPDATE"
				case "fund_wallet":
					path, body, lock = fmt.Sprintf("/api/v1/account/teams/%d/fund", team), `{"credits":10,"idempotency_key":"queued"}`, "SELECT pg_backend_pid() FROM wallets WHERE user_id=1 FOR UPDATE"
				case "redeem_wallet", "redeem_code":
					if _, e := s.Pool.Exec(ctx, "INSERT INTO redemption_codes(code_hash,credits,created_by,note) VALUES($1,10,1,'fixture')", auth.Digest("fixture")); e != nil {
						t.Fatal(e)
					}
					path, body, lock = "/api/v1/account/redeem", `{"code":"fixture"}`, "SELECT pg_backend_pid() FROM wallets WHERE user_id=1 FOR UPDATE"
					if operation == "redeem_code" {
						lock = "SELECT pg_backend_pid() FROM redemption_codes FOR UPDATE"
					}
				case "admin_user":
					method, path, body, lock = "PATCH", "/api/v1/admin/users/2", `{"enabled":false}`, "SELECT pg_backend_pid() FROM pg_advisory_xact_lock(817392106)"
				case "admin_target":
					method, path, body, lock = "PATCH", "/api/v1/admin/users/2", `{"enabled":false}`, "SELECT pg_backend_pid() FROM users WHERE id=2 FOR UPDATE"
				case "plan":
					if _, e := s.Pool.Exec(ctx, "INSERT INTO plans(name,credits,price_cents,currency,enabled) VALUES('original',10,1,'USD',true)"); e != nil {
						t.Fatal(e)
					}
					method, path, body, lock = "PATCH", "/api/v1/admin/plans/1", `{"name":"changed","credits":10,"price_cents":1,"currency":"USD","enabled":true}`, "SELECT pg_backend_pid() FROM plans WHERE id=1 FOR UPDATE"
				case "accept":
					other := register(t, h, "other")
					otherID := newTeam(t, h, other)
					code := invite(t, h, other, otherID)
					path, body, lock = "/api/v1/account/team-invites/accept", fmt.Sprintf(`{"code":%q}`, code), fmt.Sprintf("SELECT pg_backend_pid() FROM teams WHERE id=%d FOR UPDATE", otherID)
				case "tool":
					method, path, body, lock = "PATCH", "/api/v1/admin/tools/1", `{"key":"echo","name":"changed","kind":"builtin","enabled":true,"units_per_call":2,"input_schema":{"type":"object"}}`, "SELECT pg_backend_pid() FROM tools WHERE id=1 FOR UPDATE"
				case "token_insert":
					path, body, lock = "/api/v1/account/tokens", `{"name":"queued"}`, "SELECT pg_backend_pid() FROM users WHERE id=1 FOR UPDATE"
				case "adjust_ledger":
					lock = "SELECT pg_backend_pid() FROM users WHERE id=2 FOR UPDATE"
				case "fund_ledger":
					path, body, lock = fmt.Sprintf("/api/v1/account/teams/%d/fund", team), `{"credits":10,"idempotency_key":"queued"}`, "SELECT pg_backend_pid() FROM users WHERE id=1 FOR UPDATE"
				case "tool_insert":
					path, body, lock = "/api/v1/admin/tools", `{"key":"echo","name":"changed","kind":"builtin","enabled":true,"units_per_call":2,"input_schema":{"type":"object"}}`, "DELETE FROM tools WHERE id=1 RETURNING pg_backend_pid()"
				case "revoke":
					if w := request(h, "POST", "/api/v1/account/tokens", `{"name":"existing"}`, cookie); w.Code != 201 {
						t.Fatal(w.Code)
					}
					method, path, body, lock = "DELETE", "/api/v1/account/tokens/1", "", "SELECT pg_backend_pid() FROM tokens WHERE id=1 FOR UPDATE"
				case "token":
					path, body, lock = "/api/v1/account/tokens", fmt.Sprintf(`{"name":"queued","team_id":%d}`, team), "SELECT pg_backend_pid() FROM teams FOR UPDATE"
				}
				tx, e := s.Pool.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				defer func() { _ = tx.Rollback(ctx) }()
				var pid int
				if e = tx.QueryRow(ctx, lock).Scan(&pid); e != nil {
					t.Fatal(e)
				}
				done := make(chan *httptest.ResponseRecorder, 1)
				go func() { done <- request(h, method, path, body, cookie) }()
				waitForAppLock(t, s, pid)
				mutator := userExecutor(s.Pool)
				if operation == "token_insert" || operation == "fund_ledger" {
					mutator = tx
				}
				switch invalidation {
				case "disabled":
					_, e = mutator.Exec(ctx, "UPDATE users SET enabled=false WHERE id=1")
				case "logout":
					_, e = s.Pool.Exec(ctx, "DELETE FROM sessions WHERE user_id=1")
				case "expired":
					_, e = s.Pool.Exec(ctx, "UPDATE sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE user_id=1")
				case "demoted":
					_, e = mutator.Exec(ctx, "UPDATE users SET role='user' WHERE id=1")
				}
				if e != nil {
					t.Fatal(e)
				}
				if e = tx.Commit(ctx); e != nil {
					t.Fatal(e)
				}
				select {
				case w := <-done:
					want := 401
					if invalidation == "demoted" {
						want = 403
					}
					if w.Code != want {
						t.Fatalf("queued write returned %d; want %d", w.Code, want)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("request stuck")
				}
				var effects int
				if e = s.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM ledger)+(SELECT count(*) FROM tokens WHERE name='queued' OR revoked_at IS NOT NULL)+(SELECT count(*) FROM redemption_codes WHERE redeemed_by IS NOT NULL)+(SELECT count(*) FROM users WHERE id=2 AND NOT enabled)+(SELECT count(*) FROM teams WHERE name='changed')+(SELECT count(*) FROM tools WHERE name='changed')+(SELECT count(*) FROM plans WHERE name='changed')+(SELECT count(*) FROM team_invites WHERE accepted_by=1)").Scan(&effects); e != nil {
					t.Fatal(e)
				}
				if effects != 0 {
					t.Fatalf("rejected write left %d effects", effects)
				}
			})
		}
	}
}

func waitForAppLock(t *testing.T, s *store.Store, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if e := s.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("request never waited on fixture lock")
}

func TestAuthenticationBudgetsSeparatePeersAndDevicePolling(t *testing.T) {
	s, h := setup(t)
	call := func(path, peer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		r.RemoteAddr = peer
		r.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for i := range 120 {
		if w := call("/api/v1/device/token", "198.51.100.1:1"); w.Code != 400 {
			t.Fatalf("poll status %d", w.Code)
		}
		if i == 0 {
			pinAuthenticationWindow(t, s)
		}
	}
	if w := call("/api/v1/device/token", "198.51.100.1:2"); w.Code != 429 {
		t.Fatalf("same peer bypassed budget: %d", w.Code)
	}
	if w := call("/api/v1/auth/login", "198.51.100.1:1"); w.Code == 429 {
		t.Fatal("device polling consumed login budget")
	}
	if w := call("/api/v1/device/token", "203.0.113.1:1"); w.Code != 400 {
		t.Fatalf("different peer blocked: %d", w.Code)
	}
}

type delayedRequestBody struct {
	entered chan struct{}
	release chan struct{}
	body    *strings.Reader
}

func (b *delayedRequestBody) Read(p []byte) (int, error) {
	if b.entered != nil {
		close(b.entered)
		b.entered = nil
		<-b.release
	}
	return b.body.Read(p)
}
func (b *delayedRequestBody) Close() error { return nil }

func TestWritesRecheckAuthorizationAfterBodyRead(t *testing.T) {
	for _, operation := range []string{"plan", "code", "team", "accept", "approve"} {
		t.Run(operation, func(t *testing.T) {
			s, h := setup(t)
			ctx := context.Background()
			cookie := register(t, h, "operator")
			if _, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE id=1"); e != nil {
				t.Fatal(e)
			}
			path, body := "/api/v1/admin/plans", `{"name":"Changed","credits":10,"price_cents":1,"currency":"USD","enabled":true}`
			switch operation {
			case "code":
				path, body = "/api/v1/admin/redemption-codes", `{"credits":10,"note":"queued"}`
			case "team":
				path, body = "/api/v1/account/teams", `{"name":"Changed"}`
			case "accept":
				owner := register(t, h, "owner")
				id := newTeam(t, h, owner)
				code := invite(t, h, owner, id)
				path, body = "/api/v1/account/team-invites/accept", fmt.Sprintf(`{"code":%q}`, code)
			case "approve":
				w := publicRequest(h, "/api/v1/device/authorize", `{}`)
				var out struct {
					User string `json:"user_code"`
				}
				if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
					t.Fatal(e)
				}
				path, body = "/api/v1/account/devices/approve", fmt.Sprintf(`{"user_code":%q}`, out.User)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			r := httptest.NewRequest("POST", path, nil)
			r.Body = &delayedRequestBody{entered: entered, release: release, body: strings.NewReader(body)}
			r.Header.Set("Origin", "http://example.com")
			r.AddCookie(cookie)
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { w := httptest.NewRecorder(); h.ServeHTTP(w, r); done <- w }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				close(release)
				t.Fatal("handler never read body")
			}
			if _, e := s.Pool.Exec(ctx, "DELETE FROM sessions WHERE user_id=1"); e != nil {
				close(release)
				t.Fatal(e)
			}
			close(release)
			select {
			case w := <-done:
				if w.Code != 401 {
					t.Fatalf("write after revoked session returned %d", w.Code)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("request stuck")
			}
			var effects int
			if e := s.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM plans WHERE name='Changed')+(SELECT count(*) FROM redemption_codes)+(SELECT count(*) FROM teams WHERE name='Changed')+(SELECT count(*) FROM team_members WHERE user_id=1)+(SELECT count(*) FROM device_authorizations WHERE approved_user_id=1)").Scan(&effects); e != nil {
				t.Fatal(e)
			}
			if effects != 0 {
				t.Fatalf("rejected write left %d effects", effects)
			}
		})
	}
}

type userExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func TestDeviceTokenRechecksAfterUserForeignKeyWait(t *testing.T) {
	for _, invalidation := range []string{"disabled", "expired"} {
		t.Run(invalidation, func(t *testing.T) {
			s, h := setup(t)
			ctx := context.Background()
			cookie := register(t, h, "operator")
			w := publicRequest(h, "/api/v1/device/authorize", `{}`)
			var codes struct {
				Device string `json:"device_code"`
				User   string `json:"user_code"`
			}
			if e := json.Unmarshal(w.Body.Bytes(), &codes); e != nil {
				t.Fatal(e)
			}
			if w := request(h, "POST", "/api/v1/account/devices/approve", fmt.Sprintf(`{"user_code":%q}`, codes.User), cookie); w.Code != 204 {
				t.Fatal(w.Code)
			}
			if invalidation == "expired" {
				if _, e := s.Pool.Exec(ctx, "UPDATE device_authorizations SET expires_at=clock_timestamp()+interval '1 second'"); e != nil {
					t.Fatal(e)
				}
			}
			tx, e := s.Pool.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			var pid int
			if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM users WHERE id=1 FOR UPDATE").Scan(&pid); e != nil {
				t.Fatal(e)
			}
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				done <- publicRequest(h, "/api/v1/device/token", fmt.Sprintf(`{"device_code":%q}`, codes.Device))
			}()
			waitForAppLock(t, s, pid)
			want := "invalid_grant"
			if invalidation == "disabled" {
				if _, e = tx.Exec(ctx, "UPDATE users SET enabled=false WHERE id=1"); e != nil {
					t.Fatal(e)
				}
			} else {
				want = "expired_token"
				deadline := time.Now().Add(3 * time.Second)
				expired := false
				for time.Now().Before(deadline) {
					if e = tx.QueryRow(ctx, "SELECT expires_at<clock_timestamp() FROM device_authorizations").Scan(&expired); e != nil {
						t.Fatal(e)
					}
					if expired {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if !expired {
					t.Fatal("device did not expire")
				}
			}
			if e = tx.Commit(ctx); e != nil {
				t.Fatal(e)
			}
			select {
			case w := <-done:
				if w.Code != 400 || !strings.Contains(w.Body.String(), want) {
					t.Fatalf("issued invalid device grant: status %d", w.Code)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("request stuck")
			}
			var effects int
			if e = s.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM tokens)+(SELECT count(*) FROM device_authorizations WHERE consumed_at IS NOT NULL)").Scan(&effects); e != nil {
				t.Fatal(e)
			}
			if effects != 0 {
				t.Fatalf("invalid grant left %d effects", effects)
			}
		})
	}
}

func TestAuthenticationBudgetReclaimsOldPeersWithoutResettingCurrentLimit(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	for i := range 120 {
		if w := request(h, "POST", "/api/v1/auth/register", `{}`, nil); w.Code != 400 {
			t.Fatalf("initial request %d", w.Code)
		}
		if i == 0 {
			pinAuthenticationWindow(t, s)
		}
	}
	if _, e := s.Pool.Exec(ctx, "INSERT INTO rate_limits(scope,subject,window_id,used) SELECT 'public:stale',n,floor(extract(epoch FROM statement_timestamp())/60)::bigint-2,1 FROM generate_series(1,250) n"); e != nil {
		t.Fatal(e)
	}
	if w := request(h, "POST", "/api/v1/auth/register", `{}`, nil); w.Code != 429 {
		t.Fatalf("cleanup reset current budget: %d", w.Code)
	}
	var remaining int
	if e := s.Pool.QueryRow(ctx, "SELECT count(*) FROM rate_limits WHERE scope='public:stale'").Scan(&remaining); e != nil {
		t.Fatal(e)
	}
	if remaining != 150 {
		t.Fatalf("expired peer cleanup should remove only a bounded batch: remaining=%d", remaining)
	}
}
