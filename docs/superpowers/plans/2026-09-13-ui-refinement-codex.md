# 2026-09-13 全站界面精修（codex 执行、ZCode 控制）

## 目标与方式

用户要求"用 codex 优化美化全部界面，ZCode 控制"。方式：ZCode 作为控制方（批次任务书、diff 审查、测试验证、视觉验收、小修），codex CLI（gpt-6-astra, `codex exec --sandbox workspace-write`，prompt 经 stdin + 基线截图 `-i`）作为执行方。范围为 web 控制台全部界面；2026-09-10 刚完成工坊重设计，故本轮定性为**精修（refinement）而非重设计**，保留既有视觉世界。

## 环境

- 5173 被 LambChat Vite 占用、8080 被 dx-docs 占用；PluginPocket 后端实际在 8787（root 进程）。Vite 以 `PLUGINPOCKET_DEV_PORT=5174` 另启。
- 测试账号 `uibot` 经 API 注册、SQL 提权 admin（dev 库 pluginpocket@5433）。
- 演示数据直插 dev 库（users×9 / wallets / tokens×10 / tools+6 / usage_logs×72 / ledger×30 / plans×4 / codes×6 / teams×2），仅本地开发库。

## 批次与验收

### 批次1 公共侧（codex 两次会话）
改动：`styles.css`（::selection color-mix 柔化、:focus-visible 统一 2px/2px、.tabular-nums）、`LandingPage.css`（三种装备 1主+2次不对称网格、三步改细线时间轨+药丸标签、区块间距 48→104/64→96、角色一次漂浮动效 + reduced-motion 静止）、`PluginsPage.css`（卡片 hover 升起+柔和阴影+黄调边框、徽章 28px 触控、筛选 chips）。
ZCode 控制修正：删除媒体查询中被同名声明覆盖的死规则 4 行；biome format 修复单行规则。
验证：vitest 34 文件/235 测试全绿（codex 沙箱内 PluginProxy EPERM 超时为环境限制，控制方本机复跑通过）；tsc、biome 通过；截图复查首屏/三区块/步骤/移动端 390px 无横向溢出；深色首页正常。

### 批次2 控制台 + 补充（codex 两次会话）
改动：`OverviewPage.tsx` 统计卡升级 `.stats-grid>.stat-card`（20px 图标、tabular 大数字、零值真实说明）、hero 重排去裁切；`styles.css` 表格表头/行 hover/数字列右对齐、record-list 节奏、reduced-motion；`UsagePage.tsx` 三统计卡同构升级（补充会话）；i18n zh/en 同步（shell.ts、catalog.ts）。
验证：vitest 235 全绿、tsc、biome 全 src 通过；数据态截图复查（用量表格、令牌表、概览 hero）。

### 批次3+3b 运营后台（codex 两次会话）
首会话只完成 UsagePage 补充项即停；3b 任务书重跑主体。改动：全站 CSS `dashed` 清零（空状态实线、设备/设置虚线大卡废除）、设备页主卡+次级层级（DevicePage.tsx 加主面板标识）、列表行桌面信息/操作同排 12×14px（styles.css、ToolEditor.css、MarketplacePanel.css）、编辑器分区标题 17px/600、套餐/兑换码/账本数字列 .numeric 右对齐 tabular。
验证：`grep dashed` 为零；vitest 235 全绿；tsc、biome 通过；数据态截图复查工具/市场/账本/设备/套餐/兑换码/用户/团队/计费；深色概览/工具抽查；移动端 usage/tools 无横向溢出。

## 最终验证

- `pnpm --dir web exec vitest run`：34 文件 / 235 测试全绿（控制方本机）。
- `pnpm --dir web exec tsc --noEmit`：通过。
- `pnpm --dir web exec biome check src`：105 文件通过。
- `make check`（导出 .env 后完整 harness：preflight/DB/Redis/skills/biome/golangci-lint/Go race/CLI cargo/Web vitest/e2e 产品链路/集成）：EXIT=0，24 个 `test result: ok` 组 0 失败。首次运行因未导出 `PLUGINPOCKET_TEST_DATABASE_URL` 在 database-check 停止，属调用方式而非代码问题。
- 未验证范围：IAB 内 Tab 键事件无法送达 webview，焦点环未做浏览器实测（CSS 规则与键盘行为测试已覆盖）；Windows/macOS 原生端不在本轮范围；手机端为 390px 视口模拟。

