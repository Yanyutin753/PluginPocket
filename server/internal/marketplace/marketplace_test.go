package marketplace

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
)

func marketplaceDB(t *testing.T) *store.Store {
	t.Helper()
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("real PostgreSQL URL required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("marketplace_%d", time.Now().UnixNano())
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = conn.Close(ctx)
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func githubFixture(t *testing.T, token string, bodies ...string) *httptest.Server {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			http.Error(w, "not found", 404)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "missing user agent", 400)
			return
		}
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		if calls >= len(bodies) {
			http.Error(w, "unexpected call", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bodies[calls]))
		calls++
	}))
	t.Cleanup(server.Close)
	return server
}

const firstPage = `{"total_count":3,"items":[
 {"full_name":"github/github-mcp-server","name":"github-mcp-server","description":"GitHub's official MCP Server","html_url":"https://github.com/github/github-mcp-server","homepage":"","stargazers_count":32852},
 {"full_name":"firecrawl/firecrawl-mcp-server","name":"firecrawl-mcp-server","description":"Firecrawl search","html_url":"https://github.com/firecrawl/firecrawl-mcp-server","homepage":"https://firecrawl.dev","stargazers_count":7431},
 {"full_name":"pluginpocket/has we!rd chars/and-is-a-very-long-repository-name-indeed","name":"odd","description":"","html_url":"https://example/odd","homepage":"","stargazers_count":5}
]}`

func TestFetchMapsRepositories(t *testing.T) {
	server := githubFixture(t, "sekret", firstPage)
	items, err := Fetch(t.Context(), Options{BaseURL: server.URL, Token: "sekret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items=%d", len(items))
	}
	first := items[0]
	if first.Slug != "github-github-mcp-server" || first.Name != "github-mcp-server" || first.RepoURL != "https://github.com/github/github-mcp-server" || first.Stars != 32852 {
		t.Fatalf("first item mapped wrong: %+v", first)
	}
	if items[2].Slug == "" || len(items[2].Slug) > 32 {
		t.Fatalf("long or invalid slug not sanitized: %q", items[2].Slug)
	}
}

func TestFetchFailsOnRefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	}))
	t.Cleanup(server.Close)
	if _, err := Fetch(t.Context(), Options{BaseURL: server.URL}); err == nil {
		t.Fatal("403 must surface as an error")
	}
}

func TestSyncUpsertsGitHubRowsAndKeepsCurated(t *testing.T) {
	s := marketplaceDB(t)
	var curatedBefore int
	if err := s.Pool.QueryRow(t.Context(), "SELECT count(*) FROM marketplace_items WHERE source='curated'").Scan(&curatedBefore); err != nil {
		t.Fatal(err)
	}
	server := githubFixture(t, "", firstPage, `{"total_count":1,"items":[
 {"full_name":"github/github-mcp-server","name":"github-mcp-server","description":"GitHub's official MCP Server","html_url":"https://github.com/github/github-mcp-server","homepage":"","stargazers_count":40000}
]}`)
	count, err := Sync(t.Context(), s.Pool, Options{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("first sync count=%d", count)
	}
	var transport, source string
	var stars int
	if err := s.Pool.QueryRow(t.Context(), "SELECT transport,source,stars FROM marketplace_items WHERE slug='github-github-mcp-server'").Scan(&transport, &source, &stars); err != nil {
		t.Fatal(err)
	}
	if transport != "unknown" || source != "github" || stars != 32852 {
		t.Fatalf("row wrong: transport=%s source=%s stars=%d", transport, source, stars)
	}
	var curatedName string
	if err := s.Pool.QueryRow(t.Context(), "SELECT name FROM marketplace_items WHERE slug='deepwiki'").Scan(&curatedName); err != nil {
		t.Fatalf("curated seed missing: %v", err)
	}
	count, err = Sync(t.Context(), s.Pool, Options{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("second sync count=%d", count)
	}
	if err := s.Pool.QueryRow(t.Context(), "SELECT stars FROM marketplace_items WHERE slug='github-github-mcp-server'").Scan(&stars); err != nil {
		t.Fatal(err)
	}
	if stars != 40000 {
		t.Fatalf("stars not updated on resync: %d", stars)
	}
	if err := s.Pool.QueryRow(t.Context(), "SELECT count(*) FROM marketplace_items WHERE source='curated'").Scan(&count); err != nil || count != curatedBefore {
		t.Fatalf("curated rows changed: %d want %d err=%v", count, curatedBefore, err)
	}
}
