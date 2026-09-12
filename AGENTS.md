# PluginPocket 开发契约（长期维护版）

阅读顺序：README.md → docs/PLAN.md → 当前任务设计/计划 → docs/HARNESS.md；前端同时阅读 PRODUCT.md、DESIGN.md、docs/FRONTEND.md。本文是所有贡献（人类与 agent）的操作契约：与实现不一致时，先改文档再改代码；本文与产品现实脱节时，改完对应工作就地更新本文。

## 产品速查（2026-09-11）

管理员市场管理入口 `/admin/marketplace` 支持技能/装备组创建编辑与精选 GitHub 批量同步；GitHub 发布保存文件快照，技能变更同时更新引用装备组版本。

登录即武装的 MCP 生态：**插件市场**（三种 kind——HTTP MCP 插件 / Agent Skill / 装备组 bundle，GitHub 同步 + 精选）→ 服务端预设池（鉴权/计量/扣费/结算中间件）→ CLI `apply`（bridge，零密钥）与 `install`（本地直连插件与技能，一键复刻专家配置）。账号、团队、套餐、兑换码、运营后台、Rust CLI、Tauri 桌面端齐备。

### 模块地图

工具管理支持共享数据库图标（普通图片上传自动缩小压缩，静态SVG保留原格式，最终64 KiB）与管理员结算试算；技能二进制/大Git对象复用 `internal/filestore`（小文件bytea，大文件S3兼容存储），SHA256清单与CLI完整性校验。试算不执行上游调用、不写账本。

| 区域 | 位置 | 要点 |
|---|---|---|
| HTTP API / 运营后台 | `server/internal/app` + `internal/httpapi` | 账号/令牌/团队/账单/市场/元数据覆盖、计费角色（倍率折算+工具角色门槛）；HttpOnly AT/RT 会话 + Bearer 双鉴权（期限由共享PG系统配置热更新）；用量详情按本人/团队/管理员范围读取网关输入输出；**错误码注册表** `httpapi/codes.go` 为 wire 契约唯一事实源（源扫描+契约金样测试防漂移） |
| MCP 网关 | `server/internal/gateway` | `/mcp` 无状态；目录 5s 缓存；预留→执行→**结算中间件**（`tools.settlement`：content path/pattern 或 goja 沙箱 script）→失败退款 |
| 市场 | `server/internal/marketplace` | GitHub 同步 + 技能代取 + **服务端即 Codex 插件市场源**（`/marketplace.git` 哑 HTTP git 实时渲染，gateway 独享组件→bridge 条目；`/plugins` 公共 React 目录页；`pluginpocket-export` 离线导出） |
| 存储 | `server/internal/store` | pgx + 事务账本（append-only 触发器）；**SQLite 迁移轨道** `migrations_sqlite/`（ADR 0002 阶段 2 地基，已验证） |
| CLI | `cli/src` | login/apply/bridge + market/install/uninstall（按 kind 分发；托管标记 + 备份 + 清单） |
| 桌面发行 | `desktop` + `.github/workflows/release.yml` | 概览（余额/工具/接入信息卡）/装备/持久日志+**额度账单页签**（usage/ledger 经 Bearer 实时读取）/分项诊断工作台，窗口级中英双语与明暗主题；装备分列服务端 MCP 网关目录与本地直连/Skill 清单，复用共享 CLI 与受限原生命令；Windows x64、macOS/Linux 双架构；独立发布密钥，安装包 bridge 验证 + Minisign 验签后公开；v0.3.0 起内置官方 updater 自动更新（GitHub Releases `latest.json` 由 CI 验签后生成）；平台受信任证书与 GUI 验收另计 |
| Web 控制台 | `web/src` | TanStack Query + Zod + shadcn/ui；市场面板、技能文件工作区（目录/标签、CodeMirror、安全预览与全屏）、覆盖编辑器、结算策略编辑（前端语法预检）；错误文案按码双语映射 `i18n/errors.ts`（对齐后端契约 `i18n/error-codes.json`，覆盖测试锁定） |
| 压测设施 | `server/cmd/pluginpocket-server/load_test.go`（build tag `load`） | QPS / 慢上游 / 内存安全三件套，`make load-test` |

## 必须使用的技能

- `.agents/skills/superpowers/`：需求澄清、计划、TDD、系统排障、代码审查、完成前验证。
- `.agents/skills/ponytail/SKILL.md`：编码默认 full；先标准库/平台能力，再已选成熟库。精简不得删除必要测试、安全边界、错误状态、无障碍。
- `.agents/skills/impeccable/SKILL.md`：前端工作必读，使用项目产品/设计上下文；完成时用标准组件测试检查错误状态与键盘操作，桌面与移动端视觉单独人工检查；按用户要求不使用浏览器自动化验证。
- 用户已明确授权的实现与修复继续推进，不因技能模板重复要求批准或切换工作区。
- 技能按固定上游提交随仓库提供，见 `.agents/README.md`。原文不是 PluginPocket 产品需求，不照搬上游的全局配置、提交、推送或发布动作。

## 完整 TDD（强制）

1. 先写用户可观察行为的最小测试；运行聚焦测试并确认 **RED**。失败必须来自待实现行为，依赖下载、语法错误、错误导入不算有效 RED。
2. 只实现令该测试通过的最少逻辑；运行同一测试确认 **GREEN**。
3. 在绿灯下重构；先聚焦测试再相关回归。缺陷修复先添加能重现根因的失败测试。
4. 最终运行 `make check`。报告真实命令、结果和未验证范围，不用推断或测试数量替代证据。
5. 任务记录保留 RED / GREEN / REFACTOR 命令、失败原因和验收映射（追加到 `docs/superpowers/plans/` 对应执行记录）。CI 证明最终状态，顺序证据由开发记录和 PR 提供。

