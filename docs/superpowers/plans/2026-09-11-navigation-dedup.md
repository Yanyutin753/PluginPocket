# 工作空间导航去重

用户指出「工具目录」与已包含工具发现的「插件市场」重复。最小调整为删除 Shell 共用导航中的 `/tools` 项，桌面和移动导航一起生效；保留旧路由兼容直达，管理员工具管理不变。产品方案先同步至 docs/PLAN.md。

## 验收与 TDD

以下命令在 WSL Ubuntu 仓库根执行，PATH 包含 Node v26.8.2。最初 pnpm 不在 PATH、随后 shell PATH 转义失败均属环境准备，不算 RED。

- RED：`pnpm --dir web exec vitest run src/Product.test.tsx -t "exposes the public plugin directory"`，退出 1；新增断言发现主导航仍含「工具目录」链接。
- GREEN：删除该导航项后运行同一命令，退出 0，1 项通过。测试同时验证市场链接地址、焦点及 Enter 进入市场页面。
- REFACTOR：没有必要的重构；`pnpm --dir web typecheck` 退出 0；`pnpm --dir web exec vitest run src/Product.test.tsx -t "exposes|compact navigation|collapses and expands|malformed account"` 退出 0，4 项通过，覆盖市场入口、手机菜单键盘操作、侧栏收起展开、账户加载失败恢复。
- 扩大回归：`pnpm --dir web exec vitest run src/Product.test.tsx src/PluginsPage.test.tsx src/PublicHome.test.tsx`，23 通过、6 失败。公共页测试仍假定不请求账户，但当前 PublicHeader 已使用 publicSessionQuery；4 项登录/注册/注销测试未呈现预期登录标题，当前 AuthPage 已读取 publicSessionQuery，旧 mock 未适配。失败不涉及已移除的导航项；没有修改这些独立会话行为。
- 全量：`make check`，发行预检 4 项通过后 database-check 因未设置 `LOADOUT_TEST_DATABASE_URL` 退出 2；后续全量步骤未执行。

按本任务约束未使用浏览器自动化；没有进行实际桌面/移动视觉验收。组件测试不作为视觉通过证据。保留工作区其他未提交修改，未提交或推送。
