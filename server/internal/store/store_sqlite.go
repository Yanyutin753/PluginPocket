package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	_ "modernc.org/sqlite"
)

// Ledger 是钱路径契约，由 PostgreSQL（*Store）与 SQLite（*SQLiteStore）共同满足。
// app/gateway 的接线在后续域移植时切换到该接口（ADR 0002 阶段 2.1）。
type Ledger interface {
	Reserve(ctx context.Context, userID, tokenID, walletID int64, tool string, cost int64, requestKey string) (Call, error)
	ReserveTool(ctx context.Context, userID, tokenID, walletID, toolID int64, tool, requestKey string) (Call, error)
	Finish(ctx context.Context, callID int64, success bool, duration time.Duration) error
	FinishWithData(ctx context.Context, callID int64, success bool, duration time.Duration, data *CallData) error
	RecoverPending(ctx context.Context, before time.Time) (int64, error)
	AuthToken(ctx context.Context, raw string) (Principal, error)
}

var _ Ledger = (*Store)(nil)
var _ Ledger = (*SQLiteStore)(nil)

// SQLiteStore 以 modernc.org/sqlite（纯 Go）承载单机部署的账本核心；
// 事务配方沿用 ADR 0002 Spike 结论：WAL + busy_timeout + synchronous=NORMAL + _txlock=immediate。
type SQLiteStore struct{ DB *sql.DB }

// OpenSQLite 打开（或创建）单机 SQLite 数据库并执行迁移。
// 事务配方沿用 ADR 0002 Spike：WAL + busy_timeout + synchronous=NORMAL + _txlock=immediate；
// 无需 PG advisory lock——BEGIN IMMEDIATE 使写者串行，并发打开者由 busy_timeout 等待。
func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &SQLiteStore{DB: db}
	if err = s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(name TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')))"); err != nil {
		return err
	}
	files, err := migrationsSQLite.ReadDir("migrations_sqlite")
	if err != nil {
		return err
	}
	for _, f := range files {
		var applied bool
		if err = s.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=?)", f.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		content, e := migrationsSQLite.ReadFile("migrations_sqlite/" + f.Name())
		if e != nil {
			return e
		}
		if _, err = s.DB.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("migration %s: %w", f.Name(), err)
		}
		if _, err = s.DB.ExecContext(ctx, "INSERT INTO schema_migrations(name) VALUES(?)", f.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) Close() error { return s.DB.Close() }

// 时间戳为 ISO8601 UTC 文本（migrations_sqlite 的 strftime 格式），比较参数须同格式。
const sqliteTimeFormat = "2006-01-02T15:04:05.000Z"

func sqliteTimestamp(t time.Time) string { return t.UTC().Format(sqliteTimeFormat) }

func parseSqliteTimestamp(raw string) (time.Time, error) { return time.Parse(time.RFC3339, raw) }

func (s *SQLiteStore) Reserve(ctx context.Context, userID, tokenID, walletID int64, tool string, cost int64, requestKey string) (Call, error) {
	return s.reserve(ctx, userID, tokenID, walletID, 0, tool, cost, requestKey)
}

func (s *SQLiteStore) ReserveTool(ctx context.Context, userID, tokenID, walletID, toolID int64, tool, requestKey string) (Call, error) {
	if toolID <= 0 {
		return Call{}, ErrNotFound
	}
	return s.reserve(ctx, userID, tokenID, walletID, toolID, tool, 0, requestKey)
}

