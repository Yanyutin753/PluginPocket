# Task 1：账号与事务账本执行记录

2026-09-10。当前共享工作区实现，未提交、未推送。遵循项目 TDD 与 Ponytail，SQL 为 PostgreSQL 唯一生产路径。

## 已执行 TDD

- RED `cd server && go test ./internal/auth -count=1`：`TestPasswordHashAndVerification` 断言 `password must be stored as a salted hash` 失败；当时实现为可编译空壳。
- GREEN 同命令：通过。采用 x/crypto scrypt + 随机 salt，测试同时验证不同 salt、正确/错误密码和长度边界。
- RED `cd server && go test ./internal/app -run TestUnauthenticatedAndCrossOriginRequestsRejected -count=1`：`unauthenticated me: want 401 got 404`。
- GREEN 同命令：通过；覆盖无会话访问和跨源注册拒绝。

## 待执行

真实 PostgreSQL 隔离 schema 测试已编写；等待本次任务独立 PostgreSQL 实例启动后，先运行迁移、并发钱包与 HTTP 账号断言 RED，再实现持久化。

准备阶段说明：pgx 与 x/crypto 依赖下载、PostgreSQL 18.6 独立实例的编译不计作 RED；未提供 `LOADOUT_TEST_DATABASE_URL` 的数据库用例会明确 skip，不能用此结果声称数据库验收通过。

## PostgreSQL 与完整账号结果

本次测试实例：PostgreSQL 18.6，`LOADOUT_TEST_DATABASE_URL=postgres://loadout@127.0.0.1:44035/loadout_test?sslmode=disable`，每例随机 schema，结束删除；不操作真实用户数据库。

- RED `cd server && LOADOUT_TEST_DATABASE_URL=... go test ./internal/store -run TestMigrationsCreateDurableSchema -count=1`：`migration must create persistent users table`。
- GREEN 同命令：通过，连接池限制20、SQL超时15秒、嵌入迁移与事务 advisory lock。
- RED `... go test ./internal/store ./internal/app -count=1`：并发预占期望10实际0，HTTP注册期望201实际404。
- 账本实现后 `... go test ./internal/store -count=1`：并发、退款、恢复通过，append-only测试RED `ledger history must not be editable`；增加数据库触发器后同命令GREEN。
- RED `... go test ./internal/store -run TestOnlySuccessfulCallsReportChargedCost -count=1`：pending cost实际2、期望0；拆分requested_cost与实扣cost后全store GREEN。
- GREEN `... go test ./internal/app -count=1`：注册登录注销、session过期/撤销/禁用、token撤销/隔离、游标分页、管理员权限和重复调账通过。
- RED `... go test ./internal/app -run 'TestAdjustmentReplayReturnsOriginalBalanceAndRecordsActor|TestBootstrapAdmin' -count=1`：重放返回15而非原余额10，管理员引导登录401。增加原余额记录、操作者与安全引导创建后GREEN。
- REFACTOR `gofmt -w internal/store internal/auth internal/app`。
- 最终本模块 `... go test -race ./internal/store ./internal/auth ./internal/app -count=1`：全部通过（store 1.527s，auth 2.414s，app 7.056s）。未以此代替整合后的 `make check`。

## Task 4 RED / GREEN

所有数据库命令继续设置同一 `LOADOUT_TEST_DATABASE_URL`，每例独立 schema。

