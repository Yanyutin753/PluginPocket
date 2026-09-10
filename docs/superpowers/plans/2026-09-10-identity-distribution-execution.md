# Identity 多副本执行记录

执行 `2026-09-10-distributed-backend.md` 的 identity 子任务。范围限定 identity 包；复用 store 子任务提供的 `Store.AllowRequest`。未增加迁移、依赖或 Redis，未修改前端、网关或真实客户端配置。

## 审计结果与最小实现

- OAuth state、browser hash、PKCE verifier 已由 `010_identity.sql` 的 `oauth_states` 保存。回调按 state/browser hash/数据库到期时间执行 `DELETE ... RETURNING verifier`，一次消费不依赖发起实例。
- OAuth identity、账号、钱包、首次赠额、会话创建已有数据库事务；provider identity 的事务 advisory lock 防止同一 GitHub 身份并发创建两份账号。
- 邮箱验证 token hash、邮箱与有效期已在 `email_verifications`；验证在事务中原子删除 token 并更新用户。发信冷却也在数据库，不需再迁移或复制一套状态机。
- 原 `identity` 的 `sync.Mutex`、本地窗口和计数确实允许换副本/重启绕过匿名预算。已删除这三项，改为 `AllowRequest(ctx, "identity", 0, 60, 120)`，共享数据库时钟窗口。数据库错误返回安全的 503；确认额度耗尽才返回 429 和 Retry-After。数据库未配置时原不可用功能仍返回不可用状态。
- OAuth 创建网页会话原使用 `time.Now().Add(7 days)`。已改成 SQL `now()+interval '7 days' RETURNING expires_at`，Cookie Expires 使用同一返回值。OAuth state、邮箱验证的权威时间原本就是数据库时间；HTTP/SMTP 网络超时仍由 Go context/deadline 控制。

## 真实跨副本验收

`distribution_test.go` 使用真实 PostgreSQL 私有 schema、分别打开的连接池、独立 identity handler 和 `httptest.NewServer` TCP HTTP 服务。发起副本关闭后才创建回调副本，不需要粘性路由。外部 GitHub 为本地受控 HTTP provider；邮件使用交付链接的边界函数，既有 SMTP 测试另外验证真实本地 SMTP/TLS 边界。

| 测试 | 可观察行为与持久化断言 |
|---|---|
| `TestIdentityBudgetSurvivesReplicaChangeAndRestart` | 两副本合计 120 次 OAuth 发起可用；关闭两副本、重建后第 121 次仍为 429 |
| `TestGitHubStateCrossesReplicasAndSurvivesOriginatingReplicaLoss` | A 发起后消失；新 B/C 接回调；错误 browser Cookie 拒绝且不烧掉合法 state；并发回调只一次 302/一次 400、provider 仅交换一次；PKCE 匹配原 challenge；跨副本接受新会话；identity/session/首次赠额均只有一份；Cookie 到期时间与数据库一致；过期 state 和回放均拒绝 |
| `TestEmailVerificationCrossesReplicasAndConsumesOnce` | A 发信后消失；B 仍执行原冷却 429；B/C 并发验证恰一次 200/一次 400；邮箱及验证状态在另一副本可读；token 已删除；重建后回放仍拒绝；过期 token 不能在另一副本兑换 |

## RED / GREEN / REFACTOR

均在导出隔离 `LOADOUT_TEST_DATABASE_URL` 后运行。未使用业务 public schema。

1. **预算 RED**：`go -C server test -race ./internal/identity -run 'TestIdentityBudgetSurvives|TestGitHubStateCrosses|TestEmailVerificationCrosses' -count=1 -v`。
   - 第 121 次请求实际 302、期望 429；日志 `/tmp/loadout-identity-distribution-red.log`。
   - 同批 OAuth/email 跨副本测试首次即绿，明确是已有持久化行为的新增覆盖，未伪造 RED。
2. **预算 GREEN**：接入 store 子任务已验证的共享 helper 后运行同一命令，三个测试通过，`ok ... 2.277s`；日志 `/tmp/loadout-identity-distribution-green.log`。
3. **数据库会话时钟 RED**：增加 `sessions.expires_at = sessions.created_at + interval '7 days'` 的数据库生命周期不变量；`go -C server test -race ./internal/identity -run '^TestGitHubStateCrossesReplicasAndSurvivesOriginatingReplicaLoss$' -count=1 -v` 失败，`valid=false`；日志 `/tmp/loadout-identity-expiry-red.log`。
4. **数据库会话时钟 GREEN**：SQL 生成并返回 expires_at 后同一测试通过，`ok ... 1.127s`；日志 `/tmp/loadout-identity-expiry-green.log`。
5. **绿灯整理与回归**：保留共享 helper，删除旧 mutex/窗口计数；补查实际 Cookie Expires 与数据库时间精确到秒一致，以及 OAuth/email 过期拒绝。
   - 最终 `go -C server test -race ./internal/identity -count=1 -v` 全包通过，`ok ... 3.341s`；日志 `/tmp/loadout-identity-distribution-final.log`。
   - `go -C server vet ./internal/identity` 通过；已对所有改动 Go 文件执行 gofmt。

## 边界

- OAuth 回调原子消费 state 后才向外部 provider 交换一次性 code；此时执行实例消失，用户需要重新发起授权。不能安全地承诺自动重放外部一次性 code，本实现保持防重放边界。
- 已提交账号、赠额、会话和验证状态跨副本保留；测试关闭的是 handler/HTTP 服务，进程级负载均衡与节点消失由根任务另外验证。
- 测试 provider 不代表真实 GitHub 服务验收；邮件边界测试不代表真实邮件供应商交付。
- 完整 `make check` 和全系统分布式验收归根任务，不以本包回归替代。
