# ADR 0002：单机 SQLite / 集群 PG(+MySQL) 多方言部署（提议，已完成可行性 Spike）

日期：2026-09-11 · 状态：阶段 2 地基已落地（SQLite 迁移轨道全量转写并通过行为验证：建库等价、append-only 双触发器、目录修订触发器、钱包归属触发器、种子校验）；store/app 查询移植待续

## 背景

参照 new-api 的部署分层诉求：**单机小团队**期望单二进制零依赖（SQLite）即可跑通；**集群**继续走 PostgreSQL + Redis（已就绪：跨副本目录缓存、失效广播、多实例故障恢复均有验收）。当前 Loadout 仅支持 PostgreSQL。

## 调研与选型（2026-09-11 补充）

参照系 one-api/new-api 用 **GORM v2** 实现 SQLite/MySQL/PG 切换（单机默认 SQLite 文件、集群推荐 PG）。Go 生态候选：GORM/bun/ent/sqlc 均覆盖三库，哲学分 code-first（GORM/ent）与 SQL-first（bun/sqlc）。

**选定 bun**：SQL-first 最贴近现有 pgx 裸 SQL 风格（保留手写事务的精细控制），方言驱动 pg/mysqldialect/sqlitedialect；SQLite 驱动用 modernc.org/sqlite（纯 Go 无 CGO，交叉编译不受影响）；MySQL 预留 go-sql-driver + mysqldialect。迁移工具配 golang-migrate（原生 pg/mysql/sqlite 方言目录）。

## 可行性 Spike 结果（throwaway，/tmp/loadout-sqlite-spike）

用 bun + modernc SQLite 复刻账本核心（预留→结算→退款，append-only 触发器），64 并发压测：

| 场景 | PostgreSQL 基线 | SQLite（WAL + synchronous=NORMAL + _txlock=immediate） |
|---|---|---|
| 同钱包串行预留/结算 | 164 QPS | **5884 QPS** |
| 64 钱包并发 | 1416 QPS | **8620 QPS** |

正确性：12800 笔并发预留**零超扣**、终态余额精确、ok 流水逐笔一致；退款幂等（仅 pending 结算一次）。

**实证方言差异**（双轨迁移的已知坑）：`FOR UPDATE` 行锁 → `_txlock=immediate` 串行事务替代；触发器不支持 `UPDATE OR DELETE` 组合事件 → 拆两个；驱动不执行多语句 Exec → schema 逐条；`synchronous=NORMAL`（WAL 推荐档）不逐事务 fsync，掉电可能丢最近一笔扣费但不损账——单机小团队可接受（与 new-api 同级取舍），保守部署可回 FULL（实测 ~69 QPS，仍远超 60/分钟限流所需）。

## 现状盘点（为什么不能顺手做）

数据层直接使用 pgx，散布约 100+ 条 PostgreSQL 专属 SQL：`jsonb`、`timestamptz`、`FILTER (WHERE …)`、`ON CONFLICT … DO UPDATE … WHERE … RETURNING`、`FOR UPDATE OF / SKIP LOCKED`、`GENERATED ALWAYS AS IDENTITY`、账本 append-only 的 plpgsql 触发器、`statement_timeout`、`generate_series`、`extract(epoch …)`、schema 级隔离（测试靠临时 schema）。SQLite（即便配 WAL）在锁语义、RETURNING 支持、并发写、触发器方言上都有差异；账本与预留扣费是事务核心，**双方言意味着整套测试矩阵翻倍**，不是一次会话能安全完成的量。

## 决策路径（分四阶段，每阶段独立可验收）

1. **store 接口抽取**：把 `*store.Store` 的直接 SQL 收敛为接口 + 仓储实现（app/gateway 只依赖接口）；行为测试全部保持绿。
2. **bun 方言层**：仓储实现落到 bun（pg/sqlite/mysql 三方言），热路径沿用 Spike 验证的事务配方；jsonb→TEXT+json1、FILTER→CASE、`extract(epoch)`→`julianday` 逐条转写；SQLite 轨道已知差异已实证记录（ADD COLUMN 不支持 UNIQUE→独立索引、触发器事件拆分、多语句 Exec 拆分、时间统一 ISO8601 UTC 文本）。
3. **迁移双轨**：SQLite 轨道已就位（`server/internal/store/migrations_sqlite/`，17 文件与 PG 一一对应，`TestSQLite*` 验证）；接入 golang-migrate 与双库 CI 矩阵待做。
4. **部署 profile**：`LOADOUT_DATABASE_URL` 缺省落 `./loadout.db`（WAL/busy_timeout/synchronous 可配）；`SQL_DSN` 风格支持 MySQL；集群文档保持 PG+Redis 不变（Redis 侧已就绪）。

## 已知风险

- 同钱包预留事务在 SQLite 单写者下的吞吐：PG 基线 164 QPS（同钱包）/ 1416 QPS（64 钱包），SQLite 需重新基准（阶段 2 出口条件）。
- 多进程访问同一 SQLite 文件不支持（单机 profile 明确单副本；要横向扩展即切 PG+Redis，与 new-api 心智一致）。

## 替代方案

保持 PG-only + `make docker-up` 一键部署（现状已可用）；或打包嵌入式 PostgreSQL（体积与运维复杂度更高，否决）。

## 关联

- Redis 分布式能力：已实现并验收（多副本、失效广播、故障恢复），无需为 SQLite 方案重做。
- 压测基线见 `docs/superpowers/plans/2026-09-11-marketplace.md`（含 2026-09-11 晚修订）。
