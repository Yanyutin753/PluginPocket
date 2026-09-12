# PluginPocket —— 产品与技术总体方案

2026-09-12 桌面额度账单同步与图标修复：个人用量/账变端点开放 Bearer `ppt_` 双轨鉴权（`/account/usage`、`/account/ledger`，管理员端点仍仅会话），共享 CLI 新增 `LocalClient::usage/ledger` 与桌面 `local_command` 变体；桌面运行日志页新增用量明细与账变记录页签（服务端实时读取，不落本地操作日志）。同轮将图标源图 `web/public/icon-512.png` 裁除透明边距至内容占画布约 88% 并重新派生全平台 PNG/ICO/ICNS，修复任务栏/启动器图标视觉过小。执行记录见 `superpowers/plans/2026-09-12-desktop-billing-sync.md`。

2026-09-12 桌面自动更新：内置 Tauri 官方 updater 插件（Minisign 验签）与 process 重启，更新端点为 GitHub Releases 静态 `latest.json`，由 Release workflow 在五平台安装包验签通过后生成并上传、发布前完成校验，签名复用发布密钥体系。客户端启动静默检查、手动「检查更新」、下载进度与失败重试；浏览器预览不展示模拟更新。方案见 `superpowers/specs/2026-09-12-desktop-auto-update-design.md`，验收以执行记录为准。

2026-09-12 桌面装备管理台首期实施：概览、我的装备、运行日志与连接诊断侧栏；共享 CLI 读取本地托管清单并按实际客户端更新/卸载，持久化有界安全操作日志和 bridge 失败，诊断分项展示服务/凭证/配置/可执行文件。浏览器实时预览明确不具备原生能力，不返回模拟成功；版本及装备组来源未记录时不推断。范围见 `superpowers/specs/2026-09-12-desktop-workbench.md`，验收以执行记录为准。

同轮用户反馈“MCP没有”：我的装备补充独立的 PluginPocket MCP 网关分组，复用真实账号工具目录和客户端接入状态，显示可用工具与实际 bridge 目标；本地直连与 Skill 仍按安装清单管理，不把网关工具伪装成本地安装项，网关接入回概览管理。

2026-09-12 计费角色机制：`billing_roles`（name/multiplier_bp/description，种子 default 1.0× / member 0.8× / vip 0.5×）+ `users.billing_role`；倍率在 store 预留事务内按基点折算（`ceil(cost×bp/10000)`，10000bp 即原价），`usage_logs` 记录每笔生效角色与倍率，个人/团队/管理员用量与用户管理全链路展示；工具可声明 `allowed_roles` 角色门槛（空=全开放，非空仅列出的角色可调用，对标 new-api 分组开放）。管理员 `GET/PATCH /api/v1/admin/billing-roles` 调倍率、admin 用户 PATCH 指派角色、工具编辑器配置门槛。方案见 `superpowers/plans/2026-09-12-billing-roles.md`。

2026-09-12 CLI 技能生命周期修复：更新按本地已保存清单识别托管目录，允许新版增加、删除附件并清理旧文件；卸载递归核对嵌套附件。外来文件、目录与符号链接继续阻止破坏性操作，执行记录见 `superpowers/plans/2026-09-12-cli-skill-lifecycle.md`。

2026-09-12 全站质量检查：通用面板关闭后重新打开恢复普通尺寸；桌面配置操作在状态刷新或客户端检测失败时禁用，恢复后继续使用原选择。沿用现有视觉体系优化窄屏输入与市场筛选，实际证据及未验证范围见 `superpowers/plans/2026-09-12-quality-review.md`。

2026-09-12 品牌统一为 **PluginPocket（插件口袋）**，全量改名命令 `pluginpocket`、配置 `PLUGINPOCKET_*` 与 `.pluginpocket/`、包名及应用标识；按新项目运行，不提供旧版兼容或迁移。全产品展示与双语 README 同步，范围及验证见 `superpowers/plans/2026-09-12-pluginpocket-brand.md`。

2026-09-12 SQLite 单机轨道阶段 2 首增量：账本核心（Reserve/Finish/RecoverPending/AuthToken）落地 stdlib database/sql 实现（`store_sqlite.go`），行为与 PG 等价（18 项 `TestSQLite*`，并发零超扣、退款幂等、吊销即时生效），零新依赖；`Ledger` 接口为双实现契约，运行时 DSN 分流待后续域移植。方案与验证见 `superpowers/plans/2026-09-12-sqlite-ledger.md`。

2026-09-12 项目定位更新：确定为**开源项目**（全仓库 MIT，取消 open-core 闭源边界）。成功标准为自部署体验与社区采用，不是收入；外部支付不再是路线图关键路径，额度计量保留为自托管治理能力。详见 §1.2 与 §12。同日新增贡献者指南 `CONTRIBUTING.md`（M4 开源运营首项落地）。

技能工作区升级为目录树、文件标签和CodeMirror文本编辑器，支持Markdown编辑/预览/分栏、图片预览、二进制下载，以及工作区全屏；切换/关闭标签保留草稿，安全渲染不执行原始HTML。方案与验证见 `superpowers/plans/2026-09-12-skill-workbench.md`。

