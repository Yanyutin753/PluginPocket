# SQLite 账本核心实施计划（ADR 0002 阶段 2 首个增量）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为账本核心（Reserve / Finish / RecoverPending / AuthToken）提供与 PG `store.Store` 行为等价的 SQLite 实现，零新依赖，作为 ADR 0002 单机轨道阶段 2 的第一个切片。

**Architecture:** 在 `internal/store` 内并置 `SQLiteStore`（stdlib `database/sql` + 已有 `modernc.org/sqlite` 驱动），复用既有 `migrations_sqlite/` 迁移轨道；用 `Ledger` 接口声明双实现的共同契约（编译期断言），暂不改动 app/gateway 对 `*Store` 的引用（接线留给后续域移植）。PG 路径零改动。

**Tech Stack:** Go 标准库 `database/sql`、modernc.org/sqlite v1.58.0（已在 go.mod）、内嵌 `migrations_sqlite/*.sql`。

**Spec:** `docs/adr/0002-sqlite-single-node.md`（阶段路径第 1/2 步的先行切片；选型修订见 Task 1）。

## Global Constraints

- 事务配方沿用 Spike 结论：WAL + `busy_timeout` + `synchronous=NORMAL` + `_txlock=immediate`（ADR 0002 实证记录）。
- 时间戳为 ISO8601 UTC 文本 `YYYY-MM-DDTHH:MM:SS.sssZ`（`migrations_sqlite/003_billing.sql` 的 `strftime('%Y-%m-%dT%H:%M:%fZ','now')`），Go 侧扫描为字符串再解析，比较参数用同格式字符串。
- 不修改任何已发布迁移文件（双方言轨道均不可变）；只用 `?`/`?N` 位置参数。
- PG 路径（pgx）零改动；错误值复用包内既有哨兵（`ErrInsufficientBalance`/`ErrConflict`/`ErrUnauthorized`/`ErrNotFound`）。
- 测试不依赖 PG/Redis（纯 Go，`-run TestSQLite` 可独立运行）；最终回归含真实 PG 全量（`make test-server`）与 `make check`。

---

### Task 1: ADR 选型修订（先改文档）

**Files:**
- Modify: `docs/adr/0002-sqlite-single-node.md`

- [ ] **Step 1: 状态行追加阶段 2 进度**

将状态行末尾追加：`；2026-09-12 账本核心 SQLite 实现落地（stdlib database/sql，行为等价测试 TestSQLite*）`

- [ ] **Step 2: 选型段加修订注记**

在"**选定 bun**"段落末尾追加：`2026-09-12 修订：账本存储层以 stdlib database/sql + modernc.org/sqlite 落地（零新依赖，符合本仓"标准库优先"边界）；bun 推迟到出现查询构造需求的大规模移植阶段再评估。Spike 事务配方与结论不变。`

- [ ] **Step 3: 无需运行测试（纯文档），git diff --check 通过**

---

### Task 2: Ledger 接口缝 + OpenSQLite 桩 + RED 测试

**Files:**
- Create: `server/internal/store/store_sqlite.go`（桩）
- Create: `server/internal/store/store_sqlite_test.go`

**Interfaces:**
- Produces: `type Ledger interface { Reserve(...) (Call, error); ReserveTool(...) (Call, error); Finish(...) error; FinishWithData(...) error; RecoverPending(ctx, before time.Time) (int64, error); AuthToken(ctx, raw string) (Principal, error) }`；`func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error)`；`func (s *SQLiteStore) Close() error`。

- [ ] **Step 1: 写接口与桩**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

// Ledger is the money-path contract shared by the PostgreSQL and SQLite
// implementations. app/gateway wiring moves to this interface as further
// domains migrate (ADR 0002 phase 2.1).
type Ledger interface {
	Reserve(ctx context.Context, userID, tokenID, walletID int64, tool string, cost int64, requestKey string) (Call, error)
	ReserveTool(ctx context.Context, userID, tokenID, walletID, toolID int64, tool, requestKey string) (Call, error)
	Finish(ctx context.Context, callID int64, success bool, duration time.Duration) error
	FinishWithData(ctx context.Context, callID int64, success bool, duration time.Duration, data *CallData) error
	RecoverPending(ctx context.Context, before time.Time) (int64, error)
	AuthToken(ctx context.Context, raw string) (Principal, error)
}

var _ Ledger = (*Store)(nil)

type SQLiteStore struct{ DB *sql.DB }

