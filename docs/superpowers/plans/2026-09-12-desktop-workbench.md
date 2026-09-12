# Desktop Workbench Implementation Plan

> 执行：subagent-driven-development；用户已授权当前工作区连续实施。保留本文件作为顺序证据，不自动提交或发布。

**Goal:** 完成首期桌面装备管理台并启动可实时更新的预览。
**Architecture:** Rust CLI 共享库提供安全本地清单、独立诊断、持久日志；Tauri 限定命令调用它们；React Query + Zod 消费命令，浏览器明确不可用状态。
**Tech Stack:** 既有 Rust/Tauri、React/TypeScript、TanStack Query、Zod、shadcn、Lucide；不加依赖。
**Spec:** `docs/superpowers/specs/2026-09-12-desktop-workbench.md`

## Global constraints

- 先行为测试 RED，再实现 GREEN，再相关回归；最终 make check。
- 不改真实凭证/客户端配置，测试使用临时目录；不运行浏览器自动化。
- 只显示真实数据。版本若未记录则 null；bundle 展开成员，不推断安装来源。

## Task 1 — Native capabilities

Files: `cli/src/lib.rs`, 新本地能力模块及测试, `desktop/src/lib.rs`, `desktop/src/main.rs`, `desktop/tests/commands.rs`。

Interfaces: `installed` → `{slug,kind,clients,version:null|string}[]`; `logs` → `{timestamp:number,level:'info'|'error',action:string,message:string}[]`（毫秒）；`diagnostics {server:null|string}` → `{checks:{id,label,status:'ok'|'warning'|'error',detail}[]}`；`update_installed/uninstall_installed {slug,kind,clients}` → null；`export_logs {entries}` → `{path,count}`（仅允许当前真实日志，固定 exports 目录）。所有新增入口限本地操作。

- [x] RED：真实临时 HOME 写入托管装备后可列出；操作日志跨 LocalClient 重建可读且无秘密；无网络时诊断仍返回本机分项；更新/卸载保持安装目标，失败保护外来文件。
- [x] GREEN：复用 CLI 路径校验、清单与生命周期逻辑，固定安全日志，有界持久化，桥接失败也记入日志。
- [x] REFACTOR：运行 CLI 与桌面聚焦回归、fmt/clippy，记录命令与结论。

## Task 2 — Desktop workbench UI

Files: `desktop/ui/src/App.tsx`, `api.ts`, 新工作台页面组件/测试, `styles.css`。

- [x] RED：键盘切换导航；日志筛选/复制失败恢复；装备筛选、指定目标更新、行内卸载确认；诊断显示独立失败与建议；浏览器预览提示，不展示虚构数据。
- [x] GREEN：保留概览原行为，侧栏切换页面；Zod 校验新增接口，React Query 刷新；新增数据操作只在原生环境可用。复用组件和语义 token。
- [x] REFACTOR：组件测试、typecheck/build、Biome；补齐长内容与窄窗口 CSS。

## Task 3 — Review, checks, preview

- [x] 代码审查并修复必要问题；同步 PLAN/README/桌面说明/AGENTS 模块地图与前端设计记录。
- [ ] 跑 make check，记录真实结果与未验证范围。
- [x] 启动 `pnpm --dir desktop/ui dev`，HTTP 验证并在 Codex 打开 http://127.0.0.1:1420；保持 HMR 服务运行。

## Execution evidence

初始检查：桌面当前仅支持 login/logout/status/clients/doctor/apply/remove；CLI 已有安装与卸载函数。当前清单不记录装备组来源，首期明确展示成员。工作区初始 `wsl -d Ubuntu -- git status --short` 无改动。

预审：Task 1 生产上述命令对象与返回值，Task 2 按同样字段消费；共享文件仅此计划记录，由主 agent 汇总。Task 3 验证前两项，不另建服务 API。无接口冲突。

### 已执行的 TDD 顺序

下列命令从 WSL 仓库运行；Node 使用 `/home/yangyang/.nvm/versions/node/v26.8.2/bin`，pnpm 使用 `/home/yangyang/.local/share/pnpm`，Go 使用 `/home/yangyang/.local/bin`。

| 验收 | RED 命令与原因 | GREEN |
|---|---|---|
| 工作台导航、日志、装备、诊断、浏览器边界 | `pnpm --dir desktop/ui test Workbench.test.tsx`：6失败，缺少导航/页面与预览提示，非编译失败 | 同命令6通过；随后 `pnpm --dir desktop/ui test` 15通过 |
| 原生清单、持久日志、独立诊断 | `cargo test --manifest-path desktop/Cargo.toml --no-default-features --test commands workbench --locked`：3失败，缺少命令 | 同命令3通过；更新与技能增量RED缺少命令/安装日志后5通过 |
| 真实 bridge 失败日志 | `cargo test --manifest-path desktop/Cargo.toml --test native_bridge --locked`：实际进程未写日志 | 同命令通过；后续错误恢复文案缺失RED→GREEN |
| CLI 失败操作日志 | `cargo test --manifest-path cli/Cargo.toml --test workbench --locked`：日志为空 | 同命令通过；安全错误分类/持久化失败可见性RED后3通过 |
| 不接收含凭证的市场端点 | `cargo test --manifest-path desktop/Cargo.toml --no-default-features --test commands workbench_update --locked`：含凭证URL被接受 | 共享写入路径校验后通过 |
| 审查修复：同名不同kind、原生导出、技能警告 | `pnpm --dir desktop/ui test Workbench.test.tsx`：4失败（找不到不同kind按钮、未产生原生保存结果），4已有通过 | 同命令8通过 |
| 原生导出与独立kind | 原生workbench测试：3失败（不支持kind字段/export动作） | 8个workbench用例通过，覆盖真实文件/私有权限/伪造与链接拒绝 |

