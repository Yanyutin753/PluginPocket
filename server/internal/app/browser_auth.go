package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/Yanyutin753/PluginPocket/server/internal/settings"
	"github.com/jackc/pgx/v5"
)

func (a *application) browserTTL(ctx context.Context, q settings.Querier) (auth.BrowserTTL, error) {
	if a.options.Runtime == nil {
		return (auth.BrowserTTL{}).Defaults(), nil
	}
	v, err := a.options.Runtime.Read(ctx, q)
	return auth.BrowserTTL{AccessSeconds: v.AccessTokenSeconds, RefreshSeconds: v.RefreshTokenSeconds}, err
}

func (a *application) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.RefreshCookie)
	if err != nil || !strings.HasPrefix(cookie.Value, "rt_") {
		fail(w, 401, "unauthorized")
		return
	}
	ttl, err := a.browserTTL(r.Context(), nil)
	if err != nil {
		fail(w, 503, "temporarily_unavailable")
		return
	}
	tx, err := a.s.Pool.Begin(r.Context())
	if err != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id int64
	err = tx.QueryRow(r.Context(), `SELECT s.id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.session_hash=$1 AND s.expires_at>statement_timestamp() AND u.enabled FOR UPDATE OF s`, auth.Digest(cookie.Value)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 401, "unauthorized")
		return
	}
	if err != nil {
		fail(w, 500, "internal_error")
		return
	}
	access, expires, err := auth.IssueAccess(r.Context(), tx, id, ttl.AccessSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 401, "unauthorized")
		return
	}
	if err != nil {
		fail(w, 500, "internal_error")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 500, "internal_error")
		return
	}
	auth.SetAccessCookie(w, access, expires, a.options.SecureCookies, ttl.AccessSeconds)
	w.WriteHeader(http.StatusNoContent)
}
