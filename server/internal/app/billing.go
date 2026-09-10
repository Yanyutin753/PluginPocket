package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/jackc/pgx/v5"
)

type Plan struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Credits    int64     `json:"credits"`
	PriceCents int64     `json:"price_cents"`
	Currency   string    `json:"currency"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// jsonPage reads one bounded page. Queries return the public JSON projection and its ID.
func (a *application) jsonPage(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	limit, cursor, ok := pagination(w, r)
	if !ok {
		return
	}
	args = append([]any{cursor, limit + 1}, args...)
	rows, e := a.s.Pool.Query(r.Context(), query, args...)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	ids := []int64{}
	for rows.Next() {
		var raw json.RawMessage
		var id int64
		if e = rows.Scan(&raw, &id); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		items = append(items, raw)
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = strconv.FormatInt(ids[limit-1], 10)
	}
	respond(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a *application) plans(w http.ResponseWriter, r *http.Request) {
	admin := strings.Contains(r.URL.Path, "/admin/")
	if admin {
		if _, ok := a.currentUser(w, r, true); !ok {
			return
		}
	}
	a.jsonPage(w, r, "SELECT to_jsonb(p),id FROM plans p WHERE ($1::bigint=0 OR id<$1) AND ($3::boolean OR enabled) ORDER BY id DESC LIMIT $2", admin)
}
func (a *application) savePlan(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var in struct {
		Name       string `json:"name"`
		Credits    int64  `json:"credits"`
		PriceCents int64  `json:"price_cents"`
		Currency   string `json:"currency"`
		Enabled    bool   `json:"enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 || in.Credits < 1 || in.Credits > 1000000000000 || in.PriceCents < 0 || in.PriceCents > 1000000000000 || !currencyPattern.MatchString(in.Currency) {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var item Plan
	status := 201
	if r.Method == http.MethodPost {
		e = tx.QueryRow(r.Context(), "INSERT INTO plans(name,credits,price_cents,currency,enabled) VALUES($1,$2,$3,$4,$5) RETURNING id,name,credits,price_cents,currency,enabled,created_at", in.Name, in.Credits, in.PriceCents, in.Currency, in.Enabled).Scan(&item.ID, &item.Name, &item.Credits, &item.PriceCents, &item.Currency, &item.Enabled, &item.CreatedAt)
	} else {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		status = 200
		e = tx.QueryRow(r.Context(), "UPDATE plans SET name=$1,credits=$2,price_cents=$3,currency=$4,enabled=$5 WHERE id=$6 RETURNING id,name,credits,price_cents,currency,enabled,created_at", in.Name, in.Credits, in.PriceCents, in.Currency, in.Enabled, id).Scan(&item.ID, &item.Name, &item.Credits, &item.PriceCents, &item.Currency, &item.Enabled, &item.CreatedAt)
	}
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
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, status, map[string]any{"item": item})
}
func (a *application) codes(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	a.jsonPage(w, r, "SELECT jsonb_build_object('id',id,'credits',credits,'note',note,'created_at',created_at,'redeemed_at',redeemed_at),id FROM redemption_codes WHERE ($1::bigint=0 OR id<$1) ORDER BY id DESC LIMIT $2")
}
func (a *application) createCode(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.currentUser(w, r, true)
	if !ok {
		return
	}
	var in struct {
		Credits int64  `json:"credits"`
		Note    string `json:"note"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Credits < 1 || in.Credits > 1000000000000 || len(strings.TrimSpace(in.Note)) < 1 || len(in.Note) > 500 {
		fail(w, 400, "invalid_request")
		return
	}
	code, e := auth.Secret("ldr_")
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
	var item json.RawMessage
	e = tx.QueryRow(r.Context(), "INSERT INTO redemption_codes(code_hash,credits,note,created_by) VALUES($1,$2,$3,$4) RETURNING jsonb_build_object('id',id,'credits',credits,'note',note,'created_at',created_at,'redeemed_at',redeemed_at)", auth.Digest(code), in.Credits, in.Note, actor.ID).Scan(&item)
	if e != nil {
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
	respond(w, 201, map[string]any{"code": code, "item": item})
}
func (a *application) redeem(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Code) < 1 || len(in.Code) > 200 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id, credits int64
	var redeemed *time.Time
	e = tx.QueryRow(r.Context(), "SELECT id,credits,redeemed_at FROM redemption_codes WHERE code_hash=$1 FOR UPDATE", auth.Digest(in.Code)).Scan(&id, &credits, &redeemed)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if redeemed != nil {
		fail(w, 409, "already_redeemed")
		return
	}
	var wallet, balance int64
	if e = tx.QueryRow(r.Context(), "SELECT id FROM wallets WHERE user_id=$1 FOR UPDATE", u.ID).Scan(&wallet); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok := currentUserQuery(w, r, false, tx); !ok {
		return
	}
	e = tx.QueryRow(r.Context(), "UPDATE wallets SET balance=balance+$1 WHERE user_id=$2 RETURNING id,balance", credits, u.ID).Scan(&wallet, &balance)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE redemption_codes SET redeemed_by=$1,redeemed_at=now() WHERE id=$2", u.ID, id); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "INSERT INTO ledger(wallet_id,user_id,delta,kind,note,idempotency_key,actor_id,balance_after) VALUES($1,$2,$3,'redemption','Redeemed code',$4,$2,$5)", wallet, u.ID, credits, "redemption:"+strconv.FormatInt(id, 10), balance); e != nil {
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
	respond(w, 200, map[string]any{"balance": balance, "credits": credits})
}
func (a *application) orders(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	a.jsonPage(w, r, "SELECT to_jsonb(o)-'user_id',id FROM orders o WHERE user_id=$3 AND ($1::bigint=0 OR id<$1) ORDER BY id DESC LIMIT $2", u.ID)
}
func (a *application) createOrder(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, false); !ok {
		return
	}
	var in struct {
		PlanID int64 `json:"plan_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.PlanID < 1 {
		fail(w, 400, "invalid_request")
		return
	}
	var exists bool
	e := a.s.Pool.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM plans WHERE id=$1 AND enabled)", in.PlanID).Scan(&exists)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if !exists {
		fail(w, 404, "not_found")
		return
	}
	fail(w, 503, "payment_unavailable")
}
func ledgerKind(w http.ResponseWriter, r *http.Request) (string, bool) {
	kind := r.URL.Query().Get("kind")
	switch kind {
	case "", "registration", "adjustment", "redemption", "team_transfer", "reservation", "refund", "recovery":
		return kind, true
	default:
		fail(w, 400, "invalid_request")
		return "", false
	}
}
func (a *application) ledger(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	kind, ok := ledgerKind(w, r)
	if !ok {
		return
	}
	a.jsonPage(w, r, "SELECT jsonb_build_object('id',l.id,'delta',delta,'kind',kind,'note',note,'created_at',l.created_at,'balance_after',balance_after),l.id FROM ledger l JOIN wallets w ON w.id=l.wallet_id WHERE w.user_id=$3 AND ($1::bigint=0 OR l.id<$1) AND ($4='' OR kind=$4) ORDER BY l.id DESC LIMIT $2", u.ID, kind)
}
func (a *application) adminLedger(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	kind, ok := ledgerKind(w, r)
	if !ok {
		return
	}
	var userID, actorID int64
	for key, dst := range map[string]*int64{"user_id": &userID, "actor_id": &actorID} {
		if raw := r.URL.Query().Get(key); raw != "" {
			id, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || id < 1 {
				fail(w, 400, "invalid_request")
				return
			}
			*dst = id
		}
	}
	a.jsonPage(w, r, "SELECT jsonb_build_object('id',id,'user_id',user_id,'actor_id',actor_id,'wallet_id',wallet_id,'delta',delta,'kind',kind,'note',note,'created_at',created_at,'balance_after',balance_after),id FROM ledger WHERE ($1::bigint=0 OR id<$1) AND ($3::bigint=0 OR user_id=$3) AND ($4::bigint=0 OR actor_id=$4) AND ($5='' OR kind=$5) ORDER BY id DESC LIMIT $2", userID, actorID, kind)
}
