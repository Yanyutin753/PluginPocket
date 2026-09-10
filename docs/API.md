# HTTP 与 MCP 契约

机器可读定义：[OpenAPI 3.1](openapi.json)。本文件与 `server/internal/app`、`server/internal/identity`、`server/internal/gateway`、`server/cmd/loadout-server/application.go` 的实际处理器同步；不把未配置支付或身份服务写成可用功能。

## 通用规则

- 业务 REST 前缀为 `/api/v1`，网页同源访问；请求和响应使用 JSON。业务处理器错误为 `{ "error": "stable_code" }`，不返回原始上游内容或秘密。
- 网页使用 `loadout_session` Cookie：opaque 随机会话，服务端只保存哈希，7 天有效；`HttpOnly`、`SameSite=Lax`、`Path=/`，生产开启 `Secure`。退出会即时删除对应会话。启用状态与角色在服务端重新验证。
- REST 写请求必须提供匹配部署公开源的 `Origin`；浏览器同源请求自动携带。只有公开的 `/device/authorize`、`/device/token` 不要求 Origin。邮箱验证仍要求同源 Origin，即使不要求登录。
- CLI/MCP 使用 `Authorization: Bearer ldt_…`，与网页 Cookie 分离；`/account/verify` 和 `/mcp` 不接受网页会话替代 Bearer。
- 普通业务 JSON body 上限 16 KiB，身份模块上限 4 KiB；未知字段、多个 JSON 值、非法 body 返回 `400 invalid_request`。工具/套餐 PATCH 是完整定义更新；只有团队 PATCH 是按字段更新。
- 金额为整数 credit；价格字段 `price_cents` 为整数分，不使用浮点金额。单次额度调整、兑换、转入和工具成本上限为 `1e12`。Go 的字符串长度校验按 UTF-8 **字节**计算；OpenAPI 的 `maxLength` 为客户端提示，非字节长度证明。
- 时间为带时区的 ISO/RFC3339 时间字符串。概览今日/月度与汇总日期边界使用 UTC。
- 列表返回 `{ "items": [], "next_cursor": "" }`，没有 `null` 列表。默认 `limit=50`，范围 `1..100`；`cursor` 是上页最后 ID 的十进制字符串，省略/空字符串从头开始；ID 降序，空 `next_cursor` 表示结束。
- 业务数据使用 `Cache-Control: no-store`。生产总入口提供 `X-Request-ID`；客户端报错只显示安全摘要。未知 API 路径/错误 method 的路由级错误与 MCP HTTP 错误可能为纯文本，不能对每个 HTTP 错误盲目执行 JSON 解码；它们绝不能被当作 SPA 首页或模拟成功。

## 健康、能力与运维

| 方法与路径 | 响应 |
|---|---|
| `GET /healthz`、`GET /api/v1/health` | `200 {status:"ok",service:"loadout",version}` |
| `HEAD /healthz`、`HEAD /api/v1/health` | 相同状态，无 body |
| `GET /readyz` | 数据库可用：`200 {status:"ready",redis:"ready"\|"degraded"\|"disabled"}`；不可用：`503 {status:"unavailable"}`；未配置数据库：`503 {error:"database_unconfigured"}` |
| `GET /metrics` | Prometheus/OpenMetrics 文本；导出超时或并发过多可返回 503 |
| `GET /api/v1/meta` | `{github:boolean,email:boolean,payments:false}`；真实配置能力，不是虚构成功 |

`version` 必须为非空、无控制字符字符串。健康检查只证明 HTTP 服务运行；就绪检查才检查 PostgreSQL。未配置数据库的基础模式只提供原健康/静态资源处理器，不具备产品 API。指标端点本身不要求 Cookie，部署层应按运维需要限制访问范围。

## 账号与会话

以下表格中的路径省略 `/api/v1`。

| 方法与路径 | 请求 | 成功响应 |
|---|---|---|
| `POST /auth/register` | `{username,password}` | `201 {user}` + session Cookie |
| `POST /auth/login` | `{username,password}` | `200 {user}` + session Cookie |
| `POST /auth/logout` | 不要求 body | `204`，清除 Cookie/对应会话；未登录也可调用 |
| `GET /account/me` | Cookie | `{user,summary:{today_calls,month_cost,token_count}}` |
| `GET /account/verify` | ldt_ Bearer | `{username,balance,tools:["echo",...]}` |

