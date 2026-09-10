package app

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestPlansRedemptionAndUnavailablePayments(t *testing.T) {
	user, admin, h, _ := adminFixture(t)
	if w := request(h, "POST", "/api/v1/admin/plans", `{"name":"Starter","credits":100,"price_cents":1000,"currency":"CNY","enabled":true}`, user); w.Code != 403 {
		t.Fatalf("plan unauthorized %d", w.Code)
	}
	w := request(h, "POST", "/api/v1/admin/plans", `{"name":"Starter","credits":100,"price_cents":1000,"currency":"CNY","enabled":true}`, admin)
	if w.Code != 201 {
		t.Fatalf("create plan %d %s", w.Code, w.Body)
	}
	w = request(h, "GET", "/api/v1/plans", "", user)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Starter") {
		t.Fatalf("plan list %d %s", w.Code, w.Body)
	}
	if w = request(h, "POST", "/api/v1/account/orders", `{"plan_id":1}`, user); w.Code != 503 || !strings.Contains(w.Body.String(), "payment_unavailable") {
		t.Fatalf("payment unavailable %d %s", w.Code, w.Body)
	}
	if w = request(h, "GET", "/api/v1/account/orders", "", user); w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("orders %d %s", w.Code, w.Body)
	}
	w = request(h, "POST", "/api/v1/admin/redemption-codes", `{"credits":25,"note":"support grant"}`, admin)
	if w.Code != 201 {
		t.Fatalf("create code %d %s", w.Code, w.Body)
	}
	var code struct{ Code string }
	_ = json.Unmarshal(w.Body.Bytes(), &code)
	w = request(h, "GET", "/api/v1/admin/redemption-codes", "", admin)
	if w.Code != 200 || strings.Contains(w.Body.String(), code.Code) {
		t.Fatalf("code leaked %d %s", w.Code, w.Body)
	}
	body, _ := json.Marshal(map[string]string{"code": code.Code})
	w = request(h, "POST", "/api/v1/account/redeem", string(body), user)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"balance":25`) {
		t.Fatalf("redeem %d %s", w.Code, w.Body)
	}
	if w = request(h, "POST", "/api/v1/account/redeem", string(body), user); w.Code != 409 {
		t.Fatalf("repeat redemption %d", w.Code)
	}
	w = request(h, "GET", "/api/v1/account/ledger", "", user)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "redemption") {
		t.Fatalf("ledger %d %s", w.Code, w.Body)
	}
}
func TestConcurrentRedemptionCreditsOneAccountOnce(t *testing.T) {
	s, h := setup(t)
	second := New(replicaStore(t, s), Options{Origin: "http://example.com"})
	a := register(t, h, "alice")
	b := register(t, h, "bob")
	admin := register(t, h, "operator")
	_, _ = s.Pool.Exec(context.Background(), "UPDATE users SET role='admin' WHERE username='operator'")
	w := request(h, "POST", "/api/v1/admin/redemption-codes", `{"credits":30,"note":"race"}`, admin)
	if w.Code != 201 {
		t.Fatalf("create code %d", w.Code)
	}
	var c struct{ Code string }
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	body, _ := json.Marshal(map[string]string{"code": c.Code})
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	wg.Go(func() { statuses <- request(h, "POST", "/api/v1/account/redeem", string(body), a).Code })
	wg.Go(func() { statuses <- request(second, "POST", "/api/v1/account/redeem", string(body), b).Code })
	wg.Wait()
	close(statuses)
	success := 0
	for status := range statuses {
		if status == 200 {
			success++
		} else if status != 409 {
			t.Errorf("unexpected status %d", status)
		}
	}
	if success != 1 {
		t.Fatalf("redemptions %d", success)
	}
	var total, count int64
	_ = s.Pool.QueryRow(context.Background(), "SELECT sum(balance) FROM wallets").Scan(&total)
	_ = s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM ledger WHERE kind='redemption'").Scan(&count)
	if total != 30 || count != 1 {
		t.Fatalf("total=%d ledger=%d", total, count)
	}
}
func TestAdminLedgerFiltersAndPersonalIsolation(t *testing.T) {
	s, h := setup(t)
	alice := register(t, h, "alice")
	bob := register(t, h, "bob")
	admin := register(t, h, "operator")
	_, _ = s.Pool.Exec(context.Background(), "UPDATE users SET role='admin' WHERE username='operator'")
	if w := request(h, "GET", "/api/v1/admin/ledger", "", alice); w.Code != 403 {
		t.Fatalf("admin ledger unauthorized %d", w.Code)
	}
	request(h, "POST", "/api/v1/admin/users/1/balance", `{"delta":5,"note":"only alice","idempotency_key":"one"}`, admin)
	w := request(h, "GET", "/api/v1/admin/ledger?user_id=1&actor_id=3&kind=adjustment", "", admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "only alice") {
		t.Fatalf("admin ledger %d %s", w.Code, w.Body)
	}
	w = request(h, "GET", "/api/v1/account/ledger", "", bob)
	if w.Code != 200 || strings.Contains(w.Body.String(), "only alice") {
		t.Fatalf("personal ledger isolation %d %s", w.Code, w.Body)
	}
	w = request(h, "GET", "/api/v1/account/ledger?kind=redemption", "", alice)
	if w.Code != 200 || strings.Contains(w.Body.String(), "only alice") {
		t.Fatalf("personal ledger filter %d %s", w.Code, w.Body)
	}
}
