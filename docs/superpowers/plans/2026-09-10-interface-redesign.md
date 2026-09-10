# Loadout consumer interface redesign

**Goal:** 全面重设计现有 Web 页面；中文/英文、浅色/深色/跟随系统完整支持。
**Architecture:** 保留 React Router、Query、Zod 和现有业务 API。通过 React context 管理语言与主题；静态文案字典、Intl 格式化，不翻译用户数据。主题使用统一语义 CSS token。登录页品牌表达与控制台操作密度分别设计。
**Scope:** App、Auth、Overview、所有 account/operations 页、Health、共享组件样式；不修改 API 行为和真实客户端配置。当前工作区保留已有修改，不提交或发布。
**Design:** 石墨黑/瓷白表面，精确网格、精致字距、极少量朱红强调。大幅原创材质主视觉只用于登录品牌区；控制台以阅读顺序和真实数据为主。桌面稳定侧栏，窄屏可展开导航。

- [ ] Preferences TDD: 登录前可切换语言/主题；刷新保留；系统主题实时响应；存储不可用不阻断；语言切换保留输入。`pnpm --dir web test src/Preferences.test.tsx`，先缺失控件断言 RED，再实现 GREEN。
- [ ] Localization TDD: 英文真实页面的标签、表格、错误、恢复、管理页，以及时间数字格式；不改用户内容。`pnpm --dir web test src/Localization.test.tsx`。
- [ ] Redesign: 统一 token、App/Auth/Overview 结构，其他页面共享表单、表格与状态设计；原业务组件回归。
- [ ] Verification: 聚焦测试、Web 全测试/类型/构建、完整 `make check`；记录真实结果。遵循仓库禁止浏览器自动化，桌面/移动人工视觉未做则明确标记。

## Evidence

等待本轮实际执行结果后补充，既有文档的通过结果不视为本轮验证。
