package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReviewOwnerRoleAfterTeamLockWait(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	alice := register(t, h, "alice")
	bob := register(t, h, "bob")
	id := newTeam(t, h, alice)
	code := invite(t, h, alice, id)
	if w := request(h, "POST", "/api/v1/account/team-invites/accept", fmt.Sprintf(`{"code":%q}`, code), bob); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
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
	if _, e = tx.Exec(ctx, "UPDATE team_members SET role=CASE WHEN user_id=2 THEN 'owner' ELSE 'member' END WHERE team_id=$1", id); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "UPDATE teams SET name=name WHERE id=$1", id); e != nil {
		t.Fatal(e)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- request(h, "PATCH", fmt.Sprintf("/api/v1/account/teams/%d", id), `{"name":"Old owner mutation"}`, alice)
	}()
	deadline := time.Now().Add(3 * time.Second)
	waiting := false
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
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w := <-done:
		if w.Code != 403 {
			t.Fatalf("demoted owner request after lock wait: got %d %s; want 403", w.Code, w.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request stuck")
	}
}

func TestReviewRemovedMemberCannotMintSurvivingToken(t *testing.T) {
	s, h := setup(t)
	ctx := context.Background()
	alice := register(t, h, "alice")
	bob := register(t, h, "bob")
	id := newTeam(t, h, alice)
	join := func() {
		code := invite(t, h, alice, id)
		if w := request(h, "POST", "/api/v1/account/team-invites/accept", fmt.Sprintf(`{"code":%q}`, code), bob); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	join()
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int
	if e = tx.QueryRow(ctx, "SELECT pg_backend_pid() FROM teams WHERE id=$1 FOR UPDATE", id).Scan(&pid); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "LOCK TABLE tokens IN SHARE MODE"); e != nil {
		t.Fatal(e)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- request(h, "POST", "/api/v1/account/tokens", fmt.Sprintf(`{"name":"Concurrent token","team_id":%d}`, id), bob)
	}()
	deadline := time.Now().Add(3 * time.Second)
	waiting := false
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
		t.Fatal("token insert did not wait")
	}
	if _, e = tx.Exec(ctx, "DELETE FROM team_members WHERE team_id=$1 AND user_id=2", id); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "UPDATE tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE wallet_id=(SELECT id FROM wallets WHERE team_id=$1) AND user_id=2", id); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	w := <-done
	if w.Code == 404 {
		return
	}
	if w.Code != 201 {
		t.Fatalf("unexpected token response %d %s", w.Code, w.Body)
	}
	var payload struct{ Token string }
	if e = json.Unmarshal(w.Body.Bytes(), &payload); e != nil {
		t.Fatal(e)
	}
	join()
	if _, e = s.AuthToken(ctx, payload.Token); e == nil {
		t.Fatal("token minted across removal survives revocation and becomes active on rejoin")
	}
}
