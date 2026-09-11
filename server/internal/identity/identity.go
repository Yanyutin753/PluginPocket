package identity

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/Yanyutin753/loadout/server/internal/httpapi"
	"github.com/Yanyutin753/loadout/server/internal/settings"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
)

type Options struct {
	BrowserTTL             auth.BrowserTTL
	Runtime                *settings.Manager
	SMTPAllowLocalInsecure bool
	InitialCredits         *int64
	Origin                 string
	SecureCookies          bool
	Mail                   func(context.Context, string, string) error
	GitHub                 *oauth2.Config
	UserURL                string
	RequiredOrg            string
	Client                 *http.Client
}
type identity struct {
	s *store.Store
	o Options
}

func New(s *store.Store, o Options) http.Handler {
	if o.InitialCredits == nil {
		initial := int64(1000)
		o.InitialCredits = &initial
	}
	if o.UserURL == "" {
		o.UserURL = "https://api.github.com/user"
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	if o.Runtime != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			snapshot, err := o.Runtime.Read(r.Context(), nil)
			if err != nil {
				failure(w, 503, "temporarily_unavailable")
				return
			}
			current := o
			current.Runtime = nil
			current.BrowserTTL = auth.BrowserTTL{AccessSeconds: snapshot.AccessTokenSeconds, RefreshSeconds: snapshot.RefreshTokenSeconds}
			current.InitialCredits = &snapshot.InitialCredits
			current.RequiredOrg = snapshot.GitHubOrg
			current.GitHub = nil
			if snapshot.GitHubEnabled {
				endpoint := oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token", AuthStyle: oauth2.AuthStyleInParams}
				if o.GitHub != nil {
					endpoint = o.GitHub.Endpoint
				}
				scopes := []string{"read:user"}
				if snapshot.GitHubOrg != "" {
					scopes = append(scopes, "read:org")
				}
				current.GitHub = &oauth2.Config{ClientID: snapshot.GitHubClientID, ClientSecret: snapshot.GitHubClientSecret, RedirectURL: o.Origin + "/api/v1/auth/github/callback", Scopes: scopes, Endpoint: endpoint}
			}
			current.Mail = nil
			if snapshot.SMTPEnabled {
				current.Mail = SMTPMailer(SMTPConfig{Address: snapshot.SMTPAddress, From: snapshot.SMTPFrom, Username: snapshot.SMTPUsername, Password: snapshot.SMTPPassword, AllowLocalInsecure: o.SMTPAllowLocalInsecure})
			}
			New(s, current).ServeHTTP(w, r)
		})
	}
	// Options belong to this request when runtime configuration is enabled.
	a := &identity{s: s, o: o}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/meta", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]bool{"github": o.GitHub != nil, "email": o.Mail != nil, "payments": false})
	})
	mux.HandleFunc("GET /api/v1/auth/github/start", a.public("github_start", a.githubStart))
	mux.HandleFunc("GET /api/v1/auth/github/callback", a.public("github_callback", a.githubCallback))
	mux.HandleFunc("GET /api/v1/account/email", a.emailStatus)
	mux.HandleFunc("POST /api/v1/account/email/request", a.emailRequest)
	mux.HandleFunc("POST /api/v1/auth/email/verify", a.public("email_verify", a.emailVerify))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method == "POST" {
			origin := r.Header.Get("Origin")
			valid := origin != "" && origin == o.Origin
			if o.Origin == "" {
				u, e := url.Parse(origin)
				valid = e == nil && u.Host == r.Host && ((r.TLS == nil && u.Scheme == "http") || (r.TLS != nil && u.Scheme == "https"))
			}
			if !valid {
				failure(w, 403, "forbidden_origin")
				return
			}
		}

		mux.ServeHTTP(w, r)
	})
}
func (a *identity) public(family string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.s != nil {
			allowed, err := auth.AllowPublicRequest(r.Context(), a.s, r, family)
			if err != nil {
				failure(w, 503, "temporarily_unavailable")
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", "60")
				failure(w, 429, "rate_limited")
				return
			}
		}
		next(w, r)
	}
}
func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, code string) {
	httpapi.Fail(w, status, code)
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		failure(w, 400, "invalid_request")
		return false
	}
	return true
}
func (a *identity) user(w http.ResponseWriter, r *http.Request) (int64, bool) {
	digest, valid := auth.AccessDigest(r)
	if !valid {
		failure(w, 401, "unauthorized")
		return 0, false
	}
	var id int64
	e := a.s.Pool.QueryRow(r.Context(), "SELECT u.id FROM sessions s JOIN users u ON u.id=s.user_id WHERE "+auth.SessionMatch+" AND u.enabled", digest).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		failure(w, 401, "unauthorized")
		return 0, false
	}
	if e != nil {
		failure(w, 503, "temporarily_unavailable")
		return 0, false
	}
	return id, true
}
func (a *identity) emailStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := a.user(w, r)
	if !ok {
		return
	}
	var email *string
	var verified *time.Time
	if e := a.s.Pool.QueryRow(r.Context(), "SELECT email,email_verified_at FROM users WHERE id=$1", id).Scan(&email, &verified); e != nil {
		failure(w, 500, "internal_error")
		return
	}
	reply(w, 200, map[string]any{"email": email, "verified_at": verified, "configured": a.o.Mail != nil})
}
func (a *identity) emailRequest(w http.ResponseWriter, r *http.Request) {
	if a.o.Mail == nil {
		failure(w, 503, "email_unavailable")
		return
	}
	id, ok := a.user(w, r)
	if !ok {
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	address, e := mail.ParseAddress(input.Email)
	if e != nil || address.Address != input.Email || len(input.Email) > 254 {
		failure(w, 400, "invalid_request")
		return
	}
	// Keep the prior token visible until SMTP succeeds; rollback preserves it on failure.
	// The row lock also serializes resends and verification across replicas.
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		failure(w, 503, "temporarily_unavailable")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	token := rand.Text()
	var hash string
	e = tx.QueryRow(r.Context(), `INSERT INTO email_verifications(token_hash,user_id,email,expires_at) VALUES($1,$2,$3,now()+interval '30 minutes')
  ON CONFLICT(user_id) DO UPDATE SET token_hash=excluded.token_hash,email=excluded.email,expires_at=excluded.expires_at,created_at=now()
  WHERE email_verifications.created_at<now()-interval '1 minute' RETURNING token_hash`, auth.Digest(token), id, input.Email).Scan(&hash)
	if errors.Is(e, pgx.ErrNoRows) {
		failure(w, 429, "rate_limited")
		return
	}
	if e != nil {
		failure(w, 500, "internal_error")
		return
	}
	link := a.o.Origin + "/verify-email?token=" + url.QueryEscape(token)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if a.o.Mail(ctx, input.Email, link) != nil {
		failure(w, 503, "email_unavailable")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		failure(w, 503, "temporarily_unavailable")
		return
	}
	reply(w, 202, map[string]string{"status": "sent"})
}
func (a *identity) emailVerify(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Token) > 256 || input.Token == "" {
		failure(w, 400, "invalid_token")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		failure(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id int64
	var email string
	e = tx.QueryRow(r.Context(), "DELETE FROM email_verifications WHERE token_hash=$1 AND expires_at>now() RETURNING user_id,email", auth.Digest(input.Token)).Scan(&id, &email)
	if e != nil {
		failure(w, 400, "invalid_token")
		return
	}
	updated, e := tx.Exec(r.Context(), "UPDATE users SET email=$1,email_verified_at=now() WHERE id=$2 AND enabled", email, id)
	if e != nil {
		failure(w, 409, "email_unavailable")
		return
	}
	if updated.RowsAffected() != 1 {
		failure(w, 403, "forbidden")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		failure(w, 500, "internal_error")
		return
	}
	reply(w, 200, map[string]string{"status": "verified"})
}
