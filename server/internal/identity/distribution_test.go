package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"golang.org/x/oauth2"
)

func identityReplica(t *testing.T, database *store.Store, options Options) *httptest.Server {
	t.Helper()
	// A separate connection pool and handler represent independently started replicas.
	s, err := store.Open(t.Context(), database.Pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	server := httptest.NewServer(New(s, options))
	t.Cleanup(server.Close)
	return server
}

func identityHTTP(t *testing.T, method, endpoint, body string, cookies ...*http.Cookie) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://pluginpocket.test")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestIdentityBudgetSurvivesReplicaChangeAndRestart(t *testing.T) {
	s := identityDB(t)
	options := Options{Origin: "https://pluginpocket.test", GitHub: &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{AuthURL: "https://provider.test/authorize"}}}
	a, b := identityReplica(t, s, options), identityReplica(t, s, options)
	for i := range 120 {
		endpoint := a.URL
		if i%2 == 1 {
			endpoint = b.URL
		}
		response := identityHTTP(t, "GET", endpoint+"/api/v1/auth/github/start", "")
		_ = response.Body.Close()
		if response.StatusCode != 302 {
			t.Fatalf("attempt %d status=%d want=302", i+1, response.StatusCode)
		}
		if i == 0 {
			pinIdentityWindow(t, s)
		}
	}
	a.Close()
	b.Close()
	restarted := identityReplica(t, s, options)
	response := identityHTTP(t, "GET", restarted.URL+"/api/v1/auth/github/start", "")
	if response.StatusCode != 429 {
		t.Fatalf("replica restart reset shared identity budget: attempt 121 status=%d want=429", response.StatusCode)
	}
	if response.Header.Get("Retry-After") != "60" {
		t.Fatal("rate limit must provide a retry interval")
	}
}

