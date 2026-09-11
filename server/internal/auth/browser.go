package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const AccessCookie = "loadout_session"
const RefreshCookie = "loadout_refresh"

// Legacy sessions remain valid until their original expiry. Refresh tokens are
// rejected by AccessDigest before this predicate is used.
const SessionMatch = `(s.session_hash=$1 OR s.id IN (SELECT session_id FROM session_access WHERE access_hash=$1 AND expires_at>statement_timestamp())) AND s.expires_at>statement_timestamp()`

func AccessDigest(r *http.Request) (string, bool) {
	c, err := r.Cookie(AccessCookie)
	if err != nil || c.Value == "" || strings.HasPrefix(c.Value, "rt_") {
		return "", false
	}
	return Digest(c.Value), true
}

type BrowserTTL struct{ AccessSeconds, RefreshSeconds int }

func (ttl BrowserTTL) Defaults() BrowserTTL {
	if ttl.AccessSeconds == 0 {
		ttl.AccessSeconds = 900
	}
	if ttl.RefreshSeconds == 0 {
		ttl.RefreshSeconds = 604800
	}
	return ttl
}

type BrowserSession struct {
	TTL                           BrowserTTL
	Access, Refresh               string
	AccessExpires, RefreshExpires time.Time
}

func NewBrowserSession(ctx context.Context, tx pgx.Tx, userID int64, ttl BrowserTTL) (BrowserSession, error) {
	session := BrowserSession{TTL: ttl.Defaults()}
	var err error
	session.Refresh, err = Secret("rt_")
	if err != nil {
		return session, err
	}
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO sessions(user_id,session_hash,expires_at) VALUES($1,$2,statement_timestamp()+$3*interval '1 second') RETURNING id,expires_at`, userID, Digest(session.Refresh), session.TTL.RefreshSeconds).Scan(&id, &session.RefreshExpires)
	if err != nil {
		return session, err
	}
	session.Access, session.AccessExpires, err = IssueAccess(ctx, tx, id, session.TTL.AccessSeconds)
	return session, err
}

// Keep unexpired access credentials so independent tabs/replicas can refresh
// concurrently. The parent session is the single revocation authority.
func IssueAccess(ctx context.Context, tx pgx.Tx, sessionID int64, seconds int) (string, time.Time, error) {
	access, err := Secret("")
	var expires time.Time
	if err != nil {
		return "", expires, err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM session_access WHERE session_id=$1 AND expires_at<=statement_timestamp()", sessionID); err != nil {
		return "", expires, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO session_access(access_hash,session_id,expires_at) SELECT $1,id,LEAST(expires_at,statement_timestamp()+$3*interval '1 second') FROM sessions WHERE id=$2 AND expires_at>statement_timestamp() RETURNING expires_at`, Digest(access), sessionID, seconds).Scan(&expires)
	return access, expires, err
}

func SetBrowserCookies(w http.ResponseWriter, session BrowserSession, secure bool) {
	SetAccessCookie(w, session.Access, session.AccessExpires, secure, session.TTL.AccessSeconds)
	http.SetCookie(w, &http.Cookie{Name: RefreshCookie, Value: session.Refresh, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: session.TTL.RefreshSeconds, Expires: session.RefreshExpires})
}
func SetAccessCookie(w http.ResponseWriter, value string, expires time.Time, secure bool, seconds int) {
	http.SetCookie(w, &http.Cookie{Name: AccessCookie, Value: value, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: seconds, Expires: expires})
}
