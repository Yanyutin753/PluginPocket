package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func New(webDir, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			Fail(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if r.URL.Path == "/healthz" || r.URL.Path == "/api/v1/health" {
			writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok", "service": "loadout", "version": version})
			return
		}
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/") {
			Fail(w, http.StatusNotFound, "not_found")
			return
		}
		if webDir != "" {
			name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
			file := filepath.Join(webDir, filepath.FromSlash(name))
			if info, err := os.Stat(file); err == nil && info.Mode().IsRegular() {
				http.ServeFile(w, r, file)
				return
			}
			if !strings.HasPrefix(name, "assets") && path.Ext(name) == "" {
				index := filepath.Join(webDir, "index.html")
				if info, err := os.Stat(index); err == nil && info.Mode().IsRegular() {
					w.Header().Set("Cache-Control", "no-cache")
					http.ServeFile(w, r, index)
					return
				}
			}
		}
		Fail(w, http.StatusNotFound, "not_found")
	})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_ = json.NewEncoder(w).Encode(body)
	}
}
