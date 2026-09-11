# 技能与装备组管理执行记录

目标：补齐可发现的管理员市场、技能创建/编辑、GitHub 精选导入和装备组创建/编辑。

设计：复用 marketplace_items、现有 skills/bundles API、Query 与 SidePanel；新增 /admin/marketplace 导航。分类与搜索展示真实目录。技能表单支持内联文件与 GitHub owner/repo + path；精选提供已核对上游路径的快捷导入，逐项显示成功/失败，可重复同步。装备组选择已有非 bundle 条目，最多 32 项。沿用权限、文件大小与路径约束，不操作真实客户端配置。

实施顺序（writing-plans，当前工作区内执行）：
- [x] GitHub 目录/文件真实形状测试 RED → 修复文件读取 → GREEN。
- [x] 管理入口、分类、创建技能、编辑、精选导入、组合装备组及失败恢复行为测试 RED → UI/API 最少实现 → GREEN。
- [x] 装备组重复保存、冲突与版本回归 RED → 事务更新 → GREEN。
- [x] 文档同步、相关回归、make check（已执行、受其他修改阻断）、代码审查。

边界：不使用浏览器自动化；桌面与移动端视觉需人工验收。开源候选依据维护者原仓库，展示来源，不伪造星标或质量排名。

## 验证证据

命令均在 WSL Ubuntu 执行；Node PATH 指向 v26.8.2；数据库测试以 `set -a; source .env; set +a` 加载已有专用测试数据库，不打印凭证。

### RED / GREEN

- `cd server && go test ./internal/marketplace -run TestSkillContents -count=1`：目录条目不含正文时失败 `unsupported or oversized skill file`；补充逐文件 Contents 请求后同命令通过。
- `pnpm --dir web exec vitest run src/Marketplace.test.tsx -t 'publishes|selecting|recommended'`：3 个用例因缺少管理入口/创建/精选按钮失败；接通后 `vitest run src/Marketplace.test.tsx` 通过。后续批量导入用例先因缺少“一键同步全部”失败，实现后通过。
- `go test ./internal/app -run TestBundleAuthoring -count=1`：重复保存返回500，upsert 后通过；版本、无变化幂等、跨类型冲突包含在同用例。
- `go test ./internal/app -run TestGitHubSkillSync -count=1`：未重新同步即返回上游新内容，失败；快照保存/读取后通过，重新同步推进版本。
- `go test ./internal/app -run TestSkillAuthoringAccepts -count=1`：24KB技能先400；调整decodeLimit后走到第二个有效RED（引用装备组版本不变）；同步升级成员版本后通过。
- `go test ./internal/app -run TestBundleRejects -count=1`：无端点成员保存201；限制可安装成员后通过。
- UI 组件用例 `creates a bundle`：无端点候选未禁选而失败；补充禁选后通过。
- `go test ./internal/app -run TestConcurrentSkillAndBundleEditing -count=1 -timeout=30s`：并发技能/装备组更新返回500；审查定位Git失效单例与bundle行反向锁序，修复为发布事务先持有共享authoring写锁，再取行锁。

### 回归与外部证据

- `go test -race ./internal/app ./internal/marketplace`：通过，真实PG；app 157.998s，marketplace 1.847s（最终统一锁序前的全包回归）。
- `go test -race ./internal/app ./internal/marketplace -run 'TestMarketplace|TestBundle|TestSkill|TestGitHubSkill' -count=1 -timeout=60s`：通过，app 10.561s。
- `pnpm --dir web exec vitest run src/Marketplace.test.tsx`：10项通过，覆盖创建、编辑保留附属文件、类型筛选、Escape焦点恢复、批量部分失败和重试；最后执行21:08。
- `pnpm --dir web build`：TypeScript与Vite生产构建通过。
- 本次六个UI文件 `pnpm exec biome check ...` 通过。
- 一次性 `TestLiveRecommendedSkillsProbe` 对真实GitHub调用生产ResolveSkillFiles：Systematic Debugging 11文件、TDD 2文件、Frontend Design 2文件均通过，合计17.11s；探针已移除，避免正常测试联网。来源 https://github.com/obra/superpowers/tree/main/skills/systematic-debugging 、https://github.com/obra/superpowers/tree/main/skills/test-driven-development 、https://github.com/anthropics/skills/tree/main/skills/frontend-design 。日志 `.loadout/marketplace-live-probe.log`。
- 完整 `make check` 执行三次，在Biome阶段被工作区其他进行中修改阻断；本次文件的无障碍fieldset和OpenAPI格式问题已修复。日志 `.loadout/marketplace-check-final.log`。不能报告完整通过。
- Web全量运行162通过、19失败，集中登录/会话/设置等其他正在修改的页面；公共目录测试另因新增公共头部请求不再满足“全部请求只到plugins”的旧断言失败。本次市场管理10项通过。不修改其他任务的行为或断言。
- 未使用浏览器自动化；本轮桌面/移动真实视觉尚未人工验收。

### 审查

requesting-code-review 子agent两轮只读审查；长技能请求限制、不可安装成员、跨类型竞争和引用版本更新均已修复；最后补充共享发布锁消除技能/bundle与git状态锁反序。

最终统一锁序后的验证：`go test -race ./internal/app -run 'TestConcurrentSkillAndBundleEditing|TestDistributedSkillPublication|TestBundleAuthoring|TestSkillAuthoring|TestGitHubSkillSync|TestBundleRejects' -count=1 -timeout=60s` 通过（7.470s），含实际并发保存与跨副本版本串行化。

最终第三次 `make check` 仍在其他正在修改的Web文件格式检查失败（14 errors，无本次市场文件）；`build/tools/golangci-lint-2.13.2/golangci-lint run ./internal/marketplace/... ./internal/app/...` 检出其他任务的account/browser_auth/settings/tool_editor共7个诊断，本次市场文件无诊断。完整检查未通过事实保持，不覆盖其他任务进行中的修改。
