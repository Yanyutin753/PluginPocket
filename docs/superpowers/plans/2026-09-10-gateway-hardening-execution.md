# 网关 E2E 边界加固执行记录

范围：`2026-09-10-e2e-hardening.md` 任务 3；仅改 gateway 包与本记录。使用真实 PostgreSQL 唯一 schema、`httptest` HTTP 服务及官方 MCP Go SDK，未改依赖、账本实现或生产配置，未使用浏览器自动化。

## 已修复行为与 TDD 证据

下列命令均先在仓库根执行 `source /tmp/loadout-pg.env`，再 `cd server`。测试 URL 为本次恢复的临时 PostgreSQL 44035；每例创建并删除自己的 schema。环境连接失败不计 RED。

| 验收 | RED 命令与实际失败 | 最小修复 | 同命令 GREEN |
| --- | --- | --- | --- |
| 上游分页工具全部出现 | `go test ./internal/gateway -run TestCatalogIncludesEveryUpstreamPage -count=1`：遗漏 `remote__second` / `remote__third` | 使用官方 SDK `Tools` 分页迭代器；任何一页失败时舍弃该上游的残缺目录 | 通过 |
| 目录等待者取消独立返回 | `go test ./internal/gateway -run 'Test(CatalogWaiterCancellation|CanceledCatalogLeader)' -count=1`：取消等待者 300ms 后仍阻塞；取消首请求返回 nil 并把残缺目录缓存给下一请求 | `singleflight.DoChan` 与调用方 context 分离；共享目录发现有独立 5s 预算 | 通过；随后等待者测试强化为进入等待后 100ms 截止，而非预先取消 |
| 管理变更后旧发现不能恢复禁用工具 | `go test ./internal/gateway -run TestInvalidationDuringDiscovery -count=1`：`inflight discovery restored a disabled tool after invalidation` | 目录版本号；过期构建不发布、不清理新版本连接，等待者串行重建当前版本 | 通过 |
| Close 与并发建连 | `go test ./internal/gateway -run TestCloseDuringConnect -count=1`：关闭后建连仍成功、保留一个会话、接受后续连接与目录请求 | Gateway 生命周期 context；关闭取消发现和建连；发布连接前在锁内拒绝关闭状态并释放晚到会话 | 通过 |
| 连接等待者取消独立返回 | `go test ./internal/gateway -run TestConnectionWaiterCancellation -count=1`：取消等待者 300ms 后仍被别的握手阻塞 | 连接 singleflight 使用 `DoChan`；等待者监听自己的 context 与 Gateway 生命周期；连接仍沿用发起方超时 | 通过 |
| 受支持的 legacy stateful HTTP 模式中取消调用不关闭其他调用 | `go test ./internal/gateway -run TestCanceledLegacyHTTPCall -count=1`：移除取消保护后 `canceled call waited for unrelated work on the shared session` | `dropSession` 不因调用方 context 取消关闭共享连接；调用与目录错误共用该边界 | 恢复保护后通过 |
| 默认现代 stateless HTTP 取消不污染共享连接 | `go test ./internal/gateway -run TestCanceledModernHTTPCall -count=1`：真实 `Stateless:true` 上游的并发 B 因 A 的取消通知返回错误 | 在现有 HTTP RoundTripper 内，通过 SDK 标准 HTTP 头识别现代无会话取消通知，联网前明确拒绝；原 HTTP 请求取消负责停止上游 | 同命令通过；A 的远端 handler 确实停止、B 成功、后续第三次调用成功 |

Close 测试准备时曾把阻塞点设为 `initialize`，结果是 fixture 未进入而非产品行为 RED；锁定 SDK v1.7.0 首先执行 `server/discover` 后才得到表中有效 RED。

## 既有行为覆盖与重构

- `TestConcurrentCatalogRequestsBoundDiscoveryAndShareConnections` 首次即绿，属于覆盖已有行为：24 个目录请求、12 个真实 HTTP 上游，完整返回 2 个 builtin 与 12 个上游工具；实测最多 8 个并发握手、总计 12 次握手，证明发现并发有界且请求共享建连。
- 现有 `TestSlowUpstreamDoesNotHideHealthyTools` 曾要求 300ms context 到期后依然返回成功目录，与修复后的独立取消契约冲突。现在保持调用者 3s 活跃，让慢上游触发自身 2s 发现预算，验证健康工具保留；调用取消由独立测试验证。慢 HTTP fixture 兜底为 5s，不靠它先返回错误掩盖上游超时。
- 没有增加自定义分页协议、连接池或任务队列。保留已有 singleflight、errgroup（8 workers）、128 个配置行限制与 5s 缓存 TTL；目录失效不 `Forget` 并启动额外并发构建。
- 在绿灯下统一 gofmt、复用官方 HTTP 阻塞 fixture；完整网关回归覆盖原有计费、超时退款、错误摘要不泄密、配置替换、禁用、stdio 和 SSRF 边界。

