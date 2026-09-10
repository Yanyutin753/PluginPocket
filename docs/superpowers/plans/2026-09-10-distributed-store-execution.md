# Store / account 多副本执行记录

日期：2026-09-10。范围为 `server/internal/store`、`server/internal/app`；gateway、identity、进程/LB harness 由主代理与其他代理负责。按项目 Ponytail、TDD 与完成前验证执行；在共享工作区修改，无提交或推送。

## 审计与最小实现

- 匿名 auth/device 的 120/min 预算原来保存在每个 application 的 mutex/window/counter 中。改用共享 `Store.AllowRequest(ctx, scope, subject, windowSeconds, limit)`，复用 `002_gateway.sql` 的 rate_limits。单条 UPSERT 在数据库 statement_timestamp 上计窗，窗口只能前进；到限返回 false，数据库错误单独返回 error。app 使用 auth/0/60/120，失败关闭并返回 503。匿名 scope/subject 为固定服务端值，不创建任意 IP 键；密码哈希并发 gate 仍是各进程的内存资源保护。
- ledger 原唯一范围 `(wallet_id,idempotency_key)` 使管理员自选键能占据兑换/转账的内部名称。新增 `013_distributed_store.sql`，改为 `(wallet_id,kind,idempotency_key)`；调账重放只查询 adjustment。没有改动历史迁移。
- session、team invite、device 的到期时间原来部分由 Go 节点时钟生成或判定。现在持久化期限由数据库 statement_timestamp 生成；session/invite 返回的 expires_at 直接取 INSERT RETURNING。设备消费与邀请接受在数据库查询中判定有效期，设备轮询也用数据库时间判断与推进。
- 首次迁移已经在单事务中持有 PostgreSQL advisory lock；管理员 bootstrap 也持有数据库 advisory lock，既有用户不被提权或改密码。无需再添加进程锁。
- 预占先锁钱包，再新语句检查当前令牌/团队成员；结算先锁调用并检查 pending，退款与状态同事务。多个恢复进程可以选择同一批 pending，行锁与状态检查保证最多一次退款。数据库中 ledger 保持审计证据。
- 兑换码、设备消费、邀请接受、团队权限与转账均已有数据库事务/行锁；session 查验与注销也已经直接访问共享数据库。本次将相应验证扩展到独立连接池和应用实例。

## 实际 RED / GREEN

以下命令均在 `server/` 执行，测试数据库为任务专用 PostgreSQL 18.6：

```sh
export LOADOUT_TEST_DATABASE_URL='postgres://loadout@127.0.0.1:44035/loadout_test?sslmode=disable'
go test ./internal/store ./internal/app -run 'TestDistributedRateBudget|TestRateWindowOnlyAdvances|TestDistributedAuthenticationBudget' -count=1 -v
```

RED：AllowRequest 的可编译空壳使 80 请求全部通过，期望共享预算 20；未来满窗口被允许重开；两个 app 合计第 121 请求返回 400，期望 429。首次写文件误用了工作目录，产生路径错误和 no tests to run，属于准备失误，未作为 RED。

实现共享预算后，同一命令 GREEN；并额外保留既有 `TestAuthenticationEndpointsRejectExcessiveAttempts`，改为使用真实数据库 fixture。重建第三个 Store 仍不能重置预算，不同 scope 保持隔离，已有未来窗口不会被旧请求倒退。

```sh
go test ./internal/app -run 'TestLedgerIdempotencyIsScoped|TestDatabaseClock' -count=1 -v
```

RED：管理员调账使用 `redemption:1` 后，另一副本兑换同编号代码返回 500；Go 标准库 testing/synctest 的假时钟设在 2000 年、PostgreSQL 保持真实时钟时，已过期 device 返回 slow_down 429，已过期 invite 接受返回 200，新注册 session 立即返回 unauthorized 401。测试没有模拟数据库或给生产代码增加时钟注入。实现上述幂等范围及数据库时间修复后，同一命令全部 GREEN。

## 已有数据库协议的多实例回归

```sh
go test ./internal/store ./internal/app -run 'TestDistributed|TestConcurrentRedemption|TestTeamWalletMembership' -count=1 -v
```

首跑通过，作为已有协议的回归验证，不冒称这些路径原来失败：

- 三个独立 Store 对同一全新 schema 并发 Open，所有迁移完成且仅两条内置工具种子。
- 两个独立 Store 同时 bootstrap 同一操作员，只产生一个用户和一个钱包；一个实例登录得到的 cookie 在另一实例可用，注销后原实例立即拒绝。
- 两个 Store 共 40 次并发预占余额 10，恰好允许 10 次；两组恢复与 10 次失败结算竞争后，余额 10、净 ledger delta 0、退款/恢复账目共 10 条、pending 0；重建实例再恢复数量为 0。
- 两个 app 并发兑换同一代码只有一次成功，余额增加一次。
- 设备 authorize/poll/approve 分布于不同 app，轮询窗口不能因换副本绕开；两个 app 同时消费批准的设备授权只生成一个 token。
- 团队邀请在其他 app 接受，令牌在独立 Store 预占，成员在另一 app 移除后，原 Store 的 token 验证与新预占立即拒绝。

## 最终包内验证

```sh
go test -race ./internal/store ./internal/app -count=1
go vet ./internal/store ./internal/app
```

结果：race 通过，store 2.600s、app 28.281s；vet 通过。测试每例独立 schema，第二/第三实例连接同一例 schema 并使用独立 pgxpool，不需要 sticky session。

本记录只证明这些包的真实数据库多连接池/多应用行为。实际独立 Go 进程与负载均衡、完整 make check 由主代理统一执行记录；没有宣称公网性能或外部上游 exactly-once。Reserve 重放可以返回已有 pending 调用，调用者仍须控制是否重复执行外部副作用；账本预占和结算本身保持幂等。

## 补充：报表当前日/月也采用数据库时钟

主代理补充要求：全局/团队今日与 7 日统计、account/me 的今日/月统计在跨副本时也使用统一时钟。本次把三个入口的当前时刻改为一次 `SELECT statement_timestamp()`，在 Go 中按该返回值计算 UTC 边界，再绑定已有聚合 SQL 的具体时间参数。没有在 created_at 列上套函数，也没有修改报表索引或统计对象。代价是每份报表增加一次很小的数据库时钟查询。

```sh
# cwd: server，LOADOUT_TEST_DATABASE_URL 同上
go test ./internal/app -run TestDistributedReportsUseDatabaseUTCDayAndMonth -count=1 -v
```

RED：真实 PostgreSQL 中写入当前 UTC 当日两条记录（共 5 credits）和三个月前一条 99 credits 记录；正常实例正确返回 2 次 / 5 credits，testing/synctest 的偏慢实例错误返回 3 次 / 104 credits。account/me、global/team 的 1 日和 7 日窗口均复现。

```sh
go test ./internal/app -run 'TestDistributedReportsUseDatabaseUTCDayAndMonth|TestReportsReadOnlyTheRequestedTimeWindow' -count=1 -v
```

GREEN：两个实例统计一致，5 个时钟偏差断言均通过。原真实 EXPLAIN ANALYZE BUFFERS 回归保持：global、account、team 各扫描 2 行，shared hit 分别 3 / 3 / 5，shared read 全部为 0。这是原本地样本下的扫描范围证据，不是公网延迟结果。

补充最终验证：`go test -race ./internal/app -count=1` 通过（34.193s）；`go vet ./internal/app ./internal/store` 通过。app/store 生产文件已无 `time.Now` 权威时间读取；测试数据与 benchmark 的人工测量时钟保留。
