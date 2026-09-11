package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestUsageDetailAuthorizationAndLegacyData(t *testing.T) {
	s, h := setup(t)
	alice := register(t, h, "alice")
	bob := register(t, h, "bob")
	admin := register(t, h, "operator")
	ctx := context.Background()
	if _, err := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE username='operator'"); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "POST", "/api/v1/account/tokens", `{"name":"test"}`, alice); w.Code != 201 {
		t.Fatal(w.Body)
	}
	var id int64
	if err := s.Pool.QueryRow(ctx, `INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,status,request_key) SELECT user_id,id,wallet_id,'echo','denied','detail' FROM tokens RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	own := fmt.Sprintf("/api/v1/account/usage/%d", id)
	team := newTeam(t, h, alice)
	for _, tc := range []struct {
		path   string
		cookie *http.Cookie
		code   int
	}{
		{own, alice, 200}, {own, bob, 404}, {own, nil, 401},
		{fmt.Sprintf("/api/v1/admin/usage/%d", id), admin, 200},
		{fmt.Sprintf("/api/v1/admin/usage/%d", id), bob, 403},
		{fmt.Sprintf("/api/v1/account/teams/%d/usage/%d", team, id), alice, 404},
		{"/api/v1/account/usage/0", alice, 400},
	} {
		w := request(h, "GET", tc.path, "", tc.cookie)
		if w.Code != tc.code {
			t.Fatalf("%s: got %d want %d: %s", tc.path, w.Code, tc.code, w.Body)
		}
		if tc.code == 200 {
			var body struct{ Item map[string]any }
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"input_data", "output_data"} {
				v, ok := body.Item[key]
				if !ok || v != nil {
					t.Fatalf("legacy %s=%v present=%v", key, v, ok)
				}
			}
		}
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE usage_logs SET input_data=$2,output_data=$3 WHERE id=$1`, id, `{"token":"original-secret"}`, `{"content":[]}`); err != nil {
		t.Fatal(err)
	}
	w := request(h, "GET", own, "", alice)
	var saved struct {
		Item struct {
			Input  string `json:"input_data"`
			Output string `json:"output_data"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Item.Input != `{"token":"original-secret"}` || saved.Item.Output != `{"content":[]}` {
		t.Fatalf("original payload not returned: %s", w.Body)
	}
	if w := request(h, "GET", "/api/v1/account/usage", "", alice); strings.Contains(w.Body.String(), "original-secret") {
		t.Fatal("list leaked payload")
	}
	if w := request(h, "POST", "/api/v1/account/tokens", fmt.Sprintf(`{"name":"team","team_id":%d}`, team), alice); w.Code != 201 {
		t.Fatal(w.Body)
	}
	var teamCall int64
	if err := s.Pool.QueryRow(ctx, `INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,status,request_key,input_data,output_data) SELECT user_id,id,wallet_id,'echo','ok','team-detail','{}','team output' FROM tokens WHERE name='team' RETURNING id`).Scan(&teamCall); err != nil {
		t.Fatal(err)
	}
	teamPath := fmt.Sprintf("/api/v1/account/teams/%d/usage/%d", team, teamCall)
	for _, tc := range []struct {
		cookie *http.Cookie
		code   int
	}{{alice, 200}, {bob, 404}, {nil, 401}} {
		w := request(h, "GET", teamPath, "", tc.cookie)
		if w.Code != tc.code {
			t.Fatalf("team detail %d want %d", w.Code, tc.code)
		}
		if tc.code == 200 && !strings.Contains(w.Body.String(), "team output") {
			t.Fatal("team payload missing")
		}
	}
}
