package app

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// BillingRole 是计费角色：multiplier_bp 以基点表示消耗倍率（10000 = 1.0×），
// 在 store 预留事务内对工具价折算，与鉴权角色（users.role）无关。
type BillingRole struct {
	Name         string `json:"name"`
	MultiplierBP int64  `json:"multiplier_bp"`
	Description  string `json:"description"`
}

func (a *application) billingRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT name,multiplier_bp,description FROM billing_roles ORDER BY multiplier_bp")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	items := []BillingRole{}
	for rows.Next() {
		var item BillingRole
		if e = rows.Scan(&item.Name, &item.MultiplierBP, &item.Description); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"items": items})
}

func (a *application) updateBillingRole(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	name := r.PathValue("name")
	var in struct {
		MultiplierBP *int64  `json:"multiplier_bp"`
		Description  *string `json:"description"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.MultiplierBP == nil && in.Description == nil {
		fail(w, 400, "invalid_request")
		return
	}
	if in.MultiplierBP != nil && (*in.MultiplierBP < 0 || *in.MultiplierBP > 1000000) {
		fail(w, 400, "invalid_request")
		return
	}
	if in.Description != nil && len(*in.Description) > 200 {
		fail(w, 400, "invalid_request")
		return
	}
	tag, e := a.s.Pool.Exec(r.Context(), "UPDATE billing_roles SET multiplier_bp=COALESCE($1,multiplier_bp), description=COALESCE($2,description) WHERE name=$3", in.MultiplierBP, in.Description, name)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "not_found")
		return
	}
	var item BillingRole
	if e = a.s.Pool.QueryRow(r.Context(), "SELECT name,multiplier_bp,description FROM billing_roles WHERE name=$1", name).Scan(&item.Name, &item.MultiplierBP, &item.Description); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			fail(w, 404, "not_found")
			return
		}
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"item": item})
}
