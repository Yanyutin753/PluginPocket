package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsDoNotLabelUserPathsAndRequestsHaveIDs(t *testing.T) {
	metrics := New(nil)
	handler := metrics.Wrap(http.NotFoundHandler())
	for range 2 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/secret-user?token=secret", nil))
		if w.Header().Get("X-Request-ID") == "" {
			t.Error("request has no correlation ID")
		}
	}
	w := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()
	if !strings.Contains(body, "loadout_http_requests_total") || !strings.Contains(body, `code="404"`) {
		t.Fatalf("no request metrics: %s", body)
	}
	if strings.Contains(body, "secret") {
		t.Fatal("high-cardinality or sensitive path leaked into metrics")
	}
}
