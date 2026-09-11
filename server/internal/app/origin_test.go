package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserWritesUseRequestOrigin(t *testing.T) {
	for _, publicURL := range []string{"", "http://127.0.0.1:5173"} {
		t.Run("public="+publicURL, func(t *testing.T) {
			h := New(nil, Options{Origin: publicURL})
			for _, tc := range []struct {
				name, host, origin, fetchSite string
				want                          int
			}{
				{"localhost proxy", "localhost:5173", "http://localhost:5173", "", 401},
				{"IP proxy", "127.0.0.1:5173", "http://127.0.0.1:5173", "", 401},
				{"TLS terminating proxy", "loadout.example", "https://loadout.example", "", 401},
				{"modern proxy rewrites host", "backend:8787", "https://loadout.example", "same-origin", 401},
				{"external site", "localhost:5173", "https://evil.example", "", 403},
				{"different port", "localhost:5173", "http://localhost:5174", "", 403},
				{"public URL is not an allowlist", "localhost:5173", "http://127.0.0.1:5173", "", 403},
				{"cross-site metadata", "localhost:5173", "http://localhost:5173", "cross-site", 403},
				{"same-site is not same-origin", "localhost:5173", "http://localhost:5173", "same-site", 403},
				{"missing origin", "localhost:5173", "", "", 403},
				{"missing origin with metadata", "localhost:5173", "", "same-origin", 403},
				{"opaque origin", "localhost:5173", "null", "", 403},
			} {
				t.Run(tc.name, func(t *testing.T) {
					r := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/api/v1/account/tokens", nil)
					r.Header.Set("Origin", tc.origin)
					r.Header.Set("Sec-Fetch-Site", tc.fetchSite)
					// Forwarded headers supplied by a client cannot authorize a cross-origin request.
					r.Header.Set("X-Forwarded-Host", "evil.example")
					r.Header.Set("X-Forwarded-Proto", "https")
					w := httptest.NewRecorder()
					h.ServeHTTP(w, r)
					if w.Code != tc.want {
						t.Fatalf("status=%d body=%s, want %d", w.Code, w.Body, tc.want)
					}
				})
			}
		})
	}
}
