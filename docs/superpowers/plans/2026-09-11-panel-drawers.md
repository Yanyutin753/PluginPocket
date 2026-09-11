# 面板减负与右侧抽屉

用户授权：工具记录美化，同样适用于其他 panel；当前工作区实现，保留已有修改，不使用浏览器自动化。

设计：沿用中性灰白、黄色动作的工坊视觉。工具列表保留名称、短说明、成本和状态，参数进入右侧详情。工具/套餐编辑、市场安装、用户调账使用同一抽屉；令牌、兑换码、团队操作和设置用清晰入口按需打开。桌面 560px，手机全宽；标题和关闭固定，正文独立滚动，Escape 关闭并还原焦点。提交中禁止关闭；一次性凭据需显式保存/隐藏，避免意外丢失。现有分页与错误恢复保留。

步骤：先覆盖真实页面行为 RED → 共用 Radix Dialog 抽屉与页面接入 GREEN → 列表样式及回归 → make check 与代码审查。

## RED / GREEN / REFACTOR

命令在 WSL 仓库运行，PATH 使用 Node 26.8.2、~/.local/bin Go 与 ~/.cargo/bin Rust。最初 pnpm 不在 PATH 的错误不计入 RED。

- RED：`pnpm --dir web exec vitest run src/PanelFlows.test.tsx`，6 项失败：参数没有 dialog，表单默认已经可见或缺少入口。
- GREEN：同命令 6 项通过；系统配置/兑换新增 2 项 RED 后同命令 8 项通过；市场默认收起新增 1 项 RED 后 9 项通过。
- REFACTOR：`pnpm --dir web test`，阶段性 126 项全过。消费者测试显式打开/关闭抽屉，不删原业务断言。
- 审查：独立 reviewer 找出外层抽屉未关联子 mutation pending，以及市场描述截断后无完整入口。删除市场截断规则；通过已有 TanStack Query 的 mutationKey/useIsMutating 锁定外层抽屉，不增加依赖。
- 审查修复 RED：`pnpm --dir web exec vitest run src/Marketplace.test.tsx`，延迟同步/元数据失败两例因 Escape 提前卸载 dialog 失败；GREEN 同命令 5 项通过。
- `pnpm --dir web exec vitest run src/components/SidePanel.test.tsx src/PanelFlows.test.tsx`：11 项通过，含焦点圈定、嵌套选择器 Escape、locked 关闭保护与焦点还原。
- 本次修改的 26 个前端源文件和测试 `pnpm exec biome check --error-on-warnings …`：通过。
- `pnpm --dir web build`：通过 TypeScript 与 Vite 生产构建。
- 全 Web 最新阶段 `pnpm --dir web test`：132 项通过、2 项失败，失败均在并行修改的 `features/LandingPage.test.tsx`；本次抽屉相关测试通过。
- 完整 `make check` 已实际运行三次：最初受并行页面格式/ARIA 修改影响；第三次通过前端 Biome 与 TypeScript，停在 `server/cmd/loadout-server/application.go` / `server/internal/app/directory.go` 的 Go import 格式差异。未修改其他任务业务代码；不宣称全量通过。
- Impeccable detector 已运行一次：新抽屉标题 20px 和既有列表字号属于 typography 文档建议，保留明确的标题/正文/元信息层级；其余主要为旧样式的字号记录建议。

## 验收边界

按照用户契约不使用浏览器自动化。桌面/手机、浅深主题的实际视觉需人工检查；组件测试和构建不替代视觉验收。真实 E2E：`make test-e2e` 完成四端构建、Linux deb 与其余真实业务子测试，最初 Web 团队测试因保存未完成就按 Escape 而失败（关闭保护正常生效）。测试改为等待创建/加入成功清空输入再关闭，随后用相同 fixture 环境执行 `cd server && go test -race ./cmd/loadout-server -run '^TestProductJourneyWeb$' -count=1 -v`，生产与开发两套各 3 条真实 Web 旅程全部通过，Go 命令退出 0（21.300s）。原 `make test-e2e` 的退出仍为失败；不把聚焦重跑描述为完整命令全过。
