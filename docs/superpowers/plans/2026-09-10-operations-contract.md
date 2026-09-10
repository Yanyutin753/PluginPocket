# Task 4 API 契约

所有路径前缀 `/api/v1`。网页 cookie 会话写请求要求同源；管理员路由由服务器验证角色。列表统一 `{items,next_cursor}`，默认 50、上限 100。整数额度绝对值最多 1e12。错误统一 `{error:stable_code}`。

## 管理与工具

- `PATCH /admin/users/:id` `{enabled:boolean}` → `{user}`。不能停用自己或最后一个启用管理员。
- `GET /tools`（已登录）→ 公开工具列表，字段 `{id,key,name,description,kind,enabled,units_per_call,input_schema}`。
- `GET /admin/tools` → 同上加 `configured:boolean`；永不返回秘密。
- `POST /admin/tools` / `PATCH /admin/tools/:id` `{key,name,description,kind,enabled,units_per_call,input_schema,config?}` → `{item}`。builtin 仅允许 echo/time_now；HTTP/stdio 由网关校验，加密后持久化。PATCH 无 config 保留原配置。
- `GET /admin/usage` 与 `/admin/usage/export`：全局游标页（CSV 也分页），可选 `user_id`、`tool`、`status`。CSV 单元格防公式执行。用户 `/account/usage` 同样支持 tool/status。

## 套餐与兑换

- `GET /plans` 返回启用套餐；`GET /admin/plans` 管理列表。
- `POST /admin/plans` / `PATCH /admin/plans/:id` `{name,credits,price_cents,currency,enabled}` → `{item}`。字段 `{id,name,credits,price_cents,currency,enabled,created_at}`。
- `POST /admin/redemption-codes` `{credits,note}` → `{code,item}`，明文仅此一次；`GET /admin/redemption-codes` 只返回 `{id,credits,note,created_at,redeemed_at}`。
- `POST /account/redeem` `{code}` → `{balance,credits}`；已兑换返回409 `already_redeemed`，无效返回404 `not_found`。
- `GET /account/orders` 返回真实订单列表；`POST /account/orders` `{plan_id}` 外部支付未配置返回503 `payment_unavailable`，不建虚假订单。

## 团队

- `GET /account/teams` / `POST /account/teams` `{name}` → `{item}`。团队字段 `{id,name,role,balance,seat_limit}`，初始席位5。
- `GET /account/teams/:id/members` → `{id,user_id,username,role}`。
- `DELETE /account/teams/:id/members/:user_id`：owner 移除成员，或成员移除自己；最后owner不能退出。
- `POST /account/teams/:id/invites` owner → `{code,expires_at}`；`POST /account/team-invites/accept` `{code}` → `{item}`。邀请一次性、24小时有效、席位在事务中检查。
- `POST /account/teams/:id/fund` `{credits,idempotency_key}` owner 从个人钱包转到团队，返回 `{item}`；两边账本同事务，重放不会重复转移。
- `POST /account/tokens` 支持 `{name,team_id?}`，仅成员可创建；移除成员即失去团队钱包花费资格。
- `GET /account/teams/:id/usage` / `/usage/export` 团队内用量与CSV，支持 member `user_id` 过滤。

## 设备授权

- `POST /device/authorize` `{}`（公开）→ `{device_code,user_code,verification_uri,expires_in:600,interval:5}`。
- `POST /device/token` `{device_code}`（公开）→ 批准后 `{token}` 仅一次；pending=400 `authorization_pending`，过快=429 `slow_down`，过期=400 `expired_token`，已领取=400 `invalid_grant`。
- `POST /account/devices/approve` `{user_code}`：登录用户明确批准，绑定本人。

未配置外部身份/支付能力不伪造成功。完整 `make check` 由主代理在整合后执行。

## 补充查询与管理

- `GET /account/ledger`：个人钱包真实流水 `{id,delta,kind,note,created_at,balance_after:null|integer}`，标准游标分页。
- `GET /account/teams/:id` → `{item}`，必须现成员。
- `PATCH /account/teams/:id` owner `{name?,seat_limit?,owner_user_id?}`；seat_limit为1–1000且不能小于现有成员数；owner目标必须启用的现成员。
- `GET /admin/usage/summary?days=1|7` → `{items:[{tool,calls,cost,errors}],next_cursor}`。
- `GET /account/teams/:id/usage/summary?days=1|7` → `{items:[{user_id,username,calls,cost,errors}],next_cursor}`。
- 报表days默认7，UTC当日零点起覆盖1/7个自然日；errors包含error/denied/recovered。报表同样分页。
- CSV响应头 `X-Next-Cursor` 指示下一页；每页最多100条，避免全量载入内存。
- 设备批准成功204，无JSON；授权验证地址为配置公开Origin下`/devices`。
- orders字段 `{id,plan_id,credits,price_cents,currency,status,created_at}`。
- auth密码并发最多4，满载429 `auth_busy`；登录/注册/设备敏感入口共享每进程120次/分钟保护上限，超限429 `rate_limited`。

## 账本与初始赠额

- `GET /admin/ledger?user_id=&actor_id=&kind=`：管理员真实审计，字段 `{id,user_id,actor_id,wallet_id,delta,kind,note,created_at,balance_after}`，稳定ID分页。
- `/account/ledger?kind=` 可筛选个人流水；kind可为registration/adjustment/redemption/team_transfer/reservation/refund/recovery。
- 新注册默认赠1000 credits，与registration账本同事务。部署 `InitialCredits` 显式0可关闭；已有用户不自动补发或重置。
- 所有新账本写入记录事务内真实balance_after；JSON保留nullable以兼容历史无该值的记录。
