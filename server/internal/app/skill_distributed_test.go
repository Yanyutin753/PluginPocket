package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
)

func TestDistributedSkillPublicationSerializesVersions(t *testing.T) {
	f := marketplaceApp(t, nil)
	path := "/api/v1/admin/marketplace/skills"
	initial := `{"slug":"serial-skill","name":"Serial skill","source":"inline","files":{"SKILL.md":"initial"}}`
	if w := request(f.handler, "POST", path, initial, f.admin); w.Code != 201 {
		t.Fatalf("seed: %d", w.Code)
	}
	name := fmt.Sprintf("skill_publish_%d", time.Now().UnixNano())
	u, err := url.Parse(f.store.Pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("application_name", name)
	u.RawQuery = q.Encode()
	replicas := []http.Handler{}
	for range 2 {
		s, err := store.Open(t.Context(), u.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(s.Close)
		replicas = append(replicas, New(s, Options{Origin: "http://example.com"}))
	}
	blocker, err := f.store.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback(t.Context()) }()
	if _, err = blocker.Exec(t.Context(), "SELECT id FROM marketplace_items WHERE slug='serial-skill' FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	replies := make(chan *httptest.ResponseRecorder, 2)
	for i, replica := range replicas {
		go func() {
			replies <- request(replica, "POST", path, fmt.Sprintf(`{"slug":"serial-skill","name":"Serial skill","source":"inline","files":{"SKILL.md":"revision-%d"}}`, i), f.admin)
		}()
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked int
		if err = f.store.Pool.QueryRow(t.Context(), "SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock'", name).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked == 2 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("both publishers did not reach database lock")
		case <-ticker.C:
		}
	}
	if err = blocker.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	versions := map[string]bool{}
	for range 2 {
		w := <-replies
		if w.Code != 200 {
			t.Fatalf("publish: %d %s", w.Code, w.Body)
		}
		var result struct{ Item struct{ Version string } }
		if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		versions[result.Item.Version] = true
	}
	if !versions["1.0.1"] || !versions["1.0.2"] {
		t.Fatalf("concurrent content reused a version: %v", versions)
	}
}