所有 SidePanel 统一提供全屏/还原按钮，包括编辑、详情、设置与创建面板；全屏扩展至浏览器内容区并保留草稿。Escape优先关闭编辑器查找/补全，再还原全屏，普通尺寸关闭面板；保留原有提交锁定与焦点返回行为。

2026-09-12 管理员市场列表聚焦维护：移除技能/装备组逐行 CLI 安装指引与重复教程；名称、标识、类型和来源紧凑呈现，操作统一右对齐，窄屏保持可读。MCP 操作明确加入/移出工具池，技能和装备组保留编辑；公共市场安装指引不变。

2026-09-12 技能编辑默认打开 SKILL.md，附属文本按需读取与编辑，二进制保留原始文件；新建文本和附件管理就近操作，基本信息按需展开。已有 GitHub 技能默认编辑已发布快照，重新同步需显式选择并提示覆盖。初版执行记录见 `superpowers/plans/2026-09-12-skill-file-editor.md`，当前布局与预览以技能工作区方案为准。

2026-09-12 通用文件存储：共享数据库保存小文件原始字节，较大附件接入可配置S3兼容对象存储；技能以SHA256/大小/执行位清单发布，下载与CLI安装校验二进制完整性，保留旧文本格式兼容。实现与实际验收见 `superpowers/plans/2026-09-12-file-storage.md`。

2026-09-11 工具列表视觉精修：缺省及失效自定义图标使用原创薄荷工具箱插图；名称与状态同行、说明和连接信息分层，成本独立对齐，动作采用轻量按钮。窄屏自然换行，保留参数、编辑及启停功能。

公共市场登录态布局：`/plugins` 与 `/plugins/:slug` 匿名保留公开导航；会话有效时复用工作空间 Shell（侧栏、账号菜单、移动导航），不再显示独立公共页头。公开目录仍免鉴权，使用现有 publicSessionQuery 探测会话，未登录/探测失败不阻断浏览；URL 和筛选不变。

2026-09-11 工作空间导航去重：插件市场统一作为工具与插件的发现入口，桌面侧栏与移动导航移除重复的「工具目录」项；旧 `/tools` 页面仍可直达，管理员工具管理保持原有入口。

2026-09-11 公共目录视觉第二轮：用户授权实际浏览器检查；首屏采用搜索与专属工具箱插图的双栏展台，卡片增加名称字标与类型图标层级，保留12项分页与所有筛选行为。桌面、移动和浅深主题以本轮浏览器验收为准。

2026-09-11 技能与装备组管理：管理员 `/admin/marketplace` 提供独立导航、类型筛选与搜索，技能内联创建/编辑和 GitHub 导入、精选来源快捷同步、勾选 MCP/技能创建及编辑装备组。复用 `POST /api/v1/admin/marketplace/skills` 与 `POST /api/v1/admin/marketplace/bundles`（同 slug 同 kind 更新，版本随内容变化）；GitHub 文件按 Contents API 分别读取，失败不发布。执行记录见 `superpowers/plans/2026-09-11-marketplace-authoring.md`。

2026-09-12 图标存储：用户确认采用小图压缩后存共享数据库。PNG/JPEG/WebP 上传原文件最多 5 MiB，等比缩至最长 256 px 并编码 WebP（浏览器不支持时回退 PNG），目标 16 KiB、最终硬上限 64 KiB；SVG 保持静态校验与 64 KiB 上限。处理期间禁止保存，显示实际保存大小，所有副本共用；不新增 S3 依赖。执行见 `superpowers/plans/2026-09-12-tool-icon-compression.md`。

2026-09-11 工具编辑优化：工具图标支持 HTTPS 图片与上传/粘贴的静态 SVG、PNG、JPEG、WebP（解码后最多 64 KiB），随 tools 配置存数据库供多副本读取；列表增加连接类型与规则摘要。编辑器分区提供连接说明、参数示例、可视化结算规则和同引擎试算，保留高级 JSON。PATCH 省略图标/规则保持原值，图标空字符串移除，规则 {} 或 null 恢复默认。`POST /api/v1/admin/tools/settlement-preview` 接收 `{settlement,text,is_error}` 返回 `{charge}`，仅管理员可调用，不调用上游或写账本。验收见 `superpowers/plans/2026-09-11-tool-editor.md`。

2026-09-11 用量详情：个人、团队、管理员用量列表新增按需详情面板。`GET /api/v1/account/usage/{callID}`、`GET /api/v1/account/teams/{id}/usage/{callID}`、`GET /api/v1/admin/usage/{callID}` 返回 `{item}`，继承对应列表权限；输入/输出文本及截断标记仅详情返回。新调用每方向保存最多 64 KiB（按用户要求不脱敏、UTF-8 安全截断），历史未记录数据不补造，CSV 保持摘要。执行证据见 [用量详情](superpowers/plans/2026-09-11-usage-details.md)。