最终验证实际执行：

```sh
gofmt -w server/internal/gateway/gateway.go server/internal/gateway/upstream.go server/internal/gateway/hardening_test.go server/internal/gateway/upstream_test.go server/internal/gateway/transport.go server/internal/gateway/transport_test.go
source /tmp/loadout-pg.env
cd server
go test -race ./internal/gateway -count=3
go vet ./internal/gateway
```

结果：首次 race 连续三轮通过（25.834s）；追加现代 stateless 兼容修复后重跑三轮通过（`ok .../internal/gateway 25.179s`），vet 退出 0。完整 `make check` 由根任务在所有并行变更合并到当前工作区后运行，此处不声称已完成全仓验收。

## 官方 SDK 取消边界与发送前兼容处理

官方 MCP Go SDK v1.7.0 与显式 `Stateless:true` HTTP 服务配合时有并发取消问题。复现使用两个真实 `tools/call`，上游都已经进入 handler 后取消 A，保持 B 阻塞，再释放 B。只修改网关主动关闭逻辑时，B 仍失败。控制 fixture 的诊断得到：

```text
context canceled
sending "notifications/cancelled": Bad Request
calling "tools/call": sending "notifications/cancelled": Bad Request
```

定位到模块源码 `mcp/transport.go:cancelCall`：它直接通过内部 jsonrpc connection 发送 `notifications/cancelled`，绕过公开发送 middleware。该通知被 stateless HTTP 服务拒为 400，通知写错误使共享连接失效。仅跳过 `g.dropSession` 无法修复 SDK 内部连接状态。该诊断使用临时日志，日志已删除，生产不会输出上游原始错误。

首轮只验证了 **2025-11-25 legacy stateful HTTP**：fixture 对 `server/discover` 返回不支持，官方客户端自行回退 `initialize`。后续有界调查补充纠正：相同现代协议在官方 handler 的 `Stateless:false` 配置下，双调用取消也正常；不能把所有现代协议描述为都会失败。当前测试显式区分服务模式，不依靠协商推断 stateless。

进一步核查公开传输能力得到：

- `mcp/streamable_headers.go:setStandardHeaders` 为通知和请求统一设置 `Mcp-Method`；`setMCPHeaders` 在调用它之前设置 `Mcp-Protocol-Version`。
- `mcp/streamable.go:streamableClientConn.Write` 对 `http.Client.Do` 返回的本地错误使用 SDK 内部 `ErrRejected`，不会把未发送请求的错误记为连接级 writeErr。
- 因此现有 `headerTransport.RoundTrip` 可以仅凭标准头，精确识别版本至少 `2026-07-28`、无 `Mcp-Session-Id` 的 `notifications/cancelled`，关闭 body 并在网络发送前返回明确不支持错误。该请求确实未发送，没有把已收到的 HTTP 400 改写成网络失败，也没有返回伪造的 2xx。
- 原始调用的 HTTP context 取消负责断开请求。测试等待上游 A 的 handler 收到 `ctx.Done()` 并退出后，才释放 B；B 返回成功，再执行第三次调用成功，证明共享会话保持可用。

生产兼容改动只在现有 HTTP transport 中添加一个受版本、会话 ID 和方法三个条件约束的分支，不解析 MCP JSON、不重写消息、不改变锁定依赖。legacy、有会话 ID 的通知和普通工具请求继续正常转发。`TestStatelessCancellationRejectedBeforeNetworkWithoutAffectingOtherRequests` 使用真实 HTTP 请求计数验证上述分支，首次即绿，属于已有修复的边界回归。

