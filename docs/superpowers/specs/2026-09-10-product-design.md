# 完整产品实现设计

来源：docs/PLAN.md 全文及用户追加要求。架构决策见 ../../adr/0001-product-architecture.md。范围包括 M0–M3 已列产品功能；用户确认外部支付仅保留接入边界，当前交付兑换码与管理员充值。原生 iOS/Android、用户私人 OAuth 代管、M4 之后的供应商分成市场不是本次功能。

## 模块与验收

1. 数据与账本：PostgreSQL 迁移、账号个人钱包、团队钱包、不可变额度流水、原子预占/完成/退款、幂等管理员调账、分页、重启恢复与备份说明。
2. 账号：注册/登录/注销、7 天会话、管理员引导创建、密码安全校验；邮箱验证与 GitHub 登录通过明确配置接入，未配置时页面诚实显示不可用，不假登录。
3. 网关令牌：名称、仅一次显示、哈希持久化、撤销、最后使用、个人/团队钱包范围；对象级隔离与角色保护。
4. MCP：官方 SDK stateless streamable HTTP；time_now/echo；HTTP 上游命名空间、Schema 原样、连接缓存失效、失败重连；stdio 受部署白名单控制。初始化和列表免费；调用成功计费，拒绝/失败留痕；60 次/分钟 token 与每日限额，多实例结果一致。
5. Rust 本地接入：login/logout/status/doctor/version、apply/remove（Codex/Claude/Cursor、幂等、安全冲突保护、备份与原子写入）、bridge；凭证 0600、默认桥接不写 token 到客户端配置。高级 direct 模式显式选择；不自动重试非幂等调用。device-code 授权使用短期一次性请求，明确批准才签发。
6. 用户 Web：注册/登录、真实余额/用量概览、令牌、工具目录、明细与筛选、充值/兑换码、团队/成员、设备授权。所有页面覆盖等待/空/错/成功，401 统一退出，不在 localStorage 保存凭证。
7. 管理 Web/API：用户、启停、调账与备注、工具增改启停和价格、全局明细/聚合、套餐、兑换码、订单、审计导出；管理员权限在服务器执行。
8. 商业边界：套餐由管理员定义，不内置虚构价格；充值/兑换幂等、额度和流水同事务；payment provider 未配置时创建外部支付订单明确返回不可用，不能自签 webhook 或伪造到账。
9. 团队：owner/member、席位限制、一次性邀请、退出/移除、共享钱包、团队 token、按成员明细与 CSV；非成员即时不能花团队余额，最后 owner 不可直接移除。
10. Tauri：真实本地登录、apply/remove、连接/凭证/客户端状态、退出登录与托盘；共享 cli Rust 库，只暴露有限命令；开发源码和可用平台构建验证，签名发行需要外部证书不得伪造。
11. 运维与性能：readyz、指标、请求 ID、安全日志、数据库连接/上游请求有界、迁移、备份恢复文档、Compose、CI；负载与一致性测试真实运行。

## API 约定

保留 PLAN 的 /api/v1 路径，健康接口兼容基建。Web 的 auth/register、auth/login 返回 `{user}` 并设置 session cookie；auth/logout 注销。用户字段 `{id,username,role,balance}`，ID 为整数；余额为整数 credit，不使用浮点金额。列表返回 `{items,next_cursor}`，cursor 为上页最后 ID 的十进制字符串，空字符串表示结束。业务 JSON 错误统一 `{error: stable_code}`，页面翻译安全摘要。

account/me 返回 `{user,summary:{today_calls,month_cost,token_count}}`；account/verify 仅允许 ldt_，返回 `{username,balance,tools}`；令牌创建 `{name,team_id?}` 返回 `{token,item}`，列表绝不包含 token/hash。工具公共元数据不包含 config/headers/env。管理员工具配置中的秘密写入后列表只返回 `configured:true`。

余额调整 `POST admin/users/:id/balance` 接收 `{delta,note,idempotency_key}`；相同用户/幂等键同请求返回原结果，不同金额或备注冲突。兑换 `POST account/redeem` 接收 `{code}`，仅首次可到账；管理创建代码返回一次明文。

团队路径 `/account/teams`、`/account/teams/:id/members`、`/account/teams/:id/invites`、`/account/team-invites/accept`；价格/套餐 `/plans`、管理员 `/admin/plans`；本地授权 `/device/authorize`、`/device/token`、网页 `/account/devices/approve`。每一模块的详细字段和错误随该模块测试与 OpenAPI 同步，不能仅生成空路由作为完成。

## 测试契约

TDD 记录真实 RED/GREEN，Go 包内单元 + 真 PostgreSQL 集成（隔离 schema），Rust 临时 HOME + 官方 MCP 客户端/服务端测试，React Testing Library，Node 真实跨进程集成。不使用 Playwright 或其他浏览器自动化。视觉适配的 DOM 测试不能冒充真实屏幕验证；Tauri 缺系统依赖/签名也必须单独标记。

验收总表持续维护在 product-execution.md，只有有代码与通过证据的条目能打勾。外部邮件/OAuth/支付/发行配置分别标明，而不是将整模块泛称“完成”。
