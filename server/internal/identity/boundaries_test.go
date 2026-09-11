package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
)

func TestReviewTransientDatabaseFailureIsNotUnauthorized(t *testing.T) {
	s := identityDB(t)
	ctx := context.Background()
	var id int64
	if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES('dbfailure','unused') RETURNING id").Scan(&id); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Pool.Exec(ctx, "INSERT INTO sessions(user_id,session_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')", id, auth.Digest("valid_session")); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Pool.Exec(ctx, "SET statement_timeout='100ms'"); e != nil {
		t.Fatal(e)
	}
	blocker, e := pgx.Connect(ctx, s.Pool.Config().ConnString())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = blocker.Close(ctx) }()
	tx, e := blocker.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e := tx.Exec(ctx, "LOCK TABLE sessions IN ACCESS EXCLUSIVE MODE"); e != nil {
		t.Fatal(e)
	}
	h := New(s, Options{Origin: "http://local.test"})
	r := httptest.NewRequest("GET", "/api/v1/account/email", nil)
	r.AddCookie(&http.Cookie{Name: "pluginpocket_session", Value: "valid_session"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "temporarily_unavailable") {
		t.Fatalf("transient DB statement timeout must be retryable, got %d %s", w.Code, w.Body)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("same session must recover after database resumes: status=%d", w.Code)
	}
	for _, session := range []string{"", "invalid_session"} {
		r := httptest.NewRequest("GET", "/api/v1/account/email", nil)
		if session != "" {
			r.AddCookie(&http.Cookie{Name: "pluginpocket_session", Value: session})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("missing or invalid session must remain unauthorized: status=%d", w.Code)
		}
	}
}

