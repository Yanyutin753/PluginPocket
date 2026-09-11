package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (a *application) deviceAuthorize(w http.ResponseWriter, r *http.Request) {
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	device, e := auth.Secret("ldd_")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	random, e := auth.Secret("")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	userCode := strings.ToUpper(random[:10])
	if _, e = a.s.Pool.Exec(r.Context(), "DELETE FROM device_authorizations WHERE id IN (SELECT id FROM device_authorizations WHERE expires_at<now() ORDER BY id LIMIT 100)"); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = a.s.Pool.Exec(r.Context(), "INSERT INTO device_authorizations(device_hash,user_code_hash,expires_at) VALUES($1,$2,statement_timestamp()+interval '10 minutes')", auth.Digest(device), auth.Digest(userCode)); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	origin := a.options.Origin
	if origin == "" {
		origin = "http://" + r.Host
		if r.TLS != nil {
			origin = "https://" + r.Host
		}
	}
	respond(w, 200, map[string]any{"device_code": device, "user_code": userCode, "verification_uri": origin + "/devices", "expires_in": 600, "interval": 5})
}
func (a *application) approveDevice(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"user_code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Code) != 10 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	tag, e := tx.Exec(r.Context(), "UPDATE device_authorizations SET approved_user_id=$1 WHERE user_code_hash=$2 AND approved_user_id IS NULL AND consumed_at IS NULL AND expires_at>now()", u.ID, auth.Digest(strings.ToUpper(in.Code)))
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "invalid_device_code")
		return
	}
	if _, ok := currentUserQuery(w, r, false, tx); !ok {
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	w.WriteHeader(204)
}
func (a *application) deviceToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Device string `json:"device_code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Device) < 1 || len(in.Device) > 200 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id int64
	var userID *int64
	var valid, throttled bool
	var consumed *time.Time
	e = tx.QueryRow(r.Context(), "SELECT id,approved_user_id,expires_at>statement_timestamp(),next_poll_at>statement_timestamp(),consumed_at FROM device_authorizations WHERE device_hash=$1 FOR UPDATE", auth.Digest(in.Device)).Scan(&id, &userID, &valid, &throttled, &consumed)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 400, "invalid_grant")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	// The locking statement may have started before the device expired.
	if e = tx.QueryRow(r.Context(), "SELECT expires_at>statement_timestamp(),next_poll_at>statement_timestamp() FROM device_authorizations WHERE id=$1", id).Scan(&valid, &throttled); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if consumed != nil {
		fail(w, 400, "invalid_grant")
		return
	}
	if !valid {
		fail(w, 400, "expired_token")
		return
	}
	if throttled {
		w.Header().Set("Retry-After", "5")
		fail(w, 429, "slow_down")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE device_authorizations SET next_poll_at=statement_timestamp()+interval '5 seconds' WHERE id=$1", id); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if userID == nil {
		if e = tx.Commit(r.Context()); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		fail(w, 400, "authorization_pending")
		return
	}
	secret, e := auth.Secret("ppt_")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	tag, e := tx.Exec(r.Context(), "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) SELECT u.id,w.id,'Authorized device',$2,$3 FROM users u JOIN wallets w ON w.user_id=u.id WHERE u.id=$1 AND u.enabled", *userID, secret[:12], auth.Digest(secret))
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if tag.RowsAffected() != 1 {
		fail(w, 400, "invalid_grant")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE device_authorizations SET consumed_at=now() WHERE id=$1", id); e != nil {
		fail(w, 500, "internal_error")
		return
	}

	var enabled bool
	if e = tx.QueryRow(r.Context(), "SELECT d.expires_at>statement_timestamp(),u.enabled FROM device_authorizations d JOIN users u ON u.id=d.approved_user_id WHERE d.id=$1", id).Scan(&valid, &enabled); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if !valid {
		fail(w, 400, "expired_token")
		return
	}
	if !enabled {
		fail(w, 400, "invalid_grant")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]string{"token": secret})
}
