package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	BillingRole string `json:"billing_role"`
	Balance     int64  `json:"balance"`
	Enabled     bool   `json:"enabled"`
}
type Token struct {
	ID         int64      `json:"id"`
	WalletID   int64      `json:"wallet_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeLimit(w, r, dst, 16384)
}

func decodeLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		fail(w, 400, "invalid_request")
		return false
	}
	if e := d.Decode(new(any)); e != io.EOF {
		fail(w, 400, "invalid_request")
		return false
	}
	return true
}
func (a *application) currentUser(w http.ResponseWriter, r *http.Request, admin bool) (User, bool) {
	var q userQuerier
	if a.s != nil {
		q = a.s.Pool
	}
	return currentUserQuery(w, r, admin, q)
}

type userQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Read authorization after business locks using a fresh database statement timestamp.
func currentUserQuery(w http.ResponseWriter, r *http.Request, admin bool, q userQuerier) (User, bool) {
	var u User
	digest, valid := auth.AccessDigest(r)
	if !valid {
		fail(w, 401, "unauthorized")
		return u, false
	}
	e := q.QueryRow(r.Context(), "SELECT u.id,u.username,u.role,w.balance,u.enabled FROM sessions s JOIN users u ON u.id=s.user_id JOIN wallets w ON w.user_id=u.id WHERE "+auth.SessionMatch+" AND u.enabled", digest).Scan(&u.ID, &u.Username, &u.Role, &u.Balance, &u.Enabled)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 401, "unauthorized")
		return u, false
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return u, false
	}
	if admin && u.Role != "admin" {
		fail(w, 403, "forbidden")
		return u, false
	}
	return u, true
}
func (a *application) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	// Specific codes (vs blanket invalid_request) so clients can explain
	// exactly what to fix; autofilled values skip native browser validation.
	if !usernamePattern.MatchString(in.Username) {
		fail(w, 400, "invalid_username")
		return
	}
	if len(in.Password) < 12 || len(in.Password) > 1024 {
		fail(w, 400, "invalid_password")
		return
	}
	initial := int64(1000)
	if a.options.InitialCredits != nil {
		initial = *a.options.InitialCredits
	}
	if a.options.Runtime != nil {
		snapshot, err := a.options.Runtime.Read(r.Context(), nil)
		if err != nil {
			fail(w, http.StatusServiceUnavailable, "temporarily_unavailable")
			return
		}
		initial = snapshot.InitialCredits
	}
	if initial < 0 || initial > 1000000000000 {
		fail(w, 503, "registration_unavailable")
		return
	}
	hash, e := auth.HashPassword(in.Password)
	if errors.Is(e, auth.ErrBusy) {
		fail(w, 429, "auth_busy")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var u User
	e = tx.QueryRow(r.Context(), "INSERT INTO users(username,password_hash) VALUES($1,$2) RETURNING id,username,role,enabled", in.Username, hash).Scan(&u.ID, &u.Username, &u.Role, &u.Enabled)
	if e != nil {
		var pe *pgconn.PgError
		if errors.As(e, &pe) && pe.Code == "23505" {
			fail(w, 409, "username_taken")
		} else {
			fail(w, 500, "internal_error")
		}
		return
	}
	var walletID int64
	if e = tx.QueryRow(r.Context(), "INSERT INTO wallets(user_id,balance) VALUES($1,$2) RETURNING id", u.ID, initial).Scan(&walletID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if initial > 0 {
		if _, e = tx.Exec(r.Context(), "INSERT INTO ledger(wallet_id,user_id,delta,kind,note,actor_id,balance_after) VALUES($1,$2,$3,'registration','Welcome credits',$2,$3)", walletID, u.ID, initial); e != nil {
			fail(w, 500, "internal_error")
			return
		}
	}
	u.Balance = initial
	ttl, e := a.browserTTL(r.Context(), tx)
	if e != nil {
		fail(w, 503, "temporarily_unavailable")
		return
	}
	session, e := auth.NewBrowserSession(r.Context(), tx, u.ID, ttl)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	auth.SetBrowserCookies(w, session, a.options.SecureCookies)
	respond(w, 201, map[string]any{"user": u})
}
func (a *application) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	var u User
	var hash string
	e := a.s.Pool.QueryRow(r.Context(), "SELECT u.id,u.username,u.role,w.balance,u.enabled,u.password_hash FROM users u JOIN wallets w ON w.user_id=u.id WHERE username=$1", in.Username).Scan(&u.ID, &u.Username, &u.Role, &u.Balance, &u.Enabled, &hash)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		fail(w, 500, "internal_error")
		return
	}
	valid, verifyError := auth.VerifyPassword(hash, in.Password)
	if errors.Is(verifyError, auth.ErrBusy) {
		fail(w, 429, "auth_busy")
		return
	}
	if e != nil || !u.Enabled || !valid {
		fail(w, 401, "invalid_credentials")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	ttl, e := a.browserTTL(r.Context(), tx)
	if e != nil {
		fail(w, 503, "temporarily_unavailable")
		return
	}
	session, e := auth.NewBrowserSession(r.Context(), tx, u.ID, ttl)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	auth.SetBrowserCookies(w, session, a.options.SecureCookies)
	respond(w, 200, map[string]any{"user": u})
}
func (a *application) logout(w http.ResponseWriter, r *http.Request) {
	var access, refresh string
	if c, err := r.Cookie(auth.AccessCookie); err == nil {
		access = auth.Digest(c.Value)
	}
	if c, err := r.Cookie(auth.RefreshCookie); err == nil {
		refresh = auth.Digest(c.Value)
	}
	if _, err := a.s.Pool.Exec(r.Context(), "DELETE FROM sessions WHERE session_hash=$1 OR session_hash=$2 OR id IN (SELECT session_id FROM session_access WHERE access_hash=$1)", access, refresh); err != nil {
		fail(w, 500, "internal_error")
		return
	}
	for _, name := range []string{auth.AccessCookie, auth.RefreshCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: a.options.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	}
	w.WriteHeader(204)
}
func (a *application) me(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	var summary struct {
		TodayCalls int64 `json:"today_calls"`
		MonthCost  int64 `json:"month_cost"`
		TokenCount int64 `json:"token_count"`
	}
	now, ok := a.reportTime(w, r)
	if !ok {
		return
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	e := a.s.Pool.QueryRow(r.Context(), "SELECT count(*) FILTER(WHERE created_at >= $2),COALESCE(sum(cost),0) FROM usage_logs WHERE user_id=$1 AND created_at >= $3", u.ID, day, month).Scan(&summary.TodayCalls, &summary.MonthCost)
	if e == nil {
		e = a.s.Pool.QueryRow(r.Context(), "SELECT count(*) FROM tokens WHERE user_id=$1 AND revoked_at IS NULL", u.ID).Scan(&summary.TokenCount)
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"user": u, "summary": summary})
}
func (a *application) verify(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		fail(w, 401, "unauthorized")
		return
	}
	p, e := a.s.AuthToken(r.Context(), strings.TrimPrefix(header, "Bearer "))
	if e != nil {
		if errors.Is(e, store.ErrUnauthorized) {
			fail(w, 401, "unauthorized")
		} else {
			fail(w, 500, "internal_error")
		}
		return
	}
	var balance int64
	if e = a.s.Pool.QueryRow(r.Context(), "SELECT balance FROM wallets WHERE id=$1", p.WalletID).Scan(&balance); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT key FROM tools WHERE enabled ORDER BY id")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if e = rows.Scan(&name); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		names = append(names, name)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"username": p.Username, "balance": balance, "tools": names})
}
func pagination(w http.ResponseWriter, r *http.Request) (int, int64, bool) {
	limit := 50
	var cursor int64
	var e error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, e = strconv.Atoi(raw)
		if e != nil || limit < 1 || limit > 100 {
			fail(w, 400, "invalid_request")
			return 0, 0, false
		}
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cursor, e = strconv.ParseInt(raw, 10, 64)
		if e != nil || cursor < 1 {
			fail(w, 400, "invalid_request")
			return 0, 0, false
		}
	}
	return limit, cursor, true
}
func (a *application) listTokens(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	limit, cursor, ok := pagination(w, r)
	if !ok {
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT id,wallet_id,name,prefix,created_at,revoked_at,last_used_at FROM tokens WHERE user_id=$1 AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT $3", u.ID, cursor, limit+1)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	items := []Token{}
	for rows.Next() {
		var item Token
		if e = rows.Scan(&item.ID, &item.WalletID, &item.Name, &item.Prefix, &item.CreatedAt, &item.RevokedAt, &item.LastUsedAt); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = strconv.FormatInt(items[len(items)-1].ID, 10)
	}
	respond(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a *application) createToken(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	var in struct {
		Name   string `json:"name"`
		TeamID int64  `json:"team_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 80 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var walletID int64
	if in.TeamID == 0 {
		e = tx.QueryRow(r.Context(), "SELECT id FROM wallets WHERE user_id=$1", u.ID).Scan(&walletID)
	} else {
		var lockedID int64
		e = tx.QueryRow(r.Context(), "SELECT id FROM teams WHERE id=$1 FOR UPDATE", in.TeamID).Scan(&lockedID)
		if e == nil {
			e = tx.QueryRow(r.Context(), "SELECT w.id FROM wallets w JOIN team_members m ON m.team_id=w.team_id WHERE w.team_id=$1 AND m.user_id=$2", in.TeamID, u.ID).Scan(&walletID)
		}
	}
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	secret, e := auth.Secret("ppt_")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var item Token
	if _, ok := currentUserQuery(w, r, false, tx); !ok {
		return
	}
	e = tx.QueryRow(r.Context(), "INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES($1,$5,$2,$3,$4) RETURNING id,wallet_id,name,prefix,created_at", u.ID, in.Name, secret[:12], auth.Digest(secret), walletID).Scan(&item.ID, &item.WalletID, &item.Name, &item.Prefix, &item.CreatedAt)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok := currentUserQuery(w, r, false, tx); !ok {
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 201, map[string]any{"token": secret, "item": item})
}
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id < 1 {
		fail(w, 400, "invalid_request")
		return 0, false
	}
	return id, true
}
func (a *application) revokeToken(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	tag, e := tx.Exec(r.Context(), "UPDATE tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1 AND user_id=$2", id, u.ID)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "not_found")
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
func (a *application) usage(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	a.usagePage(w, r, u.ID, 0)
}
func (a *application) users(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	limit, cursor, ok := pagination(w, r)
	if !ok {
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT u.id,u.username,u.role,u.billing_role,w.balance,u.enabled FROM users u JOIN wallets w ON w.user_id=u.id WHERE ($1::bigint=0 OR u.id<$1) ORDER BY u.id DESC LIMIT $2", cursor, limit+1)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Username, &u.Role, &u.BillingRole, &u.Balance, &u.Enabled); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		items = append(items, u)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = strconv.FormatInt(items[len(items)-1].ID, 10)
	}
	respond(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a *application) adjustBalance(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.currentUser(w, r, true)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Delta int64  `json:"delta"`
		Note  string `json:"note"`
		Key   string `json:"idempotency_key"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Delta == 0 || in.Delta < -1000000000000 || in.Delta > 1000000000000 || len(strings.TrimSpace(in.Note)) == 0 || len(in.Note) > 500 || len(in.Key) < 1 || len(in.Key) > 100 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var walletID int64
	var u User
	e = tx.QueryRow(r.Context(), "SELECT u.id,u.username,u.role,w.balance,u.enabled,w.id FROM users u JOIN wallets w ON w.user_id=u.id WHERE u.id=$1 FOR UPDATE OF w", id).Scan(&u.ID, &u.Username, &u.Role, &u.Balance, &u.Enabled, &walletID)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok := currentUserQuery(w, r, true, tx); !ok {
		return
	}
	var delta int64
	var note string
	e = tx.QueryRow(r.Context(), "SELECT delta,note,balance_after FROM ledger WHERE wallet_id=$1 AND kind='adjustment' AND idempotency_key=$2", walletID, in.Key).Scan(&delta, &note, &u.Balance)
	if e == nil {
		if delta != in.Delta || note != in.Note {
			fail(w, 409, "idempotency_conflict")
			return
		}
		respond(w, 200, map[string]any{"user": u})
		return
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		fail(w, 500, "internal_error")
		return
	}
	if u.Balance+in.Delta < 0 {
		fail(w, 409, "insufficient_balance")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE wallets SET balance=balance+$1 WHERE id=$2", in.Delta, walletID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "INSERT INTO ledger(wallet_id,user_id,delta,kind,note,idempotency_key,actor_id,balance_after) VALUES($1,$2,$3,'adjustment',$4,$5,$6,$7)", walletID, id, in.Delta, in.Note, in.Key, actor.ID, u.Balance+in.Delta); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok := currentUserQuery(w, r, true, tx); !ok {
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	u.Balance += in.Delta
	respond(w, 200, map[string]any{"user": u})
}