兼容修复的聚焦验证 `go test -race ./internal/gateway -run 'TestCanceled(Legacy|Modern)HTTPCall' -count=3` 通过（2.058s）。最终模式覆盖包括 legacy stateful 与现代 stateless，之前记录的 stateless 并发失败在 Loadout HTTP 传输路径已得到 RED→GREEN；上游 SDK 本身未修改。第三方服务是否遵守请求断开取消仍取决于其实现，本测试证明的是启用官方 `PropagateRequestCancellation` 的真实上游行为。

## 独立审查重点

1. 目录发布与 `Invalidate` 使用同一 mutex；版本不符只重建，不回填旧 TTL，也不使用旧配置清理新会话。
2. `Close` 可重复调用，关闭后拒绝新的发现/连接；异步建连发布前再次检查生命周期并关闭晚到资源。
3. 取消等待者不取消共享目录工作；共享发现总预算、单上游预算与并发限制仍在。连接发起方取消仍可能让共同等待该握手的请求收到错误，但不会缓存残缺目录，也不会自动重放工具调用。
4. SDK 分页任一页失败时隐藏该上游而非展示不完整工具集合；builtin 和健康上游保持可用。
5. 现代 stateless 兼容分支必须在网络发送前拒绝，保留明确错误，不解析/伪造协议响应；测试必须同时检查远端停止、并发调用和后续调用，以及 legacy/有会话请求不受影响。

## 独立审查后：HTTP 会话租用隔离

根任务审查发现普通 RPC 错误仍可能导致网关关闭承载其他调用的共享 SDK 会话。实测确认两层问题：普通 handler 错误会让我方 `Close` 等待 B；现代协议的参数/方法错误则按官方 SDK `extractErrorStatus` 转为 HTTP 400/404，SDK 本身也会终止该会话。单纯 `errors.As(*jsonrpc.Error)` 跳过关闭无法保证 B，还会把收到 HTTP 400 后失效的连接留在缓存。

经根任务明确授权，采用官方 SDK 会话的独占租用，保持现有工程边界：

- `http_pool.go` 按配置签名缓存 HTTP 池，每池共享一个既有 SSRF 防护 `http.Client`。调用与目录发现分别租用 SDK session，直到操作结束才归还；成功保留为 idle，任何调用错误仅丢弃其独占 session，不影响 B、不自动重放。
- 同一工具行的新旧配置共用 32 个租用名额，避免配置替换期间变成旧 32 + 新 32；不同上游有独立名额。目录仍最多 8 个发现 worker。正常顺序调用复用 idle session 和底层 HTTP 连接。
- 配置替换/禁用退休旧池，关闭 idle；active 按原请求 deadline 正常完成再关闭。相同配置重新启用可以创建新池，旧池的清理通过指针比较避免删除新池。只改价格的普通 `Invalidate` 不会重新建立同签名池。
- 全局 `Close` 先取消请求，再等待租用清理。HTTP 请求（包括 SDK 自发的取消通知/DELETE）也绑定 Gateway 生命周期，响应 body 关闭时释放 context 回调，避免取消父请求后 SDK 的后台请求拖住退出。
- SDK 握手取消可能继续发送通知，租用等待方因此通过 channel 独立响应 deadline；后台握手仍占原租用名额并由全局生命周期清理，不因等待者离开而突破资源上限。
- stdio 继续共享官方 SDK 会话，仅非请求取消且非普通 JSON-RPC 响应错误才删除连接。删除了已经不可达的旧 HTTP 建连支路。未采用每调用 HTTP 状态跟踪、Wait 监视器或自写 MCP 协议。

所有命令继续采用前述测试数据库环境并在 `server` 运行。

