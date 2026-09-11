package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
)

func replicaStore(t *testing.T, s *store.Store) *store.Store {
	t.Helper()
	other, err := store.Open(context.Background(), s.Pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.Close)
	return other
}

func pinAuthenticationWindow(t *testing.T, s *store.Store) {
	t.Helper()
	// These HTTP budget tests must not reset halfway through a minute boundary.
	// Store tests independently cover clock rollback and window rollover.
	if _, err := s.Pool.Exec(context.Background(), "UPDATE rate_limits SET window_id=floor(extract(epoch FROM statement_timestamp())/60)::bigint+60 WHERE scope LIKE 'public:%'"); err != nil {
		t.Fatal(err)
	}
}

func TestDistributedAuthenticationBudget(t *testing.T) {
	s, first := setup(t)
	second := New(replicaStore(t, s), Options{Origin: "http://example.com"})
	for i := range 120 {
		h := first
		if i%2 == 1 {
			h = second
		}
		if w := request(h, "POST", "/api/v1/auth/register", `{}`, nil); w.Code != 400 {
			t.Fatalf("request %d got %d", i, w.Code)
		}
		if i == 0 {
			pinAuthenticationWindow(t, s)
		}
	}
	if w := request(second, "POST", "/api/v1/auth/register", `{}`, nil); w.Code != 429 {
		t.Fatalf("two replicas bypass shared 120/min budget: %d", w.Code)
	}
}

func TestLedgerIdempotencyIsScopedToBusinessOperation(t *testing.T) {
	s, first := setup(t)
	alice := register(t, first, "alice")
	admin := register(t, first, "operator")
	if _, err := s.Pool.Exec(context.Background(), "UPDATE users SET role='admin' WHERE username='operator'"); err != nil {
		t.Fatal(err)
	}
	second := New(replicaStore(t, s), Options{Origin: "http://example.com"})
	w := request(first, "POST", "/api/v1/admin/redemption-codes", `{"credits":25,"note":"grant"}`, admin)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	var code struct{ Code string }
	if err := json.Unmarshal(w.Body.Bytes(), &code); err != nil {
		t.Fatal(err)
	}
	// An operator's client key must not occupy another operation's namespace.
	adjust := `{"delta":5,"note":"manual","idempotency_key":"redemption:1"}`
	if w = request(first, "POST", "/api/v1/admin/users/1/balance", adjust, admin); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w = request(second, "POST", "/api/v1/account/redeem", fmt.Sprintf(`{"code":%q}`, code.Code), alice); w.Code != 200 {
		t.Fatalf("unrelated adjustment key blocked redemption: %d %s", w.Code, w.Body)
	}
	if w = request(second, "POST", "/api/v1/admin/users/1/balance", adjust, admin); w.Code != 200 {
		t.Fatalf("adjustment replay found wrong ledger kind: %d %s", w.Code, w.Body)
	}
	var balance, count int
	if err := s.Pool.QueryRow(context.Background(), "SELECT balance,(SELECT count(*) FROM ledger WHERE wallet_id=1) FROM wallets WHERE user_id=1").Scan(&balance, &count); err != nil || balance != 30 || count != 2 {
		t.Fatalf("balance=%d entries=%d err=%v", balance, count, err)
	}
}

func TestDatabaseClockControlsDeviceAndInviteExpiry(t *testing.T) {
	s, first := setup(t)
	alice := register(t, first, "alice")
	bob := register(t, first, "bob")
	second := New(replicaStore(t, s), Options{Origin: "http://example.com"})
	team := newTeam(t, first, alice)
	inviteCode := invite(t, first, alice, team)
	w := publicRequest(first, "/api/v1/device/authorize", `{}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var code struct {
		Device string `json:"device_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &code); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(context.Background(), "UPDATE device_authorizations SET expires_at=now()-interval '1 minute'; UPDATE team_invites SET expires_at=now()-interval '1 minute'"); err != nil {
		t.Fatal(err)
	}
	// Go's native fake clock is in 2000; the real database retains its own clock.
	synctest.Test(t, func(t *testing.T) {
		w := publicRequest(second, "/api/v1/device/token", fmt.Sprintf(`{"device_code":%q}`, code.Device))
		if w.Code != 400 || !strings.Contains(w.Body.String(), "expired_token") {
			t.Errorf("expired device on skewed node: %d %s", w.Code, w.Body)
		}
		w = request(second, "POST", "/api/v1/account/team-invites/accept", fmt.Sprintf(`{"code":%q}`, inviteCode), bob)
		if w.Code != 410 {
			t.Errorf("expired invite on skewed node: %d %s", w.Code, w.Body)
		}
	})
}

func TestDatabaseClockControlsSessionAndDeviceCreation(t *testing.T) {
	s, _ := setup(t)
	second := New(replicaStore(t, s), Options{Origin: "http://example.com"})
	synctest.Test(t, func(t *testing.T) {
		cookie := register(t, second, "skewed")
		if w := request(second, "GET", "/api/v1/account/me", "", cookie); w.Code != 200 {
			t.Errorf("new session expired from node clock: %d %s", w.Code, w.Body)
		}
		if w := publicRequest(second, "/api/v1/device/authorize", `{}`); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	})
	var valid bool
	if err := s.Pool.QueryRow(context.Background(), "SELECT expires_at>now()+interval '9 minutes' FROM device_authorizations").Scan(&valid); err != nil || !valid {
		t.Fatalf("device lifetime used node clock: valid=%v err=%v", valid, err)
	}
}