2026-09-11 公共插件市场改为 Apify 风格的搜索入口、类型导航与响应式卡片目录；客户端对公开 API 的完整目录先筛选再分页，每页 12 项，URL 保存 q/kind/page，筛选变化回到首页，非法/越界页码安全收敛。保留匿名详情、安装命令、错误恢复和双语主题。执行证据见 `superpowers/plans/2026-09-11-plugins-catalog-pagination.md`。

2026-09-11 插件目录接通与全站精修：用户最终要求公共 `/plugins`、`/plugins/:slug` 使用 React 页面，与前端统一布局、语言与主题；通过免鉴权 `GET /api/v1/plugins`、`GET /api/v1/plugins/{slug}` 读取公开元数据（`items/item` + `origin`），首页与工作空间均有入口。替代独立 SEO HTML，`/marketplace.git` 保持公开，见 [执行记录](superpowers/plans/2026-09-11-plugins-route-polish.md)。

同轮补齐所有页面与市场源的无粘性多副本验收：Git 提交与对象改为 PostgreSQL 持久共享，市场变更事务内失效，重启/换副本保留完整提交链；SQLite 迁移轨道同步。静态资源滚动版本仍要求共享保留新旧构建资源，见 docs/CLUSTER.md。

2026-09-11 桌面分发扩展：Windows x64（MSI/NSIS）、macOS ARM64/Intel（DMG/app）、Linux x64/ARM64（deb/rpm/AppImage），使用独立 PluginPocket 发布签名密钥，macOS ad-hoc 封印；不含手机端，不等同于平台受信任证书公证。实施及真实验收状态见 [桌面发布记录](superpowers/plans/2026-09-11-desktop-release.md)。

> **一句话定位**：登录即武装的开源自托管 MCP 装备网关 —— 部署一套，成员在网页端注册，本地一条命令把整套预设好的 MCP 工具写进 Codex / Claude Code / Cursor，开箱即用；服务端统一鉴权、计量、扣额度，后台看得见每一次调用。
>
> 名字含义：*PluginPocket* 将插件（Plugin）与口袋（Pocket）组合，表达随手携带和使用 AI 工具的产品定位。

| 项 | 值 |
|---|---|
| 文档版本 | v0.4（产品实现与本机验收完成） |
| 日期 | 2026-09-10 |
| 状态 | 本轮产品功能与本机harness完成；外部/平台边界见验收账本 |
| 仓库规划 | monorepo：`server` + `cli` + `web` + `desktop` |
| 开源策略 | 全仓库 MIT 开源（2026-09-12 起定位为开源项目，取消 open-core 闭源边界） |


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

**PluginPocket 把这四步压缩成两步**：网页注册 → 本地执行 `pluginpocket login && pluginpocket apply`。之后所有工具经统一网关调用，额度、用量、计费全部可视。

### 1.2 产品本质

- 对用户：**MCP 的“预配置装备库”**——要的不是更多配置项，是“登录完就能用的一整套工具与技能”。
- 对架构：**MCP 额度网关（MCP metered gateway）**——参考 one-api / new-api 在 LLM API 领域被大量自部署验证的形态，平移到 MCP 工具调用。
- 对项目（2026-09-12 定位）：**开源自托管项目，全仓库 MIT**——成功标准是自部署体验与社区采用；商业化（托管服务/外部支付）不再是路线图关键路径，额度计量保留为团队治理能力。
- 对生态：只托管**自有预设池**的上游凭证（自己的 key），不碰用户私人第三方账号的 OAuth token（与 Composio 的核心差异，规避最大安全责任）。

### 1.3 边界（明确不做什么）

- ❌ 不代管用户的 Gmail / GitHub 等私人账号授权（M3 前不考虑，见风险一节）；
- ❌ 不做通用 MCP 目录站（Smithery/Glama 已占位，无意义）；
- ❌ 不做企业级重型平台（HiMarket 已占位：Java + K8s 全家桶）；
- ✅ 只做：轻量、开箱即用、带额度与审计治理的开源 MCP 网关 + 一键配置。

---

## 2. 市场背景与机会判断

> 本节保留早期市场假设，不作为当前产品能力或竞品现状的验收依据；价格、排名及能力对比需在对外引用前重新核验官方来源。

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

### Persona A：个人开发者“小白”（主要用户群体）

> 用 Codex / Claude Code 写代码，听说过 MCP 很强，但不想折腾配置和 API key。

场景：在自部署服务注册 → 按部署者设置领取初始或分配额度 → `pluginpocket login` → `pluginpocket apply` → 重启 Codex，使用管理员启用的工具并查看实际用量。

### Persona B：小团队 Tech Lead

> 团队 5-20 人都用 Cursor/Claude Code，想统一工具配置、控制成本、有调用审计。

场景：在网页端创建团队、邀请成员与分配共享额度，成员创建关联团队钱包的网关令牌，再用 `pluginpocket login` 和 `pluginpocket apply` 接入；团队页面按权限查看成员和用量。CLI 不提供 `--team-code` 参数。

### Persona C：MCP 内容提供者（生态期）

> 做了一个好用的 MCP server，想给别人用并收费。

场景：申请入驻预设池 → 后台定价（每调用 N 额度）→ 按分成结算。（M4 之后）

---