| 回归 | 实际 RED | GREEN / 覆盖 |
| --- | --- | --- |
| `go test ./internal/gateway -run TestRPCErrorDoesNotCloseConcurrentCall -count=1` | handler error 阻塞 A；invalid params / method not found 中断 B；后续调用失效或不必要重建 | 现代 handler、参数、方法错误全部隔离，B 成功、C 复用；并发阶段只建立两个 session |
| `go test ./internal/gateway -run TestHTTPProtocolFailureRebuildsConnection -count=1` | 盲目跳过所有 typed RPC error 后，HTTP 400 留死缓存：只执行一次上游请求、只建立一次会话，第二请求失败 | HTTP 400/404 均仅让当前 session 退出，下一调用重新建连成功，无重放 |
| `go test ./internal/gateway -run TestCloseCancelsActiveHTTPCall -count=1` | 300ms 内 Close 未返回，释放旧 handler 后调用仍成功 | 先取消 active lease，再等待归还；Close 有界返回，旧调用返回错误 |
| `go test ./internal/gateway -run TestRetiredHTTPPool -count=1` | 退休池仍被同配置重新启用请求取得，目录缺少 `remote__wait` | 旧 active 自然成功，新池立即可用；旧 release 不删除新池 |
| `go test ./internal/gateway -run TestHTTPPoolBounds -count=1` | 配置替换绕过原 provider 的 32-call 上限 | 同行新旧配置共享名额；满载 provider 不隐藏另一个 provider；40 个请求完成、后续三次成功复用，32+1 次握手 |
| `TestHTTPDisconnectRebuildsOnlyTheFailedSession` | 首次即绿，属于已修行为的真实网络边界覆盖 | `http.Hijacker` 真正断开首个工具请求，首请求返回失败，后续请求重建成功；2 次握手、1 次实际 handler 执行，无重放 |

绿灯下清理重复的旧 HTTP 支路，并修正错误归还只丢弃其 SDK session，保留其他空闲 HTTP 连接；退休时才关闭池的 idle connections。

本阶段执行 `go test -race ./internal/gateway -count=3 -timeout=60s` 通过（34.778s）且 vet 通过；追加“同上游跨配置仍然 32”后，聚焦 race 两轮通过（6.879s）。最终冻结前执行 `go test -race ./internal/gateway -count=1 -timeout=40s` 通过（12.304s）、`go vet ./internal/gateway` 退出 0；根任务负责最终全仓 `make check`。

## 多副本语义交接

本地池与目录只优化上游访问，不是权限、额度或限频权威。每次请求鉴权、工具启用/价格查询、限流及计费走共享 PostgreSQL。租用名额限制每副本每上游 32，不能宣称是集群范围 provider 并发限制。

本轮原始版本的目录 TTL 是本地 5s，`Invalidate` 只作用当前副本，其他副本的新增/配置变化曾需等本地过期；旧列表显示禁用工具时，实际新调用仍经过数据库 enabled 检查。用户追加“后端支持分布式”后，根任务已接管共享数据库 revision 与跨真实进程验证；此文件不把尚未由本子任务执行的 revision 集成或全仓检查写成已通过。

## 最终独立审查修复：连续退休不能丢失 provider 容量

独立审查复现了配置索引和容量生命周期绑定的缺陷：保留第一池的 active lease，将其退休；同配置重新建第二池后立即退休，第二池会从配置 map 删除；第三池扫描不到仍有 active lease 的第一池，于是重新分配 32 个名额，总数可达 33。待完成握手尚未进入 sessions map，也必须算作未清理资源。

先增加真实 PostgreSQL 与官方 HTTP MCP fixture 回归 `TestRepeatedRetirementKeepsProviderCapacity`，执行 `go test ./internal/gateway -run TestRepeatedRetirementKeepsProviderCapacity -count=1`，实际 RED 为 `double retirement allowed 33 sessions for the same provider`。

最小修复将 provider 容量从配置 map 独立出来：`Gateway.httpProviders` 按工具行 ID 保存 slots 和池引用数；退休池在所有 lease（包括等待名额和未完成握手）完成后只清理一次引用，最后一个池结束时删除 provider。旧配置的清理继续通过指针比较保护新配置 map 条目。没有永久保存历史 provider ID，也没有改变每副本每 provider 32 的边界。

同一命令 GREEN（0.188s），绿灯下增加全部退休/归还后的 provider map 回收断言，并保持直接调用官方 SDK 的真实握手。`go test -race ./internal/gateway -run 'TestRepeatedRetirement|TestRetiredHTTPPool|TestHTTPPoolBounds|TestClose' -count=3 -timeout=60s` 通过（9.569s）；`go vet ./internal/gateway` 退出 0。本次只新增 Gateway 容量字段、修改池实现和池测试，未改根任务的目录 revision 逻辑。

最终 `go test -race ./internal/gateway -count=1 -timeout=60s` 通过（12.659s）。本子任务代码与记录冻结；全仓 `make check` 以及根任务后续 Redis 集成另由根任务统一验证。

## Redis 共享 HTTP 工具定义与失效

