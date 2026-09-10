# 2026-09-11 网关审查问题修复

范围：gateway 与 store；复用现有官方 MCP SDK、PG 行锁与八个发现 worker，不新增依赖。已使用 systematic-debugging、test-driven-development、ponytail(full)、verification-before-completion。完整 make check 由主任务运行，本子任务不声称完成完整 harness。

## 根因与验收映射

| 问题 | 最小修复 | 行为测试 |
|---|---|---|
| 工具在钱包锁等待期间禁用，旧调用仍执行扣费 | ReserveTool 在钱包锁取得后、预占前以 SELECT ... FOR SHARE 读取最新 enabled/cost；禁用返回工具不可用，余额保持10 | TestReviewDisableDuringWalletWait |
| 工具行锁等待期间撤销token | 工具锁取得后再核验token、用户和成员授权，之后才预占 | TestTokenRevokedDuringToolLockWait |
| 等待期间修改价格仍使用旧价 | 同一预占事务使用新价格，成功调用 usage.cost=4、ledger.delta=-4、余额10→6 | TestPriceChangeDuringWalletWait |
| provider key 与远程工具名拼接碰撞 | provider key 中每个下划线转义为 _u；拼接 __ 和原始远程工具名；注册前检查所有名称重复并拒绝冲突目录 | TestBoundaryNamespaceCollisionRoutesToWrongProvider |
| LIMIT 128 隐藏启用上游 | 按 id 游标每页128行，完成该页发现后读下一页；发现并发仍最多8；五秒总期限耗尽返回错误，不发布因总期限截断的目录 | TestBoundaryAll129EnabledUpstreamsVisible；既有 TestConcurrentCatalogRequestsBoundDiscoveryAndShareConnections |
| AuthToken 查询故障返回401 | 只有 ErrUnauthorized 返回401；其他数据库错误返回固定文本503，恢复后原凭证可继续使用 | TestGatewayAuthenticationDatabaseFailureIsRetryable |
| 错误 builtin schema 导致 AddTool panic | 导出 ValidateToolSchema 复用官方SDK AddTool验证；管理API可复用；目录跳过已损坏builtin，其他请求保持可用 | TestReviewMalformedBuiltinSchema；管理API测试由app修复任务负责 |

名称规则的单射性：转义后的 provider key 不包含双下划线，因此第一个 __ 唯一确定 key 与远程名称边界。转义 key 中的 _u 唯一解码为原始下划线（原来的 _u 变成 _uu），其余字符保持不变。两个标准 builtin（echo/time_now）均不含 __，不会与远程名字相同；其他异常目录重名在注册前报错。

兼容性：没有下划线的 provider key 公开名称不变；含下划线的 key 需要客户端刷新 tools/list，例如 foo__bar/baz 从旧的歧义名 foo__bar__baz 变成 foo_u_ubar__baz。foo/bar__baz 的公开名保持 foo__bar__baz。未保留可能重定向到错误上游的歧义别名。

## RED / GREEN / REFACTOR 顺序证据

所有数据库命令仅将根 .env 中 LOADOUT_TEST_DATABASE_URL、LOADOUT_TEST_REDIS_URL 加载到测试子进程；没有读取业务数据库配置或打印秘密。gatewayDB 为每个测试创建唯一 schema，仅删除该次创建的名称。没有执行按数据库内容搜索/清理 schema。

1. RED：从已审查的临时复现迁入正式行为测试，先运行：

```sh
cd server
go test ./internal/gateway -run 'TestReviewDisableDuringWalletWait|TestReviewMalformedBuiltinSchema|TestGatewayAuthenticationDatabaseFailureIsRetryable|TestBoundary' -count=1 -v -timeout=90s
```

退出1；六处行为断言揭示五个问题：禁用仍执行（余额9）、builtin AddTool panic、DB超时401、重名丢一项且调用B/余额2、129上游仅128项。原始输出 `/tmp/loadout-gateway-fixes-red.log`。

2. 价格行为单独 RED：

```sh
go test ./internal/gateway -run TestPriceChangeDuringWalletWait -count=1 -v
```

退出1；价格已提交为4，实际仍扣1、余额9。输出 `/tmp/loadout-gateway-price-red.log`。

3. GREEN：完成上述最小生产修复后运行相同测试，增加价格用例：