func TestReviewFailedResendPreservesPreviouslyDeliveredVerification(t *testing.T) {
	s := identityDB(t)
	ctx := context.Background()
	var id int64
	if e := s.Pool.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES('resend','unused') RETURNING id").Scan(&id); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Pool.Exec(ctx, "INSERT INTO sessions(user_id,session_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')", id, auth.Digest("valid_session")); e != nil {
		t.Fatal(e)
	}
	var delivered string
	sendCount := 0
	h := New(s, Options{Origin: "http://local.test", Mail: func(_ context.Context, _ string, link string) error {
		sendCount++
		if sendCount == 1 {
			delivered = link
			return nil
		}
		return fmt.Errorf("temporary SMTP outage")
	}})
	send := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/account/email/request", strings.NewReader(`{"email":"owner@example.com"}`))
		r.Header.Set("Origin", "http://local.test")
		r.AddCookie(&http.Cookie{Name: "pluginpocket_session", Value: "valid_session"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := send(); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	if _, e := s.Pool.Exec(ctx, "UPDATE email_verifications SET created_at=now()-interval '2 minutes'"); e != nil {
		t.Fatal(e)
	}
	if w := send(); w.Code != 503 {
		t.Fatal(w.Code, w.Body)
	}
	u, e := url.Parse(delivered)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "/api/v1/auth/email/verify", strings.NewReader(fmt.Sprintf(`{"token":%q}`, u.Query().Get("token"))))
	r.Header.Set("Origin", "http://local.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("first delivered unexpired link broken after failed resend: %d %s", w.Code, w.Body)
	}
}

func TestFailedResendSerializesWithVerificationAndLaterResend(t *testing.T) {
	for _, next := range []string{"verify", "resend"} {
		t.Run(next, func(t *testing.T) {
			s := identityDB(t)
			var id int64
			if err := s.Pool.QueryRow(t.Context(), "INSERT INTO users(username,password_hash) VALUES('concurrent','unused') RETURNING id").Scan(&id); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Pool.Exec(t.Context(), "INSERT INTO sessions(user_id,session_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')", id, auth.Digest("session")); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Pool.Exec(t.Context(), "INSERT INTO email_verifications(token_hash,user_id,email,created_at,expires_at) VALUES($1,$2,'old@example.com',now()-interval '2 minutes',now()+interval '30 minutes')", auth.Digest("old"), id); err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			failing := New(s, Options{Origin: "http://local.test", Mail: func(ctx context.Context, _, _ string) error {
				close(started)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return errors.New("SMTP unavailable")
			}})
			var delivered string
			peer := New(s, Options{Origin: "http://local.test", Mail: func(_ context.Context, _, link string) error { delivered = link; return nil }})
			request := func(h http.Handler, path, body string) int {
				r := httptest.NewRequest("POST", path, strings.NewReader(body))
				r.Header.Set("Origin", "http://local.test")
				r.AddCookie(&http.Cookie{Name: "pluginpocket_session", Value: "session"})
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w.Code
			}
			failed, following := make(chan int, 1), make(chan int, 1)
			go func() { failed <- request(failing, "/api/v1/account/email/request", `{"email":"failed@example.com"}`) }()
			<-started
			path, body, want := "/api/v1/auth/email/verify", `{"token":"old"}`, 200
			if next == "resend" {
				path, body, want = "/api/v1/account/email/request", `{"email":"new@example.com"}`, 202
			}
			go func() { following <- request(peer, path, body) }()
			// Observe the real PostgreSQL wait, or an early HTTP rejection, before failing SMTP.
			deadline := time.After(3 * time.Second)
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			waiting := false
			for !waiting {
				select {
				case code := <-following:
					t.Fatalf("concurrent %s rejected before failed delivery resolved: status=%d", next, code)
				case <-deadline:
					t.Fatal("concurrent request never reached the token row")
				case <-ticker.C:
					if err := s.Pool.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM pg_locks l JOIN pg_class c ON c.oid=l.relation JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_stat_activity a ON a.pid=l.pid WHERE n.nspname=current_schema() AND c.relname='email_verifications' AND a.wait_event_type='Lock')`).Scan(&waiting); err != nil {
						t.Fatal(err)
					}
				}
			}
			unblock()
			if code := <-failed; code != 503 {
				t.Fatalf("failed send status=%d", code)
			}
			if code := <-following; code != want {
				t.Fatalf("following %s status=%d want=%d", next, code, want)
			}
			email := "old@example.com"
			if next == "resend" {
				link, err := url.Parse(delivered)
				if err != nil {
					t.Fatal(err)
				}
				if code := request(peer, "/api/v1/auth/email/verify", fmt.Sprintf(`{"token":%q}`, link.Query().Get("token"))); code != 200 {
					t.Fatalf("later successful token overwritten: status=%d", code)
				}
				email = "new@example.com"
			}
			var verified string
			if err := s.Pool.QueryRow(t.Context(), "SELECT email FROM users WHERE id=$1 AND email_verified_at IS NOT NULL", id).Scan(&verified); err != nil || verified != email {
				t.Fatalf("verified=%q want=%q error=%v", verified, email, err)
			}
			if code := request(peer, "/api/v1/auth/email/verify", `{"token":"old"}`); code != 400 {
				t.Fatalf("old token revived: status=%d", code)
			}
		})
	}
}

func TestIdentityBudgetsIgnoreWrongMethodsAndSeparateCallersAndFlows(t *testing.T) {
	for _, wrongMethod := range []bool{true, false} {
		t.Run(fmt.Sprintf("wrong_method_%t", wrongMethod), func(t *testing.T) {
			s := identityDB(t)
			h := New(s, Options{Origin: "http://local.test", GitHub: &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: "https://provider.test/authorize"}}})
			call := func(method, path, peer string) int {
				r := httptest.NewRequest(method, path, nil)
				r.RemoteAddr = peer
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w.Code
			}
			method, want := "GET", 302
			if wrongMethod {
				method, want = "DELETE", 405
			}
			for i := range 120 {
				if code := call(method, "/api/v1/auth/github/start", "192.0.2.1:1000"); code != want {
					t.Fatalf("attempt=%d status=%d want=%d", i, code, want)
				}
				if i == 0 && !wrongMethod {
					pinIdentityWindow(t, s)
				}
			}
			if wrongMethod {
				if code := call("GET", "/api/v1/auth/github/start", "192.0.2.1:2000"); code != 302 {
					t.Fatalf("wrong methods consumed valid request budget: status=%d", code)
				}
			} else {
				if code := call("GET", "/api/v1/auth/github/start", "192.0.2.1:2000"); code != 429 {
					t.Fatalf("source port reset caller budget: status=%d", code)
				}
				if code := call("GET", "/api/v1/auth/github/start", "192.0.2.2:1000"); code != 302 {
					t.Fatalf("one caller exhausted a different caller budget: status=%d", code)
				}
				if code := call("GET", "/api/v1/auth/github/callback", "192.0.2.1:1000"); code != 400 {
					t.Fatalf("OAuth starts exhausted callback budget: status=%d", code)
				}
			}
		})
	}
}