用户后续明确要求接入 Redis。根任务授权网关完整范围，本子任务新增 `Options.Cache *cache.Client`，只缓存 HTTP 上游原始 MCP 工具定义。缓存中没有工具配置、认证信息、余额、权限或 SDK 会话；执行仍租用本机 HTTP session，权限/启用/价格/余额/限频继续以共享 PostgreSQL 为权威。stdio 发现依赖本机命令 allowlist，始终直接发现，不共享 metadata。

共享 key 包含缓存格式版本、数据库 catalog revision、工具行 ID 与加密配置 hash、加密密钥 hash、AllowPrivate。部署 namespace 必须在同一数据库的副本间一致，不同部署独立。每次请求原有 PostgreSQL revision 查询继续执行，数据库配置更新不受缓存 TTL 或 Pub/Sub 丢消息影响。Redis 与本地目录各自 TTL 为 5 秒；未修改数据库的纯远端元数据变更在最坏情况下可接近 10 秒才呈现，不将两层缓存声称为总共 5 秒。

New 启动一个订阅，使用随机 source 标识实例。Invalidate 先本地刷新再通过已有 cache client 有界发布；订阅忽略自身 source，只刷新本地，避免再次发布形成循环。Close 取消并等待订阅退出，cache client 的所有权留给 application。Redis miss、Get/Set 失败或损坏数据均直接发现；不缓存失败/不完整发现。

所有下列命令在 `server` 下执行，并先加载 `source /tmp/loadout-pg.env` 与 `source /tmp/loadout-redis.env`。每个测试使用唯一 PostgreSQL schema 与随机 Redis namespace，不清空共享 Redis。

| 验收 | RED | GREEN / 边界 |
| --- | --- | --- |
| `go test ./internal/gateway -run TestRedisSharesHTTPMetadataAcrossGateways -count=1` | 两副本进行了 2 次 tools/list，预期共享后仅 1 次 | 0.079s；B 通过共享 metadata 后真实 tools/call 成功；检查实际 Redis 内容为原始工具定义及 PTTL 在 0–5 秒 |
| `go test ./internal/gateway -run TestRedisInvalidationRefreshesPeerAndClosesSubscription -count=1` | 两个 gateway 没有订阅共享失效 channel | 与共享测试一同 GREEN 0.201s；真实 Redis subscriber 数 2→1，peer 目录失效，来源不重复处理；Close 后外部 client 仍可 Ping |
| `TestRedisMetadataRevisionAndLocalSecurityIsolation` | 首次即绿，已实现 key 隔离的边界回归 | DB revision 变化后发现新工具；无私网权限或错误解密密钥的副本无法借已有 metadata 暴露 HTTP 工具 |
| `TestRedisMetadataFailureFallsBackToUpstream` 的 disabled/unavailable/closed/malformed/null-tool | 首次即绿，已有回源行为的边界回归 | 无缓存、真实拒绝连接、已关闭 client、坏 JSON、null 工具都再次请求真实上游，不拖满发现预算 |
| `TestRedisDoesNotShareMachineLocalStdioMetadata` | 首次即绿，既有机器边界回归 | A 用真实官方 SDK stdio 子进程发现成功，B 没有命令 allowlist 不得显示该工具；Redis metadata keys 为 0 |
| `TestRedisMetadataFailureFallsBackToUpstream/invalid` 与 `TestInvalidUpstreamMetadata` | 独立审查发现合法 JSON 的 `type:string` 或非法 `x-mcp-header` 被缓存接收，回源仅 1 次；官方 SDK middleware 返回错误 schema 也进入 serving catalog | 0.361s；以官方 SDK AddTool 为统一校验边界，在局部接住它明确报告校验失败的 panic。缓存错误回源，上游错误只隐藏该上游，保留 builtin |

绿灯下仅整理测试构造，不增接口/依赖/手写协议。Redis 聚焦 race 在 schema 审查前跑三轮通过（7.996s）；审查修复后的最终全包结果另列。根任务已独立完成两个真实 Go 进程的共享发现与 Redis outage 业务回源验证，本记录不替代根任务 process harness 与全仓 make check 结果。

审查缺陷实际聚焦命令：`go test ./internal/gateway -run 'TestRedisMetadataFailureFallsBackToUpstream/invalid|TestInvalidUpstreamMetadata' -count=1`。最终 `go test -race ./internal/gateway -count=1 -timeout=60s` 通过（14.964s），`go vet ./internal/gateway` 退出 0。Redis 网关实现及记录冻结，交根任务完整 harness。