`user` 为 `{id,username,role:"user"|"admin",balance,enabled}`。用户名为 3–32 个 ASCII 字母、数字、下划线、横线；注册密码 12–1024 字节。注册赠额来自部署配置，默认 1000，显式配置 0 可关闭；UI 必须读响应余额，不能写死赠额。账号、钱包、赠额账本和会话同事务创建。

登录失败为 `401 invalid_credentials`；重名 `409 username_taken`；密码计算饱和 `429 auth_busy`；入口频率限制为 `429 rate_limited`。过期/无效网页会话返回 `401 unauthorized`，客户端清理受保护缓存并重新登录。

`account/verify.tools` 是启用预设池的 key；远程上游的完整命名空间工具应通过 MCP `tools/list` 获取。

## 网关令牌

| 方法与路径 | 请求 | 成功响应 |
|---|---|---|
| `GET /account/tokens` | `limit,cursor` | `{items:[Token],next_cursor}` |
| `POST /account/tokens` | `{name,team_id?}` | `201 {token,item:Token}` |
| `DELETE /account/tokens/:id` | 无 body | `204` |

`Token` 为 `{id,wallet_id,name,prefix,created_at,revoked_at,last_used_at}`，后两项可为 `null`。名称 1–80 字节。省略或 `team_id=0` 使用个人钱包，正整数要求当前团队成员。列表不会返回明文或哈希；创建的 `ldt_` 明文只出现一次，网页离页/隐藏后清除。撤销只允许令牌创建者，重复撤销保持原撤销时间；越权或不存在返回 404。

## 工具目录与管理

| 方法与路径 | 请求 | 成功响应 |
|---|---|---|
| `GET /tools` | 登录；`limit,cursor` | 公共工具元数据页 |
| `GET /admin/tools` | 管理员；`limit,cursor` | 管理元数据页，增加 `configured` |
| `POST /admin/tools` | 完整工具定义 | `201 {item}` |
| `PATCH /admin/tools/:id` | 完整工具定义 | `200 {item}` |

公共字段：`{id,key,name,description,kind,enabled,units_per_call,input_schema}`。`kind` 为 `builtin/http/stdio`；不返回 `config`、`headers`、`env` 或秘密。

工具写请求只允许 `{key,name,description,kind,enabled,units_per_call,input_schema,config?}`；不得回传列表里的 `id/configured`。key 遵循用户名字符规则，name 1–80 字节，description 最多 2000 字节，`input_schema` 必须是 `type:"object"` 的 JSON Schema。`units_per_call` 为非负整数 credit，不是浮点价格。

- `builtin` 只支持 `echo`、`time_now`；不提供任意命令执行能力。
- `http` 配置 `{url,headers?}`，默认只允许公开 HTTPS 地址；显式开发配置才允许私网，连接时重新检查目标且禁止重定向。
- `stdio` 配置 `{command,args?,env?}`，command 为服务器部署白名单的别名；默认不可启动任意本地进程。
- PATCH 省略 config 保留原配置；更换连接类型可能需要新 config。秘密写入后加密存储，列表只返回配置状态。成功更新使网关目录缓存失效。

## 用量、汇总与 CSV

| 方法与路径 | 范围与筛选 |
|---|---|
| `GET /account/usage` | 当前用户；`limit,cursor,tool,status` |
| `GET /admin/usage` | 全局，管理员；增加 `user_id` |
| `GET /account/teams/:id/usage` | 团队钱包，当前成员；增加 `user_id` |
| `GET /admin/usage/export` | 全局 CSV，与明细相同筛选和游标 |
| `GET /account/teams/:id/usage/export` | 团队 CSV，与明细相同筛选和游标 |
| `GET /admin/usage/summary?days=1或7` | `{items:[{tool,calls,cost,errors}],next_cursor}`，支持 `limit/cursor` |
| `GET /account/teams/:id/usage/summary?days=1或7` | `{items:[{user_id,username,calls,cost,errors}],next_cursor}`，支持 `limit/cursor` |