## 4. 产品功能总览

| 模块 | 功能 | 优先级 |
|---|---|---|
| **账号体系** | 用户名密码注册/登录（P0）；邮箱验证（P1）；GitHub OAuth（P1） | P0 |
| **网关令牌** | 创建/吊销网关 token（`ppt_` 前缀，只显示一次）（P0）；token 分设备命名（P1） | P0 |
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
│  │ admin.html     │     │   └─ stdio ──► pluginpocket bridge    │    │
│  └──────┬───────┘      │                  │(注入token转发)  │    │
│         │ HTTPS        └──────────────────┼────────────────┘    │
└─────────┼─────────────────────────────────┼─────────────────────┘
          ▼                                 ▼ streamable HTTP + Bearer
┌─────────────────────── PluginPocket 服务端 ──────────────────────────┐
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
所有客户端一律配置一个本地 stdio 命令（`pluginpocket bridge`），由它带着 token 转发到网关。

- 收益 1：客户端配置文件里**不落任何密钥**；
- 收益 2：token 轮换 / 换服务器只需改 `~/.pluginpocket/config.json`，**不动各客户端配置**；
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
2. 控制台 → 令牌页 → 「创建令牌」→ 复制 ppt_xxxx（只显示这一次）
3. 终端：安装原生 pluginpocket 二进制后执行 pluginpocket login --server https://api.xxx.com
   → 粘贴令牌 → CLI 调 /api/v1/account/verify 校验 → 写 ~/.pluginpocket/config.json (0600)
4. pluginpocket apply            # 自动检测已装的客户端（codex/claude/cursor）
   → 逐个写入 bridge 配置（带标记块，幂等）
5. 重启 Codex/Claude/Cursor → 工具全部出现 → 直接用
```

### 6.2 一次工具调用的完整链路

```
Codex(用户按 F5 调用 time_now)
  └─ stdio ─► pluginpocket bridge（本地，读 ~/.pluginpocket/config.json）
       └─ HTTP+Bearer ppt_xxx ─► 网关 /mcp (tools/call)
            ├─ 1. 鉴权：ppt_xxx → sha256 → tokens 表 → user
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
网页吊销旧令牌 → 创建新令牌 → pluginpocket login（只更新 ~/.pluginpocket/config.json）
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

- **网页会话**：HttpOnly/SameSite Cookie，服务端可立即注销；写请求保留 Origin 必填边界，并使用 Go `http.CrossOriginProtection` 根据浏览器 Fetch Metadata 或 Origin/Host 判断同源。前端相对路径 `/api` 反代不依赖固定 `PLUGINPOCKET_PUBLIC_URL`；该配置只用于公开链接、身份回调与 Secure Cookie 策略。代理保留 Host、Origin、Sec-Fetch-Site，不把跨站请求改写为同源。
- **网关令牌**：`Authorization: Bearer ppt_xxx`（opaque，仅用于 `/mcp` 与 verify）

错误响应统一 `{"error":"<code>"}`：码表唯一事实源在 `server/internal/httpapi/codes.go` 注册表（含码→HTTP 状态映射），机器契约 `web/src/i18n/error-codes.json` 由测试锁定，前端 `web/src/i18n/errors.ts` 按码双语映射（完整表见 `docs/API.md` 错误码注册表）。

### 8.1 认证与账号（网页会话）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | `{username, password}` → `{user}` + session cookie，送配置的初始额度；用户名格式不符 400 `invalid_username`，密码长度不符 400 `invalid_password` |
| POST | `/api/v1/auth/login` | `{username, password}` → `{user}` + session cookie |
| GET | `/api/v1/account/me` | 用户信息 + 今日/累计用量摘要 |
| GET | `/api/v1/account/usage?limit=50` | 本人调用明细（会话 Cookie 或 Bearer `ppt_` 令牌均可，2026-09-12 起桌面工作台经 Bearer 同步） |
| GET | `/api/v1/account/ledger?kind=` | 本人账变流水（预留/退款/恢复/充值等，鉴权同上） |

### 8.2 网关令牌（网页会话）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/account/tokens` | 列表（含前缀、末次使用、吊销态；**不含完整 token**） |
| POST | `/api/v1/account/tokens` | `{name}` → `{token: "ppt_..."}`（仅此一次返回明文） |
| DELETE | `/api/v1/account/tokens/:id` | 吊销 |

### 8.3 CLI 校验（网关令牌）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/account/verify` | `pluginpocket login` 用：返回 `{username, balance, tools:[...]}` 供展示 |
| GET | `/api/v1/account/usage?limit=100` | CLI/桌面工作台拉取本人用量明细（Bearer；个人端点同时保留会话 Cookie 鉴权） |
| GET | `/api/v1/account/ledger` | CLI/桌面工作台拉取本人账变流水（Bearer；个人端点同时保留会话 Cookie 鉴权） |

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