- 管理员/工具/全局CSV：`go test ./internal/app -run 'TestAdminCanDisable|TestAdminTools|TestGlobalUsage' -count=1`，RED 三个缺失路由404；实现角色保护、工具配置AES-GCM、启停与CSV公式保护后，同命令GREEN。
- 套餐/兑换/订单：`go test ./internal/app -run 'TestPlansRedemption|TestConcurrentRedemption' -count=1`，RED管理套餐/兑换码404；003迁移与真实事务后GREEN。两个用户并发兑同一码仅一人获得30 credits，单条账本；外部支付明确503且不建订单。
- 密码资源边界：`go test ./internal/auth -run TestPasswordWorkIsBoundedWithoutWaiting -count=1`，RED 32个并发全部进入密码运算；4槽非阻塞gate后GREEN，繁忙立即返回auth_busy（HTTP429）。
- 登录/注册速率：`go test ./internal/app -run TestAuthenticationEndpointsRejectExcessiveAttempts -count=1`，RED第121个请求仍400；增加每进程每分钟120个登录/注册/设备端点共享请求预算后GREEN。该上限是部署实例的保护上限，不宣称每用户独立配额。
- 团队：`go test ./internal/app -run 'TestTeamWallet|TestTeamSeats' -count=1`，RED创建团队404；004迁移、钱包成员授权、幂等转账、邀请、席位和owner交接后GREEN。移除后AuthToken与Reserve均拒绝，20起始额度转入团队并消费2后总余额18。
- 设备：`go test ./internal/app -run TestDevice -count=1`，RED公开authorize因Origin检查403；005迁移与精确两公开POST豁免、批准绑定及一次性领取后GREEN；pending、slow_down、过期与不可重绑均验证。
- 报表：`go test ./internal/app -run TestUsageSummaries -count=1`，RED summary404；SQL聚合后GREEN（2调用、3成本、1错误），验证1/7日与非法窗口拒绝。
- 安全补充检查：`go test ./internal/app -run TestManagementWrites -count=1`，现有实现首次即通过，属回归非新RED。覆盖管理写路由匿名401、普通用户403、非法输入400，以及用户业务写入口的匿名/非法输入。
- REFACTOR `gofmt -w internal/store internal/auth internal/app`；`go test -race ./internal/store ./internal/auth ./internal/app -count=1` 全通过（store 1.984s、auth 3.905s、app 31.931s）。之后新增安全矩阵未修改业务实现，完整回归与make check交主代理整合运行。

## 最后审查修复

- 邀请输入：`go test ./internal/app -run TestInviteRejects -count=1`，RED未知role字段返回201；严格解码后同命令GREEN。
- 慢请求体：`go test ./internal/app -run TestTeamWriteDoesNotLock -count=1`，RED首个PATCH等待请求体时持团队行锁，第二PATCH阻塞超过1秒；把读取/校验JSON移到事务前后GREEN。该测试使用受控请求体阻塞，执行真实数据库锁行为。
- 初始额度对照原PLAN修复：`go test ./internal/app -run TestRegistrationCredits -count=1`，RED默认注册余额0而非1000；`Options.InitialCredits` nil默认1000、显式0禁用，注册钱包与registration账本同事务，`go test ./internal/app -run 'TestRegistrationCredits|TestRegistrationLogin' -count=1` GREEN。既有账本测试setup显式0，未放宽余额断言。
- 审计查询与余额：`go test ./internal/app ./internal/store -run 'TestAdminLedger|TestReservationAndRefundLedger' -count=1`，RED管理员账本404、reservation balance_after为空；增加管理员user_id/actor_id/kind筛选、本人kind筛选、预占与退款真实结余后同命令GREEN。

## Benchmark

新增 `BenchmarkReserveFinish/serial` 与 `/parallel`，真实PostgreSQL、每次新request_key、先Reserve再Finish(true)，schema和预充钱包不计时，默认RunParallel并发跟随Go GOMAXPROCS。主代理统一运行两项并记录环境。

初版仅parallel实际执行：`go test ./internal/store -run '^$' -bench BenchmarkReserveFinish -benchtime=1s -count=1`，Linux amd64 / AMD Ryzen 7 9700X / Go -16，261次，4959901 ns/op、9004 B/op、81 allocs/op，PASS，无调用错误。这是并发共享钱包的平均每操作基准时间，不是单请求p50/p95，也不是独占机器的吞吐承诺。

最终代码验证（包括赠额、审计与慢请求修复）：`gofmt -w internal/store internal/auth internal/app`；设置真实DB后 `go test -race ./internal/store ./internal/auth ./internal/app -count=1` 通过（store 1.766s、auth 3.099s、app 20.252s）；紧接 `go vet ./internal/store ./internal/auth ./internal/app` 退出0，无输出。独立代码审查与整合 `make check` 由主代理接续，未声称已完成全仓验收。

## 独立审查后的授权修复

根代理授权将独立只读审查发现的identity问题修复；CLI reviewer提供团队竞态真实PostgreSQL复现，根代理授权本模块修复。

