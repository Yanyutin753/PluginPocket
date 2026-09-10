package app

import (
	"net/http"
	"time"
)

func reportDays(w http.ResponseWriter, r *http.Request) (int, bool) {
	switch r.URL.Query().Get("days") {
	case "", "7":
		return 7, true
	case "1":
		return 1, true
	default:
		fail(w, 400, "invalid_request")
		return 0, false
	}
}
func (a *application) globalSummary(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	days, ok := reportDays(w, r)
	if !ok {
		return
	}
	now, ok := a.reportTime(w, r)
	if !ok {
		return
	}
	a.jsonPage(w, r, `SELECT jsonb_build_object('tool',tool,'calls',count(*),'cost',sum(cost),'errors',count(*) FILTER(WHERE status IN ('error','denied','recovered'))),min(id)
 FROM usage_logs WHERE created_at >= $3
 GROUP BY tool HAVING ($1::bigint=0 OR min(id)<$1) ORDER BY min(id) DESC LIMIT $2`, reportStart(now, days))
}
func (a *application) teamSummary(w http.ResponseWriter, r *http.Request) {
	_, team, ok := a.teamAccess(w, r)
	if !ok {
		return
	}
	days, ok := reportDays(w, r)
	if !ok {
		return
	}
	now, ok := a.reportTime(w, r)
	if !ok {
		return
	}
	a.jsonPage(w, r, `SELECT jsonb_build_object('user_id',u.id,'username',u.username,'calls',count(*),'cost',sum(l.cost),'errors',count(*) FILTER(WHERE l.status IN ('error','denied','recovered'))),u.id
 FROM usage_logs l JOIN users u ON u.id=l.user_id WHERE l.wallet_id=$3 AND l.created_at >= $4
 AND ($1::bigint=0 OR u.id<$1) GROUP BY u.id,u.username ORDER BY u.id DESC LIMIT $2`, team.WalletID, reportStart(now, days))
}

// Bind a concrete UTC boundary so PostgreSQL can use joint time/owner statistics.
func reportStart(now time.Time, days int) time.Time {
	return now.UTC().Truncate(24*time.Hour).AddDate(0, 0, 1-days)
}

func (a *application) reportTime(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	var now time.Time
	if err := a.s.Pool.QueryRow(r.Context(), "SELECT statement_timestamp()").Scan(&now); err != nil {
		fail(w, 500, "internal_error")
		return time.Time{}, false
	}
	return now.UTC(), true
}