统一错误码：`401 {error:"unauthorized"}`（token 无效/吊销）、`403 {error:"forbidden"}`（非 admin）、HTTP 级错误统一 JSON envelope（见第 8 章错误码注册表）；`402` 语义在工具结果中表达（MCP 层面返回 isError 文案，不用 HTTP 402，避免破坏协议）。

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
- **只收录 HTTP MCP**：市场安装接口拒绝非 http 传输；stdio 仍属部署者受控能力（工具管理 + `PLUGINPOCKET_STDIO_COMMANDS` 允许名单），不进入市场。
- **目录来源**：
  - curated：随迁移种子的精选公共端点（DeepWiki、Context7、Microsoft Learn），已知 endpoint 可默认直装；
  - github：管理员触发同步 GitHub Search（`mcp-server in:name topic:mcp-server` 按星标，可选 `PLUGINPOCKET_GITHUB_TOKEN` 提额）；同步失败目录保持原样。
- **双通道产品语义**：
  - **服务端池**（计量）：管理员市场安装 → `tools` 行 + 密封配置 → 网关目录 → 全部客户端经 bridge 受益；
  - **本地直连**（不经计量）：`pluginpocket install <slug>` 把公共 HTTP 端点直接写进客户端配置（`pluginpocket-<slug>` 托管条目，独立标记与备份），不写入任何凭证。
- **元数据覆盖**：`tool_metadata_overrides` 按 (tool_id, remote_name) 覆盖上游工具的描述与参数 InputSchema（目录构建时应用，无效 schema 整体忽略）；管理员可在工具页实时发现上游工具并编辑覆盖。
- **服务端即插件市场源（2026-09-11 终态，2026-09-12 修正插件 MCP 声明，产品主线）**："登录即武装"完整闭环——运营方在服务端定制插件（**网关独享 MCP**（`transport=gateway`，凭证密封、按次计量）+ 技能 + 装备组），服务端在 `/marketplace.git` 以哑 HTTP git 裸仓**实时**渲染官方 Codex 插件市场（`internal/marketplace/{exporter,gitrepo,registry}.go`，懒构建+变更失效缓存），`/plugins` 提供对外 SEO 目录页（服务端直出 HTML）。用户：`codex plugin marketplace add <origin>/marketplace.git` → `codex plugin add <插件>`——MCP 服务器（独享为 **stdio bridge 条目** `{"command":"pluginpocket","args":["bridge"]}`；公共 HTTP 端点为 `{"type":"http","url":...}`）内联进插件 `.codex-plugin/plugin.json` 的 `mcpServers` 字段（Codex 不加载根目录 `mcp.json`，该文件已停写），经登录凭证走网关计量；CLI `pluginpocket install <slug>` 对 gateway 成员自动确保 bridge、不落直连条目。仍可用 `build/pluginpocket-export` 导出目录树推 git 仓库（公共策展源路径）。经真实 Codex CLI 0.153.4 验证：服务端源接入/列目录/安装/插件与 apply 两条 bridge 路径的工具调用与真实扣费、marketplace upgrade 后插件重装生效。
- **生态三件套（2026-09-11 晚增补）**：市场条目分 `kind`——
  - `mcp`：HTTP MCP 插件（池内计量安装 / CLI 本地直连安装）；
  - `skill`：Agent Skill（SKILL.md 及配套文件）；来源 inline（目录内维护）或 github (repo, path)（服务端经 contents API 解析，CLI 只连 PluginPocket：`GET /api/v1/marketplace/{slug}/files`）；CLI 写入 `~/.codex/skills/<slug>/`、`~/.claude/skills/<slug>/`，清单登记文件集，目录含外来文件时拒绝卸载；
  - `bundle` 装备组：mcp/skill 成员集合，`pluginpocket install <bundle>` 一条命令复刻专家配置（普通用户的核心场景）；不允许嵌套。
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

### 10.1 命令一览（可执行文件名 `pluginpocket`）

```
pluginpocket login [--server URL] [--token ppt_xxx]   # 登录：校验并保存凭证
pluginpocket logout                                   # 清除本地凭证
pluginpocket status                                   # 当前账号/余额/已配置客户端
pluginpocket doctor                                   # 体检：服务连通性（M0.0）；凭证/客户端检测/配置状态（M0）
pluginpocket apply [--clients codex,claude,cursor]    # 写入配置（默认自动检测全部）
              [--direct]                         # 强制 direct 模式（默认 bridge）
              [--remove]                         # 移除已写入的配置
pluginpocket bridge                                   # [内部] 本地 stdio 转发进程，由客户端拉起
pluginpocket version
```

### 10.2 登录与凭证存储

- `~/.pluginpocket/config.json`（Windows: `%USERPROFILE%\.pluginpocket\config.json`），权限 0600：
  `{ "serverUrl": "https://api.xxx.com", "token": "ppt_xxx", "username": "alice" }`
- 登录即调 `GET /api/v1/account/verify` 校验，失败提示重新粘贴。
- 已实现 device-code 流：`pluginpocket login --device` 输出网页码，浏览器登录后自动回写 token（更好体验）。

### 10.3 配置写入目标（三种客户端）

**Codex** —— `~/.codex/config.toml`（CLI 与 IDE 插件共享），写入标记块：

