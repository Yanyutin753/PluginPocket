package marketplace

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func serveRepo(t *testing.T, repo func() map[string][]byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, ok := repo()[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)
	return server
}

func gitClone(t *testing.T, url, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary required")
	}
	clone := exec.CommandContext(t.Context(), "git", "clone", "--quiet", url, dir)
	clone.Env = append(clone.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir())
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}
	t.Log("clone ok")
}

func readCloned(t *testing.T, dir, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("cloned file %s missing: %v", path, err)
	}
	return string(raw)
}

// BuildGitMarketplace 把导出树打包成哑 HTTP git 裸仓文件集：
// 任何静态文件服务器（含 PluginPocket 自身）即可被 `git clone`，从而被
// `codex plugin marketplace add http://host/marketplace.git` 接入。
func TestGitMarketplaceCloneableViaDumbHTTP(t *testing.T) {
	tree := map[string][]byte{
		".agents/plugins/marketplace.json":           []byte(`{"name":"pluginpocket","plugins":[]}`),
		"plugins/deepwiki/mcp.json":                  []byte(`{"mcpServers":{"deepwiki":{"type":"streamable-http","url":"https://mcp.deepwiki.com/mcp"}}}`),
		"plugins/deepwiki/.codex-plugin/plugin.json": []byte(`{"name":"deepwiki","version":"1.0.0"}`),
	}
	repo, err := BuildGitMarketplace(tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if refs := string(repo["info/refs"]); !strings.Contains(refs, "refs/heads/main") || len(strings.TrimSpace(refs)) < 41 {
		t.Fatalf("info/refs malformed: %q", refs)
	}
	if head := string(repo["HEAD"]); !strings.Contains(head, "refs/heads/main") {
		t.Fatalf("HEAD malformed: %q", head)
	}
	server := serveRepo(t, func() map[string][]byte { return repo })
	clone := filepath.Join(t.TempDir(), "clone")
	gitClone(t, server.URL, clone)
	if got := readCloned(t, clone, ".agents/plugins/marketplace.json"); !strings.Contains(got, "pluginpocket") {
		t.Fatalf("cloned manifest wrong: %s", got)
	}
	if got := readCloned(t, clone, "plugins/deepwiki/mcp.json"); !strings.Contains(got, "streamable-http") {
		t.Fatalf("cloned mcp.json wrong: %s", got)
	}
}

// 同一服务器重建后必须反映新目录（克隆默认跟随远端最新 main）。
func TestGitMarketplaceServesFreshContentAfterRebuild(t *testing.T) {
	var mu sync.Mutex
	first, err := BuildGitMarketplace(map[string][]byte{".agents/plugins/marketplace.json": []byte(`{"name":"v1"}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	current := first
	server := serveRepo(t, func() map[string][]byte {
		mu.Lock()
		defer mu.Unlock()
		return current
	})
	cloneA := filepath.Join(t.TempDir(), "a")
	gitClone(t, server.URL, cloneA)
	if got := readCloned(t, cloneA, ".agents/plugins/marketplace.json"); !strings.Contains(got, "v1") {
		t.Fatalf("first clone wrong: %s", got)
	}
	second, err := BuildGitMarketplace(map[string][]byte{".agents/plugins/marketplace.json": []byte(`{"name":"v2"}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	current = second
	mu.Unlock()
	cloneB := filepath.Join(t.TempDir(), "b")
	gitClone(t, server.URL, cloneB)
	if got := readCloned(t, cloneB, ".agents/plugins/marketplace.json"); !strings.Contains(got, "v2") {
		t.Fatalf("stale marketplace served: %s", got)
	}
}

func TestGitRegistryServesAndInvalidates(t *testing.T) {
	s := marketplaceDB(t)
	registry := NewGitRegistry(s.Pool, Options{})
	// registry.Files 返回哑 HTTP 裸仓文件集（git 对象 + refs），用真实 clone 验证内容。
	var mu sync.Mutex
	serveRegistry := func() map[string][]byte {
		files, err := registry.Files(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if files["info/refs"] == nil || files["HEAD"] == nil {
			t.Fatalf("bare repo metadata missing: %d entries", len(files))
		}
		return files
	}
	current := serveRegistry()
	server := serveRepo(t, func() map[string][]byte {
		mu.Lock()
		defer mu.Unlock()
		return current
	})
	cloneA := filepath.Join(t.TempDir(), "a")
	gitClone(t, server.URL, cloneA)
	if got := readCloned(t, cloneA, ".agents/plugins/marketplace.json"); !strings.Contains(got, "deepwiki") {
		t.Fatalf("registry clone missing seed plugin: %s", got)
	}
	if _, err := s.Pool.Exec(t.Context(), `INSERT INTO marketplace_items(slug,name,source,transport,kind)
	 VALUES ('fresh-probe','Fresh','curated','gateway','mcp')`); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	current = serveRegistry()
	mu.Unlock()
	cloneB := filepath.Join(t.TempDir(), "b")
	gitClone(t, server.URL, cloneB)
	if got := readCloned(t, cloneB, "plugins/fresh-probe/.codex-plugin/plugin.json"); !strings.Contains(got, "pluginpocket") {
		t.Fatalf("invalidated registry must serve new gateway plugin: %s", got)
	}
}
