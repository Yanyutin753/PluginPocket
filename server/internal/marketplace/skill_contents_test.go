package marketplace

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkillTextCompatibilityFetchesFileBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/example/skills/git/trees/HEAD":
			_, _ = w.Write([]byte(`{"tree":[{"type":"blob","mode":"100644","sha":"1111111111111111111111111111111111111111","size":8,"path":"skills/review/SKILL.md"}]}`))
		case "/repos/example/skills/git/blobs/1111111111111111111111111111111111111111":
			_, _ = w.Write([]byte(`{"type":"file","path":"skills/review/SKILL.md","encoding":"base64","content":"IyBSZXZpZXc="}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	files, err := ResolveSkillFiles(t.Context(), Options{BaseURL: server.URL}, "example/skills", "skills/review")
	if err != nil {
		t.Fatal(err)
	}
	if files["SKILL.md"] != "# Review" {
		t.Fatalf("files=%v", files)
	}
}
