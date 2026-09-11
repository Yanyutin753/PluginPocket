# 有数据后的分页与界面优化

用户明确授权逐页检查、分页优化、全部界面美化；延续现有工具箱品牌、日夜主题与多端支持，在当前工作区完成，不提交/推送。

## 当前证据与设计

真实浏览器当前数据：令牌50条使页面高6851px；兑换码50条7225px；用户50行4089px；账单4041px。原有加载更多只追加，无返回上页、当前范围、每页条数。

采用已选TanStack Query和API游标，每页默认10，可选20/50，上一页使用缓存，下一页成功后才替换内容，失败保留当前内容并重试同一游标；筛选/页容量变化回首页。记录范围根据已加载实际条数计算，不伪造总数/末页。全部主列表迁移，选择器候选数据维持累加。

统一Pagination组件与usePagedList；表格添加scope和data-label使手机呈字段卡片，桌面数值右对齐/表头固定。压缩记录元信息间距、状态标签语义颜色。账单套餐网格+独立分页；概览压缩欢迎区并前置真实指标。登录/注册/设置/服务状态/客户端沿用品牌体系并纳入最终截图巡检。

## TDD记录

1. Product.test.tsx 先修改游标翻页期望：当前页替换且可返回；运行 pnpm --dir web exec vitest run src/Product.test.tsx，pagination-red.log，2例缺少下一页按钮为有效RED。
2. 实现分页；Product/UsageSummary/Pagination回归22通过（data-polish-green.log），包含缓存上页、页容量、末页、失败重试及筛选切换。
3. 清除筛选按钮先失败（filter-clear-red.log），实现按钮与表单按筛选重建，随后上述同组GREEN。
4. Pagination.test 增补过期异步响应/禁用重复点击的回归。全web首次106通过（data-polish-web.log）。后续最终完整check与视觉验收待记录。

## 验证计划

最终桌面/移动浅深主题逐页截图，真实点击分页/容量/筛选，检查无溢出和破图；管理员/团队数据流只读浏览，不触发业务写入。React标准组件测试检查键盘与错误；独立审查；make check；git diff --check。


## 最终验收与审查

- 浏览器22个入口（含登录/注册/公共页、账号、全部管理员页、真实团队详情/用量、服务状态）×1440/390×浅/深=88张当前截图，无页面横向溢出或破图；另核对2560大屏4个核心页、320英文分页、选择菜单、真实用户第二页、空筛选清除恢复。当前团队用量为空时明确展示空态，不用fixture伪造有数据。图片文件及DOM度量存于Codex visualizations/2026/09/10/01a08bfd-0b27-7442-828c-49b57a546d35/ios/data-review。已审阅前后contactsheet和重点全尺寸截图。
- 同1440视口，令牌页面6851→1691px，兑换码7225→1856px，用户4089→1141px，账单4041→2001px。页面内容逐页替换，不为缩短页面丢弃数据。
- Code review确认清除筛选需同步清理CSV结果/游标：review_workshop新增ExportFilterReset.test.tsx先RED 2例；清除handler添加exporter.reset后GREEN，覆盖迟到响应。
- 翻页后手机停留列表底部的缺陷：pagination-focus-red.log先断言目标region聚焦失败；显式listRef在页变化后滚动并聚焦当前列表，GREEN并浏览器证实新页首条在视口顶部。空列表隐藏分页先pagination-empty-red.log失败，修改后pagination-empty-green.log通过。
- 最后独立前端验证：pnpm --dir web test，110通过（data-pagination-web-final.log）；pnpm --dir desktop/ui test，7通过（data-pagination-desktop-final.log）；两端build与git diff --check通过。客户端验证为现有预览/组件层，不声称实际改写本地客户端配置。
- 完整check过程：data-pagination-final-check.log被CSS specificity warning阻断，调整规则顺序后data-pagination-final-check-2.log完整通过；之后继续修复CSV/焦点。最新完整轮次遇到另一组后端改动中的import排序和g.execute签名编译错误（data-pagination-complete-check.log/-2.log）。仅修正session_errors_test.go两条import排序，未改后端业务逻辑。后端接口随后由并行工作更新并编译通过，最新完整检查日志data-pagination-complete-check-3.log，待退出结果记录。

静态设计扫描（本轮一次）58条均为建议项，0条主要问题，data-pagination-design-scan.json。窄屏分页保留44px选择触控区域、箭头按钮有名称；数字右对齐只用于桌面数据列。最终截图发现兑换码横排按钮继承align-self:start，已明确为end使按钮与输入框底边对齐。


### 最终结果

最新完整 make check（data-pagination-complete-check-3.log）退出1：Go gateway 的边界/目录测试和store.TestDistributedConcurrentFirstMigrations失败；后者实际seed tools=5、期望2。此时工作区另有后端网关/种子数据改动，未修改其业务逻辑或弱化断言。不能声称最终全仓harness通过，上一轮完整成功也不替代此结果。前端110测试、客户端7测试、两端构建与当前前端Biome均通过；最后仅修正横排生成按钮对齐，追加web build再次验证。临时UI参数已恢复中文/深色，正式页面保留可检查。