func (s *SQLiteStore) reserve(ctx context.Context, userID, tokenID, walletID int64, toolID int64, tool string, cost int64, requestKey string) (Call, error) {
	if cost < 0 || requestKey == "" || len(requestKey) > 200 || tool == "" {
		return Call{}, errors.New("invalid_request")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Call{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// _txlock=immediate 已串行化写者：无需 FOR UPDATE 行锁，SELECT 即为事务内一致读。
	var balance int64
	err = tx.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE id=?1", walletID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return Call{}, ErrUnauthorized
	}
	if err != nil {
		return Call{}, err
	}
	if toolID != 0 {
		err = tx.QueryRowContext(ctx, "SELECT cost FROM tools WHERE id=?1 AND enabled", toolID).Scan(&cost)
		if errors.Is(err, sql.ErrNoRows) {
			return Call{}, ErrNotFound
		}
		if err != nil {
			return Call{}, err
		}
	}
	checkAuthorization := func() error {
		var allowed bool
		err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM tokens t JOIN users u ON u.id=t.user_id JOIN wallets w ON w.id=t.wallet_id WHERE t.id=?1 AND t.user_id=?2 AND t.wallet_id=?3 AND t.revoked_at IS NULL AND u.enabled AND (w.user_id=?2 OR EXISTS(SELECT 1 FROM team_members m WHERE m.team_id=w.team_id AND m.user_id=?2)))", tokenID, userID, walletID).Scan(&allowed)
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
	var createdAt string
	err = tx.QueryRowContext(ctx, "SELECT id,user_id,token_id,wallet_id,tool,cost,status,duration_ms,created_at,requested_cost FROM usage_logs WHERE token_id=?1 AND request_key=?2", tokenID, requestKey).Scan(&c.ID, &c.UserID, &c.TokenID, &c.WalletID, &c.Tool, &c.Cost, &c.Status, &c.DurationMS, &createdAt, &requestedCost)
	if err == nil {
		if c.WalletID != walletID || c.Tool != tool || requestedCost != cost {
			return Call{}, ErrConflict
		}
		if c.Status == "denied" {
			return c, ErrInsufficientBalance
		}
		if c.CreatedAt, err = parseSqliteTimestamp(createdAt); err != nil {
			return Call{}, err
		}
		return c, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Call{}, err
	}
	status := "pending"
	if balance < cost {
		status = "denied"
	}
	err = tx.QueryRowContext(ctx, "INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,requested_cost,status,request_key) VALUES(?1,?2,?3,?4,?5,?6,?7) RETURNING id,user_id,token_id,wallet_id,tool,cost,status,duration_ms,created_at", userID, tokenID, walletID, tool, cost, status, requestKey).Scan(&c.ID, &c.UserID, &c.TokenID, &c.WalletID, &c.Tool, &c.Cost, &c.Status, &c.DurationMS, &createdAt)
	if err != nil {
		return Call{}, err
	}
	if c.CreatedAt, err = parseSqliteTimestamp(createdAt); err != nil {
		return Call{}, err
	}
	if status == "pending" {
		if _, err = tx.ExecContext(ctx, "UPDATE wallets SET balance=balance-?1 WHERE id=?2", cost, walletID); err != nil {
			return Call{}, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO ledger(wallet_id,user_id,call_id,delta,kind,balance_after) VALUES(?1,?2,?3,?4,'reservation',?5)", walletID, userID, c.ID, -cost, balance-cost); err != nil {
			return Call{}, err
		}
	}
	// 与 PG 版一致：写后重查授权，撤销窗口内完成写入即回滚（SQLite 单写者下窗口更小，语义保留）。
	if err = checkAuthorization(); err != nil {
		return Call{}, err
	}
	if err = tx.Commit(); err != nil {
		return Call{}, err
	}
	if status == "denied" {
		return c, ErrInsufficientBalance
	}
	return c, nil
}

func (s *SQLiteStore) Finish(ctx context.Context, callID int64, success bool, duration time.Duration) error {
	return s.FinishWithData(ctx, callID, success, duration, nil)
}

func (s *SQLiteStore) FinishWithData(ctx context.Context, callID int64, success bool, duration time.Duration, data *CallData) error {
	status := "error"
	if success {
		status = "ok"
	}
	_, err := s.finish(ctx, callID, status, duration, data)
	return err
}

func (s *SQLiteStore) finish(ctx context.Context, callID int64, status string, duration time.Duration, data *CallData) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var walletID, userID, cost int64
	var current string
	err = tx.QueryRowContext(ctx, "SELECT wallet_id,user_id,requested_cost,status FROM usage_logs WHERE id=?1", callID).Scan(&walletID, &userID, &cost, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if data != nil && (current == "pending" || current == "denied") {
		if _, err = tx.ExecContext(ctx, "UPDATE usage_logs SET input_data=?2,output_data=?3,input_truncated=?4,output_truncated=?5 WHERE id=?1", callID, data.InputData, data.OutputData, data.InputTruncated, data.OutputTruncated); err != nil {
			return false, err
		}
	}
	if current == "denied" && data != nil {
		return false, tx.Commit()
	}
	if current != "pending" {
		return false, nil
	}
	if status != "ok" {
		var balance int64
		if err = tx.QueryRowContext(ctx, "UPDATE wallets SET balance=balance+?1 WHERE id=?2 RETURNING balance", cost, walletID).Scan(&balance); err != nil {
			return false, err
		}
		kind := "refund"
		if status == "recovered" {
			kind = "recovery"
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO ledger(wallet_id,user_id,call_id,delta,kind,note,balance_after) VALUES(?1,?2,?3,?4,?5,?6,?7)", walletID, userID, callID, cost, kind, status, balance); err != nil {
			return false, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE usage_logs SET status=?1,duration_ms=?2,finished_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),cost=CASE WHEN ?1='ok' THEN requested_cost ELSE 0 END WHERE id=?3", status, max(0, duration.Milliseconds()), callID); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLiteStore) RecoverPending(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for {
		rows, err := s.DB.QueryContext(ctx, "SELECT id FROM usage_logs WHERE status='pending' AND created_at<?1 ORDER BY id LIMIT 100", sqliteTimestamp(before))
		if err != nil {
			return total, err
		}
		ids := []int64{}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				_ = rows.Close()
				return total, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		for _, id := range ids {
			changed, e := s.finish(ctx, id, "recovered", 0, nil)
			if e != nil {
				return total, e
			}
			if changed {
				total++
			}
		}
	}
}

// PG 版为单条 UPDATE...FROM...RETURNING；SQLite RETURNING 不支持引用目标表以外的列，
// 在 _txlock=immediate 写事务串行下以 SELECT+UPDATE 等价实现（写者间无交错）。
func (s *SQLiteStore) AuthToken(ctx context.Context, raw string) (Principal, error) {
	if !strings.HasPrefix(raw, "ppt_") {
		return Principal{}, ErrUnauthorized
	}
	var p Principal
	err := s.DB.QueryRowContext(ctx, `SELECT u.id,t.id,t.wallet_id,u.username,u.role
 FROM tokens t JOIN users u ON u.id=t.user_id JOIN wallets w ON w.id=t.wallet_id
 WHERE t.token_hash=?1 AND t.revoked_at IS NULL AND u.enabled AND
 (w.user_id=u.id OR EXISTS(SELECT 1 FROM team_members m WHERE m.team_id=w.team_id AND m.user_id=u.id))`,
		auth.Digest(raw)).Scan(&p.UserID, &p.TokenID, &p.WalletID, &p.Username, &p.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrUnauthorized
	}
	if err != nil {
		return p, err
	}
	if _, err = s.DB.ExecContext(ctx, "UPDATE tokens SET last_used_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?1", p.TokenID); err != nil {
		return p, err
	}
	return p, nil
}