func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
	return nil, errors.New("sqlite store not implemented")
}
func (s *SQLiteStore) Close() error { return s.DB.Close() }
```

- [ ] **Step 2: 写行为测试（PG `store_test.go` 的等价面 + AuthToken 补测）**

测试文件骨架（完整断言见 Step 3 运行清单）：

```go
package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/auth"
)

func sqliteTestStore(t testing.TB) *SQLiteStore {
	t.Helper()
	s, err := OpenSQLite(context.Background(), t.TempDir()+"/ledger.db")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func sqliteFixture(t testing.TB, s *SQLiteStore) (int64, int64, int64) {
	t.Helper()
	ctx := context.Background()
	var u, w, tok int64
	must := func(q string, args ...any) *sql.Row { ... } // 直接用 s.DB.QueryRow
	// INSERT INTO users(username,password_hash) VALUES('tester','hash') RETURNING id
	// INSERT INTO wallets(user_id,balance) VALUES(?,10) RETURNING id
	// INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) VALUES(?,?,'test','ldt_x','hash') RETURNING id
	return u, w, tok
}
```

测试清单（逐条对应 `store_test.go` 的 PG 语义）：

1. `TestSQLiteMigrationsIdempotentReopen`：打开→fixture→关闭→重开同一路径：不重复迁移、余额持久。
2. `TestSQLiteConcurrentReservationsNeverOverdrawAndRefundOnce`：40 并发 Reserve（余额 10、单价 1）恰好 10 成功、余额 0；同一 call Finish 两次仅退款一次（余额 1）；ledger 汇总 −9。
3. `TestSQLiteReservationReplayRecoveryAndOwnership`：同 key 同参幂等返回同 call；不同 cost → `ErrConflict`；他人钱包 → `ErrUnauthorized`；`RecoverPending(now+1m)=1` 再 `=0`；恢复后余额 10。
4. `TestSQLiteOnlySuccessfulCallsReportChargedCost`：pending cost=0；超额 denied 返回 `ErrInsufficientBalance` 且 cost=0；成功调用 finish 后 cost=请求值；失败调用 cost=0。
5. `TestSQLiteReservationAndRefundLedgerCapturePostBalance`：预留后 `balance_after=7`、退款后 `=10`。
6. `TestSQLiteTeamWalletMembershipAuthorizesReserve`：钱包归属团队、用户为成员 → Reserve 成功（覆盖授权子查询的 team 路径；PG 的锁等待重查测试是 PG 行锁专属，SQLite 单写者无此交错窗口，语义由两次 checkAuthorization 保留）。
7. `TestSQLiteReserveToolUsesToolPricing`：启用工具 cost=5 → ReserveTool 按工具价；禁用/不存在工具 → `ErrNotFound`；toolID≤0 → `ErrNotFound`。
8. `TestSQLiteAuthTokenPrincipalAndRevocation`：`auth.Digest("ldt_x")` 插入 token；AuthToken 返回正确 Principal 并更新 last_used_at；吊销后 → `ErrUnauthorized`；禁用用户 → `ErrUnauthorized`；非 `ldt_` 前缀 → `ErrUnauthorized`。
9. `BenchmarkSQLiteReserveFinish`（serial）：与 PG `BenchmarkReserveFinish` 同形态，仅观测不做断言。
10. `var _ Ledger = (*SQLiteStore)(nil)` 编译断言放在实现文件中。

- [ ] **Step 3: 运行确认 RED**

Run: `cd server && go test ./internal/store -run 'TestSQLite' -count=1`
Expected: 全部 FAIL，失败信息为 `sqlite store not implemented`（来自桩的行为缺失，非编译/导入错误）。

---

### Task 3: 迁移 runner（重开测试 GREEN）

**Files:**
- Modify: `server/internal/store/store_sqlite.go`

- [ ] **Step 1: 实现 OpenSQLite（DSN、PRAGMA、迁移）**

```go
func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil { return nil, err }
	s := &SQLiteStore{DB: db}
	if err = s.migrate(ctx); err != nil { _ = db.Close(); return nil, err }
	return s, nil
}
func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(name TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')))`); err != nil { return err }
	files, err := migrationsSQLite.ReadDir("migrations_sqlite")
	if err != nil { return err }
	for _, f := range files {
		var applied bool
		if err = s.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", f.Name()).Scan(&applied); err != nil { return err }
		if applied { continue }
		content, e := migrationsSQLite.ReadFile("migrations_sqlite/" + f.Name())
		if e != nil { return e }
		if _, err = s.DB.ExecContext(ctx, string(content)); err != nil { return fmt.Errorf("migration %s: %w", f.Name(), err) }
		if _, err = s.DB.ExecContext(ctx, "INSERT INTO schema_migrations(name) VALUES($1)", f.Name()); err != nil { return err }
	}
	return nil
}
```

注意：modernc DSN 的 `$1` 具名参数在该驱动下可用性以实测为准，不可用则改 `?`（迁移记录 INSERT 单参数无歧义）。无需 PG advisory lock：`_txlock=immediate` + busy_timeout 串行化写者。

- [ ] **Step 2: 聚焦 GREEN**

Run: `cd server && go test ./internal/store -run 'TestSQLiteMigrationsIdempotentReopen' -count=1`
Expected: PASS。其余测试仍 RED（`not implemented` 之外的断言失败——桩已具备打开能力）。

---

### Task 4: Reserve 路径（并发/幂等/授权 GREEN）

**Files:**
- Modify: `server/internal/store/store_sqlite.go`

- [ ] **Step 1: 实现 reserve（与 `store.go:120-205` 逐语句对应）**

方言转写要点（完整 SQL 在实现中与 PG 版一一对照）：
- `SELECT balance FROM wallets WHERE id=$1 FOR UPDATE` → `SELECT balance FROM wallets WHERE id=?1`（写者已被 `_txlock=immediate` 串行化，无行锁必要）。
- `tools ... FOR SHARE` → `SELECT cost FROM tools WHERE id=?1 AND enabled`（SQLite 整数真值）。
- `checkAuthorization` 子查询原样（`?N` 参数）。
- 幂等查询/`INSERT ... RETURNING` 原样；`created_at` 扫描为字符串后 `parseSqliteTimestamp`。
- 拒绝/不足/冲突的哨兵错误与 PG 版逐一相同；`pgx.ErrNoRows` → `sql.ErrNoRows`。
- 两次 `checkAuthorization`（写后重查）保留，与 PG 语义一致。

辅助函数：

```go
const sqliteTimeFormat = "2006-01-02T15:04:05.000Z"
func sqliteTimestamp(t time.Time) string { return t.UTC().Format(sqliteTimeFormat) }
func parseSqliteTimestamp(raw string) (time.Time, error) { return time.Parse(time.RFC3339, raw) }
```

- [ ] **Step 2: 聚焦 GREEN（并发 + 幂等 + 授权 + 工具定价）**

Run: `cd server && go test ./internal/store -run 'TestSQLiteConcurrent|TestSQLiteReservation|TestSQLiteTeamWallet|TestSQLiteReserveTool' -count=1`
Expected: PASS。`Finish` 类测试仍因 finish 未实现而 RED（Reserve 已可用，pending 留存）。

---

### Task 5: Finish / Refund / RecoverPending（GREEN）

**Files:**
- Modify: `server/internal/store/store_sqlite.go`

- [ ] **Step 1: 实现 finish 与 RecoverPending（对应 `store.go:206-298`）**

转写要点：
- `FOR UPDATE` 去除（同上）；`UPDATE wallets SET balance=balance+?1 WHERE id=?2 RETURNING balance` SQLite 原生支持。
- `finished_at=now()` → `finished_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')`。
- `cost=CASE WHEN ?1='ok' THEN requested_cost ELSE 0 END` 原样。
- `RecoverPending` 的 `created_at<$1` 传 `sqliteTimestamp(before)`（同格式文本按字典序即时间序）。
- 数据列（input_data 等）与截断标志更新原样；bool 参数直接绑定。

