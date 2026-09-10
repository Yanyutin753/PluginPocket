package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProductJourneyRecoveryUsesDatabaseClock(t *testing.T) {
	f := newProductFixture(t)
	p := f.server()
	user, id := p.register("clock-recovery")
	token := p.token(user)
	p.stop(true)
	database, err := store.Open(f.ctx, f.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	principal, err := database.AuthToken(f.ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	call, err := database.Reserve(f.ctx, id, principal.TokenID, principal.WalletID, "echo", 7, "clock-recovery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.conn.Exec(f.ctx, "UPDATE usage_logs SET created_at=statement_timestamp()-interval '6 minutes' WHERE id=$1", call.ID); err != nil {
		t.Fatal(err)
	}
	// synctest puts this worker's clock in 2000; PostgreSQL keeps actual time.
	synctest.Test(t, func(t *testing.T) {
		count, err := recoverExpiredCalls(f.ctx, database)
		if err != nil || count != 1 {
			t.Fatalf("skewed recovery worker: count=%d err=%v", count, err)
		}
	})
	if f.balance(id) != 1000 {
		t.Fatal("expired call was not refunded by the skewed worker")
	}
}

func TestProductJourneyCatalogChangesReachOtherReplicas(t *testing.T) {
	f := newProductFixture(t)
	a, b := f.server(), f.server()
	user, id := a.register("catalog-peer")
	session := b.mcp(a.token(user))
	list, err := session.ListTools(f.ctx, nil)
	if err != nil || len(list.Tools) < 2 {
		t.Fatal("cannot warm peer catalog")
	}
	admin := a.admin()
	var toolID int64
	if err := f.conn.QueryRow(f.ctx, "SELECT id FROM tools WHERE key='echo'").Scan(&toolID); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/admin/tools/%d", toolID)
	update := map[string]any{"key": "echo", "name": "Echo", "kind": "builtin", "enabled": false, "units_per_call": 1, "input_schema": map[string]any{"type": "object", "properties": map[string]any{"message": map[string]any{"type": "string"}}}}
	a.api(admin, "PATCH", path, update, 200)
	list, err = session.ListTools(f.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "echo" {
			t.Fatal("another replica advertised a disabled tool from its warm catalog")
		}
	}
	update["enabled"], update["units_per_call"] = true, 9
	a.api(admin, "PATCH", path, update, 200)
	result, err := session.CallTool(f.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "cross replica"}})
	if err != nil || result.IsError || f.balance(id) != 991 {
		t.Fatalf("peer did not apply re-enabled tool and current price: err=%v result=%v balance=%d", err, result, f.balance(id))
	}
}

func TestProductJourneyWithoutStickySessionsSurvivesReplicaExit(t *testing.T) {
	f := newProductFixture(t)
	var proxies []*httputil.ReverseProxy
	var replicas []*productProcess
	var calls [2]atomic.Int64
	var sequence atomic.Uint64
	probe := &http.Client{Timeout: 200 * time.Millisecond}
	balancer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := int(sequence.Add(1)-1) % len(proxies)
		for offset := range proxies {
			i := (start + offset) % len(proxies)
			response, err := probe.Get(replicas[i].origin + "/readyz")
			if err != nil {
				continue
			}
			if err := response.Body.Close(); err != nil {
				t.Errorf("fixture cleanup failed: %v", err)
			}
			if response.StatusCode != 200 {
				continue
			}
			calls[i].Add(1)
			proxies[i].ServeHTTP(w, r)
			return
		}
		http.Error(w, "no healthy replica", http.StatusServiceUnavailable)
	}))
	t.Cleanup(balancer.Close)
	for range 2 {
		p := f.serverWithOrigin(balancer.URL)
		target, _ := url.Parse(p.origin)
		replicas = append(replicas, p)
		proxies = append(proxies, httputil.NewSingleHostReverseProxy(target))
	}
	p := &productProcess{f: f, origin: balancer.URL}
	user, id := p.register("balanced-user")
	p.api(user, "GET", "/account/me", nil, 200)
	token := p.token(user)
	session := p.mcp(token)
	for range 4 {
		result, err := session.CallTool(f.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "balanced"}})
		if err != nil || result.IsError {
			t.Fatalf("balanced MCP call: %v %v", err, result)
		}
	}
	device := p.api(p.client(), "POST", "/device/authorize", map[string]any{}, 200)
	p.api(user, "POST", "/account/devices/approve", map[string]any{"user_code": device["user_code"]}, 204)
	if device["verification_uri"] != balancer.URL+"/devices" {
		t.Fatal("device authorization leaked a replica-local URL")
	}
	team := p.api(user, "POST", "/account/teams", map[string]string{"name": "Balanced team"}, 201)["item"].(map[string]any)
	teamID := int64(team["id"].(float64))
	fund := map[string]any{"credits": 17, "idempotency_key": "across-replicas"}
	for range 2 {
		p.api(user, "POST", fmt.Sprintf("/account/teams/%d/fund", teamID), fund, 200)
	}
	if calls[0].Load() == 0 || calls[1].Load() == 0 {
		t.Fatal("journey did not reach both replicas")
	}
	// Stop the issuing node before device consumption; the load balancer probes
	// readiness and routes subsequent requests to the remaining real process.
	replicas[0].stop(false)
	p.api(user, "GET", "/account/me", nil, 200)
	deviceToken := p.api(p.client(), "POST", "/device/token", map[string]any{"device_code": device["device_code"]}, 200)["token"].(string)
	deviceSession := p.mcp(deviceToken)
	result, err := deviceSession.CallTool(f.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "after replica exit"}})
	if err != nil || result.IsError || f.balance(id) != 978 {
		t.Fatalf("surviving replica lost authorization/accounting: %v balance=%d", err, f.balance(id))
	}
	p.api(p.client(), "POST", "/device/token", map[string]any{"device_code": device["device_code"]}, 400)
	// Restart the old node and replay its old session cookie after logout.
	origin, _ := url.Parse(p.origin)
	cookies := user.Jar.Cookies(origin)
	p.api(user, "POST", "/auth/logout", nil, 204)
	replicas[0].start()
	user.Jar.SetCookies(origin, cookies)
	for range 2 {
		p.api(user, "GET", "/account/me", nil, 401)
	}
}
