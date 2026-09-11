# Loadout —— 产品与技术总体方案

2026-09-11 桌面分发扩展：Windows x64（MSI/NSIS）、macOS ARM64/Intel（DMG/app）、Linux x64/ARM64（deb/rpm/AppImage），使用独立 Loadout 发布签名密钥，macOS ad-hoc 封印；不含手机端，不等同于平台受信任证书公证。实施及真实验收状态见 [桌面发布记录](superpowers/plans/2026-09-11-desktop-release.md)。

> **一句话定位**：登录即武装的 MCP 订阅网关 —— 用户在网页端注册/充值，本地一条命令把整套预设好的 MCP 工具写进 Codex / Claude Code / Cursor，开箱即用；服务端统一鉴权、计量、扣额度，后台看得见每一次调用。
>
> 名字来源：游戏术语 *Loadout*（预设装备 / 出战配置）—— 角色进场前配好的整套武器和配件，一键上身直接开战。

| 项 | 值 |
|---|---|
| 文档版本 | v0.4（产品实现与本机验收完成） |
| 日期 | 2026-09-10 |
| 状态 | 本轮产品功能与本机harness完成；外部/平台边界见验收账本 |
| 仓库规划 | monorepo：`server` + `cli` + `web` + `desktop` |
| 开源策略 | open-core（核心网关与 CLI 开源，运营侧保留） |


2026-09-11 增加管理员系统配置热更新：注册赠送额度、GitHub/SMTP运行时配置使用PostgreSQL版本化保存，设计见 [系统配置](superpowers/specs/2026-09-11-runtime-settings.md)。

2026-09-11 增加独立本地演示数据生成器，覆盖普通用户与管理员自己的数据；沿用真实 REST、MCP 和账本，仅模拟外部工具结果。规模与运行方式见 [演示说明](DEMO.md)，双副本/存活 bridge 的 MCP 配置热重载验证见 [本轮记录](superpowers/plans/2026-09-11-demo-validation.md)。

当前实施以 [完整产品设计](superpowers/specs/2026-09-10-product-design.md)、[架构 ADR](adr/0001-product-architecture.md)、[实施计划](superpowers/plans/2026-09-10-product.md) 与 [验收账本](superpowers/plans/2026-09-10-product-execution.md) 为准。用户确认响应式 Web + Tauri 桌面；充值先做兑换码/管理员调账，外部支付保留接口。当前数据模型、Cookie会话与预占账本已同步；详细REST字段以 [API](API.md) / [OpenAPI](openapi.json) 和可执行迁移为准。

---

本轮界面工作：用户选择 AI 装备工坊视觉全面重设计（[实施记录](superpowers/plans/2026-09-10-workshop-ui.md)），保留中英文与浅色/深色/跟随系统。实施与验证见 [界面重设计](superpowers/plans/2026-09-10-interface-redesign.md)。

## 目录