- [ ] **Step 2: 聚焦 GREEN**

Run: `cd server && go test ./internal/store -run 'TestSQLiteOnlySuccessful|TestSQLiteReservationAndRefund|TestSQLiteReservationReplay' -count=1`
Expected: PASS。

---

### Task 6: AuthToken（GREEN）

**Files:**
- Modify: `server/internal/store/store_sqlite.go`

- [ ] **Step 1: 实现 AuthToken（SELECT + UPDATE 两步，避免 SQLite RETURNING 跨表限制）**

```go
func (s *SQLiteStore) AuthToken(ctx context.Context, raw string) (Principal, error) {
	if !strings.HasPrefix(raw, "ldt_") { return Principal{}, ErrUnauthorized }
	var p Principal
	err := s.DB.QueryRowContext(ctx, `SELECT u.id,t.id,t.wallet_id,u.username,u.role
 FROM tokens t JOIN users u ON u.id=t.user_id JOIN wallets w ON w.id=t.wallet_id
 WHERE t.token_hash=?1 AND t.revoked_at IS NULL AND u.enabled AND
 (w.user_id=u.id OR EXISTS(SELECT 1 FROM team_members m WHERE m.team_id=w.team_id AND m.user_id=u.id))`,
		auth.Digest(raw)).Scan(&p.UserID, &p.TokenID, &p.WalletID, &p.Username, &p.Role)
	if errors.Is(err, sql.ErrNoRows) { return p, ErrUnauthorized }
	if err != nil { return p, err }
	if _, err = s.DB.ExecContext(ctx, `UPDATE tokens SET last_used_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?1`, p.TokenID); err != nil { return p, err }
	return p, nil
}
```

