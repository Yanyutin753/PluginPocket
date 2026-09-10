package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/store"
)

func newTeam(t *testing.T, h http.Handler, c *http.Cookie) int64 {
	t.Helper()
	w := request(h, "POST", "/api/v1/account/teams", `{"name":"Builders"}`, c)
	if w.Code != 201 {
		t.Fatalf("create team %d %s", w.Code, w.Body)
	}
	var p struct{ Item struct{ ID int64 } }
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	return p.Item.ID
}
func invite(t *testing.T, h http.Handler, c *http.Cookie, id int64) string {
	t.Helper()
	w := request(h, "POST", fmt.Sprintf("/api/v1/account/teams/%d/invites", id), `{}`, c)
	if w.Code != 201 {
		t.Fatalf("invite %d %s", w.Code, w.Body)
	}
	var p struct{ Code string }
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	return p.Code
}
func TestTeamWalletMembershipAndRemovalAuthorization(t *testing.T) {
	s, h := setup(t)
	other := replicaStore(t, s)
	second := New(other, Options{Origin: "http://example.com"})
	owner := register(t, h, "alice")
	member := register(t, h, "bob")
	admin := register(t, h, "operator")
	ctx := context.Background()
	_, _ = s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='operator'")
	id := newTeam(t, h, owner)
	base := fmt.Sprintf("/api/v1/account/teams/%d", id)
	if w := request(h, "GET", base, "", member); w.Code != 404 {
		t.Fatalf("nonmember visibility %d", w.Code)
	}
	tokenBody := fmt.Sprintf(`{"name":"Team","team_id":%d}`, id)
	if w := request(h, "POST", "/api/v1/account/tokens", tokenBody, member); w.Code != 404 {
		t.Fatalf("nonmember token %d", w.Code)
	}
	code := invite(t, h, owner, id)
	join := fmt.Sprintf(`{"code":%q}`, code)
	if w := request(second, "POST", "/api/v1/account/team-invites/accept", join, member); w.Code != 200 {
		t.Fatalf("accept %d %s", w.Code, w.Body)
	}
	if w := request(second, "POST", "/api/v1/account/team-invites/accept", join, member); w.Code != 409 {
		t.Fatalf("repeat invite %d", w.Code)
	}
	request(h, "POST", "/api/v1/admin/users/1/balance", `{"delta":20,"note":"fund","idempotency_key":"fund"}`, admin)
	for range 2 {
		if w := request(h, "POST", base+"/fund", `{"credits":10,"idempotency_key":"transfer"}`, owner); w.Code != 200 {
			t.Fatalf("fund %d %s", w.Code, w.Body)
		}
	}
	w := request(h, "GET", base, "", owner)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"balance":10`) {
		t.Fatalf("team balance %d %s", w.Code, w.Body)
	}
	w = request(h, "POST", "/api/v1/account/tokens", tokenBody, member)
	if w.Code != 201 {
		t.Fatalf("team token %d %s", w.Code, w.Body)
	}
	var token struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &token)
	p, e := s.AuthToken(ctx, token.Token)
	if e != nil {
		t.Fatal(e)
	}
	call, e := other.Reserve(ctx, p.UserID, p.TokenID, p.WalletID, "echo", 2, "teamcall")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, call.ID, true, 0); e != nil {
		t.Fatal(e)
	}
	if w = request(second, "DELETE", base+"/members/2", "", owner); w.Code != 204 {
		t.Fatalf("remove member %d %s", w.Code, w.Body)
	}
	if _, e = s.AuthToken(ctx, token.Token); e != store.ErrUnauthorized {
		t.Fatalf("removed member token: %v", e)
	}
	if _, e = s.Reserve(ctx, p.UserID, p.TokenID, p.WalletID, "echo", 1, "removed"); e != store.ErrUnauthorized {
		t.Fatalf("removed member reserve: %v", e)
	}
	if w = request(h, "DELETE", base+"/members/1", "", owner); w.Code != 409 {
		t.Fatalf("last owner exit %d", w.Code)
	}
	var total int64
	_ = s.Pool.QueryRow(ctx, "SELECT sum(balance) FROM wallets").Scan(&total)
	if total != 18 {
		t.Fatalf("shared wallet accounting total=%d", total)
	}
}
func TestTeamSeatsAndOwnerTransfer(t *testing.T) {
	_, h := setup(t)
	owner := register(t, h, "alice")
	member := register(t, h, "bob")
	third := register(t, h, "charlie")
	id := newTeam(t, h, owner)
	base := fmt.Sprintf("/api/v1/account/teams/%d", id)
	if w := request(h, "PATCH", base, `{"seat_limit":1}`, owner); w.Code != 200 {
		t.Fatalf("set seats %d %s", w.Code, w.Body)
	}
	code := invite(t, h, owner, id)
	body := fmt.Sprintf(`{"code":%q}`, code)
	if w := request(h, "POST", "/api/v1/account/team-invites/accept", body, member); w.Code != 409 {
		t.Fatalf("full seats %d", w.Code)
	}
	request(h, "PATCH", base, `{"seat_limit":2}`, owner)
	if w := request(h, "POST", "/api/v1/account/team-invites/accept", body, member); w.Code != 200 {
		t.Fatalf("accept after seat increase %d", w.Code)
	}
	if w := request(h, "PATCH", base, `{"owner_user_id":3}`, owner); w.Code != 400 {
		t.Fatalf("transfer to nonmember %d", w.Code)
	}
	if w := request(h, "PATCH", base, `{"seat_limit":1}`, owner); w.Code != 409 {
		t.Fatalf("shrink below members %d", w.Code)
	}
	if w := request(h, "PATCH", base, `{"owner_user_id":2}`, owner); w.Code != 200 {
		t.Fatalf("transfer %d %s", w.Code, w.Body)
	}
	if w := request(h, "POST", base+"/invites", `{}`, owner); w.Code != 403 {
		t.Fatalf("former owner create invite %d", w.Code)
	}
	if w := request(h, "DELETE", base+"/members/1", "", owner); w.Code != 204 {
		t.Fatalf("former owner leaves %d", w.Code)
	}
	if w := request(h, "GET", base+"/usage", "", third); w.Code != 404 {
		t.Fatalf("nonmember usage %d", w.Code)
	}
}
func TestInviteRejectsUnrecognizedInput(t *testing.T) {
	_, h := setup(t)
	owner := register(t, h, "alice")
	id := newTeam(t, h, owner)
	w := request(h, "POST", fmt.Sprintf("/api/v1/account/teams/%d/invites", id), `{"role":"owner"}`, owner)
	if w.Code != 400 {
		t.Fatalf("unrecognized invite input accepted: %d", w.Code)
	}
}

type delayedBody struct {
	entered chan struct{}
	release chan struct{}
	reader  *strings.Reader
	once    sync.Once
}

func (d *delayedBody) Read(p []byte) (int, error) {
	d.once.Do(func() { close(d.entered) })
	<-d.release
	return d.reader.Read(p)
}
func TestTeamWriteDoesNotLockWhileWaitingForRequestBody(t *testing.T) {
	_, h := setup(t)
	owner := register(t, h, "alice")
	id := newTeam(t, h, owner)
	path := fmt.Sprintf("/api/v1/account/teams/%d", id)
	body := &delayedBody{entered: make(chan struct{}), release: make(chan struct{}), reader: strings.NewReader(`{"name":"Slow request"}`)}
	r := httptest.NewRequest("PATCH", "http://example.com"+path, body)
	r.Header.Set("Origin", "http://example.com")
	r.AddCookie(owner)
	first := make(chan struct{})
	go func() { h.ServeHTTP(httptest.NewRecorder(), r); close(first) }()
	<-body.entered
	second := make(chan int, 1)
	go func() { second <- request(h, "PATCH", path, `{"seat_limit":6}`, owner).Code }()
	select {
	case code := <-second:
		if code != 200 {
			t.Errorf("second mutation %d", code)
		}
	case <-time.After(time.Second):
		t.Error("waiting for request body held the team database lock")
	}
	close(body.release)
	<-first
}
