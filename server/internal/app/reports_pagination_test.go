package app

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestUsageSummaryPaginationRetainsEveryGroup(t *testing.T) {
	user, admin, handler, exec := adminFixture(t)
	response := request(handler, http.MethodPost, "/api/v1/account/tokens", `{"name":"summary"}`, user)
	if response.Code != http.StatusCreated {
		t.Fatal(response.Code)
	}
	exec(`INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key)
 SELECT t.user_id,t.id,t.wallet_id,'summary-'||n,0,'denied','summary-'||n FROM tokens t CROSS JOIN generate_series(1,51) n`)
	cursor := ""
	seen := map[string]bool{}
	for _, count := range []int{50, 1} {
		response = request(handler, http.MethodGet, "/api/v1/admin/usage/summary?days=7&cursor="+cursor, "", admin)
		if response.Code != http.StatusOK {
			t.Fatal(response.Code)
		}
		var page struct {
			Items []struct{ Tool string }
			Next  string `json:"next_cursor"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != count {
			t.Fatalf("got %d groups, want %d", len(page.Items), count)
		}
		for _, item := range page.Items {
			if seen[item.Tool] {
				t.Fatalf("duplicate group %q", item.Tool)
			}
			seen[item.Tool] = true
		}
		cursor = page.Next
	}
	if len(seen) != 51 || cursor != "" {
		t.Fatalf("incomplete pagination: groups=%d cursor=%q", len(seen), cursor)
	}
}