PG 用单条 `UPDATE...FROM...RETURNING`；SQLite 在 `_txlock=immediate` 写事务串行下以 SELECT+UPDATE 等价实现（注释说明差异原因）。

- [ ] **Step 2: 聚焦 GREEN**

Run: `cd server && go test ./internal/store -run 'TestSQLiteAuthToken' -count=1`
Expected: PASS。

---

### Task 7: 回归、文档同步与提交

- [ ] **Step 1: SQLite 全量聚焦**

Run: `cd server && go test ./internal/store -run 'TestSQLite' -count=1 -v`
Expected: 全部 PASS（benchmark 不默认运行）。

- [ ] **Step 2: store 包全量（真实 PG）**

Run: 加载 `.env` 后 `cd server && go test -race ./internal/store -count=1`
Expected: PASS（PG 行为零回归 + SQLite 新增）。

- [ ] **Step 3: 静态检查**

Run: `make lint`
Expected: golangci-lint 0 issues（新增文件已格式化）。

- [ ] **Step 4: 全量 `make check`（后台，日志 `.loadout/sqlite-ledger-check.log`）**

- [ ] **Step 5: 文档同步**：PLAN.md 顶部变更记录一行；本计划追加执行记录（RED/GREEN 命令与结果、未验证范围）；ADR 状态行如 Task 1。

- [ ] **Step 6: 提交（不 push）**：`feat(store): SQLite ledger core with behavioral parity (ADR 0002 phase 2 slice)`

## 未验证范围（预定）

- `AllowRequest`（rate.go）、app/gateway 其余 100+ 条 SQL 未移植——后续域增量。
- 运行时 DSN 分流（`sqlite://` 启动单机模式）未接线：等查询覆盖完成后作为部署 profile 增量。
- SQLite 侧同钱包吞吐正式基准（ADR 出口条件）记录于 benchmark 观测，不作为本增量验收。
- 多进程访问同一 SQLite 文件不支持（ADR 已声明，单机单副本 profile）。

## 执行记录（2026-09-12，inline 会话）

- Task 1 ADR 修订先行（选型注记 + 状态行）。
- **RED**：`go test ./internal/store -run TestSQLite -count=1` —— 9 项新测试全部失败于桩错误 `sqlite store not implemented`（行为缺失，非编译/导入问题）。
- **GREEN 分步**：迁移 runner → `TestSQLiteMigrationsIdempotentReopen` PASS；reserve → `TestSQLiteTeamWalletMembershipAuthorizesReserve` PASS（其余失败点均落在 Finish 调用行）；finish/recover → Replay/OnlySuccessful/BalanceAfter/Concurrent PASS；AuthToken → PASS。
- 最终聚焦：`go test ./internal/store -run TestSQLite -count=1` **18/18 PASS**（含 9 项既有迁移轨道测试无回归）。
- 全包回归：加载 `.env` 后 `go test -race ./internal/store -count=1` ok（25.1s，真实 PG + SQLite）。
- 基准观测：`BenchmarkSQLiteReserveFinish`（serial）655,585 ns/op ≈ **1526 QPS**（ADR 记录 PG 同钱包基线 164 QPS）；正式出口基准待部署 profile 时复测。
- 缺陷修复：首轮 `make check` 在 lint 停止——errcheck 报 `store_sqlite.go` 两处 `rows.Close()` 未检查（pgx 的 Close 无返回值、database/sql 有，方言差异）；按仓库惯例改显式 `_ = rows.Close()` 后重跑。
- 全量：`make check`（加载 `.env`）退出 0，日志 `.loadout/sqlite-ledger-check-2.log`。
- 文档同步：PLAN.md 顶部变更记录、AGENTS.md 模块地图存储行、ADR 0002 状态行与选型注记。
