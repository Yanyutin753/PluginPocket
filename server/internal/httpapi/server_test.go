package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	handler := New("", "test-version")
	for _, path := range []string{"/healthz", "/api/v1/health"} {
		t.Run(path, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
			var body map[string]string
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if res.Code != 200 || body["status"] != "ok" || body["service"] != "pluginpocket" || body["version"] != "test-version" {
				t.Fatalf("unexpected health: %d %v", res.Code, body)
			}
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("health must not be cached")
			}
			if res.Header().Get("Content-Type") != "application/json" {
				t.Fatal("health must be JSON")
			}
		})
	}
}

func TestRoutingBoundaries(t *testing.T) {
	web := t.TempDir()
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("<!doctype html><title>PluginPocket</title>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(web, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "assets", "app.js"), []byte("console.log('pluginpocket')"), 0600); err != nil {
		t.Fatal(err)
	}
	handler := New(web, "test")
	for _, tc := range []struct {
		method, path string
		status       int
		contains     string
	}{
		{"GET", "/", 200, "<title>PluginPocket</title>"},
		{"GET", "/dashboard", 200, "<title>PluginPocket</title>"},
		{"GET", "/assets/app.js", 200, "console.log"},
		{"GET", "/assets/missing.js", 404, "not_found"},
		{"GET", "/assets/", 404, "not_found"},
		{"GET", "/api/v1/missing", 404, "not_found"},
		{"GET", "/api", 404, "not_found"},
		{"GET", "/mcp", 404, "not_found"},
		{"POST", "/healthz", 405, "method_not_allowed"},
		{"POST", "/dashboard", 405, "method_not_allowed"},
		{"HEAD", "/healthz", 200, ""},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
			if res.Code != tc.status || !strings.Contains(res.Body.String(), tc.contains) {
				t.Fatalf("got %d %s", res.Code, res.Body.String())
			}
			if tc.method == "HEAD" && res.Body.Len() != 0 {
				t.Fatal("HEAD must not send a body")
			}
			if tc.status >= 400 && res.Header().Get("Content-Type") != "application/json" {
				t.Fatal("errors must be JSON")
			}
		})
	}
}

func TestMissingWebDoesNotBreakHealth(t *testing.T) {
	handler := New(filepath.Join(t.TempDir(), "missing"), "dev")
	for path, status := range map[string]int{"/": 404, "/healthz": 200} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest("GET", path, nil))
		if res.Code != status {
			t.Fatalf("%s: got %d", path, res.Code)
		}
	}
}