1. [项目定位](#1-项目定位)
2. [市场背景与机会判断](#2-市场背景与机会判断)
3. [目标用户与核心场景](#3-目标用户与核心场景)
4. [产品功能总览](#4-产品功能总览)
5. [总体架构](#5-总体架构)
6. [核心流程](#6-核心流程)
7. [数据模型设计](#7-数据模型设计)
8. [API 设计](#8-api-设计)
9. [MCP 网关详细设计](#9-mcp-网关详细设计)
10. [本地端 CLI 详细设计](#10-本地端-cli-详细设计)
11. [网页控制台设计](#11-网页控制台设计)
12. [计量计费与商业化](#12-计量计费与商业化)
13. [安全设计](#13-安全设计)
14. [技术选型与理由](#14-技术选型与理由)
15. [代码仓库结构](#15-代码仓库结构)
16. [里程碑与验收标准](#16-里程碑与验收标准)
17. [风险与对策](#17-风险与对策)
18. [竞品对照](#18-竞品对照)
19. [命名与备选](#19-命名与备选)
20. [术语表](#20-术语表)

---

## 1. 项目定位

### 1.1 解决什么问题

一个想用 MCP 的普通开发者（尤其是 Codex / Claude Code / Cursor 用户）今天要经历：

1. 到处找有哪些好用的 MCP server（目录站质量参差）；
2. 自己申请一堆第三方 API key（搜索、浏览、地图、数据库…）；
3. 手改各客户端配置文件（`config.toml` / `.claude.json` / `mcp.json`），格式各异、坑多；
4. 换一台机器全部重来；团队里每个人各配各的，没人管得住调用量和成本。

**Loadout 把这四步压缩成两步**：网页注册 → 本地执行 `loadout login && loadout apply`。之后所有工具经统一网关调用，额度、用量、计费全部可视。

### 1.2 产品本质

- 对用户：**MCP 的“预配置订阅”**——买的不是技术，是“登录完就能用的一整套工具”。
- 对架构：**MCP 额度网关（MCP metered gateway）**——参考 one-api / new-api 在 LLM API 领域已验证的商业模式，平移到 MCP 工具调用。
- 对生态：只托管**自有预设池**的上游凭证（自己的 key），不碰用户私人第三方账号的 OAuth token（与 Composio 的核心差异，规避最大安全责任）。

### 1.3 边界（明确不做什么）

- ❌ 不代管用户的 Gmail / GitHub 等私人账号授权（M3 前不考虑，见风险一节）；
- ❌ 不做通用 MCP 目录站（Smithery/Glama 已占位，无意义）；
- ❌ 不做企业级重型平台（HiMarket 已占位：Java + K8s 全家桶）；
- ✅ 只做：轻量、开箱即用、按额度计费的 MCP 套餐网关 + 一键配置。

---

## 2. 市场背景与机会判断

> 详细调研过程见对话记录，此处只留结论。

### 2.1 事实

| 事实 | 来源 |
|---|---|
| Codex 支持 `~/.codex/config.toml` 中 `[mcp_servers.x]` 配置 stdio 与远程 HTTP（`url` + `bearer_token_env_var`），CLI 与 IDE 插件共享配置 | OpenAI 官方配置文档 |
| Claude Code / Cursor 均支持 HTTP + 自定义 Header 的 MCP，配置文件是**本地**的（`~/.claude.json`、`~/.cursor/mcp.json`） | 官方文档 |
| “账号级 MCP 配置下发”只有 Cursor Team/Enterprise 档提供，且社区反馈同步有 bug；Codex、Claude Code 均无 | Cursor 论坛 |
| Smithery（最大 MCP 托管市场）2026-03 起取消免费托管，付费 $10–20+/月，商业成立 | Smithery 公告 |
| one-api / new-api（LLM API 额度网关）在国内有 2 万+ star 与大量付费中转生态，“额度 + 后台计费”模式已被充分验证 | GitHub |
| “账号 + 服务端预设套餐 + 本地一键配置 + 额度计费”这条完整链路，开源界目前为空白 | 调研结论 |

### 2.2 判断

- 需求真实：配置繁琐是 MCP 采用率的第一道门槛，Reddit/知乎高频抱怨。
- 付费意愿已被验证：Smithery 转付费、new-api 生态、x402 按次付费协议的兴起。
- 空窗期存在：官方（OpenAI/Anthropic）暂无账号级 MCP 套餐服务；Cursor 仅团队档且体验有坑。
- 差异化护城河：预设池选品质量 + 国内本地化（支付、国内 MCP 源、微信/飞书/高德类工具）+ 轻量部署。

---

## 3. 目标用户与核心场景

### Persona A：个人开发者“小白”（主要付费群体）

> 用 Codex / Claude Code 写代码，听说过 MCP 很强，但不想折腾配置和 API key。

场景：注册 → 充 20 块 → `loadout login` → `loadout apply` → 重启 Codex，搜索/浏览/文档工具全部就位。用多少扣多少。

### Persona B：小团队 Tech Lead

> 团队 5-20 人都用 Cursor/Claude Code，想统一工具配置、控制成本、有调用审计。

场景：管理员在后台建团队额度池，成员各自 `loadout login --team-code XXXX`，配置自动一致；后台看每人用量、月度成本报表、超限熔断。

### Persona C：MCP 内容提供者（生态期）

> 做了一个好用的 MCP server，想给别人用并收费。

场景：申请入驻预设池 → 后台定价（每调用 N 额度）→ 按分成结算。（M4 之后）

---

## 4. 产品功能总览

| 模块 | 功能 | 优先级 |
|---|---|---|
| **账号体系** | 用户名密码注册/登录（P0）；邮箱验证（P1）；GitHub OAuth（P1） | P0 |
| **网关令牌** | 创建/吊销网关 token（`ldt_` 前缀，只显示一次）（P0）；token 分设备命名（P1） | P0 |
| **MCP 网关** | 单一 streamable HTTP 端点，聚合预设工具（P0）；builtin 工具（P0）；HTTP 上游（P0）；stdio 上游（P1，默认关闭）；工具启停与倍率（P0 后台 / P1 API） | P0 |
| **计量扣费** | 每次 tools/call 记账（时长/状态/工具/成本）（P0）；余额原子扣减（P0）；调用限频（P1） | P0 |
| **本地 CLI** | `login` / `apply` / `status` / `doctor` / `logout`（P0）；`bridge` 本地转发进程（P0）；device-code 网页授权登录（P2） | P0 |
| **配置写入** | Codex config.toml（P0）；Claude .claude.json（P0）；Cursor mcp.json（P0）；幂等/标记/移除（P0） | P0 |
| **用户控制台** | 余额、用量明细、token 管理（P0）；充值（P2） | P0 |
| **管理后台** | 用户列表/调余额（P0）；工具池管理（P0）；全局用量（P0）；套餐与兑换码（P2） | P0 |
| **支付** | Stripe/Paddle 海外（P2）；微信/支付宝（P2，视主体资质） | P2 |
| **团队版** | 席位、共享额度、审计导出（P3） | P3 |
| **桌面 App** | Tauri 壳：登录 + 一键 apply + 状态托盘（P3，CLI 先行验证） | P3 |

---

## 5. 总体架构

### 5.1 架构图

```
┌──────────────────────────── 用户侧 ────────────────────────────┐
│                                                                │
│  网页控制台(用户/管理)          本地开发机                       │
│  ┌──────────────┐      ┌─────────────────────────────────┐    │
│  │ login/dashboard│     │ Codex / Claude Code / Cursor     │    │
│  │ admin.html     │     │   └─ stdio ──► loadout bridge    │    │
│  └──────┬───────┘      │                  │(注入token转发)  │    │
│         │ HTTPS        └──────────────────┼────────────────┘    │
└─────────┼─────────────────────────────────┼─────────────────────┘
          ▼                                 ▼ streamable HTTP + Bearer
┌─────────────────────── Loadout 服务端 ──────────────────────────┐
│  ┌─────────┐  ┌──────────────┐  ┌────────────────────────────┐  │
│  │ API 层   │  │ MCP 网关      │  │ 预设上游池                  │  │
│  │ auth/账号 │─►│ /mcp 端点     │─►│ builtin(time/echo…)        │  │
│  │ tokens   │  │ 鉴权→白名单    │  │ http 上游(搜索/浏览/文档…)   │  │
│  │ admin    │  │ 计量→扣额度    │  │ stdio 上游(P1,沙箱内)       │  │
│  └────┬────┘  └──────┬───────┘  └────────────────────────────┘  │
│       ▼              ▼                                           │
│  ┌──────────────────────────────┐                                │
│  │ PostgreSQL 18.6 + pgx pool  │                                │
│  │ users/tokens/tools/usage_logs│                                │
│  └──────────────────────────────┘                                │
└──────────────────────────────────────────────────────────────────┘
```

### 5.2 组件职责

| 组件 | 职责 | 关键约束 |
|---|---|---|
| **API 层** | 注册/登录（网页会话）、token CRUD、用量查询、管理接口 | 网页会话与网关 token 双轨制 |
| **MCP 网关** | 唯一工具入口：鉴权 → 按套餐过滤工具 → 转发调用 → 计量扣费 | 无状态（每请求新建 transport），水平扩展友好 |
| **预设上游池** | 管理员维护的工具集合，三种类型：builtin / http / stdio | 连接懒加载 + 缓存 + 失败重连 |
| **bridge（CLI 内置）** | 本地 stdio MCP server，读取本地凭证，转发到网关 | 客户端配置零密钥；token 轮换不触碰客户端配置 |
| **网页控制台** | 用户端 + 管理端 | P0 起 React + TypeScript + Vite，Go 托管生产资源 |

### 5.3 三个关键设计决策

**决策一：bridge 模式为默认，direct 模式可选。**
所有客户端一律配置一个本地 stdio 命令（`loadout bridge`），由它带着 token 转发到网关。

- 收益 1：客户端配置文件里**不落任何密钥**；
- 收益 2：token 轮换 / 换服务器只需改 `~/.loadout/config.json`，**不动各客户端配置**；
- 收益 3：绕开各客户端远程认证差异（Codex 只支持 `bearer_token_env_var`，体验差）；
- 代价：本地多一跳（实际开销通过后续 bridge 端到端基准测量）。

direct 模式（客户端直连网关 HTTP 端点 + Authorization 头）作为高级选项保留，Claude/Cursor 支持良好，Codex 不推荐。

**决策二：网关无状态（stateless streamable HTTP）。**
每个 `/mcp` 请求独立创建 transport + server 实例，会话不落内存。牺牲一点开销，换来：进程重启不断连、负载均衡随便加、实现简单。MCP 官方 SDK 有 stateless 模式支持。

**决策三：只托管自有预设池凭证。**
网关连上游用的 key 全部是**运营方自己申请的**（搜索、浏览等），用户的私人 OAuth 永不经过服务端。安全责任边界清晰，也避开 Composio 那条重资产路线。

---

## 6. 核心流程

### 6.1 用户首次接入（目标 < 2 分钟）

```
1. 网页 /login 注册（送初始额度，如 1000 credits）
2. 控制台 → 令牌页 → 「创建令牌」→ 复制 ldt_xxxx（只显示这一次）
3. 终端：安装原生 loadout 二进制后执行 loadout login --server https://api.xxx.com
   → 粘贴令牌 → CLI 调 /api/v1/account/verify 校验 → 写 ~/.loadout/config.json (0600)
4. loadout apply            # 自动检测已装的客户端（codex/claude/cursor）
   → 逐个写入 bridge 配置（带标记块，幂等）
5. 重启 Codex/Claude/Cursor → 工具全部出现 → 直接用
```

### 6.2 一次工具调用的完整链路

```
Codex(用户按 F5 调用 time_now)
  └─ stdio ─► loadout bridge（本地，读 ~/.loadout/config.json）
       └─ HTTP+Bearer ldt_xxx ─► 网关 /mcp (tools/call)
            ├─ 1. 鉴权：ldt_xxx → sha256 → tokens 表 → user
            ├─ 2. 事务预占钱包额度并写 pending；不足 → denied
            ├─ 3. 转发上游（builtin 直调 / http 上游转发）
            ├─ 4. 成功结算；失败原路退款；不持事务等待上游
            ├─ 5. usage_logs 与 ledger 保存真实状态、时长、额度
            └─ 6. 返回结果 → bridge → Codex 展示
```

### 6.3 额度耗尽 / 续费

- 余额不足时：所有 tools/call 返回结构化错误（`isError:true` + 提示文案），记 `denied` 账。
- 用户网页充值（M2 前：管理员后台手动加额度）→ 立即恢复，无需改任何本地配置。
- tools/list 不扣费、不受余额影响（余额为 0 仍能看到工具列表和价格）。

### 6.4 token 轮换（体现 bridge 价值）

```
网页吊销旧令牌 → 创建新令牌 → loadout login（只更新 ~/.loadout/config.json）
→ 各客户端配置零改动，bridge 下一次调用自动用新 token
```

---

## 7. 数据模型设计

当前为 PostgreSQL 18；可执行 DDL 见 `server/internal/store/migrations/`，不维护 SQLite 兼容层。

| 表组 | 责任 |
|---|---|
| users / sessions / tokens | 账号、可撤销哈希会话、独立网关令牌 |
| wallets / ledger / usage_logs | 个人与团队钱包、事务流水、预占/结算/退款/恢复 |
| tools / rate_limits | 加密预设池配置、多实例一致的调用限额 |
| plans / redemption_codes / orders | 套餐、一次性兑换、真实充值记录 |
| teams / team_members / team_invites | 席位、owner/member、一次性邀请 |
| device_authorizations | 设备授权、轮询间隔、一次性令牌领取 |
| oauth_states / oauth_identities / email_verifications | PKCE/state、提供方身份映射、邮箱验证 |

额度使用 bigint，时间使用 timestamptz；主外键、唯一约束和游标索引均随迁移管理。钱包变更和 ledger 必须同一事务，授权在行锁取得后重新查询，避免等待期间角色变化留下旧权限。

---

## 8. API 设计

前缀 `/api/v1`。鉴权两轨：

- **网页会话**：HttpOnly/SameSite `loadout_session` Cookie，7天；写请求校验 Origin，服务端可立即注销
- **网关令牌**：`Authorization: Bearer ldt_xxx`（opaque，仅用于 `/mcp` 与 verify）

### 8.1 认证与账号（网页会话）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | `{username, password}` → `{user}` + session cookie，送配置的初始额度 |
| POST | `/api/v1/auth/login` | `{username, password}` → `{user}` + session cookie |
| GET | `/api/v1/account/me` | 用户信息 + 今日/累计用量摘要 |
| GET | `/api/v1/account/usage?limit=50` | 本人调用明细 |

### 8.2 网关令牌（网页会话）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/account/tokens` | 列表（含前缀、末次使用、吊销态；**不含完整 token**） |
| POST | `/api/v1/account/tokens` | `{name}` → `{token: "ldt_..."}`（仅此一次返回明文） |
| DELETE | `/api/v1/account/tokens/:id` | 吊销 |

### 8.3 CLI 校验（网关令牌）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/account/verify` | `loadout login` 用：返回 `{username, balance, tools:[...]}` 供展示 |

### 8.4 管理后台（网页会话 + role=admin）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/admin/users` | 用户列表 + 余额 |
| POST | `/api/v1/admin/users/:id/balance` | `{delta}` 加/减额度（delta 可负） |
| GET | `/api/v1/admin/usage?limit=100&user_id=` | 全局调用明细 |
| GET | `/api/v1/admin/tools` | 工具池列表 |
| POST | `/api/v1/admin/tools` | 新增 `{key, name, kind, config, units_per_call}` |
| PATCH | `/api/v1/admin/tools/:id` | `{enabled?, units_per_call?, config?}` |

### 8.5 MCP 网关端点（网关令牌）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/mcp` | JSON-RPC：`initialize` / `tools/list` / `tools/call`（stateless streamable HTTP） |
| GET | `/mcp` | 无状态模式下不支持 SSE 长连接 → 405 |
| DELETE | `/mcp` | 无会话可断开 → 405 |

统一错误码：`401 {error:"unauthorized"}`（token 无效/吊销）、`403 {error:"forbidden"}`（非 admin）、`402` 语义在工具结果中表达（MCP 层面返回 isError 文案，不用 HTTP 402，避免破坏协议）。

---

## 9. MCP 网关详细设计

### 9.1 端点与传输

- 协议：MCP **Streamable HTTP**（`application/json` POST 请求-响应；无状态）。
- 网关使用官方 Go MCP SDK 的无状态 Streamable HTTP handler；具体选项随实现时锁定的 SDK 验证，禁止自行拼接协议。
- 鉴权在 Go HTTP middleware 层完成（解析 Bearer → tokens 表 → request context），未过鉴权根本不进 MCP 层。

### 9.2 工具注册与命名

- 工具定义快照缓存 5 秒（避免每次请求都打上游 listTools）。
- **命名规则**：
  - builtin 工具：裸名，如 `time_now`、`echo`；
  - 上游工具：`${serverKey}__${upstreamName}`，如 `websearch__search`。
  - 前缀防冲突 + 用户可读可归因；usage_logs 记录完整名。
- `tools/list` 返回上游原始 JSON Schema（网关不做 schema 改写，校验交给上游）。

### 9.3 上游管理

| kind | 连接方式 | 生命周期 |
|---|---|---|
| builtin | 进程内函数 | 常驻 |
| http | Go MCP SDK Client + Streamable HTTP transport | 懒加载，按 rowId 缓存；config 变更签名比对后重建 |
| stdio | Go MCP SDK Client + stdio transport | 同上；**P1 且默认 disabled**（安全，见 §13） |

上游故障策略：listTools 失败 → 跳过该 server 并记日志（不拖垮整个列表）；callTool 失败 → 断开缓存连接、下次重建、向用户返回 `isError` + 错误摘要。

### 9.3a 插件市场（2026-09-11 增补）

- **定位**：服务端维护的 MCP 插件目录，运营方浏览/同步/一键装入预设池；用户经 CLI 浏览并本地直连安装。
- **只收录 HTTP MCP**：市场安装接口拒绝非 http 传输；stdio 仍属部署者受控能力（工具管理 + `LOADOUT_STDIO_COMMANDS` 允许名单），不进入市场。
- **目录来源**：
  - curated：随迁移种子的精选公共端点（DeepWiki、Context7、Microsoft Learn），已知 endpoint 可默认直装；
  - github：管理员触发同步 GitHub Search（`mcp-server in:name topic:mcp-server` 按星标，可选 `LOADOUT_GITHUB_TOKEN` 提额）；同步失败目录保持原样。
- **双通道产品语义**：
  - **服务端池**（计量）：管理员市场安装 → `tools` 行 + 密封配置 → 网关目录 → 全部客户端经 bridge 受益；
  - **本地直连**（不经计量）：`loadout install <slug>` 把公共 HTTP 端点直接写进客户端配置（`loadout-<slug>` 托管条目，独立标记与备份），不写入任何凭证。
- **元数据覆盖**：`tool_metadata_overrides` 按 (tool_id, remote_name) 覆盖上游工具的描述与参数 InputSchema（目录构建时应用，无效 schema 整体忽略）；管理员可在工具页实时发现上游工具并编辑覆盖。
- **服务端即插件市场源（2026-09-11 终态，产品主线）**："登录即武装"完整闭环——运营方在服务端定制插件（**网关独享 MCP**（`transport=gateway`，凭证密封、按次计量）+ 技能 + 装备组），服务端在 `/marketplace.git` 以哑 HTTP git 裸仓**实时**渲染官方 Codex 插件市场（`internal/marketplace/{exporter,gitrepo,registry}.go`，懒构建+变更失效缓存），`/plugins` 提供对外 SEO 目录页（服务端直出 HTML）。用户：`codex plugin marketplace add <origin>/marketplace.git` → `codex plugin add <插件>`——独享 MCP 以 **stdio bridge 条目**（`{"command":"loadout","args":["bridge"]}`）进入插件 mcp.json，经登录凭证走网关计量；CLI `loadout install <slug>` 对 gateway 成员自动确保 bridge、不落直连条目。仍可用 `build/loadout-export` 导出目录树推 git 仓库（公共策展源路径）。全部经真实 Codex CLI 0.153.4 验证：服务端源接入/列目录/安装/stdio mcp 接受/独享工具经 bridge 真实扣费。
- **生态三件套（2026-09-11 晚增补）**：市场条目分 `kind`——
  - `mcp`：HTTP MCP 插件（池内计量安装 / CLI 本地直连安装）；
  - `skill`：Agent Skill（SKILL.md 及配套文件）；来源 inline（目录内维护）或 github (repo, path)（服务端经 contents API 解析，CLI 只连 Loadout：`GET /api/v1/marketplace/{slug}/files`）；CLI 写入 `~/.codex/skills/<slug>/`、`~/.claude/skills/<slug>/`，清单登记文件集，目录含外来文件时拒绝卸载；
  - `bundle` 装备组：mcp/skill 成员集合，`loadout install <bundle>` 一条命令复刻专家配置（普通用户的核心场景）；不允许嵌套。
  发布（管理员导入）与消费（用户安装）分离；文件路径安全校验（拒 `..`/绝对路径/控制字符，≤32 文件、单文件 ≤256KB）。

### 9.4 计量与扣费（核心）

```
tools/call → 当前令牌/成员/工具权限 → 数据库共享限额
  → 短事务锁钱包、重新验证权限、预占额度、写 pending/ledger
  → 提交后执行上游（30秒，无自动重放）
  → 成功结算 ok；失败同事务退款+error；崩溃超期恢复为 recovered
```

初始化与列表免费；error/denied/recovered 的 cost 为0。成功扣费不会在上游执行后才竞争余额。协议层未知工具/无效 JSON 请求由官方 SDK 拒绝，不进入额度账本；已分派工具的禁用、限频、余额不足会留拒绝流水。详细幂等和竞态证据见产品执行记录。

### 9.5 限流与防滥用（P1）

- 单 token：默认60次/固定分钟窗（PostgreSQL 原子计数，多实例共享）；
- 单用户：每日10000次调用上限（当前统一限制）；
- 超限返回 `isError("rate limited")` 并记 `denied`；
- coding agent 循环调用是主要滥用形态，超时（30s）+ 限频 + 日上限三道闸。

---

## 10. 本地端 CLI 详细设计

### 10.1 命令一览（可执行文件名 `loadout`）

```
loadout login [--server URL] [--token ldt_xxx]   # 登录：校验并保存凭证
loadout logout                                   # 清除本地凭证
loadout status                                   # 当前账号/余额/已配置客户端
loadout doctor                                   # 体检：服务连通性（M0.0）；凭证/客户端检测/配置状态（M0）
loadout apply [--clients codex,claude,cursor]    # 写入配置（默认自动检测全部）
              [--direct]                         # 强制 direct 模式（默认 bridge）
              [--remove]                         # 移除已写入的配置
loadout bridge                                   # [内部] 本地 stdio 转发进程，由客户端拉起
loadout version
```

### 10.2 登录与凭证存储

- `~/.loadout/config.json`（Windows: `%USERPROFILE%\.loadout\config.json`），权限 0600：
  `{ "serverUrl": "https://api.xxx.com", "token": "ldt_xxx", "username": "alice" }`
- 登录即调 `GET /api/v1/account/verify` 校验，失败提示重新粘贴。
- 已实现 device-code 流：`loadout login --device` 输出网页码，浏览器登录后自动回写 token（更好体验）。

### 10.3 配置写入目标（三种客户端）

**Codex** —— `~/.codex/config.toml`（CLI 与 IDE 插件共享），写入标记块：

```toml
# --- loadout begin ---
[mcp_servers.loadout]
command = "C:\\Users\\me\\.local\\bin\\loadout.exe"
args = ["bridge"]
env = { LOADOUT_CONFIG = "C:\\Users\\me\\.loadout\\config.json" }
# --- loadout end ---
```

**Claude Code** —— `~/.claude.json` 的 `mcpServers.loadout`：

```json
{ "mcpServers": { "loadout": {
    "type": "stdio",
    "command": "C:\\Users\\me\\.local\\bin\\loadout.exe",
    "args": ["bridge"],
    "env": { "LOADOUT_CONFIG": "C:\\Users\\me\\.loadout\\config.json" } } } }
```

**Cursor** —— `~/.cursor/mcp.json`（同构）：

```json
{ "mcpServers": { "loadout": {
    "command": "C:\\Users\\me\\.local\\bin\\loadout.exe",
    "args": ["bridge"],
    "env": { "LOADOUT_CONFIG": "C:\\Users\\me\\.loadout\\config.json" } } } }
```

**direct 模式**（`--direct`，Claude/Cursor 推荐、Codex 不推荐）：

```jsonc
// Claude ~/.claude.json
{ "mcpServers": { "loadout": { "type": "http", "url": "https://api.xxx.com/mcp",
  "headers": { "Authorization": "Bearer ldt_xxx" } } } }
// Cursor ~/.cursor/mcp.json 同构
```
```toml
# Codex direct（需自设环境变量，体验差，仅文档说明）
[mcp_servers.loadout]
url = "https://api.xxx.com/mcp"
bearer_token_env_var = "LOADOUT_TOKEN"
```

### 10.4 写入算法（幂等 + 标记 + 冲突防御）

- **JSON 类**（claude/cursor）：读文件（容忍不存在）→ 设/删 `mcpServers.loadout` 键 → 格式化写回。天然幂等。
- **TOML 类**（codex）：不做全量重序列化（会毁掉用户注释）。用标记块：
  - 已有 `# --- loadout begin --- ... # --- loadout end ---` → 整块替换；
  - 没有 → 文件末尾追加（TOML 表声明放末尾恒合法）；
  - 检测到无标记的 `[mcp_servers.loadout]` → **拒绝写入并提示人工处理**（防误覆盖用户手写配置）。
  - 字符串值使用 Rust TOML 序列化库编码，测试覆盖转义、Unicode 和 Windows 路径。
- bridge 命令解析：Rust `std::env::current_exe()` 获取绝对二进制路径，参数固定为 `bridge`，避免 PATH、Node shim 与脚本路径依赖。

### 10.5 bridge 进程设计

```
stdin/stdout (JSON-RPC over stdio, MCP 协议)
  ┌─ loadout bridge ─────────────────────────────┐
  │ Server(stdio transport)                       │
  │  tools/list  ──► 转发网关 tools/list           │
  │  tools/call  ──► 转发网关 tools/call           │
  │ Client(StreamableHTTP + Bearer ldt_xxx)       │
  │  失败 → 重建连接重试一次 → 再失败返回 isError    │
  └───────────────────────────────────────────────┘
```

- 凭证从 `LOADOUT_CONFIG` 环境变量或默认路径读取；
- 额度不足/限流的错误原样透传（文案前缀 `[loadout]`）；
- 进程随客户端生命周期（客户端退出 → stdio 关闭 → 进程退出）。

---

## 11. 网页控制台设计

P0 起使用 React + TypeScript + Vite、Tailwind CSS、shadcn/ui、TanStack Query、Zod 和 Lucide。开发期代理 Go API；生产静态产物由 Go 托管。深色开发者工具风格，前端规范见 [FRONTEND.md](FRONTEND.md)。

### 11.1 用户端（`/`，index.html）

| 区域 | 内容 |
|---|---|
| 登录页 `/login` | 用户名/密码 + 注册切换 |
| 概览卡片 | 当前余额、今日调用、本月消耗、token 数 |
| 令牌管理 | 列表（名称/前缀/末次使用/状态）；「创建」弹窗 → **明文只展示一次** + 复制按钮 + CLI 用法提示（`loadout login --token ldt_xxx`）；吊销按钮（二次确认） |
| 用量明细 | 表格：时间 / 工具 / 状态 / 耗时 / 消耗（分页） |

### 11.2 管理端（同页 admin 标签页，role=admin 可见）

| 区域 | 内容 |
|---|---|
| 用户管理 | 列表（注册时间/余额/累计调用）；额度调整（输入正负数，写审计备注） |
| 工具池 | 列表（key/类型/倍率/启用）；新增（key/name/kind/config JSON/倍率）；启停开关、倍率编辑 |
| 全局用量 | 最近调用流 + 按工具聚合统计（今日/7日） |

### 11.3 交互原则

- 所有 API 失败给中文 toast；401 统一跳登录页；
- token 明文只在创建响应里出现一次，刷新即不可见；
- 移动端可用（响应式栅格）。

---

## 12. 计量计费与商业化

### 12.1 计费单位

- **1 credit = 1 次成功调用 × 工具倍率（units_per_call）**。
- 贵工具（如网页抓取）倍率高（如 5），便宜工具（echo 类）倍率 1。
- P2 演进：部分上游按 token/字节数计量（网关读取上游返回的 usage 字段），规则引擎同 new-api「倍率」模型。

### 12.2 套餐设计（P2）

| 套餐 | 价格 | 内容 |
|---|---|---|
| Free | 0 | 注册送 1000 credits（够体验 ~几百次） |
| Pay-as-you-go | 充值即用 | 1 元 = 100 credits（示例） |
| Pro 月订阅 | ¥29/月 | 月度 credits + 少量溢价工具折扣 |
| Team | 按席位 | 共享池 + 审计 + 限频策略（P3） |

### 12.3 open-core 边界

| 开源（MIT） | 保留（闭源/云服务） |
|---|---|
| MCP 网关核心、bridge、CLI、配置写入器 | 支付对接、运营后台高级报表、托管服务本身 |

逻辑：核心越开源，越多人自部署 → 生态与信誉；赚钱靠“懒得自部署 + 要稳定服务 + 要本土支付”的大多数（参照 one-api → new-api → 各家中转站的分层现实）。

---

## 13. 安全设计

| 威胁 | 对策 |
|---|---|
| 密码泄露 | scrypt 加盐哈希；登录失败不区分"用户不存在/密码错" |
| 网关 token 泄露 | 库中只存 sha256；明文仅创建时一次；可吊销；多设备分 token |
| 会话劫持 | 随机 opaque session 哈希入库、HttpOnly/SameSite/Secure cookie；7天过期，可撤销 |
| 本地凭证泄露 | `~/.loadout/config.json` 0600；客户端配置零密钥（bridge 模式） |
| **服务端任意命令执行（stdio 上游）** | stdio 上游 = 服务器上跑任意命令。**P0 默认禁用**；P1 开启时仅 admin 可配 + 命令白名单；生产建议放容器/独立沙箱节点 |
| 上游凭证泄露 | AES-GCM 加密 JSONB，部署密钥；管理列表不返回秘密 |
| 滥用/刷量 | 限频（60/min/token）、日上限、30s 超时、error/denied 全留痕 |
| SQL 注入 | 全部参数化查询 |
| 传输 | 生产强制 HTTPS（反代终止 TLS） |
| 越权 | 管理接口双层校验（session + role），对象级鉴权（只能看自己的 token/用量） |

**明确声明（写进 README）**：Loadout 不托管用户第三方账号的 OAuth token；上游凭证均为运营方自有。

---

## 14. 技术选型与理由

| 层 | 选型 | 理由 |
|---|---|---|
| 后端 | 最新稳定 Go、net/http、slog | 标准库优先，原生二进制、明确 HTTP 生命周期 |
| 数据库 | PostgreSQL 18.6 + pgx | 在持久化任务引入驱动和迁移，基建不伪造数据 |
| MCP | 官方 Go SDK；客户端官方 Rust rmcp | 协议复用官方实现；在 MCP 任务锁定最新稳定版 |
| 本地端 | 最新稳定 Rust、clap、reqwest、serde | 无 Node 运行依赖；CLI 与 Tauri 共享 Rust 逻辑 |
| 前端 | 最新稳定 React / TypeScript / Vite | 从第一天使用正式组件工程 |
| UI | Tailwind CSS / shadcn/ui / Lucide | 语义 token、可访问组件、统一图标 |
| 状态与契约 | TanStack Query / Zod | 请求生命周期、重试和运行时边界校验 |
| 测试 | Go testing / cargo test / Vitest / Testing Library / Node 内置测试运行器 | 完整 TDD，真实三端集成 harness |
| 质量 | Biome + TypeScript / golangci-lint (含 go vet、staticcheck、errcheck) + gofmt/goimports + go mod tidy / rustfmt + clippy | CI 与本地同一检查入口 |
| 分发 | 原生二进制；Go + Web 单容器 | CLI 不依赖 npm 安装；Tauri Linux deb |

“最新”指搭建时查询官方发布源的最新稳定版并锁定，更新走 PR 和完整 harness；不跟随 beta/rc，不在 CI 浮动升级。当前具体版本以 manifests、工具链文件和锁文件为准。

## 15. 代码仓库结构

```text
loadout/
├── .agents/skills/             # 固定版本的三套技能，随仓库共享
├── .github/workflows/          # 与本地一致的 CI
├── docs/PLAN.md                # 产品总体方案
├── docs/HARNESS.md             # RED/GREEN/REFACTOR 与验证入口
├── docs/FRONTEND.md            # React 开发规范
├── server/
│   ├── cmd/loadout-server/     # Go 进程入口
│   └── internal/              # 当前 config/httpapi；业务模块按需增加
├── cli/
│   ├── src/                   # Rust CLI；后续 bridge/config writers
│   └── tests/                 # CLI 黑盒行为测试
├── web/src/                   # React、API、components/ui、设计 token
├── tests/                    # Node 标准库：真实 HTTP / CLI 与进程集成
├── Makefile                   # help/dev/up/down/status/check/build
├── scripts/dev.mjs            # 本地开发后台生命周期；复用 pnpm dev
├── pnpm-workspace.yaml        # JS 工具和 Web 工作区
├── rust-toolchain.toml        # Rust 固定稳定工具链
└── .env.example               # 当前已实现的环境变量
```

M0.0 环境变量：`LOADOUT_ADDR`（默认 `127.0.0.1:8787`）、`LOADOUT_WEB_DIR`（默认 `web/dist`，相对服务工作目录）。数据库、会话密钥和额度配置随对应业务任务增加，不在基建接收无效选项。

本地开发提供 `make dev` 前台启动，以及 `make up/down/restart/status/logs` 后台管理 Go + Vite；CLI 是按需运行的命令，不作为常驻服务。`make ready` 串行执行完整 `check` 后再 `up`。后台状态与日志默认保存在 `.loadout/`，可用 `LOADOUT_RUN_DIR` 隔离；通过本地 Unix socket 控制本次开发进程，不按端口或进程名杀进程。后台启动须等待真实 API 和 Web 就绪，重复启动保持现有实例，启动失败清理已启动的子进程。

---

根目录开发启动与控制命令通过 Node 原生 `--env-file-if-exists=.env` 加载本地配置，已导出的环境变量优先；独立 Go 二进制仍仅读取环境变量。未配置数据库时只运行健康基建模式，账号页面需要业务数据库才能工作。

## 16. 里程碑与验收标准

### M0.0 —— 工程基建（已交付）

React 状态页 → Go 健康 API ← Rust doctor；完整 TDD / harness、三套开发技能、固定依赖、CI、容器定义。详细范围见 [基建设计](superpowers/specs/2026-09-10-foundation-design.md)。不等同于下面的 M0 业务验收完成。

### M0 —— 端到端业务链路（目标 1 周）

范围：server（账号/令牌/网关/builtin 工具/计量扣费/React 控制台）+ CLI（login/apply/bridge/status/doctor）。

**验收 = 冒烟脚本全绿**，覆盖：

1. 注册/登录 → 创建网关令牌；
2. `loadout login`（临时 HOME）→ verify 通过；
3. `loadout apply` → 三客户端配置文件生成且内容正确、重复 apply 幂等；
4. MCP 客户端 → 网关直连：listTools ≥2、callTool 成功；
5. MCP 客户端 → bridge：listTools/callTool 成功（全链路）；
6. 两次调用后余额精确减少、usage_logs 有 2 条 ok 记录；
7. 吊销 token → 网关返回 401。

### M1 —— 可运营（2 周）

- 管理后台完整（用户/工具池/全局用量）；HTTP 上游管理与真实协议 fixture 验证；运营接入自有供应商凭证；限频 + 日上限 + 30s 超时；Docker compose 一键部署；结构化日志；API 文档（OpenAPI）。
- 验收：后台新增 http 上游 → 10 秒内对用户可见可调用可计费。

### M2 —— 商业闭环（2-3 周）

- 套餐、兑换码、管理员充值与流水（用户确认外部支付仅预留接口）；device-code 登录；React 控制台报表扩展；用量报表导出。
- 验收：管理员充值/兑换一次 → 额度到账 → 调用扣减 → 账本一致；外部实收排除。

### M3 —— 团队版（3-4 周）

- 团队/席位/共享额度/审计导出/SSO（GitHub Org）；（谨慎评估）用户自有 OAuth 的本地代管——token 留本地不落服务端。

### M4 —— 开源运营

- 官网 + 文档站 + 一键部署（Railway/Fly/Zeabur）；贡献者指南；预设池共建流程（PR 审核）；监控告警（Prometheus）。

---

## 17. 风险与对策

| # | 风险 | 概率/影响 | 对策 |
|---|---|---|---|
| 1 | **上游吸收**：OpenAI/Anthropic 官方出账号级 MCP 市场 | 中/高 | 差异化在本土化（支付/国内工具源/中文支持）+ 跨客户端中立性 + 轻量自部署；官方动作出现时快速跟进兼容 |
| 2 | **成本失控**：预设池上游 API 费用 > 收入 | 中/高 | 每工具独立倍率定价；限频三道闸；监控单用户成本；贵工具只进付费档 |
| 3 | **stdio 上游 RCE** | 低/致命 | P0 禁用；P1 admin-only + 白名单 + 沙箱部署 |
| 4 | **支付合规**（国内） | 中/中 | 主体资质先行；初期用管理员手动加额度 + 兑换码过渡；海外 Stripe/Paddle |
| 5 | Cursor Team Sync 变好用 | 中/中 | 只覆盖 Cursor 一家；多客户端 + 自部署 + 计费深度是差异 |
| 6 | MCP 协议演进（如 UCP/竞争协议） | 中/中 | 网关层隔离协议细节；抽象 transport；出现新协议加适配器 |
| 7 | 命名商标（loadout 为常见游戏词汇） | 低/低 | 开源项目用 `loadout` 风险小；商用前查标，必要时换名（§19 备选） |

---

## 18. 竞品对照

| 维度 | **Loadout** | HiMarket | Composio | Smithery CLI / mcpm | 1MCP / MCPJungle | Cursor Team Sync |
|---|---|---|---|---|---|---|
| 登录即全套预设 | ✅ 核心 | 部分（企业市场） | ❌（开发者自助） | ❌ 无账号体系 | ❌ | ✅（仅 Cursor 团队档） |
| 本地一键写配置 | ✅ 三客户端 | ❌ | ❌ | ✅ 但无服务端 | ❌ | ✅ 仅自家 |
| 计量 + 额度 | ✅ 核心 | ✅ 企业级 | ✅ | ❌ | ❌ | ❌ |
| 网页管理后台 | ✅ | ✅ 重型 | ✅ | ❌ | ❌ | ✅ |
| 轻量自部署 | ✅（Go + PostgreSQL） | ❌（Java+K8s） | ❌ 闭源 | — | ✅ 但无商业层 | ❌ |
| 国内本地化 | ✅ 规划 | 部分 | ❌ | ❌ | ❌ | ❌ |
| 开源 | ✅ core MIT | ✅ | ❌ | 部分 | ✅ | ❌ |

**定位一句话**：比 HiMarket 轻，比 Composio 开放，比 Smithery/mcpm 多了账号与额度，比 Cursor 多了跨客户端与中立性。

---

## 19. 命名与备选

**主名：Loadout**（`loadout`，CLI 同名）

- 含义：游戏"预设装备/出战配置"——登录即武装，一词条精确命中产品体验；
- 短、好念、跨语言无歧义，npm scope 可用 `@loadout/cli`；
- Slogan：*"Your AI, fully loaded."*（你的 AI，满装出战）

备选（如需更换，目录与标识符批量替换即可，M0 阶段成本≈0）：

| 名字 | 隐喻 | 备注 |
|---|---|---|
| **pitstop** | F1 进站：进站全配好、按次计费 | 计费隐喻更贴 |
| **mcpdock** | 扩展坞：一插全都有 | 直白但组合词 |
| **outfitter** | 装备商：给你配齐上山装备 | 长一点 |
| **buffet** | 自助餐：付一笔随便用 | 偏订阅隐喻 |
| **quiver** | 箭袋：满袋箭随时发射 | 简洁 |

---

## 20. 术语表

| 术语 | 含义 |
|---|---|
| credit | 计费单位；1 credit = 1 次成功调用 × 工具倍率 |
| 网关令牌 / `ldt_` | 用户创建的 opaque 密钥，调 `/mcp` 用，库中只存哈希 |
| 网页会话 | 可撤销 opaque Cookie，网页控制台用，7天 |
| bridge | CLI 内置本地 stdio→HTTP 转发进程，客户端配置零密钥的关键 |
| direct 模式 | 客户端直连网关 HTTP 端点的配置方式（高级选项） |
| 上游 / 预设池 | 网关聚合的 MCP server 集合（builtin/http/stdio 三类） |
| 标记块 | 写入 TOML 时包裹的 `# --- loadout begin/end ---` 注释，用于幂等替换与安全移除 |
| denied / ok / error | 调用账单三态：拒付（余额不足/限频）/ 成功扣费 / 上游失败不扣费 |

---

*本文档为 Loadout 项目的唯一开发依据；实现与文档冲突时，先改文档再改代码。*
