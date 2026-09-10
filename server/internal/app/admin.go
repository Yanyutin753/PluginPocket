package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/gateway"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Tool struct {
	ID          int64           `json:"id"`
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Kind        string          `json:"kind"`
	Enabled     bool            `json:"enabled"`
	Units       int64           `json:"units_per_call"`
	Schema      json.RawMessage `json:"input_schema"`
	Configured  *bool           `json:"configured,omitempty"`
}

func (a *application) setUserEnabled(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.currentUser(w, r, true)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Enabled == nil {
		fail(w, 400, "invalid_request")
		return
	}
	if id == actor.ID && !*in.Enabled {
		fail(w, 409, "cannot_disable_self")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, e = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(817392106)"); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var u User
	e = tx.QueryRow(r.Context(), "SELECT u.id,u.username,u.role,w.balance,u.enabled FROM users u JOIN wallets w ON w.user_id=u.id WHERE u.id=$1 FOR UPDATE OF u", id).Scan(&u.ID, &u.Username, &u.Role, &u.Balance, &u.Enabled)
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
	if !*in.Enabled && u.Role == "admin" {
		var others int
		e = tx.QueryRow(r.Context(), "SELECT count(*) FROM users WHERE role='admin' AND enabled AND id<>$1", id).Scan(&others)
		if e != nil {
			fail(w, 500, "internal_error")
			return
		}
		if others == 0 {
			fail(w, 409, "last_admin")
			return
		}
	}
	if _, e = tx.Exec(r.Context(), "UPDATE users SET enabled=$1 WHERE id=$2", *in.Enabled, id); e != nil {
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
	u.Enabled = *in.Enabled
	respond(w, 200, map[string]any{"user": u})
}
func (a *application) listTools(w http.ResponseWriter, r *http.Request) {
	admin := strings.HasPrefix(r.URL.Path, "/api/v1/admin/")
	if _, ok := a.currentUser(w, r, admin); !ok {
		return
	}
	limit, cursor, ok := pagination(w, r)
	if !ok {
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT id,key,name,description,kind,enabled,cost,input_schema,config<>'{}'::jsonb FROM tools WHERE ($1::boolean OR enabled) AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT $3", admin, cursor, limit+1)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	items := []Tool{}
	for rows.Next() {
		var item Tool
		var configured bool
		if e = rows.Scan(&item.ID, &item.Key, &item.Name, &item.Description, &item.Kind, &item.Enabled, &item.Units, &item.Schema, &configured); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		if admin {
			item.Configured = &configured
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
func (a *application) saveTool(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var in struct {
		Key         string          `json:"key"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Kind        string          `json:"kind"`
		Enabled     bool            `json:"enabled"`
		Units       int64           `json:"units_per_call"`
		Schema      json.RawMessage `json:"input_schema"`
		Config      json.RawMessage `json:"config"`
	}
	if !decode(w, r, &in) {
		return
	}
	var schema map[string]any
	if !usernamePattern.MatchString(in.Key) || len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 || len(in.Description) > 2000 || in.Units < 0 || in.Units > 1000000000000 || json.Unmarshal(in.Schema, &schema) != nil || schema["type"] != "object" {
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
	var previousKind string
	var config []byte
	if r.Method == http.MethodPatch {
		var ok bool
		id, ok = pathID(w, r)
		if !ok {
			return
		}
		e := tx.QueryRow(r.Context(), "SELECT kind,config FROM tools WHERE id=$1 FOR UPDATE", id).Scan(&previousKind, &config)
		if errors.Is(e, pgx.ErrNoRows) {
			fail(w, 404, "not_found")
			return
		}
		if e != nil {
			fail(w, 500, "internal_error")
			return
		}
	}
	switch in.Kind {
	case "builtin":
		if gateway.ValidateToolSchema(in.Schema) != nil {
			fail(w, 400, "invalid_request")
			return
		}
		if in.Key != "echo" && in.Key != "time_now" {
			fail(w, 400, "invalid_request")
			return
		}
		config = []byte(`{}`)
	case "http", "stdio":
		if len(in.Config) > 0 {
			if a.options.Gateway == nil || a.options.Gateway.ValidateConfig(in.Kind, in.Config) != nil {
				fail(w, 400, "invalid_request")
				return
			}
			var e error
			config, e = gateway.SealConfig(a.options.EncryptionKey, in.Config)
			if e != nil {
				fail(w, 503, "upstream_unavailable")
				return
			}
		} else if len(config) == 0 || previousKind != in.Kind {
			fail(w, 400, "invalid_request")
			return
		}
	default:
		fail(w, 400, "invalid_request")
		return
	}
	var item Tool
	if _, ok := currentUserQuery(w, r, true, tx); !ok {
		return
	}
	if id == 0 {
		e = tx.QueryRow(r.Context(), "INSERT INTO tools(key,name,description,kind,enabled,cost,input_schema,config) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id", in.Key, in.Name, in.Description, in.Kind, in.Enabled, in.Units, []byte(in.Schema), config).Scan(&id)
	} else {
		_, e = tx.Exec(r.Context(), "UPDATE tools SET key=$1,name=$2,description=$3,kind=$4,enabled=$5,cost=$6,input_schema=$7,config=$8,updated_at=now() WHERE id=$9", in.Key, in.Name, in.Description, in.Kind, in.Enabled, in.Units, []byte(in.Schema), config, id)
	}
	if e != nil {
		var pe *pgconn.PgError
		if errors.As(e, &pe) && pe.Code == "23505" {
			fail(w, 409, "key_taken")
		} else {
			fail(w, 500, "internal_error")
		}
		return
	}
	if _, ok := currentUserQuery(w, r, true, tx); !ok {
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	configured := in.Kind != "builtin"
	item = Tool{ID: id, Key: in.Key, Name: in.Name, Description: in.Description, Kind: in.Kind, Enabled: in.Enabled, Units: in.Units, Schema: in.Schema, Configured: &configured}
	if a.options.Gateway != nil {
		a.options.Gateway.Invalidate()
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	respond(w, status, map[string]any{"item": item})
}
func (a *application) globalUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var uid int64
	if raw := r.URL.Query().Get("user_id"); raw != "" {
		var e error
		uid, e = strconv.ParseInt(raw, 10, 64)
		if e != nil || uid < 1 {
			fail(w, 400, "invalid_request")
			return
		}
	}
	a.usagePage(w, r, uid, 0)
}
func (a *application) usagePage(w http.ResponseWriter, r *http.Request, userID, walletID int64) {
	limit, cursor, ok := pagination(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "pending" && status != "ok" && status != "error" && status != "denied" && status != "recovered" {
		fail(w, 400, "invalid_request")
		return
	}
	tool := r.URL.Query().Get("tool")
	if len(tool) > 200 {
		fail(w, 400, "invalid_request")
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT id,user_id,token_id,wallet_id,tool,cost,status,duration_ms,created_at FROM usage_logs WHERE ($1::bigint=0 OR user_id=$1) AND ($2::bigint=0 OR wallet_id=$2) AND ($3::bigint=0 OR id<$3) AND ($4='' OR status=$4) AND ($5='' OR tool=$5) ORDER BY id DESC LIMIT $6", userID, walletID, cursor, status, tool, limit+1)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	items := []store.Call{}
	for rows.Next() {
		var c store.Call
		if e = rows.Scan(&c.ID, &c.UserID, &c.TokenID, &c.WalletID, &c.Tool, &c.Cost, &c.Status, &c.DurationMS, &c.CreatedAt); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		items = append(items, c)
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
	if strings.HasSuffix(r.URL.Path, "/export") {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="loadout-usage.csv"`)
		w.Header().Set("X-Next-Cursor", next)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"id", "user_id", "tool", "cost", "status", "duration_ms", "created_at"})
		for _, c := range items {
			_ = writer.Write([]string{strconv.FormatInt(c.ID, 10), strconv.FormatInt(c.UserID, 10), safeCSV(c.Tool), strconv.FormatInt(c.Cost, 10), c.Status, strconv.FormatInt(c.DurationMS, 10), c.CreatedAt.Format(time.RFC3339)})
		}
		writer.Flush()
		return
	}
	respond(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func safeCSV(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