第三方原样组件与工具配置不复制其内部测试；通过消费方行为测试、真实构建和集成检查验证。自有行为必须 TDD，禁止先写实现再补测试、删除失败断言、无理由 skip、以源码字符串断言代替行为。**基准/压测类资产**（benchmark、`load` tag）以测量正确性为准：无超扣、计量逐笔一致、断言通过。

### 测试分层（入口与要求）

| 层 | 命令 | 要求 |
|---|---|---|
| 服务端单元/集成 | `cd server && go test -race ./...` | 需 `PLUGINPOCKET_TEST_DATABASE_URL`（真实 PG）；每例临时 schema |
| SQLite 轨道 | `go test -run TestSQLite ./internal/store` | 纯 Go 零外部依赖；任一轨道迁移改动必须双方言同步过 |
| CLI | `cargo test --manifest-path cli/Cargo.toml --locked` | 含插件/技能/装备组端到端 |
| Web | `cd web && pnpm exec vitest run` | 行为 + 键盘 + 错误态；新文案进 i18n 双语 |
| 压测 | `make load-test`（tag `load`） | 改动网关热路径/结算/限流后建议跑 |
| 全量 | `make check` | 提交前必过；环境受限未跑的检查必须明说 |

## 工程边界

- React + TypeScript / Go / Rust 单仓库；按搭建时最新稳定版固定精确版本及锁文件。升级先查官方源，经完整 harness 后更新；不用 beta/rc 或浮动 latest。
- `web` 只通过 API 访问后端；TanStack Query 管服务端状态，Zod 校验不可信响应；基础 UI 采用 shadcn/ui，样式和图标遵守前端规范；动效尊重 `prefers-reduced-motion`。
- Go 标准库优先，不手写 MCP 协议、密码算法或数据库驱动；协议阶段采用官方 SDK（MCP：`modelcontextprotocol/go-sdk`；JS 结算：`goja` 沙箱——零宿主绑定、200ms 中断、脚本 ≤8KB、编译缓存）。
- CLI stdout 将用于 MCP 协议，bridge 阶段严禁诊断混入 stdout。HTTP 错误不打印原始响应、令牌或包含凭证的 URL。
- **结算是钱的路径**：预留-结算-退款必须走 store 事务；任何新判定（settlement 规则/脚本）失败侧退款，先测后上；账本 append-only 由双方言触发器保证，禁止绕过。
- 市场只收 HTTP MCP；stdio 是部署者受控能力（`PLUGINPOCKET_STDIO_COMMANDS` 允许名单）。技能文件路径跨平台安全（拒逃逸/链接/大小写别名），≤32文件、单文件≤8MiB、总计≤32MiB；SKILL.md为UTF-8，附件保持原字节和可执行位，下载先验SHA256/大小。S3使用官方SDK，不手写签名；云对象和PG引用一起备份。
- 客户端配置零密钥：bridge 条目不落令牌；本地直连只写公共端点；修改真实客户端配置需用户明确授权。
- Web `/api` 同源反代按 Go `http.CrossOriginProtection` 校验（保留写请求 Origin 必填），不把 `PLUGINPOCKET_PUBLIC_URL` 用作来源白名单；代理保留 Host/Origin/Sec-Fetch-Site，生产公开链接、OAuth/邮件及 Secure Cookie 仍使用规范公开地址。
- 新业务模块有真实需求才创建；未实现功能不得返回模拟成功、虚构余额或账单。
- 本地根目录 `pnpm run dev`（VS Code：`PluginPocket: dev`）运行 Go/Air 自动重载 + Vite；固定 Air 版本在 Makefile，`.env` 修改需重启整个任务。
- 变更在当前工作区完成，保留无关修改；没有用户指令不 push、发布或修改真实客户端配置。

## 文档同步义务（改哪类东西必须动哪份文档）

| 改动 | 必须同步 |
|---|---|
| 产品能力/流程 | `docs/PLAN.md` 对应章节（先改后码） |
| 环境变量/部署 | `docs/ENVIRONMENT.md` + `.env.example` |
| API 面 | `docs/PLAN.md` API 表 + web `api.ts` Zod 模型 |
| 迁移（任一轨道） | 对方轨道同步转写 + `TestSQLite*` 断言更新 |
| 压测/容量结论 | plans/ 执行记录；架构决策进 `docs/adr/` |
| 产品门面 | README 快速开始与项目状态（一句实话） |
| 本契约与现实脱节 | 本文件就地修订（见下） |

## Definition of Done

契约与实现一致；RED/GREEN 证据可追溯；相关测试和完整 `make check` 通过（受限项明示）；前端桌面/移动/键盘/错误恢复可用，动效尊重 reduced-motion；结算与账本路径含并发正确性证据（必要时 `make load-test`）；PR 有验证记录。因环境限制未跑的检查必须明确说明，不能写“全部通过”。

## 本文的长期更新要求

- 合入改变模块地图/测试分层/工程边界的 PR，就地更新对应小节（一句话级别，不写教程）。
- 每个里程碑后核对“产品速查”是否仍真实，失真即改。
- 执行细节一律进 `docs/superpowers/plans/` 执行记录；本文只留不变量与入口。