```toml
# --- pluginpocket begin ---
[mcp_servers.pluginpocket]
command = "C:\\Users\\me\\.local\\bin\\pluginpocket.exe"
args = ["bridge"]
env = { PLUGINPOCKET_CONFIG = "C:\\Users\\me\\.pluginpocket\\config.json" }
# --- pluginpocket end ---
```

**Claude Code** —— `~/.claude.json` 的 `mcpServers.pluginpocket`：

```json
{ "mcpServers": { "pluginpocket": {
    "type": "stdio",
    "command": "C:\\Users\\me\\.local\\bin\\pluginpocket.exe",
    "args": ["bridge"],
    "env": { "PLUGINPOCKET_CONFIG": "C:\\Users\\me\\.pluginpocket\\config.json" } } } }
```

**Cursor** —— `~/.cursor/mcp.json`（同构）：

```json
{ "mcpServers": { "pluginpocket": {
    "command": "C:\\Users\\me\\.local\\bin\\pluginpocket.exe",
    "args": ["bridge"],
    "env": { "PLUGINPOCKET_CONFIG": "C:\\Users\\me\\.pluginpocket\\config.json" } } } }
```

**direct 模式**（`--direct`，Claude/Cursor 推荐、Codex 不推荐）：

```jsonc
// Claude ~/.claude.json
{ "mcpServers": { "pluginpocket": { "type": "http", "url": "https://api.xxx.com/mcp",
  "headers": { "Authorization": "Bearer ppt_xxx" } } } }
// Cursor ~/.cursor/mcp.json 同构
```
```toml
# Codex direct（需自设环境变量，体验差，仅文档说明）
[mcp_servers.pluginpocket]
url = "https://api.xxx.com/mcp"
bearer_token_env_var = "PLUGINPOCKET_TOKEN"
```

### 10.4 写入算法（幂等 + 标记 + 冲突防御）

- **JSON 类**（claude/cursor）：读文件（容忍不存在）→ 设/删 `mcpServers.pluginpocket` 键 → 格式化写回。天然幂等。
- **TOML 类**（codex）：不做全量重序列化（会毁掉用户注释）。用标记块：
  - 已有 `# --- pluginpocket begin --- ... # --- pluginpocket end ---` → 整块替换；
  - 没有 → 文件末尾追加（TOML 表声明放末尾恒合法）；
  - 检测到无标记的 `[mcp_servers.pluginpocket]` → **拒绝写入并提示人工处理**（防误覆盖用户手写配置）。
  - 字符串值使用 Rust TOML 序列化库编码，测试覆盖转义、Unicode 和 Windows 路径。
- bridge 命令解析：Rust `std::env::current_exe()` 获取绝对二进制路径，参数固定为 `bridge`，避免 PATH、Node shim 与脚本路径依赖。

### 10.5 bridge 进程设计

```
stdin/stdout (JSON-RPC over stdio, MCP 协议)
  ┌─ pluginpocket bridge ─────────────────────────────┐
  │ Server(stdio transport)                       │
  │  tools/list  ──► 转发网关 tools/list           │
  │  tools/call  ──► 转发网关 tools/call           │
  │ Client(StreamableHTTP + Bearer ppt_xxx)       │
  │  失败 → 重建连接重试一次 → 再失败返回 isError    │
  └───────────────────────────────────────────────┘
```

- 凭证从 `PLUGINPOCKET_CONFIG` 环境变量或默认路径读取；
- 额度不足/限流的错误原样透传（文案前缀 `[pluginpocket]`）；
- 进程随客户端生命周期（客户端退出 → stdio 关闭 → 进程退出）。

---

## 11. 网页控制台设计

P0 起使用 React + TypeScript + Vite、Tailwind CSS、shadcn/ui、TanStack Query、Zod 和 Lucide。开发期代理 Go API；生产静态产物由 Go 托管。深色开发者工具风格，前端规范见 [FRONTEND.md](FRONTEND.md)。

### 11.1 用户端（`/`，index.html）

| 区域 | 内容 |
|---|---|
| 登录页 `/login` | 用户名/密码 + 注册切换 |
| 概览卡片 | 当前余额、今日调用、本月消耗、token 数 |
| 令牌管理 | 列表（名称/前缀/末次使用/状态）；「创建」弹窗 → **明文只展示一次** + 复制按钮 + CLI 用法提示（`pluginpocket login --token ppt_xxx`）；吊销按钮（二次确认） |
| 用量明细 | 表格：时间 / 工具 / 状态 / 耗时 / 消耗（分页） |

### 11.2 管理端（同页 admin 标签页，role=admin 可见）

| 区域 | 内容 |
|---|---|
| 用户管理 | 列表（注册时间/余额/累计调用）；额度调整（输入正负数，写审计备注） |
| 工具池 | 列表（key/类型/倍率/启用）；新增（key/name/kind/config JSON/倍率）；启停开关、倍率编辑 |
| 全局用量 | 最近调用流 + 按工具聚合统计（今日/7日） |

### 11.3 交互原则

- 数据面板采用摘要列表 + 按需右侧抽屉：工具参数、工具/套餐编辑、插件安装、调账，以及令牌/兑换码创建、团队操作和账号配置；窄屏抽屉全宽，支持键盘退出与返回触发入口，提交中和未保存的一次性凭据阻止误关闭。