- `go test ./internal/identity -run 'TestGitHubStartBudget|TestDisabledEmail' -count=1`：RED匿名121次OAuth起始请求全部302，禁用用户邮箱验证虚假200；新增identity公开端点常数预算120/min、每次start清理最多1000个过期state，检查邮箱UPDATE实际行数，失败不消耗token。同命令GREEN；`go test -race ./internal/identity -count=1` 与 `go vet ./internal/identity` 通过。
- 将独立reviewer的两个真实锁等待测试加入 `internal/app/teams_race_test.go`。`go test ./internal/app -run 'TestReviewOwnerRole|TestReviewRemovedMember' -count=1` RED：owner交接后旧owner PATCH仍200，跨移除窗口创建的token在重新加入后生效；teamWrite与团队token创建改为先单独锁teams，再用新SQL快照读member权限，token INSERT保持同事务。
- 检查同类Store路径，`go test ./internal/store -run TestReservationRechecks -count=1` RED：钱包锁等待期间删除成员，Reserve仍成功；改为先锁钱包，再独立SQL重新检查token/user/wallet/member授权。
- `go test ./internal/app ./internal/store -run 'TestReviewOwnerRole|TestReviewRemovedMember|TestReservationRechecks' -count=1` GREEN。
- 其余只读review：disabled upstream退休问题已报主代理并由主代理修；禁用工具拒绝未写usage问题通过/tmp overlay实证（TestReviewDisabledCallIsAudited rows=0），已报主代理，本网关文件未改。

本轮审查修复最终验证：`gofmt` 后真实DB `go test -race ./internal/app ./internal/store ./internal/identity -count=1` 全通过（app18.794s、store1.737s、identity1.765s）；随后 `go vet ./internal/app ./internal/store ./internal/identity` 退出0。

## 报表历史扫描性能回归

范围：只新增 `011_reporting_indexes.sql`，未修改任何既有迁移。新增 user/created_at、wallet/created_at、global created_at 三个B-tree索引，以及 user/time、wallet/time 两组原生MCV统计；概览把当月UTC起点放进WHERE，概览/报表绑定请求时计算的UTC时间参数。

真实测试 `TestReportsReadOnlyTheRequestedTimeWindow` 通过 pgx QueryTracer 捕获 HTTP handler 实际执行的汇总SQL和参数，再执行 `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)`。未断言源码字符串、索引名字、固定执行计划或毫秒阈值，未设置enable_seqscan、强制hint或改变优化器成本。断言有效时窗最多3行时读取不超过100个scan rows/共享缓冲页，同时验证非UTC数据库会话（Pacific/Honolulu）下今日次数2、月成本5不变。

数据：本人/团队5万条三个月前历史，月边界前1微秒一条与当日2条；全局报告测完后，加入另一账号5万条当期调用，验证账户与团队选择性。每例独立schema，使用普通ANALYZE。旧日志含大量不同工具名，不能依赖少量工具值掩盖时间索引缺失。

RED命令：`LOADOUT_TEST_DATABASE_URL=... go test ./internal/app -run TestReportsReadOnlyTheRequestedTimeWindow -count=1 -v`。旧SQL/索引下三项均失败。最终相同命令GREEN：

| 实际SQL路径 | 旧scan rows | 新scan rows | 旧共享缓冲页 | 新共享缓冲页 | 旧执行ms | 新执行ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| global 7日（先测） | 2 | 2 | 430 | 3 | 0.697 | 0.052 |
| account/me（另账号5万当期） | 100003 | 2 | 1771 | 3 | 14.439 | 0.018 |
| team 7日（另账号5万当期） | 100002 | 2 | 1780 | 5 | 7.647 | 0.045 |

这些是PostgreSQL 18.6本机隔离实例的暖缓存EXPLAIN执行，Shared Read均0，页计数为Shared Hit+Read；scan rows累计usage_logs节点的Actual Rows/Rows Removed/Loops，并不计B-tree内部扫描的键。全局旧scan rows仅2但读430页，说明只看返回行数会漏掉索引读放大。毫秒仅作本次观测，不设验收阈值，不代表公网延迟、冷盘性能或容量承诺。

诊断过程：只有3索引时，全局降到3页，但个人/团队在另一账号活动高度相关的分布下仍扫描50002行、953/954页（优化器独立选择率估计约25000行，真实2）。只添加MCV而保留SQL内now()/时区interval的STABLE边界表达式仍未改善；临时EXPLAIN常量边界对照证明确定边界可使用联合统计，遂改为生产请求绑定UTC timestamp后达到最终GREEN。该临时诊断未作为最终业务验收依据。索引增加每次usage INSERT的维护成本与磁盘空间；MCV在ANALYZE采集，本身无逐行写入索引维护，不承诺省去统计更新。

最终稳定性与回归：相同真实业务性能测试 `-count=3 -v` 三次全部通过；每次独立建schema、重新生成数据并普通ANALYZE，三次global/me/team共享页均稳定为3/3/5，scan rows均2，未调整采样设置。最终 `go test -race ./internal/app ./internal/store -count=1` 通过（app23.010s、store1.736s），随后 `go vet ./internal/app ./internal/store` 退出0。代码已冻结，主代理执行最终全仓make check与benchmark。
