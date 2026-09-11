# 2026-09-12 当前开发市场补同步 GitHub 技能

用户反馈技能未与 GitHub 同步。实查运行中 5173 公共 `/api/v1/plugins`，只有 commit-style / verify-before-done 两个内置技能，另有 GitHub MCP 条目。

原因：通用 `/admin/marketplace/sync` 只搜索 MCP 仓库；GitHub 技能由精选面板调用 `/admin/marketplace/skills` 单独发布，当前数据库未执行该导入。现有数据库此前重新创建，参见 existing-local-services 执行记录。

按用户要求，通过现有管理员 API 对前端现有三个精选来源执行真实 GitHub 同步，未修改应用代码、客户端配置或其他市场条目。脚本读取本地环境凭据，仅在内存持有会话，结束注销，不输出凭据。

验证命令：`/home/yangyang/.nvm/versions/node/v26.8.2/bin/node --env-file=.env .loadout/sync-recommended-skills.mjs`，退出 0。

- superpowers-debugging：发布 1.0.0，11 文件。
- superpowers-tdd：发布 1.0.0，2 文件。
- anthropic-frontend-design：发布 1.0.0，2 文件。
- 随后重新读取公共目录，确认上述三个技能与两个内置技能均已返回，共 5 个技能。

此次为现有功能的数据同步操作，无实现行为变更；不适用代码 RED/GREEN/REFACTOR，未运行 make check，也未做浏览器自动化或人工视觉验收。未将手动同步宣称为定时自动同步。

## 扩展：页面统一同步入口

用户进一步要求技能也支持同步。采用有界扩展：现有页面“同步 GitHub”编排已有 MCP/技能 API，不新增端点、数据库结构或定时任务。技能集合为未收录精选条目与已有 GitHub 技能（slug 去重）；已改为 inline 的技能及同 slug 其他类型跳过，保护用户内容。单项失败继续其他项，展示分类成功数与失败名称，再次点击可重试。

实施记录：
- [x] 文档先行：PLAN / README 描述统一同步与自定义内容保护。
- [x] RED：`pnpm --dir web exec vitest run src/Marketplace.test.tsx -t "syncs GitHub skills"`；新行为断言失败，原 MCP 502 后直接停止，没有技能请求及分类结果；日志 `.loadout/skill-sync-red.log`。此前一次 shell PATH 解析失败仅为环境准备，不算 RED。
- [x] GREEN：`pnpm --dir web exec vitest run src/Marketplace.test.tsx`，12项通过，包含 MCP/技能部分失败、剩余技能继续、自定义保护、键盘 Enter 触发、重试及目录重新读取；日志 `.loadout/skill-sync-green.log`。
- [x] REFACTOR：复用现有 recommendations 与 Zod schema；Biome格式检查/格式化四个相关文件通过；`pnpm --dir web build` TypeScript及生产构建通过，日志 `.loadout/skill-sync-build.log`。
- [x] Impeccable机械检查 MarketplacePanel.tsx 返回空 findings，日志 `.loadout/skill-sync-design.json`。未使用浏览器自动化；桌面/移动人工视觉未验证。
- [x] 只读代码审查与共享工作区统一 make check 完成，结果见文末。其他任务正在执行 `.loadout/file-storage-check.log` 的全量检查，本任务不并行重复启动。

本次仅改变Web按钮编排；服务端 `/admin/marketplace/sync` 仍为MCP同步API，技能继续通过 `/admin/marketplace/skills` 发布。

审查/回归补充：
- 独立只读审查未发现需修复问题，另运行聚焦技能同步用例通过；未修改文件。
- 格式化后 `pnpm --dir web exec vitest run src/Marketplace.test.tsx src/Localization.test.tsx`：30项通过，日志 `.loadout/skill-sync-regression.log`。
- `git diff --check`：通过。

最终统一验证（已完成）：
- [x] 共享工作区协调任务执行固定PATH下 `node --env-file=.env .loadout/tool-editor-check.mjs`（内部 `make check`），退出0；日志 `.loadout/file-storage-check.log`。覆盖Go全量race、CLI、Web、桌面测试/Linux deb、真实服务E2E、进程启停/热重载及HTTP集成；本任务读取日志确认末尾4项HTTP集成全部通过。
- [x] 为覆盖运行期间最新UI，协调任务补充全仓Biome、完整Vitest、Web生产build及 `git diff --check`，退出0；本任务复核 `.loadout/file-final-web.log`：32测试文件、220项通过，生产构建完成。
- [x] 只读审查无需修复项。本次实现已完成，未推送/发布。
- 尚未验收：桌面/移动人工视觉、Windows原生及真实云存储账号；不以DOM/构建通过代替这些范围。