- 公共页识别浏览器会话，已登录显示工作空间入口；登录/注册页恢复有效会话，网络失败可重试。受保护 API 的 401 先经 HttpOnly RT 刷新默认 15 分钟 AT 并重试一次；RT 默认绝对有效期 7 天，同页并发共享刷新。POST `/api/v1/auth/refresh` 成功 204 + Cookie，失败 401/403/500。旧会话在原期限内兼容，注销撤销全部关联 AT；所有凭据状态由共享 PG 管理，支持 K8s 无粘性多副本。
- token 明文只在创建响应里出现一次，刷新即不可见；
- 移动端可用（响应式栅格）。

---

## 12. 计量计费与商业化

> 2026-09-12 定位更新：项目以**开源自托管**为目标，本节计量/额度/套餐保留为自托管治理能力（限额、审计、团队分配、兑换码）；外部支付维持预留接口，商业化不再是里程碑关键路径。

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

### 12.3 开源边界（2026-09-12 更新）

**全仓库 MIT 开源，不再保留闭源边界。** 原 open-core 表（支付对接/高级报表保留闭源）取消：外部支付本就未实现；若未来实现（自托管方接入 Stripe/微信/支付宝等）同样以 MIT 开源，由社区按需贡献适配器。托管/发行服务若出现，属于部署服务而非代码闭源。

---

## 13. 安全设计

| 威胁 | 对策 |
|---|---|
| 密码泄露 | scrypt 加盐哈希；登录失败不区分"用户不存在/密码错" |
| 网关 token 泄露 | 库中只存 sha256；明文仅创建时一次；可吊销；多设备分 token |
| 会话劫持 | 随机 opaque AT/RT 哈希入库、HttpOnly/SameSite/Secure Cookie；默认15分钟/7天，系统配置可热更新，可撤销 |
| 本地凭证泄露 | `~/.pluginpocket/config.json` 0600；客户端配置零密钥（bridge 模式） |
| **服务端任意命令执行（stdio 上游）** | stdio 上游 = 服务器上跑任意命令。**P0 默认禁用**；P1 开启时仅 admin 可配 + 命令白名单；生产建议放容器/独立沙箱节点 |
| 上游凭证泄露 | AES-GCM 加密 JSONB，部署密钥；管理列表不返回秘密 |
| 滥用/刷量 | 限频（60/min/token）、日上限、30s 超时、error/denied 全留痕 |
| SQL 注入 | 全部参数化查询 |
| 传输 | 生产强制 HTTPS（反代终止 TLS） |
| 越权 | 管理接口双层校验（session + role），对象级鉴权（只能看自己的 token/用量） |

**明确声明（写进 README）**：PluginPocket 不托管用户第三方账号的 OAuth token；上游凭证均为运营方自有。

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
pluginpocket/
├── .agents/skills/             # 固定版本的三套技能，随仓库共享
├── .github/workflows/          # 与本地一致的 CI
├── docs/PLAN.md                # 产品总体方案
├── docs/HARNESS.md             # RED/GREEN/REFACTOR 与验证入口
├── docs/FRONTEND.md            # React 开发规范
├── server/
│   ├── cmd/pluginpocket-server/     # Go 进程入口
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

M0.0 环境变量：`PLUGINPOCKET_ADDR`（默认 `127.0.0.1:8787`）、`PLUGINPOCKET_WEB_DIR`（默认 `web/dist`，相对服务工作目录）。数据库、会话密钥和额度配置随对应业务任务增加，不在基建接收无效选项。

本地开发提供根目录 `pnpm run dev` / `make dev` 前台启动（VS Code 任务固定根目录），Go 通过固定版本 Air 监听 `server/` 的 Go、SQL、go.mod/go.sum 变更自动编译重启；编译失败停止旧程序，修复后恢复；`.env` 修改需重启整个任务。另提供 `make up/down/restart/status/logs` 后台管理 Go + Vite；CLI 是按需运行的命令，不作为常驻服务。`make ready` 串行执行完整 `check` 后再 `up`。后台状态与日志默认保存在 `.pluginpocket/`，可用 `PLUGINPOCKET_RUN_DIR` 隔离；通过本地 Unix socket 控制本次开发进程，不按端口或进程名杀进程。后台启动须等待真实 API 和 Web 就绪，重复启动保持现有实例，启动失败清理已启动的子进程。

---

根目录开发启动与控制命令通过 Node 原生 `--env-file-if-exists=.env` 加载本地配置，已导出的环境变量优先；独立 Go 二进制仍仅读取环境变量。未配置数据库时只运行健康基建模式，账号页面需要业务数据库才能工作。

## 16. 里程碑与验收标准

### M0.0 —— 工程基建（已交付）

React 状态页 → Go 健康 API ← Rust doctor；完整 TDD / harness、三套开发技能、固定依赖、CI、容器定义。详细范围见 [基建设计](superpowers/specs/2026-09-10-foundation-design.md)。不等同于下面的 M0 业务验收完成。

