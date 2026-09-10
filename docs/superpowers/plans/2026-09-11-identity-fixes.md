# Identity 审查修复执行记录

2026-09-11。用户授权修复服务端审查发现；当前工作区执行，保留其他模块的改动。范围为 `server/internal/identity/**`，共享匿名限流 helper 由 app 子任务在 `auth` 包实现；未新增依赖或数据库迁移。

## 根因与验收

| 问题 | 根因与最小实现 | 行为验收 |
| --- | --- | --- |
| 暂时数据库故障导致前端误认为已退出 | `user` 原将所有查询错误转换为 401；仅 `pgx.ErrNoRows` 返回 401，其他查询故障返回 `503 temporarily_unavailable` | 有效会话查询因真实 PostgreSQL 表锁触发 statement timeout 返回 503；释放锁后同一 Cookie 返回 200；缺失/无效 Cookie 仍 401 |
| 失败重发破坏首封邮件链接 | 原先已提交覆盖旧 token 后发送，失败又删除新 token；改为在数据库事务内 upsert、发信，发送成功才提交，失败回滚保留旧记录 | 成功首封 → 冷却后 SMTP 失败 → 原未过期链接仍可验证 |
| 重发/验证竞态 | 同一 token 行的事务锁跨实例串行化发送与验证；不引入补偿 UPDATE，避免覆盖更新成功的结果 | SMTP 阻塞期间发起旧 token 验证或后续重发；通过 `pg_locks` 确认真实锁等待，失败释放后分别验证成功或新邮件成功；新链接可用，旧 token 不复活 |
| 共享匿名 subject 0 被错误方法/单个来源占满 | 限流放入 ServeMux 已匹配的业务 handler，调用 `auth.AllowPublicRequest`；按 `github_start`、`github_callback`、`email_verify` 分配独立预算 | 错误方法不计数；同 IP 换端口不能重置预算；其他 IP 与 OAuth 回调不被 OAuth 发起预算耗尽；原跨副本/重启预算测试继续通过 |
| golangci 标准诊断 | 对已完成读取的 Body、清理 Close/Rollback 显式忽略返回错误；SMTP fixture 的响应写入显式忽略错误，已有协议行为测试覆盖交付结果 | standard golangci identity 0 issues，真实 SMTP 本地 TLS 边界及全包 race 回归通过 |

公共预算采用 app 子任务的统一实现：每个 family 每真实 TCP peer IP 120/min、全局同 family 1200/min；不信任转发头，以 PostgreSQL 数据库窗口为权威。经过反向代理时相同 peer 会共享调用方额度；部署需要统一考虑代理入口策略。超额返回 429 与 Retry-After: 60，预算存储故障返回 503。

发送事务借助 PostgreSQL 现有行锁，不持有全局 Go 锁。SMTP 使用既有 10 秒 context 上限（实际 SMTPMailer 的网络 deadline 更短）；邮件交付和数据库提交无法跨系统原子提交，因此“SMTP 已接受但随后数据库提交失败”仍返回 503，不能宣称该封链接已激活。普通 SMTP 发送失败会完整回滚原 token、邮箱和有效期。

## RED / GREEN / REFACTOR 证据

运行前仅从 `.env` 提取 `LOADOUT_TEST_DATABASE_URL` / `LOADOUT_TEST_REDIS_URL` 到命令环境，不打印内容。测试沿用 `identityDB` 创建唯一 schema，并仅清理该测试明确创建的 schema；没有按数据库内容扫描清理。

1. 迁入原 overlay 的两项真实行为复现，修复前运行：
   - `go -C server test ./internal/identity -run '^TestReview(TransientDatabaseFailure|FailedResend)' -count=1 -v -timeout=60s`
   - **RED**：暂时 statement timeout 实际 401；失败重发后首封链接实际 400。退出 1，`/tmp/loadout-identity-fixes-red.log`。
2. 添加并发验证与后续重发测试，修复前运行：
   - `go -C server test ./internal/identity -run '^TestFailedResendSerializes' -count=1 -v -timeout=60s`
   - **RED**：失败发送尚未结束，旧 token 验证提前 400、后续重发提前 429。退出 1，`/tmp/loadout-identity-concurrency-red.log`。
3. 实施会话错误区分与发送事务后：
   - `go -C server test ./internal/identity -run 'TestReview(TransientDatabaseFailure|FailedResend)|TestFailedResendSerializes' -count=1 -v -timeout=60s`
   - **GREEN**：全部通过，退出 0，`/tmp/loadout-identity-fixes-green.log`。
4. 匿名预算变更前：
   - `go -C server test ./internal/identity -run '^TestIdentityBudgetsIgnore' -count=1 -v -timeout=60s`
   - **RED**：错误方法消耗合法请求预算；A 耗尽后 B 实际 429。退出 1，`/tmp/loadout-identity-budget-red.log`。
   - 接入共享 helper、移动到匹配的 handler 后同一命令 **GREEN**，退出 0，`/tmp/loadout-identity-budget-green.log`。
5. **REFACTOR 与最终包回归**：显式处理清理调用、gofmt，并增强 503 后恢复及无效会话保持 401 的断言：
   - `go -C server test -race ./internal/identity -count=1 -v -timeout=120s`：退出 0，5.804s，`/tmp/loadout-identity-fixes-final.log`。覆盖跨实例/重启预算、OAuth state/PKCE/一次性、邮件重放/过期/禁用及 SMTP TLS fixture。
   - `go -C server vet ./internal/identity`：退出 0。
   - 在 `server` 下 `/tmp/loadout-golangci-ocjz6oni/golangci-lint run ./internal/identity/...`：退出 0，`0 issues.`。

## 验证边界与主任务同步

- 完整 `make check` 由主任务执行，本子任务的包测试不能代替完整 harness。
- 主任务需更新 API/部署的匿名限流配额、peer 语义与邮件重发失败保留旧链接约定；旧 `identity,0,120/min` 记录属于历史实现。
- 未访问真实 GitHub/邮件供应商，没有浏览器自动化、服务重启、提交、推送或修改真实客户端配置。