func TestGitHubStateCrossesReplicasAndSurvivesOriginatingReplicaLoss(t *testing.T) {
	s := identityDB(t)
	var exchanges atomic.Int64
	var expectedChallenge atomic.Value
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			exchanges.Add(1)
			if err := r.ParseForm(); err != nil || oauth2.S256ChallengeFromVerifier(r.Form.Get("code_verifier")) != expectedChallenge.Load() {
				t.Error("callback replica lost the original PKCE verifier")
				http.Error(w, "invalid_grant", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "fixture-provider-token", "token_type": "bearer"})
		case "/user":
			if r.Header.Get("Authorization") != "Bearer fixture-provider-token" {
				t.Error("provider request was not authenticated")
			}
			_ = json.NewEncoder(w).Encode(map[string]int{"id": 7654})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	options := Options{Origin: "https://pluginpocket.test", UserURL: provider.URL + "/user", GitHub: &oauth2.Config{
		ClientID: "fixture-client", ClientSecret: "fixture-secret", RedirectURL: "https://pluginpocket.test/api/v1/auth/github/callback",
		Endpoint: oauth2.Endpoint{AuthURL: provider.URL + "/authorize", TokenURL: provider.URL + "/token", AuthStyle: oauth2.AuthStyleInParams},
	}}
	a := identityReplica(t, s, options)
	start := identityHTTP(t, "GET", a.URL+"/api/v1/auth/github/start", "")
	if start.StatusCode != 302 {
		t.Fatalf("OAuth start status=%d", start.StatusCode)
	}
	authorize, err := url.Parse(start.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := authorize.Query().Get("state")
	expectedChallenge.Store(authorize.Query().Get("code_challenge"))
	if state == "" || authorize.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("OAuth start did not issue state and PKCE")
	}
	cookies := start.Cookies()
	a.Close()
	// Both callback replicas are created after the initiating server disappears.
	b, c := identityReplica(t, s, options), identityReplica(t, s, options)
	path := "/api/v1/auth/github/callback?code=fixture-code&state=" + url.QueryEscape(state)
	forged := identityHTTP(t, "GET", b.URL+path, "", &http.Cookie{Name: "pluginpocket_oauth_state", Value: "wrong-browser"})
	if forged.StatusCode != 400 {
		t.Fatalf("state was not bound to its initiating browser: %d", forged.StatusCode)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, replica := range []*httptest.Server{b, c} {
		wg.Go(func() {
			response := identityHTTP(t, "GET", replica.URL+path, "", cookies...)
			statuses <- response.StatusCode
			if response.StatusCode == 302 {
				var session string
				var cookieExpiry time.Time
				for _, cookie := range response.Cookies() {
					if cookie.Name == "pluginpocket_session" && cookie.HttpOnly {
						session = cookie.Value
						cookieExpiry = cookie.Expires
					}
				}
				if session == "" {
					t.Error("callback did not issue an HttpOnly session")
					return
				}
				var databaseExpiry time.Time
				if err := s.Pool.QueryRow(t.Context(), "SELECT expires_at FROM session_access WHERE access_hash=$1", auth.Digest(session)).Scan(&databaseExpiry); err != nil || !cookieExpiry.Equal(databaseExpiry.Truncate(time.Second)) {
					t.Errorf("session cookie expiry differs from stored expiry: err=%v", err)
				}
				// The created session is immediately accepted on either replica.
				status := identityHTTP(t, "GET", b.URL+"/api/v1/account/email", "", &http.Cookie{Name: "pluginpocket_session", Value: session})
				if status.StatusCode != 200 {
					t.Errorf("new session rejected on peer replica: %d", status.StatusCode)
				}
			}
		})
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[302] != 1 || counts[400] != 1 || exchanges.Load() != 1 {
		t.Fatalf("OAuth concurrent consumption statuses=%v provider exchanges=%d", counts, exchanges.Load())
	}
	var users, sessions, grants int
	if err := s.Pool.QueryRow(t.Context(), `SELECT
 (SELECT count(*) FROM oauth_identities WHERE provider='github' AND subject='7654'),
 (SELECT count(*) FROM sessions),
 (SELECT count(*) FROM ledger WHERE kind='registration')`).Scan(&users, &sessions, &grants); err != nil || users != 1 || sessions != 1 || grants != 1 {
		t.Fatalf("OAuth durable state users=%d sessions=%d grants=%d err=%v", users, sessions, grants, err)
	}
	var databaseLifetime bool
	if err := s.Pool.QueryRow(t.Context(), "SELECT bool_and(expires_at>=created_at+interval '7 days' AND expires_at<created_at+interval '7 days 1 second') FROM sessions").Scan(&databaseLifetime); err != nil || !databaseLifetime {
		t.Fatalf("OAuth sessions must use the database clock for their seven-day lifetime: valid=%v err=%v", databaseLifetime, err)
	}
	replay := identityHTTP(t, "GET", c.URL+path, "", cookies...)
	if replay.StatusCode != 400 || exchanges.Load() != 1 {
		t.Fatal("OAuth replay reached the provider")
	}
	expiring := identityHTTP(t, "GET", c.URL+"/api/v1/auth/github/start", "")
	if expiring.StatusCode != 302 {
		t.Fatalf("second OAuth start status=%d", expiring.StatusCode)
	}
	redirect, err := url.Parse(expiring.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	expiredState := redirect.Query().Get("state")
	if _, err := s.Pool.Exec(t.Context(), "UPDATE oauth_states SET expires_at=now()-interval '1 second' WHERE state_hash=$1", auth.Digest(expiredState)); err != nil {
		t.Fatal(err)
	}
	expired := identityHTTP(t, "GET", b.URL+"/api/v1/auth/github/callback?code=fixture-code&state="+url.QueryEscape(expiredState), "", expiring.Cookies()...)
	if expired.StatusCode != 400 || exchanges.Load() != 1 {
		t.Fatal("peer replica accepted an expired OAuth state")
	}
}

func TestEmailVerificationCrossesReplicasAndConsumesOnce(t *testing.T) {
	s := identityDB(t)
	var user int64
	if err := s.Pool.QueryRow(t.Context(), "INSERT INTO users(username,password_hash) VALUES('distributed-email','unused') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(t.Context(), "INSERT INTO sessions(user_id,session_hash,expires_at) VALUES($1,$2,now()+interval '1 hour')", user, auth.Digest("fixture-session")); err != nil {
		t.Fatal(err)
	}
	mail := make(chan string, 1)
	options := Options{Origin: "https://pluginpocket.test", Mail: func(_ context.Context, recipient, link string) error {
		if recipient != "owner@example.com" {
			t.Errorf("unexpected verification recipient")
		}
		mail <- link
		return nil
	}}
	a := identityReplica(t, s, options)
	cookie := &http.Cookie{Name: "pluginpocket_session", Value: "fixture-session"}
	request := identityHTTP(t, "POST", a.URL+"/api/v1/account/email/request", `{"email":"owner@example.com"}`, cookie)
	if request.StatusCode != 202 {
		t.Fatalf("verification email request status=%d", request.StatusCode)
	}
	var link string
	select {
	case link = <-mail:
	case <-time.After(time.Second):
		t.Fatal("verification link was not delivered")
	}
	u, err := url.Parse(link)
	if err != nil || u.Query().Get("token") == "" {
		t.Fatal("verification link has no token")
	}
	encoded, err := json.Marshal(map[string]string{"token": u.Query().Get("token")})
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	b, c := identityReplica(t, s, options), identityReplica(t, s, options)
	limited := identityHTTP(t, "POST", b.URL+"/api/v1/account/email/request", `{"email":"owner@example.com"}`, cookie)
	if limited.StatusCode != 429 {
		t.Fatalf("email resend budget did not survive replica loss: %d", limited.StatusCode)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, replica := range []*httptest.Server{b, c} {
		wg.Go(func() {
			statuses <- identityHTTP(t, "POST", replica.URL+"/api/v1/auth/email/verify", string(encoded)).StatusCode
		})
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[200] != 1 || counts[400] != 1 {
		t.Fatalf("concurrent email token consumption statuses=%v", counts)
	}
	status := identityHTTP(t, "GET", c.URL+"/api/v1/account/email", "", cookie)
	var profile struct {
		Email    string     `json:"email"`
		Verified *time.Time `json:"verified_at"`
	}
	if err := json.NewDecoder(status.Body).Decode(&profile); err != nil || status.StatusCode != 200 || profile.Email != "owner@example.com" || profile.Verified == nil {
		t.Fatalf("email verification was not visible on peer replica: status=%d err=%v", status.StatusCode, err)
	}
	var pending int
	if err := s.Pool.QueryRow(t.Context(), "SELECT count(*) FROM email_verifications WHERE user_id=$1", user).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("verification token not consumed: pending=%d err=%v", pending, err)
	}
	restarted := identityReplica(t, s, options)
	replay := identityHTTP(t, "POST", restarted.URL+"/api/v1/auth/email/verify", string(encoded))
	if replay.StatusCode != 400 {
		t.Fatalf("consumed verification token revived after handler recreation: %d", replay.StatusCode)
	}
	resend := identityHTTP(t, "POST", b.URL+"/api/v1/account/email/request", `{"email":"owner@example.com"}`, cookie)
	if resend.StatusCode != 202 {
		t.Fatalf("new verification request status=%d", resend.StatusCode)
	}
	select {
	case link = <-mail:
	case <-time.After(time.Second):
		t.Fatal("new verification link was not delivered")
	}
	u, err = url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(t.Context(), "UPDATE email_verifications SET expires_at=now()-interval '1 second' WHERE user_id=$1", user); err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(map[string]string{"token": u.Query().Get("token")})
	if err != nil {
		t.Fatal(err)
	}
	expired := identityHTTP(t, "POST", restarted.URL+"/api/v1/auth/email/verify", string(encoded))
	if expired.StatusCode != 400 {
		t.Fatalf("peer replica accepted an expired email token: %d", expired.StatusCode)
	}
}
