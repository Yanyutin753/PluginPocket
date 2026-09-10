package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUpstreamAddressPolicy(t *testing.T) {
	for _, address := range []string{"file:///tmp/a", "http://example.com/mcp", "https://user:password@example.com/mcp", "https://127.0.0.1/mcp", "https://[::1]/mcp", "https://169.254.169.254/latest", "https://10.0.0.1/mcp", "https://localhost/mcp"} {
		if _, err := upstreamClient(address, nil, false); err == nil {
			t.Errorf("accepted forbidden address %s", address)
		}
	}
	if _, err := upstreamClient("https://example.com/mcp", nil, false); err != nil {
		t.Fatal(err)
	}
}

func TestStatelessCancellationRejectedBeforeNetworkWithoutAffectingOtherRequests(t *testing.T) {
	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	client, err := upstreamClient(upstream.URL, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	for _, test := range []struct {
		name, version, session, method string
		rejected                       bool
	}{
		{"modern stateless cancellation", "2026-07-28", "", "notifications/cancelled", true},
		{"legacy cancellation", "2025-11-25", "legacy-session", "notifications/cancelled", false},
		{"modern session cancellation", "2026-07-28", "modern-session", "notifications/cancelled", false},
		{"modern tool call", "2026-07-28", "", "tools/call", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Mcp-Protocol-Version", test.version)
			req.Header.Set("Mcp-Session-Id", test.session)
			req.Header.Set("Mcp-Method", test.method)
			before := received.Load()
			response, err := client.Do(req)
			if test.rejected {
				if err == nil || response != nil || received.Load() != before {
					t.Fatal("unsupported cancellation was sent or reported as a successful response")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusNoContent || received.Load() != before+1 {
				t.Fatal("supported request did not reach upstream")
			}
		})
	}
}

func TestUpstreamHeadersAndNoRedirect(t *testing.T) {
	visited := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visited = true }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer operator-key" {
			t.Error("missing configured header")
		}
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer origin.Close()
	client, err := upstreamClient(origin.URL, map[string]string{"Authorization": "Bearer operator-key"}, true)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 302 || visited {
		t.Fatal("followed upstream redirect")
	}
}

func TestDialRejectsPrivateResolvedAddress(t *testing.T) {
	_, err := publicDial(context.Background(), "tcp", "127.0.0.1:443")
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("private resolution not rejected: %v", err)
	}
}