### M0 —— 端到端业务链路（目标 1 周）

范围：server（账号/令牌/网关/builtin 工具/计量扣费/React 控制台）+ CLI（login/apply/bridge/status/doctor）。

**验收 = 冒烟脚本全绿**，覆盖：

1. 注册/登录 → 创建网关令牌；
2. `pluginpocket login`（临时 HOME）→ verify 通过；
3. `pluginpocket apply` → 三客户端配置文件生成且内容正确、重复 apply 幂等；
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
| 2 | **成本失控**（公共实例运营场景）：预设池上游 API 费用 > 收入 | 中/高 | 每工具独立倍率定价；限频三道闸；监控单用户成本；自部署方天然自担上游成本 |
| 3 | **stdio 上游 RCE** | 低/致命 | P0 禁用；P1 admin-only + 白名单 + 沙箱部署 |
| 4 | **支付合规**（国内） | 中/中 | 主体资质先行；初期用管理员手动加额度 + 兑换码过渡；海外 Stripe/Paddle |
| 5 | Cursor Team Sync 变好用 | 中/中 | 只覆盖 Cursor 一家；多客户端 + 自部署 + 计费深度是差异 |
| 6 | MCP 协议演进（如 UCP/竞争协议） | 中/中 | 网关层隔离协议细节；抽象 transport；出现新协议加适配器 |
| 7 | 命名商标（pluginpocket 为常见游戏词汇） | 低/低 | 开源项目用 `pluginpocket` 风险小；商用前查标，必要时换名（§19 备选） |

---

## 18. 竞品对照

| 维度 | **PluginPocket** | HiMarket | Composio | Smithery CLI / mcpm | 1MCP / MCPJungle | Cursor Team Sync |
|---|---|---|---|---|---|---|
| 登录即全套预设 | ✅ 核心 | 部分（企业市场） | ❌（开发者自助） | ❌ 无账号体系 | ❌ | ✅（仅 Cursor 团队档） |
| 本地一键写配置 | ✅ 三客户端 | ❌ | ❌ | ✅ 但无服务端 | ❌ | ✅ 仅自家 |
| 计量 + 额度 | ✅ 核心 | ✅ 企业级 | ✅ | ❌ | ❌ | ❌ |
| 网页管理后台 | ✅ | ✅ 重型 | ✅ | ❌ | ❌ | ✅ |
| 轻量自部署 | ✅（Go + PostgreSQL） | ❌（Java+K8s） | ❌ 闭源 | — | ✅ 但无商业层 | ❌ |
| 国内本地化 | ✅ 规划 | 部分 | ❌ | ❌ | ❌ | ❌ |
| 开源 | ✅ 全仓库 MIT | ✅ | ❌ | 部分 | ✅ | ❌ |

**定位一句话**：比 HiMarket 轻，比 Composio 开放，比 Smithery/mcpm 多了账号与额度，比 Cursor 多了跨客户端与中立性。

---

## 19. 命名与备选

**主名：PluginPocket**（`pluginpocket`，CLI 同名）

- 含义：游戏"预设装备/出战配置"——登录即武装，一词条精确命中产品体验；
- 短、好念、跨语言无歧义，npm scope 可用 `@pluginpocket/cli`；
- Slogan：*"Your AI superpowers, in your pocket."*（你的 AI，满装出战）

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
| 网关令牌 / `ppt_` | 用户创建的 opaque 密钥，调 `/mcp` 用，库中只存哈希 |
| 网页会话 | 可撤销 opaque AT/RT Cookie，网页控制台用，默认15分钟/7天 |
| bridge | CLI 内置本地 stdio→HTTP 转发进程，客户端配置零密钥的关键 |
| direct 模式 | 客户端直连网关 HTTP 端点的配置方式（高级选项） |
| 上游 / 预设池 | 网关聚合的 MCP server 集合（builtin/http/stdio 三类） |
| 标记块 | 写入 TOML 时包裹的 `# --- pluginpocket begin/end ---` 注释，用于幂等替换与安全移除 |
| denied / ok / error | 调用账单三态：拒付（余额不足/限频）/ 成功扣费 / 上游失败不扣费 |

---

*本文档为 PluginPocket 项目的唯一开发依据；实现与文档冲突时，先改文档再改代码。*

浏览器鉴权期限由系统配置 `access_token_seconds` / `refresh_token_seconds` 热更新（默认900/604800秒，范围60–86400 / 60–31536000，RT≥AT）；所有副本签发时读共享PG，不延长已签发RT，凭据过期判断使用数据库时钟。

插件市场公共与工作空间目录的首屏展台随浅色/深色/系统偏好切换；深色采用深薄荷底与高对比文字，并使用保留原色的透明插图，保留筛选与搜索行为。

2026-09-12 市场管理“同步 GitHub”同时调用 MCP 同步与技能发布：导入未收录的精选技能，并刷新现有 GitHub 来源技能，按 slug 去重；同 slug 已改为自定义内容的条目跳过。每项独立失败，显示 MCP/技能成功数量与失败名称，可重新执行；完成后刷新管理及公共市场缓存。复用现有 API，不新增定时任务。