func TestDistributedBootstrapAndSessions(t *testing.T) {
	s, first := setup(t)
	other := replicaStore(t, s)
	second := New(other, Options{Origin: "http://example.com"})
	var wg sync.WaitGroup
	for _, replica := range []*store.Store{s, other} {
		wg.Go(func() {
			if err := BootstrapAdmin(context.Background(), replica, "operator", "correct horse battery"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var users, wallets int
	if err := s.Pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM wallets)").Scan(&users, &wallets); err != nil || users != 1 || wallets != 1 {
		t.Fatalf("users=%d wallets=%d err=%v", users, wallets, err)
	}
	login := request(first, "POST", "/api/v1/auth/login", `{"username":"operator","password":"correct horse battery"}`, nil)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body)
	}
	cookie := login.Result().Cookies()[0]
	if w := request(second, "GET", "/api/v1/account/me", "", cookie); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(second, "POST", "/api/v1/auth/logout", `{}`, cookie); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(first, "GET", "/api/v1/account/me", "", cookie); w.Code != 401 {
		t.Fatalf("logout on replica did not revoke origin session: %d", w.Code)
	}
}

func TestDistributedDeviceApprovalAndConsumption(t *testing.T) {
	s, first := setup(t)
	second := New(replicaStore(t, s), Options{Origin: "http://example.com"})
	user := register(t, first, "alice")
	w := publicRequest(first, "/api/v1/device/authorize", `{}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var code struct {
		Device string `json:"device_code"`
		User   string `json:"user_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &code); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"device_code":%q}`, code.Device)
	w = publicRequest(second, "/api/v1/device/token", body)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "authorization_pending") {
		t.Fatal(w.Code, w.Body)
	}
	if w = publicRequest(first, "/api/v1/device/token", body); w.Code != 429 {
		t.Fatalf("poll window bypassed on different replica: %d", w.Code)
	}
	if w = request(second, "POST", "/api/v1/account/devices/approve", fmt.Sprintf(`{"user_code":%q}`, code.User), user); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := s.Pool.Exec(context.Background(), "UPDATE device_authorizations SET next_poll_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for _, replica := range []http.Handler{first, second} {
		wg.Go(func() { results <- publicRequest(replica, "/api/v1/device/token", body).Code })
	}
	wg.Wait()
	close(results)
	successes := 0
	for status := range results {
		if status == 200 {
			successes++
		} else if status != 400 {
			t.Errorf("unexpected consumption status %d", status)
		}
	}
	if successes != 1 {
		t.Fatalf("issued %d tokens", successes)
	}
	var tokens int
	if err := s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM tokens").Scan(&tokens); err != nil || tokens != 1 {
		t.Fatalf("tokens=%d err=%v", tokens, err)
	}
}

func TestDistributedReportsUseDatabaseUTCDayAndMonth(t *testing.T) {
	s, first := setup(t)
	owner := register(t, first, "reporter")
	admin := register(t, first, "operator")
	if _, err := s.Pool.Exec(context.Background(), "UPDATE users SET role='admin' WHERE username='operator'"); err != nil {
		t.Fatal(err)
	}
	teamID := newTeam(t, first, owner)
	if w := request(first, "POST", "/api/v1/account/tokens", fmt.Sprintf(`{"name":"Reporting","team_id":%d}`, teamID), owner); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := s.Pool.Exec(context.Background(), `INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key,created_at)
 SELECT t.user_id,t.id,t.wallet_id,'echo',v.cost,'ok',v.key,v.created_at FROM tokens t CROSS JOIN (VALUES
 (99,'history',date_trunc('month',statement_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'-interval '3 months'),
 (2,'today',date_trunc('day',statement_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
 (3,'now',statement_timestamp())) AS v(cost,key,created_at)`); err != nil {
		t.Fatal(err)
	}
	second := New(replicaStore(t, s), Options{Origin: "http://example.com"})
	check := func(t *testing.T, h http.Handler) {
		t.Helper()
		w := request(h, "GET", "/api/v1/account/me", "", owner)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var me struct {
			Summary struct {
				TodayCalls int `json:"today_calls"`
				MonthCost  int `json:"month_cost"`
			}
		}
		if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
			t.Fatal(err)
		}
		if me.Summary.TodayCalls != 2 || me.Summary.MonthCost != 5 {
			t.Errorf("account UTC window drift: %s", w.Body)
		}
		for _, days := range []int{1, 7} {
			for _, tc := range []struct {
				path   string
				cookie *http.Cookie
			}{
				{fmt.Sprintf("/api/v1/admin/usage/summary?days=%d", days), admin},
				{fmt.Sprintf("/api/v1/account/teams/%d/usage/summary?days=%d", teamID, days), owner},
			} {
				w = request(h, "GET", tc.path, "", tc.cookie)
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body)
				}
				var report struct{ Items []struct{ Calls, Cost int } }
				if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if len(report.Items) != 1 || report.Items[0].Calls != 2 || report.Items[0].Cost != 5 {
					t.Errorf("%s UTC window drift: %s", tc.path, w.Body)
				}
			}
		}
	}
	check(t, first)
	synctest.Test(t, func(t *testing.T) { check(t, second) })
}