REFACTOR：前端拆分 Overview/EquipmentPage/LogsPage/DiagnosticsPage；Zod校验native返回值。构建首次因TS目标不支持 `toSorted` 失败，改用过滤后新数组的 `sort`，同一生产构建通过。Biome自动格式化与可访问标签修正后 `pnpm exec biome check desktop/ui/src` 通过。CLI和原生桌面全量cargo tests、两crate clippy `--all-targets --locked -- -D warnings` 与fmt检查均通过（实现agent执行）。

### 审查与范围裁定

- 请求代码审查发现三项：技能卸载保护说明不实、Linux blob下载被原生导航策略阻止、同名不同kind不可分别管理。全部按上述RED/GREEN修复；独立复查确认三项已解决，未发现修复中的新可执行问题。
- 技能卸载沿用文件名所有权校验，不引入未经需求授权的内容快照系统；行内确认明确已编辑内容也会删除。该说明取代草案中笼统的“所有手动改动均保留”。
- 使用原生固定路径日志导出，保持不提供任意路径/文件系统能力；不放宽导航来源。
- Impeccable detector运行一次，仅报告字号阶梯advisory；既有桌面字号与本次信息密度延续设计文档。按用户契约未进行浏览器自动化；无桌面/手机截图，不宣称视觉验收。
- 用户追加要求“客户端前后端都启动”：保留已有Go服务 :8787 和Web :5173；启动桌面Vite :1420与Tauri原生开发进程。运行于WSL，读取WSL HOME的客户端配置；未执行真实登录/配置写入。

### 完整验证与预览

初次 `make check` 在收窄PATH后找不到Go，停于lint，不算有效产品检查；补上 `.local/bin` 后通过Node `--env-file=.env` 加载现有测试连接重跑，输出到忽略目录 `build/desktop-workbench-check.log`。最终结果待本轮命令完成后追加。

实时预览：`pnpm --dir desktop/ui dev`（Vite HMR）持续运行；Windows侧 `Invoke-WebRequest http://127.0.0.1:1420/` 与 `/src/App.tsx` 均200，已在Codex预览打开。`pnpm --dir desktop/ui tauri dev` 启动真实Rust进程；WSLg打印EGL警告，不能以进程存在替代窗口视觉验收。与本次无关的 `2026-09-12-admin-tools-migration-fix.md` 修改保持原样。

### 用户实时反馈：补齐网关 MCP 分组

用户追加视觉精修：原生截图发现默认960px窗口过早降为单栏。保持功能不变，将默认窗口调整为1180×800、概览双栏断点降低到900px，缩小顶部与插画区域，统一面板细边框和网关薄荷色层次，明确中文字体回退。纯样式修改以现有行为回归、生产构建及原生窗口观察验证，不新增镜像CSS测试。

只读检查本机清单的键（不输出凭证）确认三个 bridge + 两个 skill 安装记录，使用已保存凭证读取原服务 `/api/v1/account/verify` 返回200及8个启用工具key；未触发真实工具调用或更改配置。根因为列表按本地安装清单展示，略过了网关。新增 `GatewayEquipment.tsx` 复用既有status命令，独立展示服务器目录与实际配置客户端，支持搜索/过滤和回概览管理，不伪装为本地安装。

- RED `pnpm --dir desktop/ui test Workbench.test.tsx -t 'shows gateway MCP'`：找不到网关组标题；GREEN 同命令通过。
- RED `pnpm --dir desktop/ui test Workbench.test.tsx -t 'rejects a malformed gateway'`：错误目录被当作空列表；GREEN 依据account.go真实字符串数组契约收紧Zod后通过。
- 回归 `pnpm --dir desktop/ui test`：19通过。最新gateway改动需包含在最终生产构建/全量检查中。
- Windows `Get-Process` 确认 `msrdc` 窗口标题为 `[WARN:COPY MODE] PluginPocket · 装备管理台 (Ubuntu)`；仅证明确有原生窗口，未对内容截图验收。

网关过滤错误态补测：RED/GREEN 命令为 pnpm --dir desktop/ui test Workbench.test.tsx -t 'keeps gateway failure'。先因客户端过滤隐藏读取错误而失败，改为仅在存在account时按客户端隐藏，随后通过。
视觉精修：生产构建通过；使用computer-use查看真实WSLg概览与装备页，确认双栏及网关/本地分组；截图 build/desktop-workbench-equipment.png。WSLg中文渲染仍偏细，未宣称Windows/macOS或手机视觉验收。
第二次全量检查在desktop tsc发现测试ByRoleOptions不支持exact，移除该多余选项后生产构建通过，最终全量检查重跑中。


独立定向复查确认网关客户端过滤错误态已修复，无新增问题。预览 :1420 与后端 :8787/healthz 最新检查均HTTP200。

用户反馈右侧滚动带动侧栏抖动：根因是workbench仅min-height，页面整体滚动并依赖sticky侧栏。改为固定视口网格、minmax(0,1fr)内容轨道，main独立overflow与稳定滚动条；侧栏仅自身溢出时滚动。纯CSS布局按原生人工滚动验证，不用源码断言伪造行为测试。
最终make check返回2：Go/CLI/Web/desktop与产品旅程已通过，但既有process测试 a development subprocess failure shuts down its sibling 超时30s，需定向复测；不能宣称全量通过。

滚动修复验收：computer-use原生截图中右侧概览从标题滚到下方，侧栏品牌/导航/底部与顶栏位置保持一致。pnpm --dir desktop/ui build通过。
全量检查补完：make test-process test-dev退出0（4个进程测试、7个开发测试），node --test tests/integration.test.mjs退出0（4个）；先前单次make check仍记录退出2，不改写为通过。最新纯CSS改动完成构建和原生滚动验收。
