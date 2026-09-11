package demo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCanceledExpansionDoesNotStartCleanupRequests(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(204) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Expand(ctx, server.URL, "admin", "unused"); err == nil {
		t.Fatal("canceled expansion reported success")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("canceled expansion sent %d cleanup requests without any login", got)
	}
}

func TestFailedLoginDoesNotSendAnonymousLogouts(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/api/v1/auth/login" {
			w.WriteHeader(401)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()
	if err := Expand(t.Context(), server.URL, "admin", "unused"); err == nil {
		t.Fatal("failed login reported success")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("failed login sent %d requests, want only the login", got)
	}
}
