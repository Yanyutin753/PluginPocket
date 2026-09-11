package marketplace

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkillContentsFetchesFileBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/example/skills/contents/skills/review":
			_, _ = w.Write([]byte(`[{"type":"file","name":"SKILL.md","path":"skills/review/SKILL.md"}]`))
		case "/repos/example/skills/contents/skills/review/SKILL.md":
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
