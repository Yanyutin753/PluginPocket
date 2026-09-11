package app

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestBundleAuthoringUpdatesAndKeepsKinds(t *testing.T) {
	f := marketplaceApp(t, nil)
	for _, step := range []struct {
		body    string
		code    int
		version string
	}{
		{`{"slug":"my-pack","name":"My pack","includes":["deepwiki"]}`, 201, "1.0.0"},
		{`{"slug":"my-pack","name":"Updated pack","includes":["deepwiki"]}`, 200, "1.0.1"},
		{`{"slug":"my-pack","name":"Updated pack","includes":["deepwiki"]}`, 200, "1.0.1"},
		{`{"slug":"deepwiki","name":"Collision","includes":["my-pack"]}`, 400, ""},
		{`{"slug":"deepwiki","name":"Collision","includes":["context7"]}`, 409, ""},
	} {
		w := request(f.handler, "POST", "/api/v1/admin/marketplace/bundles", step.body, f.admin)
		if w.Code != step.code {
			t.Fatalf("code=%d want=%d body=%s", w.Code, step.code, w.Body)
		}
		if step.version != "" && !strings.Contains(w.Body.String(), `"version":"`+step.version+`"`) {
			t.Fatalf("version missing: %s", w.Body)
		}
	}
}

func TestGitHubSkillSyncPublishesStableSnapshot(t *testing.T) {
	content := "first"
	f := marketplaceApp(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			_, _ = fmt.Fprintf(w, `{"tree":[{"path":"skills/pro/SKILL.md","type":"blob","mode":"100644","sha":"1111111111111111111111111111111111111111","size":%d}]}`, len(content))
			return
		}
		_, _ = fmt.Fprintf(w, `{"encoding":"base64","content":%q}`, base64Of(content))
	})
	body := `{"slug":"snapshot-skill","name":"Snapshot","source":"github","repo":"example/skills","path":"skills/pro"}`
	w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin)
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body)
	}
	content = "second"
	w = request(f.handler, "GET", "/api/v1/marketplace/snapshot-skill/files", "", f.user)
	if !strings.Contains(w.Body.String(), "first") {
		t.Fatalf("published content changed without sync: %s", w.Body)
	}
	w = request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"version":"1.0.1"`) {
		t.Fatalf("sync must bump version: %d %s", w.Code, w.Body)
	}
}

func TestSkillAuthoringAcceptsLongContentAndUpdatesBundles(t *testing.T) {
	f := marketplaceApp(t, nil)
	body := fmt.Sprintf(`{"slug":"long-skill","name":"Long skill","source":"inline","files":{"SKILL.md":%q}}`, strings.Repeat("content ", 3000))
	w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin)
	if w.Code != 201 {
		t.Fatalf("long valid skill: %d %s", w.Code, w.Body)
	}
	w = request(f.handler, "POST", "/api/v1/admin/marketplace/bundles", `{"slug":"long-pack","name":"Long pack","includes":["long-skill"]}`, f.admin)
	if w.Code != 201 {
		t.Fatalf("bundle: %d %s", w.Code, w.Body)
	}
	body = `{"slug":"long-skill","name":"Long skill","source":"inline","files":{"SKILL.md":"changed"}}`
	w = request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin)
	if w.Code != 200 {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	var version string
	if err := f.store.Pool.QueryRow(t.Context(), "SELECT version FROM marketplace_items WHERE slug='long-pack'").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "1.0.1" {
		t.Fatalf("bundle version=%s", version)
	}
}

func TestBundleRejectsUninstallableMembersAndSkillKindCollision(t *testing.T) {
	f := marketplaceApp(t, nil)
	if _, err := f.store.Pool.Exec(t.Context(), `INSERT INTO marketplace_items(slug,name,source,transport,kind) VALUES('unresolved','Unresolved','github','unknown','mcp')`); err != nil {
		t.Fatal(err)
	}
	w := request(f.handler, "POST", "/api/v1/admin/marketplace/bundles", `{"slug":"broken-pack","name":"Broken pack","includes":["unresolved"]}`, f.admin)
	if w.Code != 400 {
		t.Fatalf("uninstallable bundle: %d %s", w.Code, w.Body)
	}
	w = request(f.handler, "POST", "/api/v1/admin/marketplace/skills", `{"slug":"deepwiki","name":"Collision","source":"inline","files":{"SKILL.md":"test"}}`, f.admin)
	if w.Code != 409 {
		t.Fatalf("kind collision: %d %s", w.Code, w.Body)
	}
}

func TestConcurrentSkillAndBundleEditing(t *testing.T) {
	f := marketplaceApp(t, nil)
	skill := `{"slug":"member-skill","name":"Member","source":"inline","files":{"SKILL.md":"first"}}`
	bundle := `{"slug":"member-pack","name":"Pack","includes":["member-skill"]}`
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", skill, f.admin); w.Code != 201 {
		t.Fatal(w.Code)
	}
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/bundles", bundle, f.admin); w.Code != 201 {
		t.Fatal(w.Code)
	}
	replies := make(chan int, 16)
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			<-start
			body := fmt.Sprintf(`{"slug":"member-skill","name":"Member","source":"inline","files":{"SKILL.md":"revision %d"}}`, i)
			replies <- request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin).Code
		}()
		go func() {
			<-start
			body := fmt.Sprintf(`{"slug":"member-pack","name":"Pack %d","includes":["member-skill"]}`, i)
			replies <- request(f.handler, "POST", "/api/v1/admin/marketplace/bundles", body, f.admin).Code
		}()
	}
	close(start)
	for range 16 {
		if code := <-replies; code != 200 {
			t.Errorf("concurrent save status=%d", code)
		}
	}
}
