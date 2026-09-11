package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
)

func (a *application) usageDetail(w http.ResponseWriter, r *http.Request) {
	var userID, walletID int64
	if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
		if _, ok := a.currentUser(w, r, true); !ok {
			return
		}
	} else if r.PathValue("id") != "" {
		_, team, ok := a.teamAccess(w, r)
		if !ok {
			return
		}
		walletID = team.WalletID
	} else {
		user, ok := a.currentUser(w, r, false)
		if !ok {
			return
		}
		userID = user.ID
	}
	id, err := strconv.ParseInt(r.PathValue("callID"), 10, 64)
	if err != nil || id < 1 {
		fail(w, 400, "invalid_request")
		return
	}
	var item struct {
		store.Call
		store.CallData
	}
	err = a.s.Pool.QueryRow(r.Context(), `SELECT id,user_id,token_id,wallet_id,tool,cost,status,duration_ms,created_at,input_data,output_data,input_truncated,output_truncated FROM usage_logs WHERE id=$1 AND ($2::bigint=0 OR user_id=$2) AND ($3::bigint=0 OR wallet_id=$3)`, id, userID, walletID).Scan(&item.ID, &item.UserID, &item.TokenID, &item.WalletID, &item.Tool, &item.Cost, &item.Status, &item.DurationMS, &item.CreatedAt, &item.InputData, &item.OutputData, &item.InputTruncated, &item.OutputTruncated)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if err != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"item": item})
}
