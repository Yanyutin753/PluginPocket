package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/config"
	"github.com/jackc/pgx/v5"
)

func TestNoDatabaseCannotReportProductReady(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	h, closeHandler, err := applicationHandler(context.Background(), config.Config{WebDir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeHandler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "database_unconfigured") {
		t.Fatalf("unconfigured readiness %d %s", w.Code, w.Body.String())
	}
}

func TestConfiguredDatabaseServesProductRoutes(t *testing.T) {
	raw := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	schema := fmt.Sprintf("process_%d", time.Now().UnixNano())
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	handler, closeHandler, err := applicationHandler(ctx, config.Config{DatabaseURL: u.String(), PublicURL: "http://local.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeHandler()
	request := httptest.NewRequest(http.MethodPost, "http://local.test/api/v1/auth/register", strings.NewReader(`{"username":"newuser","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://local.test")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 201 {
		t.Fatalf("registration route returned %d: %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/readyz", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "ready") {
		t.Fatalf("database readiness %d: %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/mcp", nil))
	if response.Code != 401 {
		t.Fatalf("gateway wiring returned %d", response.Code)
	}
}
