# 本地演示数据与功能验证

演示数据走真实 REST API 和 MCP 网关，账本、余额、权限与错误状态使用正常后端逻辑。本地 mock 只替代 MCP 上游工具结果。不会模拟支付成功。

**按用户最新要求，数据已真实写入 http://127.0.0.1:5173 当前使用的数据库（根 `.env` 的 `PLUGINPOCKET_DATABASE_URL`）。** 使用原有管理员账号登录即可，不需要切换到另一个实例，也没有更改管理员密码。

5173 数据库实查：61 个账号、234 个令牌、702 条调用、984 条账本、12 个团队、12 个套餐、60 张兑换码。531 条调用成功、168 条错误退款、3 条额度不足拒绝；所有钱包余额与账本汇总一致。

最初用于隔离验证的 `5174` / API `8788` / mock MCP `8790` 仍单独存在；配置为 `.pluginpocket/demo.env`，schema 为 `pluginpocket_demo_20260911`。该环境多 2 条远程 mock 调用。当前 5173 配置禁止私网上游，因此数据生成使用正常内置 MCP 工具；没有更改该安全配置。

| 登录账号 | 用途 |
|---|---|
| 原有管理员账号 | **5173** 管理后台；自己的余额、60 个令牌、团队与调用记录；原密码不变 |
| `demo_admin` | 仅在最初的 **5174** 隔离环境中存在 |
| `demo_owner` | 个人用量、兑换记录、团队所有者 |
| `demo_member` | 多团队成员与共享钱包调用 |
| `demo_empty` | 零余额和拒绝调用 |
| `demo_disabled` | 已停用，验证登录拒绝 |
| `demo_user_01` 至 `demo_user_56` | 长列表、调用用量和令牌分页；编号为 10 的倍数的账号已停用 |

新建 `demo_*` 演示账号的密码为 `PluginPocket-demo-only-2026!`；5173 原有管理员继续使用原密码。

完整数据包括 60 个普通用户、1 个管理员、至少 700 条调用、12 个团队、12 个套餐、60 张兑换码。成功、退款错误、余额不足、已使用/未使用兑换码、启用/停用套餐、有效/吊销令牌都可见。每个钱包余额等于自身账本 delta 之和。订单列表保持空状态，因为支付尚未接入；设备授权走真实批准与兑换流程。

## 运行与复用

以下命令从仓库根目录运行，使用项目锁定的 Node 26 与 Go 工具链。

```sh
# 当前独立演示环境的开发服务管理（不要省略 env-file）
node --env-file=.pluginpocket/demo.env scripts/dev.mjs status
node --env-file=.pluginpocket/demo.env scripts/dev.mjs up
node --env-file=.pluginpocket/demo.env scripts/dev.mjs down

# 用户当前 5173 开发实例仍按原方式管理
make status
make up

# 构建并在前台运行本地 MCP fixture；Ctrl+C 停止
make build-demo
./build/pluginpocket-demo -listen 127.0.0.1:8790
```

在另一个**空的、本地演示环境**复用数据生成器：先准备独立 PostgreSQL 数据库/schema 和 Redis namespace，按正常部署配置启动 PluginPocket。允许本地上游只用于演示环境，并配置独立加密密钥；管理员用户名和密码通过环境变量传入。然后执行：

```sh
# -origin 必须匹配服务的 PLUGINPOCKET_PUBLIC_URL；指向本地 Web 或 API 入口
node --env-file=.pluginpocket/demo.env -e 'require("node:child_process").execFileSync("./build/pluginpocket-demo", ["-origin", "http://127.0.0.1:5174", "-upstream", "http://127.0.0.1:8790"], {stdio:"inherit"})'
```

首次执行生成全部数据，重复执行在用户名冲突处拒绝，不重复加钱。`-more` 仅用于将本次最初的 4 用户小数据集扩充一次；完整数据集无需再执行。初始化中途失败会保留已经成功创建的数据，明确报错；不自动清库、覆盖数据或吞掉冲突。

本次写入 5173 的实际命令为（**已经执行，请勿重复**）：

```sh
node --env-file=.env -e 'require("node:child_process").execFileSync("./build/pluginpocket-demo", ["-origin", "http://127.0.0.1:5173"], {stdio:"inherit"})'
```

种子产生的令牌明文不写入日志。可在令牌页面创建自己的演示令牌，调用 `echo`、`time_now`。5174 额外可调用 `demo__success` 和 `demo__failure`，要求 mock MCP 进程仍运行。

## 验证范围

`make check` 重新验证真实 PostgreSQL/Redis、Go race、React/桌面组件、Rust CLI/桌面 bridge、生产构建、开发与生产 HTTP 旅程、进程生命周期和故障恢复。

新增 `TestProductJourneyMCPHotReload`：两个 Go 副本和同一个 Rust bridge 持续运行，管理员新增/替换上游 URL 与 schema、改价、禁用、重新启用；验证另一副本和 bridge 的下一次 `tools/list`、实际调用、在途旧调用完成及准确扣费。此测试不证明客户端会自动刷新自身 UI 工具缓存，也不声称实现了 `notifications/tools/list_changed` 推送。

已有测试涵盖 allowlist stdio、HTTP 上游失败退款、并发会话隔离、Redis 故障与跨副本失效、OAuth/SMTP 协议 fixture。真实 GitHub/SMTP 供应商、浏览器桌面/移动视觉、原生桌面窗口/托盘和其他操作系统仍需对应环境验证。没有使用浏览器自动化。

实际 RED/GREEN 和完整检查结果见 [本次执行记录](superpowers/plans/2026-09-11-demo-validation.md)。