明细项：`{id,user_id,token_id,wallet_id,tool,cost,status,duration_ms,created_at}`。status 为 `pending/ok/error/denied/recovered`；成功使用 `ok`，失败使用 `error`，不是 `success/failed`。`tool` 为精确匹配，最长 200 字节。

汇总默认最近 7 个 UTC 日期，含今天；`days=1` 表示 UTC 今天。汇总按分组分页，支持 `days,limit,cursor`（默认50组，最大100组），`next_cursor` 为空时结束；不继承明细的用户/工具筛选。控制台可加载后续分组，失败重试保留已显示结果。

CSV 响应为 `text/csv; charset=utf-8`，`Content-Disposition: attachment; filename="loadout-usage.csv"`。每个响应仅一页；从 `X-Next-Cursor` 继续，空字符串结束。列为 `id,user_id,tool,cost,status,duration_ms,created_at`。字符串单元格进行公式转义；客户端不能把单页文件宣称为全部用量导出。

## 用户管理、余额和账本

| 方法与路径 | 请求/筛选 | 成功响应 |
|---|---|---|
| `GET /admin/users` | `limit,cursor` | `{items:[User],next_cursor}` |
| `PATCH /admin/users/:id` | `{enabled:boolean}` | `{user}` |
| `POST /admin/users/:id/balance` | `{delta,note,idempotency_key}` | `{user}` |
| `GET /account/ledger` | `limit,cursor,kind` | 当前个人钱包流水页 |
| `GET /admin/ledger` | 管理员；`limit,cursor,kind,user_id,actor_id` | 全局账本页 |

禁止停用自己或最后一位启用管理员（`cannot_disable_self/last_admin`）。delta 为非零整数，绝对值最多 `1e12`；note 1–500 字节；幂等键 1–100 字节。相同用户/幂等键、相同金额和备注返回原结果，不重复调账；不同内容返回 `409 idempotency_conflict`，扣为负数返回 `409 insufficient_balance`。

个人流水项：`{id,delta,kind,note,created_at,balance_after:null|integer}`；管理员增加 `{user_id,actor_id:null|integer,wallet_id}`。kind 可筛选 `registration/adjustment/redemption/team_transfer/reservation/refund/recovery`。账本不允许修改或删除，钱包变化与账本在同一事务完成。订单与额度流水是两类真实记录；不能以虚构支付订单代替兑换或人工调账。

## 套餐、兑换码与订单

| 方法与路径 | 请求 | 成功响应 |
|---|---|---|
| `GET /plans` | 公开，分页 | 启用套餐页 |
| `GET /admin/plans` | 管理员，分页 | 全部套餐页 |
| `POST /admin/plans` | 完整套餐定义 | `201 {item}` |
| `PATCH /admin/plans/:id` | 完整套餐定义 | `200 {item}` |
| `GET /admin/redemption-codes` | 管理员，分页 | 兑换码元数据页 |
| `POST /admin/redemption-codes` | `{credits,note}` | `201 {code,item}`，只返回一次明文 |
| `POST /account/redeem` | `{code}` | `{balance,credits}` |
| `GET /account/orders` | 当前用户，分页 | 真实订单页 |
| `POST /account/orders` | `{plan_id}` | 当前合法套餐也返回 `503 payment_unavailable`，没有成功订单 |

套餐写请求 `{name,credits,price_cents,currency,enabled}`；响应项增加 `id,created_at`。name 1–80 字节，credits 为正整数，price_cents 可为 0，currency 为三个大写字母。PATCH 不接收 id/created_at。

兑换码元数据为 `{id,credits,note,created_at,redeemed_at}`；`redeemed_at` 可为 null。code 明文以 `ldr_` 开头，仅创建响应返回；无效兑换码 404、已用兑换码 `409 already_redeemed`。兑换与入账同事务。

订单项为 `{id,plan_id,credits,price_cents,currency,status,created_at}`。外部支付尚未配置，因此购买不会创建占位订单、伪造支付成功或余额；已有真实订单读取按数据库记录返回。没有支付 webhook 或伪造到账端点。

