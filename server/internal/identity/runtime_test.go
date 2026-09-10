package identity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/Yanyutin753/loadout/server/internal/settings"
	"golang.org/x/oauth2"
)

func saveRuntime(t *testing.T, m *settings.Manager, v settings.Values, revision int64) {
	t.Helper()
	tx, err := m.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = m.Write(t.Context(), tx, v, revision, 1); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeIdentityUpdatesMetaOAuthAndSignupCredits(t *testing.T) {
	s := identityDB(t)
	if _, err := s.Pool.Exec(t.Context(), "INSERT INTO users(username,password_hash) VALUES('settings-admin','unused')"); err != nil {
		t.Fatal(err)
	}
	m := &settings.Manager{Pool: s.Pool, Key: bytes.Repeat([]byte{1}, 32), Origin: "https://loadout.test"}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"fixture-token","token_type":"bearer"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":6789}`))
	}))
	defer provider.Close()
	h := New(s, Options{Origin: m.Origin, Runtime: m, UserURL: provider.URL + "/user", GitHub: &oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: provider.URL + "/authorize", TokenURL: provider.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}}})
	call := func(path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("/api/v1/meta"); w.Code != 200 || !strings.Contains(w.Body.String(), `"github":false`) {
		t.Fatalf("runtime disabled GitHub reported enabled: status=%d body=%s", w.Code, w.Body)
	}
	values := settings.Values{InitialCredits: 37, GitHubEnabled: true, GitHubClientID: "client-one", GitHubClientSecret: "private-secret"}
	saveRuntime(t, m, values, 0)
	start := call("/api/v1/auth/github/start")
	redirect, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if start.Code != 302 || redirect.Query().Get("client_id") != "client-one" {
		t.Fatalf("saved client not active: status=%d", start.Code)
	}
	values.GitHubClientID = "client-two"
	values.InitialCredits = 81
	saveRuntime(t, m, values, 1)
	start = call("/api/v1/auth/github/start")
	redirect, err = url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if start.Code != 302 || redirect.Query().Get("client_id") != "client-two" {
		t.Fatalf("updated client not active: status=%d", start.Code)
	}
	callback := call("/api/v1/auth/github/callback?code=fixture&state="+url.QueryEscape(redirect.Query().Get("state")), start.Result().Cookies()...)
	if callback.Code != 302 {
		t.Fatalf("local OAuth callback status=%d body=%s", callback.Code, callback.Body)
	}
	var balance int64
	if err = s.Pool.QueryRow(t.Context(), "SELECT balance FROM wallets w JOIN oauth_identities o ON o.user_id=w.user_id WHERE o.subject='6789'").Scan(&balance); err != nil || balance != 81 {
		t.Fatalf("OAuth grant=%d want=81 error=%v", balance, err)
	}
	values.GitHubEnabled = false
	saveRuntime(t, m, values, 2)
	if w := call("/api/v1/auth/github/start"); w.Code != 503 {
		t.Fatalf("disabled GitHub still starts: status=%d", w.Code)
	}
	if _, err = s.Pool.Exec(t.Context(), "ALTER TABLE runtime_settings RENAME TO unavailable_settings"); err != nil {
		t.Fatal(err)
	}
	if w := call("/api/v1/meta"); w.Code != 503 {
		t.Fatalf("failed settings read fell back to stale metadata: status=%d", w.Code)
	}
}

func TestRuntimeIdentityEnablesLocalSMTPAndDisablesItWithoutRestart(t *testing.T) {
	s := identityDB(t)
	if _, err := s.Pool.Exec(t.Context(), "INSERT INTO users(username,password_hash) VALUES('mail-owner','unused'); INSERT INTO sessions(user_id,session_hash,expires_at) VALUES(1,'"+auth.Digest("runtime-session")+"',now()+interval '1 hour')"); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	delivered := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = conn.Write([]byte("220 local.test ESMTP\r\n"))
		scan := bufio.NewScanner(conn)
		var body strings.Builder
		data := false
		for scan.Scan() {
			line := scan.Text()
			if data && line != "." {
				body.WriteString(line + "\n")
				continue
			}
			response := "250 OK\r\n"
			switch line {
			case "DATA":
				data = true
				response = "354 go ahead\r\n"
			case ".":
				data = false
			case "QUIT":
				_, _ = conn.Write([]byte("221 bye\r\n"))
				delivered <- body.String()
				return
			}
			_, _ = conn.Write([]byte(response))
		}
	}()
	m := &settings.Manager{Pool: s.Pool, Origin: "https://loadout.test"}
	h := New(s, Options{Origin: m.Origin, Runtime: m, SMTPAllowLocalInsecure: true})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", m.Origin)
		r.AddCookie(&http.Cookie{Name: "loadout_session", Value: "runtime-session"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	values := settings.Values{SMTPEnabled: true, SMTPAddress: listener.Addr().String(), SMTPFrom: "runtime@example.com"}
	saveRuntime(t, m, values, 0)
	w := call("GET", "/api/v1/meta", "")
	var meta map[string]bool
	if err = json.Unmarshal(w.Body.Bytes(), &meta); err != nil || !meta["email"] {
		t.Fatalf("SMTP enabled not reflected in meta: status=%d", w.Code)
	}
	if w = call("POST", "/api/v1/account/email/request", `{"email":"owner@example.com"}`); w.Code != 202 {
		t.Fatalf("configured local SMTP send status=%d body=%s", w.Code, w.Body)
	}
	select {
	case body := <-delivered:
		if !strings.Contains(body, "From: <runtime@example.com>") || !strings.Contains(body, "verify-email?token=") {
			t.Fatal("runtime SMTP sender or verification link missing")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("local SMTP received no mail")
	}
	values.SMTPEnabled = false
	saveRuntime(t, m, values, 1)
	if w = call("POST", "/api/v1/account/email/request", `{"email":"owner@example.com"}`); w.Code != 503 {
		t.Fatalf("disabled SMTP still accepts email request: status=%d", w.Code)
	}
	if w = call("GET", "/api/v1/account/email", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("email status retained old config: status=%d", w.Code)
	}
}
