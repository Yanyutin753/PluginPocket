# 桌面额度账单同步与图标修复

## 范围与原因

用户反馈两点：桌面客户端图标与其他应用相比视觉过小；运行日志不包含额度账单，应当同步。排查结论：

- 图标根因不在打包链路（`tauri.conf.json` 引用齐全、CI 直接使用仓库静态文件），而是源图 `web/public/icon-512.png` 可见内容仅占画布约 35%，`tauri icon` 派生的全平台图标（hicolor PNG、ICO、ICNS、托盘）全部继承透明边距。
- 账单缺口是三层断链：桌面"运行日志"只是本地操作摘要（`~/.pluginpocket/operations.json`，200 条上限，与计费无关）；服务端 `/account/usage`、`/{ledger}` 仅认会话 Cookie；CLI 无任何用量/账单拉取函数。

修复分层：图标（本记录）、服务端 Bearer 双轨 + CLI 拉取（本记录）、桌面 UI 页签（另行记录于 desktop/README 工坊界面节）。

## 图标修复

- `python3 + PIL`：按 alpha bbox 裁剪源图，等比放大至 512 画布的 88%，居中回写 `web/public/icon-512.png`。该文件仅被 desktop README 的图标生成命令引用，Web 界面不依赖。
- `pnpm --dir desktop/ui tauri icon ../web/public/icon-512.png --output /tmp/pluginpocket-icons` 重新派生；按 README 约定只覆盖 `desktop/icons/` 中配置引用的 32/128/256 PNG、ICO、ICNS（256 取 128x128@2x.png）。
- 验证：PIL 复测新文件内容占比 87–100%（原 35–50%）；ICO 含 16–256 六档。视觉上与其他应用图标一致需在真实平台任务栏/启动器人工确认（本环境未做，见限制）。

## RED / GREEN / REFACTOR（服务端）

1. RED：`cd server && PLUGINPOCKET_TEST_DATABASE_URL=... go test ./internal/app -run TestUsageAndLedgerAcceptScopedBearerToken -count=1`，失败于 `bearer usage 401 {"error":"unauthorized"}` —— 失败来自缺失的 Bearer 鉴权行为（本机自建 `pluginpocket_test` PG 角色与库跑真实集成）。
2. GREEN：`account.go` 新增 `currentUserOrToken`（Bearer `ppt_` 优先，回退会话；`AuthToken` 查询已含 `enabled` 与吊销判定），`usage` 与 `ledger` 两个 handler 改用它；管理员端点不动。首跑 ledger 断言写错（兑换账单 note 为固定文案），改用 alice=5/bob=6 的 delta 隔离断言后通过。
3. 回归：`go test -race ./internal/app -count=1` 全包通过（189s），含契约金样与分布式测试。

## RED / GREEN / REFACTOR（CLI 与桌面命令）

1. RED：`cargo test --manifest-path cli/Cargo.toml --locked --test billing`，编译失败 `no method named usage/ledger found for LocalClient`。
2. GREEN：`cli/src/lib.rs` 新增 `UsageCall/UsagePage/LedgerEntry/LedgerPage`（PartialEq 供测试断言），`cli/src/config.rs` 新增 `usage()`/`ledger()`（bearer 请求、失败与坏响应各自安全文案），`LocalClient::usage/ledger` 从凭证读取服务端；`desktop/src/lib.rs` `LocalCommand` 新增 `Usage`/`Ledger` 变体。同命令 2 项通过。
3. 回归：`cargo test --manifest-path cli/Cargo.toml --locked` 全部通过；`cargo test --manifest-path desktop/Cargo.toml --locked --no-default-features` 通过。

## 验收与限制

- 服务端个人端点新增 Bearer 双轨：管理员面未开放令牌鉴权（设计如此）；伪造令牌 401、用户隔离（alice 看不到 bob 的 usage/ledger）有测试覆盖。
- 桌面 UI 由 Codex（gpt-6-astra，`codex exec --approve-for-me`）按简报实施：运行日志页 radix Tabs 三页签（本地操作/用量明细/账变记录）、Zod 校验 `api.usage/ledger`、加载/错误/重试/空态、键盘页签切换、中英切换（`i18n.tsx`）、概览信息卡、窄窗口横向滚动与 reduced-motion。组件测试 35 项通过、`pnpm --dir desktop/ui build` 通过。
- 人工复核修正 Codex 产出三处：新页签内表头与三处文案硬编码中文（英文模式混排）；第二轮 i18n 扫尾后字典中 72 处"中文→中文"恒等映射批量替换为真实英文翻译（`中文` 语言名保留原文）；`workbench.css` 概览卡选择器特异性降级警告（`make lint` `--error-on-warnings` 会失败），作用域提升至 `.workbench` 修复。
- 完整 harness：`make check`（真实 PG/Redis）结果见下补记。
- 图标在真实 Windows 任务栏/macOS Dock 的视觉效果未在本环境人工确认，以填充率数值与既有发行流程为准。

## 补记：make check

`PLUGINPOCKET_TEST_DATABASE_URL=postgres://pp_test:test@127.0.0.1:5432/pluginpocket_test PLUGINPOCKET_TEST_REDIS_URL=redis://127.0.0.1:6379 make check` 首次运行失败于 `workbench.css` 概览卡选择器特异性警告（`lint` 阶段 `--error-on-warnings`）；选择器作用域提升至 `.workbench` 后复跑，退出码 0。各阶段真实结果：release-preflight 5 项通过；biome 全仓 156 文件无告警；golangci-lint 通过；server 全包 `go test -race` 通过（app 包 190s）；CLI cargo 测试全过；Web vitest 34 文件 235 项通过；desktop/ui vitest 4 文件 35 项通过；desktop cargo 双模式通过；E2E `TestProductJourney*` 全部 PASS（含真实 CLI、Redis 共享目录、重启保账、双实例零超扣、崩溃恢复、管理端计费退款、桌面 bridge 连真实服务）。