## 团队、席位和共享钱包

| 方法与路径 | 请求 | 成功响应 |
|---|---|---|
| `GET /account/teams` | 分页 | 我的团队页 |
| `POST /account/teams` | `{name}` | `201 {item}` |
| `GET /account/teams/:id` | 当前成员 | `{item}` |
| `PATCH /account/teams/:id` | owner；`{name?,seat_limit?,owner_user_id?}` | `{item}` |
| `GET /account/teams/:id/members` | 当前成员，分页 | 成员页 |
| `DELETE /account/teams/:id/members/:user_id` | owner 移除，或成员退出自己 | `204` |
| `POST /account/teams/:id/invites` | owner，无需 body | `201 {code,expires_at}` |
| `POST /account/team-invites/accept` | `{code}` | `{item}` |
| `POST /account/teams/:id/fund` | owner；`{credits,idempotency_key}` | `{item}` |

团队项：`{id,name,role:"owner"|"member",balance,seat_limit}`；新团队 5 席位、余额 0，创建者是 owner。成员项：`{id,user_id,username,role}`。

席位为 `1..1000`，不可少于现有成员；所有权只能交接给现成员，交接者成为 member；最后 owner 不可退出。邀请单次使用、24 小时有效，事务中校验席位；失败包括 `invite_used/invite_expired/already_member/team_full`。

fund 从当前 owner 个人钱包转入团队；credits 正整数且最多 `1e12`，幂等键 1–100 字节。双钱包与双边流水同事务；同一操作重试不重复划转。离开/被移除的成员立即失去团队钱包花费资格，不能依靠尚未撤销的旧团队令牌继续使用。

## 本地设备授权

1. CLI/桌面调用 `POST /device/authorize`，body `{}`，无需 Cookie/Origin，返回 `{device_code,user_code,verification_uri,expires_in:600,interval:5}`。
2. 用户打开返回的 `verification_uri`（当前为站点 `/devices`），输入 **10 字符** user_code 并明确批准。网页也支持 `/device` 别名；授权码大小写不敏感。
3. 登录网页调用 `POST /account/devices/approve`，body `{user_code}`，成功 `204`。无效、过期或已批准：`409 invalid_device_code`。
4. 设备调用 `POST /device/token`，body `{device_code}`；批准后首次返回 `{token:"ldt_…"}`。未批准 `400 authorization_pending`；轮询过快 `429 slow_down` 并带 `Retry-After:5`；过期 `400 expired_token`；已消费或无效 `400 invalid_grant`。

客户端只按服务端 interval 轮询授权，不自动批准；领取的网关令牌只出现一次。网页会话与设备的长期网关令牌不共用同一个秘密。

## GitHub 与邮箱验证

| 方法与路径 | 行为 |
|---|---|
| `GET /auth/github/start` | 配置后 302 到官方授权页，使用 PKCE 与 10 分钟 state Cookie |
| `GET /auth/github/callback?state=&code=` | 校验 state、读取真实 GitHub 用户，可限制组织；创建会话并 302 回首页 |
| `GET /account/email` | `{email:null|string,verified_at:null|ISO,configured:boolean}` |
| `POST /account/email/request` | `{email}` → `202 {status:"sent"}`，仅实际邮件发送成功后返回 |
| `POST /auth/email/verify` | `{token}` → `{status:"verified"}`，一次性；不要求登录但校验 Origin |

meta.github 为 true 才显示 GitHub 登录入口。未配置返回 `503 github_unavailable`，失败包括 `invalid_state/github_failed/organization_required`。不会把 GitHub access token 暴露给网页或当作网关令牌。

邮件链接指向 `/verify-email?token=…`；网页由用户点击确认后才提交，成功后清理 URL token。未配置/发送失败 `503 email_unavailable`，频率限制 `429 rate_limited`，无效/过期链接 `400 invalid_token`；邮箱冲突也可能 `409 email_unavailable`。不得在页面假装已经发送或验证。

## MCP 协议端点

