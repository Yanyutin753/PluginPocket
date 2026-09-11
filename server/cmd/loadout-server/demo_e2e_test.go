package main

import (
	"net/http/httptest"
	"testing"

	"github.com/Yanyutin753/loadout/server/internal/demo"
)

func TestProductJourneyDemoData(t *testing.T) {
	f := newProductFixture(t)
	p := f.server()
	upstream := httptest.NewServer(demo.Upstream())
	defer upstream.Close()
	// The seed must exercise real account, team, token and accounting handlers.
	if err := demo.Seed(f.ctx, p.origin, upstream.URL, "operator", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	var users int
	if err := f.conn.QueryRow(f.ctx, "SELECT count(*) FROM users WHERE username LIKE 'demo_%'").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 60 {
		t.Fatalf("demo accounts=%d want=60", users)
	}
	for query, minimum := range map[string]int{
		"SELECT count(*) FROM usage_logs WHERE status='error' AND cost=0":                   100,
		"SELECT count(*) FROM usage_logs":                                                   700,
		"SELECT count(*) FROM redemption_codes":                                             60,
		"SELECT count(*) FROM plans":                                                        12,
		"SELECT count(*) FROM teams":                                                        12,
		"SELECT count(*) FROM tokens t JOIN users u ON u.id=t.user_id WHERE u.role='admin'": 60,
	} {
		var n int
		if err := f.conn.QueryRow(f.ctx, query).Scan(&n); err != nil || n < minimum {
			t.Fatalf("demo volume: %s count=%d minimum=%d err=%v", query, n, minimum, err)
		}
	}
	for _, query := range []string{
		"SELECT count(*) FROM tokens t JOIN users u ON u.id=t.user_id WHERE u.role='admin' AND t.revoked_at IS NULL",
		"SELECT count(*) FROM usage_logs l JOIN users u ON u.id=l.user_id WHERE u.role='admin' AND l.status='ok'",
		"SELECT count(*) FROM team_members m JOIN users u ON u.id=m.user_id WHERE u.role='admin' AND m.role='owner'",
	} {
		var n int
		if err := f.conn.QueryRow(f.ctx, query).Scan(&n); err != nil || n == 0 {
			t.Fatalf("missing administrator demo scenario: %s count=%d err=%v", query, n, err)
		}
	}
	for _, query := range []string{
		"SELECT count(*) FROM teams",
		"SELECT count(*) FROM team_members WHERE role='member'",
		"SELECT count(*) FROM plans WHERE enabled",
		"SELECT count(*) FROM plans WHERE NOT enabled",
		"SELECT count(*) FROM tokens WHERE revoked_at IS NOT NULL",
		"SELECT count(*) FROM redemption_codes WHERE redeemed_at IS NOT NULL",
		"SELECT count(*) FROM redemption_codes WHERE redeemed_at IS NULL",
		"SELECT count(*) FROM device_authorizations WHERE consumed_at IS NOT NULL",
		"SELECT count(*) FROM usage_logs WHERE status='ok'",
		"SELECT count(*) FROM usage_logs WHERE status='denied'",
		"SELECT count(*) FROM usage_logs WHERE status='error' AND cost=0",
	} {
		var n int
		if err := f.conn.QueryRow(f.ctx, query).Scan(&n); err != nil || n == 0 {
			t.Fatalf("missing demo scenario: %s count=%d err=%v", query, n, err)
		}
	}
	var inconsistent int
	if err := f.conn.QueryRow(f.ctx, "SELECT count(*) FROM wallets w WHERE balance <> COALESCE((SELECT sum(delta) FROM ledger l WHERE l.wallet_id=w.id),0)").Scan(&inconsistent); err != nil || inconsistent != 0 {
		t.Fatalf("demo wallet/ledger mismatch=%d err=%v", inconsistent, err)
	}
	if err := demo.Expand(f.ctx, p.origin, "operator", "correct horse battery staple"); err == nil {
		t.Fatal("repeated expansion must refuse duplicate financial changes")
	}
	if err := demo.Seed(f.ctx, p.origin, "", "operator", "correct horse battery staple"); err == nil {
		t.Fatal("repeated seed must refuse duplicate demo data")
	}
}
