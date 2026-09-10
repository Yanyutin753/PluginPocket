package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInsufficientBalance = errors.New("insufficient_balance")
var ErrConflict = errors.New("idempotency_conflict")
var ErrUnauthorized = errors.New("unauthorized")
var ErrNotFound = errors.New("not_found")

type Store struct{ Pool *pgxpool.Pool }
type Principal struct {
	UserID, TokenID, WalletID int64
	Username, Role            string
}
type Call struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	TokenID    int64     `json:"token_id"`
	WalletID   int64     `json:"wallet_id"`
	Tool       string    `json:"tool"`
	Cost       int64     `json:"cost"`
	Status     string    `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, rawURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(rawURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 20
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "15000"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	s := &Store{Pool: pool}
	if err = s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(817392104)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, f := range files {
		var applied bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", f.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		content, e := migrations.ReadFile("migrations/" + f.Name())
		if e != nil {
			return e
		}
		if _, err = tx.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("migration %s: %w", f.Name(), err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(name) VALUES($1)", f.Name()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) Close() { s.Pool.Close() }
func (s *Store) Reserve(ctx context.Context, userID, tokenID, walletID int64, tool string, cost int64, requestKey string) (Call, error) {
	return s.reserve(ctx, userID, tokenID, walletID, 0, tool, cost, requestKey)
}

// ReserveTool reads availability and price after obtaining the wallet lock.
func (s *Store) ReserveTool(ctx context.Context, userID, tokenID, walletID, toolID int64, tool, requestKey string) (Call, error) {
	if toolID <= 0 {
		return Call{}, ErrNotFound
	}
	return s.reserve(ctx, userID, tokenID, walletID, toolID, tool, 0, requestKey)
}

func (s *Store) reserve(ctx context.Context, userID, tokenID, walletID, toolID int64, tool string, cost int64, requestKey string) (Call, error) {
	if cost < 0 || requestKey == "" || len(requestKey) > 200 || tool == "" {
		return Call{}, errors.New("invalid_request")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Call{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var balance int64
	err = tx.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1 FOR UPDATE", walletID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, ErrUnauthorized
	}
	if err != nil {
		return Call{}, err
	}
	if toolID != 0 {
		err = tx.QueryRow(ctx, "SELECT cost FROM tools WHERE id=$1 AND enabled FOR SHARE", toolID).Scan(&cost)
		if errors.Is(err, pgx.ErrNoRows) {
			return Call{}, ErrNotFound
		}
		if err != nil {
			return Call{}, err
		}
	}
	checkAuthorization := func() error {
		var allowed bool
		err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tokens t JOIN users u ON u.id=t.user_id JOIN wallets w ON w.id=t.wallet_id WHERE t.id=$1 AND t.user_id=$2 AND t.wallet_id=$3 AND t.revoked_at IS NULL AND u.enabled AND (w.user_id=$2 OR EXISTS(SELECT 1 FROM team_members m WHERE m.team_id=w.team_id AND m.user_id=$2)))", tokenID, userID, walletID).Scan(&allowed)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrUnauthorized
		}
		return nil
	}
	if err = checkAuthorization(); err != nil {
		return Call{}, err
	}

	var c Call
	var requestedCost int64
	err = tx.QueryRow(ctx, "SELECT id,user_id,token_id,wallet_id,tool,cost,status,duration_ms,created_at,requested_cost FROM usage_logs WHERE token_id=$1 AND request_key=$2", tokenID, requestKey).Scan(&c.ID, &c.UserID, &c.TokenID, &c.WalletID, &c.Tool, &c.Cost, &c.Status, &c.DurationMS, &c.CreatedAt, &requestedCost)
	if err == nil {
		if c.WalletID != walletID || c.Tool != tool || requestedCost != cost {
			return Call{}, ErrConflict
		}
		if c.Status == "denied" {
			return c, ErrInsufficientBalance
		}
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Call{}, err
	}
	status := "pending"
	if balance < cost {
		status = "denied"
	}
	err = tx.QueryRow(ctx, "INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,requested_cost,status,request_key) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,user_id,token_id,wallet_id,tool,cost,status,duration_ms,created_at", userID, tokenID, walletID, tool, cost, status, requestKey).Scan(&c.ID, &c.UserID, &c.TokenID, &c.WalletID, &c.Tool, &c.Cost, &c.Status, &c.DurationMS, &c.CreatedAt)
	if err != nil {
		return Call{}, err
	}
	if status == "pending" {
		if _, err = tx.Exec(ctx, "UPDATE wallets SET balance=balance-$1 WHERE id=$2", cost, walletID); err != nil {
			return Call{}, err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO ledger(wallet_id,user_id,call_id,delta,kind,balance_after) VALUES($1,$2,$3,$4,'reservation',$5)", walletID, userID, c.ID, -cost, balance-cost); err != nil {
			return Call{}, err
		}
	}
	// Inserts can wait on foreign-key row locks after the earlier check.
	// Recheck after every write, so revocation during that wait rolls back
	// the reservation and ledger before the caller can execute the tool.
	if err = checkAuthorization(); err != nil {
		return Call{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Call{}, err
	}
	if status == "denied" {
		return c, ErrInsufficientBalance
	}
	return c, nil
}
func (s *Store) Finish(ctx context.Context, callID int64, success bool, duration time.Duration) error {
	status := "error"
	if success {
		status = "ok"
	}
	_, err := s.finish(ctx, callID, status, duration)
	return err
}
func (s *Store) finish(ctx context.Context, callID int64, status string, duration time.Duration) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var walletID, userID, cost int64
	var current string
	err = tx.QueryRow(ctx, "SELECT wallet_id,user_id,requested_cost,status FROM usage_logs WHERE id=$1 FOR UPDATE", callID).Scan(&walletID, &userID, &cost, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if current != "pending" {
		return false, nil
	}
	if status != "ok" {
		var balance int64
		if err = tx.QueryRow(ctx, "UPDATE wallets SET balance=balance+$1 WHERE id=$2 RETURNING balance", cost, walletID).Scan(&balance); err != nil {
			return false, err
		}
		kind := "refund"
		if status == "recovered" {
			kind = "recovery"
		}
		if _, err = tx.Exec(ctx, "INSERT INTO ledger(wallet_id,user_id,call_id,delta,kind,note,balance_after) VALUES($1,$2,$3,$4,$5,$6,$7)", walletID, userID, callID, cost, kind, status, balance); err != nil {
			return false, err
		}
	}
	if _, err = tx.Exec(ctx, "UPDATE usage_logs SET status=$1,duration_ms=$2,finished_at=now(),cost=CASE WHEN $1='ok' THEN requested_cost ELSE 0 END WHERE id=$3", status, max(0, duration.Milliseconds()), callID); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
func (s *Store) RecoverPending(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for {
		rows, err := s.Pool.Query(ctx, "SELECT id FROM usage_logs WHERE status='pending' AND created_at<$1 ORDER BY id LIMIT 100", before)
		if err != nil {
			return total, err
		}
		ids := []int64{}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return total, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		for _, id := range ids {
			changed, e := s.finish(ctx, id, "recovered", 0)
			if e != nil {
				return total, e
			}
			if changed {
				total++
			}
		}
	}
}
func (s *Store) AuthToken(ctx context.Context, raw string) (Principal, error) {
	if !strings.HasPrefix(raw, "ldt_") {
		return Principal{}, ErrUnauthorized
	}
	var p Principal
	err := s.Pool.QueryRow(ctx, "UPDATE tokens t SET last_used_at=now() FROM users u,wallets w WHERE t.token_hash=$1 AND t.revoked_at IS NULL AND u.id=t.user_id AND u.enabled AND w.id=t.wallet_id AND (w.user_id=u.id OR EXISTS(SELECT 1 FROM team_members m WHERE m.team_id=w.team_id AND m.user_id=u.id)) RETURNING u.id,t.id,t.wallet_id,u.username,u.role", auth.Digest(raw)).Scan(&p.UserID, &p.TokenID, &p.WalletID, &p.Username, &p.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrUnauthorized
	}
	return p, err
}