`POST /mcp` 由官方 MCP Go SDK 处理 JSON-RPC 2.0，配置为 stateless、JSON response、传播请求取消，请求 body 上限 1 MiB。客户端使用 SDK 协商协议版本并携带 `MCP-Protocol-Version`，Accept 声明 `application/json, text/event-stream`。当前部署仅 POST；已鉴权的 GET/DELETE 等返回 405，未鉴权返回 401，不支持持久 HTTP session 的恢复语义。

标准流程为 `initialize` → `notifications/initialized` → `tools/list` / `tools/call`。OpenAPI 仅描述传输边界，不重新发明 SDK 的完整消息协议。内置工具为 `echo/time_now`；远程 HTTP/stdio 工具带预设池命名空间。初始化和列表不收费，调用经过鉴权、限频、预占和结算：成功保留扣费，失败退款，异常待结算由恢复任务处理；查询实际账本确认结果。工具业务错误可通过 `result.isError=true` 返回，不等同于 HTTP 非 2xx。

网关鉴权/方法/上游目录不可用等 HTTP 错误可能为纯文本；客户端不得打印原始上游 body、令牌或包含凭证的 URL。网络失败后不得自动重放非幂等工具调用；`loadout bridge` 的 stdout 专用于协议，诊断写 stderr。

## 文档校验

OpenAPI 覆盖实际 app/identity 注册的 51 个方法/路径，并另列健康、就绪、指标与 MCP，共 60 个 operation。对应字段、错误和鉴权规则以 `openapi.json` 为机器可读入口；接口增改应同时更新此文档与 Go/React/CLI 的行为测试。本文覆盖契约，不把静态文档校验当作真实数据库或多端端到端测试证明。

### 认证入口预算与并发失效

公开认证按功能族分别限流；每个直接连接地址每族120次/分钟，每族部署共享上限1200次/分钟，由PostgreSQL固定窗口保证跨副本一致。错误HTTP方法不消耗预算，设备轮询不会消耗密码登录或注册族预算。调用方取实际RemoteAddr，不信任任意X-Forwarded-For；NAT或反代后的连接可能共享地址预算，部署层应同时限流。

资金及管理写入在锁等待后使用数据库当前时间重新验证会话、账号和所需角色；设备令牌签发时检查是否已过期。邮箱验证重发在短事务中发送邮件，SMTP失败回滚并保留此前仍有效的链接；并发重发和验证以数据库行锁串行。会话查询的暂时数据库故障返回可重试5xx，不伪装成未登录。

## 管理员系统配置

- `GET /admin/settings`：有效管理员Cookie；返回 `{item,secret_writes_available}`。item包含 revision、initial_credits、github_enabled、github_client_id、github_org、github_client_secret_set、smtp_enabled、smtp_address、smtp_from、smtp_username、smtp_password_set。不返回任何密钥原文。
- `PATCH /admin/settings`：管理员Cookie和同源Origin；提交 revision 与上述所有公开可编辑字段（不提交 *_set），可选 github_client_secret/smtp_password。secret省略保留，空串清除；开启集成须有完整配置。返回同GET结构。
- 保存使用乐观版本控制，另一管理员已更新时409 `settings_conflict`；格式错误400 `invalid_request`；加密不可用503 `settings_encryption_unavailable`；未配置运行时管理器503 `settings_unavailable`。其他存储失败返回5xx，无伪成功或旧值回退。
- 环境变量为首次保存之前的默认值，保存后PostgreSQL为准。新HTTP请求读取当前快照；已执行请求按开始时快照结束。更新管理员与时间记录在单行配置中。
- 注册赠送额度只影响之后创建的密码/GitHub账号，不更改已有钱包；GitHub/SMTP开关及参数影响后续授权/发信和meta能力。更换OAuth配置时已有授权流程可能需要重新发起。

### 工具名称兼容性

远程工具公开名为 `escaped_key__remote_name`；provider key中每个 `_` 编码为 `_u`，远程原名不变。无下划线provider保留旧名称；含下划线provider需客户端刷新tools/list。例如 `foo/bar__baz` 为 `foo__bar__baz`，`foo__bar/baz` 为 `foo_u_ubar__baz`，不会相互覆盖。目录按ID有界分批读取所有启用上游，每批128行、最多8路并发，整体超时不发布截断目录。