```sh
go test ./internal/gateway -run 'TestReviewDisableDuringWalletWait|TestReviewMalformedBuiltinSchema|TestGatewayAuthenticationDatabaseFailureIsRetryable|TestPriceChangeDuringWalletWait|TestBoundary' -count=1 -v -timeout=90s
```

退出0，六个测试通过。输出 `/tmp/loadout-gateway-fixes-green.log`。

4. REFACTOR / 回归：统一受影响文件 HTTP 状态常量、显式处理测试清理返回值、等价简化 URL scheme 条件；加强价格测试的 usage/ledger 断言和坏Schema请求成功断言。

```sh
go test -race -count=1 ./internal/gateway ./internal/store
```

退出0，gateway 16.746s、store 3.023s。真实 PostgreSQL / Redis 环境已加载。输出 `/tmp/loadout-gateway-fixes-regression.log`。

5. 主任务复核发现新增工具 FOR SHARE 也会等待，授权查询必须处于最终行锁之后。新增正式用例并先运行 RED：

```sh
go test ./internal/gateway -run TestTokenRevokedDuringToolLockWait -count=1 -v -timeout=30s
```

RED退出1，工具行锁等待期间token已撤销，旧顺序仍执行且余额9；输出 `/tmp/loadout-gateway-tool-lock-red.log`。将授权查询整体移到钱包锁和工具锁之后，同一命令GREEN退出0；输出 `/tmp/loadout-gateway-tool-lock-green.log`。价格限额沿用原规则：admit限制调用次数，与单次价格无关；钱包余额预占使用新价格，未引入新的资金日预算语义。

6. 最终受影响包静态检查：

```sh
/tmp/loadout-golangci-ocjz6oni/golangci-lint run ./internal/gateway/... ./internal/store/...
```

退出0，0 issues。首次检查报告的import分组（含store原有文件）和URL scheme等价条件已修正。与完整harness的全仓库lint结果区分。

最终工具锁授权修复后再次执行 `go test -race -count=1 ./internal/gateway ./internal/store`：退出0，gateway 16.539s、store 2.715s；最终输出覆盖保存在 `/tmp/loadout-gateway-fixes-regression.log`。

## 最终外键锁等待窗口补充

主任务独立审查发现：显式钱包/工具锁之后的 usage_logs、ledger INSERT 还会因为 user_id/token_id 外键取得行锁并等待。此前的授权重验位于这些 INSERT 之前，因此仍可能在等待期间提交账号禁用/token撤销后继续执行。

新增 `TestAuthorizationChangedDuringReservationForeignKeyWait`，分别锁 users 和 tokens 行；通过 pg_stat_activity 确认调用正在 `INSERT INTO usage_logs` 外键锁处等待，再在持锁事务中提交禁用/撤销。验收要求调用返回错误、余额保持10、usage和ledger均为0。

```sh
cd server
go test ./internal/gateway -run TestAuthorizationChangedDuringReservationForeignKeyWait -count=1 -v -timeout=30s
```

RED退出1，两个子例均为 error=false、balance=9、usage=1、ledger=1；日志 `/tmp/loadout-gateway-fk-lock-red.log`。之后将局部授权查询复用为闭包，保留写入前检查，并在所有 usage/wallet/ledger 写入完成后、Commit之前再次核验。失败会回滚整个事务，网关无法开始执行工具。相同命令GREEN退出0，两个子例通过；日志 `/tmp/loadout-gateway-fk-lock-green.log`。

补充修复后的相关回归 `go test -race -count=1 ./internal/gateway ./internal/store` 退出0：gateway 16.693s、store 2.859s；日志 `/tmp/loadout-gateway-fk-lock-regression.log`。此时lint仅报告另一并行任务正在编辑的 `store/rate.go` import尚未gofmt，已通知其负责人；本次reserve/gateway文件没有新增诊断。全仓库最终lint由主任务汇总。

## 验证范围

本子任务只运行聚焦与相关Go回归，未运行完整make check、真实外部供应商/生产服务、浏览器或客户端人工验证。目录分页的工作页和发现并发有上限，最终工具目录及连接池仍随实际启用上游数量增长；五秒发现预算没有移除。完整验收与公开名称迁移文档由主任务汇总。
