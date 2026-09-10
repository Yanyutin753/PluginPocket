package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/jackc/pgx/v5"
)

type Team struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Balance   int64  `json:"balance"`
	SeatLimit int    `json:"seat_limit"`
	WalletID  int64  `json:"-"`
}

const teamSelect = "SELECT t.id,t.name,m.role,w.balance,t.seat_limit,w.id FROM teams t JOIN team_members m ON m.team_id=t.id JOIN wallets w ON w.team_id=t.id"

func readTeam(row pgx.Row) (Team, error) {
	var t Team
	e := row.Scan(&t.ID, &t.Name, &t.Role, &t.Balance, &t.SeatLimit, &t.WalletID)
	return t, e
}
func (a *application) teamAccess(w http.ResponseWriter, r *http.Request) (User, Team, bool) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return u, Team{}, false
	}
	id, ok := pathID(w, r)
	if !ok {
		return u, Team{}, false
	}
	team, e := readTeam(a.s.Pool.QueryRow(r.Context(), teamSelect+" WHERE t.id=$1 AND m.user_id=$2", id, u.ID))
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return u, team, false
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return u, team, false
	}
	return u, team, true
}
func (a *application) teamWrite(w http.ResponseWriter, r *http.Request, owner bool) (User, Team, pgx.Tx, bool) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return u, Team{}, nil, false
	}
	id, ok := pathID(w, r)
	if !ok {
		return u, Team{}, nil, false
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return u, Team{}, nil, false
	}
	// Acquire the team lock before reading membership so a wait cannot retain a stale role snapshot.
	var lockedID int64
	e = tx.QueryRow(r.Context(), "SELECT id FROM teams WHERE id=$1 FOR UPDATE", id).Scan(&lockedID)
	var team Team
	if e == nil {
		if _, ok := currentUserQuery(w, r, false, tx); !ok {
			_ = tx.Rollback(context.Background())
			return u, Team{}, nil, false
		}
		team, e = readTeam(tx.QueryRow(r.Context(), teamSelect+" WHERE t.id=$1 AND m.user_id=$2", id, u.ID))
	}
	if e != nil {
		_ = tx.Rollback(context.Background())
		if errors.Is(e, pgx.ErrNoRows) {
			fail(w, 404, "not_found")
		} else {
			fail(w, 500, "internal_error")
		}
		return u, team, nil, false
	}
	if owner && team.Role != "owner" {
		_ = tx.Rollback(context.Background())
		fail(w, 403, "forbidden")
		return u, team, nil, false
	}
	return u, team, tx, true
}
func (a *application) teams(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	a.jsonPage(w, r, "SELECT jsonb_build_object('id',t.id,'name',t.name,'role',m.role,'balance',w.balance,'seat_limit',t.seat_limit),t.id FROM teams t JOIN team_members m ON m.team_id=t.id JOIN wallets w ON w.team_id=t.id WHERE m.user_id=$3 AND ($1::bigint=0 OR t.id<$1) ORDER BY t.id DESC LIMIT $2", u.ID)
}
func (a *application) team(w http.ResponseWriter, r *http.Request) {
	_, team, ok := a.teamAccess(w, r)
	if !ok {
		return
	}
	respond(w, 200, map[string]any{"item": team})
}
func (a *application) createTeam(w http.ResponseWriter, r *http.Request) {
	u, ok := a.currentUser(w, r, false)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	team := Team{Name: in.Name, Role: "owner", SeatLimit: 5}
	if e = tx.QueryRow(r.Context(), "INSERT INTO teams(name) VALUES($1) RETURNING id", in.Name).Scan(&team.ID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "INSERT INTO wallets(team_id) VALUES($1)", team.ID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "INSERT INTO team_members(team_id,user_id,role) VALUES($1,$2,'owner')", team.ID, u.ID); e != nil {
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
	respond(w, 201, map[string]any{"item": team})
}
func (a *application) updateTeam(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, false); !ok {
		return
	}
	var in struct {
		Name  *string `json:"name"`
		Seats *int    `json:"seat_limit"`
		Owner *int64  `json:"owner_user_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, team, tx, ok := a.teamWrite(w, r, true)
	if !ok {
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if in.Name == nil && in.Seats == nil && in.Owner == nil {
		fail(w, 400, "invalid_request")
		return
	}
	if in.Name != nil {
		if len(strings.TrimSpace(*in.Name)) < 1 || len(*in.Name) > 80 {
			fail(w, 400, "invalid_request")
			return
		}
		team.Name = *in.Name
	}
	if in.Seats != nil {
		if *in.Seats < 1 || *in.Seats > 1000 {
			fail(w, 400, "invalid_request")
			return
		}
		var count int
		if e := tx.QueryRow(r.Context(), "SELECT count(*) FROM team_members WHERE team_id=$1", team.ID).Scan(&count); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		if count > *in.Seats {
			fail(w, 409, "seats_in_use")
			return
		}
		team.SeatLimit = *in.Seats
	}
	if in.Owner != nil && *in.Owner != u.ID {
		var exists bool
		if e := tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM team_members m JOIN users u ON u.id=m.user_id WHERE team_id=$1 AND user_id=$2 AND u.enabled)", team.ID, *in.Owner).Scan(&exists); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		if !exists {
			fail(w, 400, "invalid_request")
			return
		}
		if _, e := tx.Exec(r.Context(), "UPDATE team_members SET role=CASE WHEN user_id=$1 THEN 'owner' ELSE 'member' END WHERE team_id=$2", *in.Owner, team.ID); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		team.Role = "member"
	}
	if _, e := tx.Exec(r.Context(), "UPDATE teams SET name=$1,seat_limit=$2 WHERE id=$3", team.Name, team.SeatLimit, team.ID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok := currentUserQuery(w, r, false, tx); !ok {
		return
	}
	if e := tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"item": team})
}
func (a *application) members(w http.ResponseWriter, r *http.Request) {
	_, team, ok := a.teamAccess(w, r)
	if !ok {
		return
	}
	a.jsonPage(w, r, "SELECT jsonb_build_object('id',m.id,'user_id',m.user_id,'username',u.username,'role',m.role),m.id FROM team_members m JOIN users u ON u.id=m.user_id WHERE team_id=$3 AND ($1::bigint=0 OR m.id<$1) ORDER BY m.id DESC LIMIT $2", team.ID)
}
func (a *application) removeMember(w http.ResponseWriter, r *http.Request) {
	u, team, tx, ok := a.teamWrite(w, r, false)
	if !ok {
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	target, e := strconv.ParseInt(r.PathValue("user_id"), 10, 64)
	if e != nil || target < 1 {
		fail(w, 400, "invalid_request")
		return
	}
	if team.Role != "owner" && target != u.ID {
		fail(w, 403, "forbidden")
		return
	}
	var role string
	e = tx.QueryRow(r.Context(), "SELECT role FROM team_members WHERE team_id=$1 AND user_id=$2", team.ID, target).Scan(&role)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if role == "owner" {
		fail(w, 409, "last_owner")
		return
	}
	if _, e = tx.Exec(r.Context(), "DELETE FROM team_members WHERE team_id=$1 AND user_id=$2", team.ID, target); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE wallet_id=$1 AND user_id=$2", team.WalletID, target); e != nil {
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
	w.WriteHeader(204)
}
func (a *application) createInvite(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, false); !ok {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	u, team, tx, ok := a.teamWrite(w, r, true)
	if !ok {
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	code, e := auth.Secret("ldi_")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var expires time.Time
	if e = tx.QueryRow(r.Context(), "INSERT INTO team_invites(team_id,code_hash,created_by,expires_at) VALUES($1,$2,$3,statement_timestamp()+interval '24 hours') RETURNING expires_at", team.ID, auth.Digest(code), u.ID).Scan(&expires); e != nil {
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
	respond(w, 201, map[string]any{"code": code, "expires_at": expires})
}
func (a *application) acceptInvite(w http.ResponseWriter, r *http.Request) {
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
	var teamID int64
	e = tx.QueryRow(r.Context(), "SELECT team_id FROM team_invites WHERE code_hash=$1", auth.Digest(in.Code)).Scan(&teamID)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var seats int
	if e = tx.QueryRow(r.Context(), "SELECT seat_limit FROM teams WHERE id=$1 FOR UPDATE", teamID).Scan(&seats); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var id int64
	var accepted *int64
	var valid bool
	e = tx.QueryRow(r.Context(), "SELECT id,accepted_by,expires_at>statement_timestamp() FROM team_invites WHERE code_hash=$1 FOR UPDATE", auth.Digest(in.Code)).Scan(&id, &accepted, &valid)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if accepted != nil {
		fail(w, 409, "invite_used")
		return
	}
	if !valid {
		fail(w, 410, "invite_expired")
		return
	}
	var exists bool
	var count int
	e = tx.QueryRow(r.Context(), "SELECT count(*),COALESCE(bool_or(user_id=$2),false) FROM team_members WHERE team_id=$1", teamID, u.ID).Scan(&count, &exists)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if exists {
		fail(w, 409, "already_member")
		return
	}
	if count >= seats {
		fail(w, 409, "team_full")
		return
	}
	if _, e = tx.Exec(r.Context(), "INSERT INTO team_members(team_id,user_id,role) VALUES($1,$2,'member')", teamID, u.ID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE team_invites SET accepted_by=$1 WHERE id=$2", u.ID, id); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	team, e := readTeam(tx.QueryRow(r.Context(), teamSelect+" WHERE t.id=$1 AND m.user_id=$2", teamID, u.ID))
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
	respond(w, 200, map[string]any{"item": team})
}
func (a *application) fundTeam(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, false); !ok {
		return
	}
	var in struct {
		Credits int64  `json:"credits"`
		Key     string `json:"idempotency_key"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, team, tx, ok := a.teamWrite(w, r, true)
	if !ok {
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if in.Credits < 1 || in.Credits > 1000000000000 || len(in.Key) < 1 || len(in.Key) > 100 {
		fail(w, 400, "invalid_request")
		return
	}
	var prior int64
	e := tx.QueryRow(r.Context(), "SELECT credits,balance_after FROM team_transfers WHERE team_id=$1 AND user_id=$2 AND idempotency_key=$3", team.ID, u.ID, in.Key).Scan(&prior, &team.Balance)
	if e == nil {
		if prior != in.Credits {
			fail(w, 409, "idempotency_conflict")
			return
		}
		respond(w, 200, map[string]any{"item": team})
		return
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		fail(w, 500, "internal_error")
		return
	}
	rows, e := tx.Query(r.Context(), "SELECT id,balance,user_id IS NOT NULL FROM wallets WHERE user_id=$1 OR team_id=$2 ORDER BY id FOR UPDATE", u.ID, team.ID)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var personalID, balance int64
	for rows.Next() {
		var id, amount int64
		var personal bool
		if e = rows.Scan(&id, &amount, &personal); e != nil {
			rows.Close()
			fail(w, 500, "internal_error")
			return
		}
		if personal {
			personalID = id
			balance = amount
		} else {
			team.Balance = amount
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok := currentUserQuery(w, r, false, tx); !ok {
		return
	}
	if balance < in.Credits {
		fail(w, 409, "insufficient_balance")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE wallets SET balance=balance+CASE WHEN id=$1 THEN -$2::bigint ELSE $2::bigint END WHERE id=$1 OR id=$3", personalID, in.Credits, team.WalletID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	team.Balance += in.Credits
	var transferID int64
	if e = tx.QueryRow(r.Context(), "INSERT INTO team_transfers(team_id,user_id,credits,idempotency_key,balance_after) VALUES($1,$2,$3,$4,$5) RETURNING id", team.ID, u.ID, in.Credits, in.Key, team.Balance).Scan(&transferID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	key := "transfer:" + strconv.FormatInt(transferID, 10)
	if _, e = tx.Exec(r.Context(), "INSERT INTO ledger(wallet_id,user_id,delta,kind,note,idempotency_key,actor_id,balance_after) VALUES($1,$2,-$3::bigint,'team_transfer','Fund team',$4,$2,$5),($6,$2,$3,'team_transfer','Team funding',$4,$2,$7)", personalID, u.ID, in.Credits, key, balance-in.Credits, team.WalletID, team.Balance); e != nil {
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
	respond(w, 200, map[string]any{"item": team})
}
func (a *application) teamUsage(w http.ResponseWriter, r *http.Request) {
	_, team, ok := a.teamAccess(w, r)
	if !ok {
		return
	}
	var userID int64
	if raw := r.URL.Query().Get("user_id"); raw != "" {
		var e error
		userID, e = strconv.ParseInt(raw, 10, 64)
		if e != nil || userID < 1 {
			fail(w, 400, "invalid_request")
			return
		}
	}
	a.usagePage(w, r, userID, team.WalletID)
}
