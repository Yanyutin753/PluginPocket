# 管理员市场列表精简

用户要求：管理员列表不应逐行展示面向安装用户的说明；同类条目统一优化。

## 实施与验收

- 共享 MarketplacePanel 同时覆盖市场管理页和工具管理中的市场面板；技能/装备组仅显示编辑，去掉 CLI 指引和重复教程。
- 名称与类型同行，描述按需显示，标识/来源紧凑排列；去掉自定义内容/组合安装的重复说明，GitHub 来源不重复。
- 行内操作轻量右对齐；MCP 明确安装到工具池/移出工具池，已加入状态靠近元信息。安装面板去掉重复标题。
- 沿用现有15/13/12px列表文字规范、语义主题色和共享按钮，不新增依赖；公共市场安装指引不变。

## TDD 证据

所有命令在 WSL Ubuntu、Node 26.8.2 环境运行。

1. RED：`cd web && pnpm exec vitest run src/Marketplace.test.tsx -t "keeps skill"`，退出1。实际技能行包含 `用 CLI 安装：loadout install review-skill`，违反技能/装备组只呈现管理操作的断言。
2. RED：`cd web && pnpm exec vitest run src/Marketplace.test.tsx -t "lists the marketplace"`，退出1。安装后未找到明确的“已加入工具池”状态。
3. GREEN：实现后上述两例通过；整文件另一个既有 GitHub 安装测试仍查找旧“已安装”文案，更新为新的工具池状态并保留真实安装请求与结果验证。
4. GREEN/REFACTOR 回归：`cd web && pnpm exec vitest run src/Marketplace.test.tsx src/PluginsPage.test.tsx src/Localization.test.tsx`，退出0，44项通过。覆盖编辑、Enter打开/Escape返回焦点、筛选、创建、同步失败恢复、MCP安装、公共目录与双语。
5. `pnpm exec biome check web/src/features/operations/MarketplacePanel.tsx web/src/features/operations/MarketplacePanel.css web/src/Marketplace.test.tsx web/src/i18n/marketplace.ts`，退出0。
6. Impeccable detect 两个组件文件，退出0；仅报告设计元数据未列全17/15/12px字号的advisory，列表字号遵循既有设计正文与共享列表规则。

## 全量与限制

独立review发现安装按钮的aria-label未包含新的完整可见文案。先更新两处行为测试，`cd web && pnpm exec vitest run src/Marketplace.test.tsx -t "installs a GitHub item"`退出1（按“安装到工具池 github-mcp-server”无法定位按钮）；补齐aria-label和英文键后整文件12项通过、Biome通过。第12项为其他任务并行追加的同步回归，本任务不修改其业务实现。

共享工作区统一执行的 `make check` 已退出0（固定PATH后 `node --env-file=.env .loadout/tool-editor-check.mjs` 内部运行），日志 `.loadout/file-storage-check.log`。覆盖Go race、CLI、Web、桌面测试/Linux deb构建、真实服务e2e与进程/dev/集成。针对最新UI另补Biome全仓、完整Vitest、Web生产构建和 `git diff --check`，整体退出0，日志 `.loadout/file-final-web.log`。已核对两份日志末尾：集成4项通过、无失败，Web构建成功；不重复运行全量检查。

没有使用浏览器自动化；尚未完成桌面/移动端人工视觉验收，DOM测试不证明实际排版。保留所有无关工作区改动，不提交、不推送。
