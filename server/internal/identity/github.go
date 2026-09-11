package identity

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
)

func (a *identity) githubStart(w http.ResponseWriter, r *http.Request) {
	if a.o.GitHub == nil {
		failure(w, 503, "github_unavailable")
		return
	}
	if _, e := a.s.Pool.Exec(r.Context(), "DELETE FROM oauth_states WHERE state_hash IN (SELECT state_hash FROM oauth_states WHERE expires_at<now() ORDER BY expires_at LIMIT 1000)"); e != nil {
		failure(w, 500, "internal_error")
		return
	}
	state, browser, verifier := rand.Text(), rand.Text(), oauth2.GenerateVerifier()
	_, e := a.s.Pool.Exec(r.Context(), "INSERT INTO oauth_states(state_hash,browser_hash,verifier,expires_at) VALUES($1,$2,$3,now()+interval '10 minutes')", auth.Digest(state), auth.Digest(browser), verifier)
	if e != nil {
		failure(w, 500, "internal_error")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loadout_oauth_state", Value: browser, Path: "/api/v1/auth/github", HttpOnly: true, Secure: a.o.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	http.Redirect(w, r, a.o.GitHub.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}
func (a *identity) githubCallback(w http.ResponseWriter, r *http.Request) {
	if a.o.GitHub == nil {
		failure(w, 503, "github_unavailable")
		return
	}
	cookie, e := r.Cookie("loadout_oauth_state")
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if e != nil || state == "" || len(state) > 256 || code == "" || len(code) > 2048 {
		failure(w, 400, "invalid_state")
		return
	}
	var verifier string
	e = a.s.Pool.QueryRow(r.Context(), "DELETE FROM oauth_states WHERE state_hash=$1 AND browser_hash=$2 AND expires_at>now() RETURNING verifier", auth.Digest(state), auth.Digest(cookie.Value)).Scan(&verifier)
	if e != nil {
		failure(w, 400, "invalid_state")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loadout_oauth_state", Value: "", Path: "/api/v1/auth/github", HttpOnly: true, Secure: a.o.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.o.Client)
	token, e := a.o.GitHub.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if e != nil {
		failure(w, 502, "github_failed")
		return
	}
	client := a.o.GitHub.Client(ctx, token)
	req, e := http.NewRequestWithContext(ctx, "GET", a.o.UserURL, nil)
	if e != nil {
		failure(w, 502, "github_failed")
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	response, e := client.Do(req)
	if e != nil {
		failure(w, 502, "github_failed")
		return
	}
	defer func() { _ = response.Body.Close() }()
	var profile struct {
		ID int64 `json:"id"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&profile) != nil || profile.ID <= 0 {
		failure(w, 502, "github_failed")
		return
	}
	if a.o.RequiredOrg != "" {
		request, e := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/memberships/orgs/"+a.o.RequiredOrg, nil)
		if e != nil {
			failure(w, 403, "organization_required")
			return
		}
		membership, e := client.Do(request)
		if e != nil {
			failure(w, 403, "organization_required")
			return
		}
		var body struct {
			State string `json:"state"`
		}
		e = json.NewDecoder(io.LimitReader(membership.Body, 1<<20)).Decode(&body)
		_ = membership.Body.Close()
		if e != nil || membership.StatusCode != 200 || body.State != "active" {
			failure(w, 403, "organization_required")
			return
		}
	}
	tx, e := a.s.Pool.Begin(ctx)
	if e != nil {
		failure(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	subject := strconv.FormatInt(profile.ID, 10)
	// The lock is scoped to provider identity; concurrent first callbacks cannot create two accounts.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "github:"+subject); e != nil {
		failure(w, 500, "internal_error")
		return
	}
	var id int64
	e = tx.QueryRow(ctx, "SELECT user_id FROM oauth_identities WHERE provider='github' AND subject=$1", subject).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		username := "gh_" + rand.Text()[:16]
		if e = tx.QueryRow(ctx, "INSERT INTO users(username,password_hash) VALUES($1,'oauth-only') RETURNING id", username).Scan(&id); e != nil {
			failure(w, 500, "internal_error")
			return
		}
		var wallet int64
		if e = tx.QueryRow(ctx, "INSERT INTO wallets(user_id,balance) VALUES($1,$2) RETURNING id", id, *a.o.InitialCredits).Scan(&wallet); e != nil {
			failure(w, 500, "internal_error")
			return
		}
		if _, e = tx.Exec(ctx, "INSERT INTO ledger(wallet_id,user_id,delta,balance_after,kind,note,idempotency_key) SELECT $1,$2,$3,$3,'registration','Initial credits',$4 WHERE $3::bigint > 0", wallet, id, *a.o.InitialCredits, fmt.Sprintf("registration:%d", id)); e != nil {
			failure(w, 500, "internal_error")
			return
		}
		if _, e = tx.Exec(ctx, "INSERT INTO oauth_identities(provider,subject,user_id) VALUES('github',$1,$2)", subject, id); e != nil {
			failure(w, 500, "internal_error")
			return
		}
	} else if e != nil {
		failure(w, 500, "internal_error")
		return
	}
	var enabled bool
	if e = tx.QueryRow(ctx, "SELECT enabled FROM users WHERE id=$1", id).Scan(&enabled); e != nil || !enabled {
		failure(w, 403, "forbidden")
		return
	}
	session, e := auth.NewBrowserSession(ctx, tx, id, a.o.BrowserTTL)
	if e != nil {
		failure(w, 500, "internal_error")
		return
	}
	if e = tx.Commit(ctx); e != nil {
		failure(w, 500, "internal_error")
		return
	}
	auth.SetBrowserCookies(w, session, a.o.SecureCookies)
	http.Redirect(w, r, "/", http.StatusFound)
}