## 截图证据

- 基线：`/tmp/pp-shots/baseline/`（17 张）
- 终版：`/tmp/pp-shots/final/`（含浅/深/移动/焦点检查）

## 备注

- 本轮无行为变更，未新增测试（现有 235 测试锁定行为与键盘可访问性）；新文案全部进 i18n 双语。
- codex 沙箱内 PluginProxy 监听 127.0.0.1 被拒（EPERM）属沙箱限制，非回归。
- 演示数据仅在本地 dev 库；清理可按 created_at>='2026-09-13' 或用户名清单删除。

## 第二轮：高级感升级（P1/P2，同日追加）

用户要求"继续美化 高级一点"。方向：既有世界内的编辑部级质感（分层阴影、发丝线、着色表面、更强字阶），codex 同模式执行。

### P1 全局质感 + 公共侧
- `styles.css`：浅深两套 `--shadow-sm/md/lg` 分层阴影令牌；卡片纵向渐变表面 + 顶部 1px 内高光；主按钮渐变 + 主色着色投影 + active 下压；`status-badge[data-tone]` 着色（success/danger/neutral）；滚动条发丝化。
- `LandingPage.*`：display 字阶上调（clamp 上限 ~5rem、-0.04em）；角色软阴影平台（::after 径向椭圆）；首屏主色氛围光 ≤6%；统计行发丝竖线 + 28px/750 tabular；终端块窗口顶栏三圆点；CTA 箭头 hover 位移（reduced-motion 静止）。
- `PluginsPage.*`：图标容器 44px+ 着色圆角方 + shadow-sm；kind 徽章 data-tone 着色药丸（HTTP=primary 系/技能=success 系/装备组=中性）；查看详情改 ghost 链接。
- 验证：vitest 235 全绿、tsc、biome（4 条既有 specificity 警告）；视觉复查首屏（统计分隔线、渐变按钮、平台阴影、终端卡）与目录卡片。

### P2 控制台 + 运营后台
- `styles.css`：侧栏分组标题 11px/0.08em + 激活项着色药丸 + 3px 主色指示条；顶栏发丝线；stat-card 图标 40px 着色容器（按位次 primary/success/neutral）+ 数字 clamp 1.9–2.15rem/750；表格表头 11.5px/650/0.05em + 行发丝分隔；命令块 12.5px + shadow-md。
- `SidePanel.css`：面板阴影、标题 17px/700、分区发丝线、底部操作栏分隔。
- 验证：vitest 235 全绿、tsc、biome；视觉复查概览（深色侧栏药丸+指示条、着色统计卡、深色终端卡）与账本（着色类型徽章、发丝表格）；工具/设备页经计算样式验证（行发丝线、标题 16px/650、实线边框）；移动端 390px 七页（首页/目录/概览/用量/设备/账本/工具）无横向溢出。
- `make check`（完整 harness）：EXIT=0。

### 已知限制（第二轮）
- IAB 截图通道在长会话后多次卡滞，设备/工具页以计算样式替代像素级视觉确认；移动端以溢出检测替代截图。

### 控制方 lint 修正（P2 后）
`make lint`（biome --error-on-warnings）暴露 P1/P2 引入的 8 条 noDescendingSpecificity。修复方式全部为级联等价重构，无 biome-ignore：删除 1 条死代码（`.tool-records .record-main h2` 20px 已被后位 15px 全覆盖）、3 条既有例外规则（`tbody tr:last-child td`、`.tool-records .record-main h2`、`.tool-records > li > .action-row`）移至 P2 系统规则之后集中注释、SidePanel/LandingPage 的选择器重排与合并。修复后 biome 0 警告；vitest 235 全绿；级联行为经计算样式回归（末行无边框/表头 11.5px/行发丝线/标题 16px/650 均不变）。
