# 前端开发规范

本项目使用 React + TypeScript + Vite。全部直接依赖锁定搭建时最新稳定版，版本见 `web/package.json`、`pnpm-lock.yaml`；不手写成熟库已经解决的基础能力。

## 库的职责

| 需求 | 统一选择 | 使用边界 |
|---|---|---|
| 组件与交互 | React 函数组件、hooks | 不在 render 中执行副作用 |
| 基础组件 | shadcn/ui | 官方 CLI 加入源码，统一在 components/ui；业务页组合组件 |
| 样式 | Tailwind CSS、CSS 变量 | 颜色使用语义 token，禁止页面散落颜色常量 |
| 服务端状态 | TanStack Query | queryKey、取消信号、错误/重试集中管理，不在 useEffect 手写请求状态机 |
| 边界验证 | Zod | API unknown → schema.parse；禁止 `as Foo` 跳过验证 |
| 图标 | Lucide | 统一图标，装饰图标 aria-hidden；按钮必须有可访问名称 |
| 变体与 class 合并 | CVA、cn（shadcn 当前官方默认） | 组件变体集中定义，禁止复制多套按钮样式 |
| 单元与组件测试 | Vitest、Testing Library、user-event | 验证用户行为、语义 role，不绑定 DOM 结构 |
| 跨端集成 | Node 内置 test runner、assert、fetch、child_process | 真实 Go API、生产资源与 Rust 二进制 |
| 可访问性 | Testing Library + user-event、Biome | 语义与键盘行为自动检查；视觉人工检查 |
| 格式与 lint | Biome | 本地与 CI 同规则；CI 只检查，不自动改写 |

路由统一使用已选 React Router，复杂表单使用已选 React Hook Form；具体精确版本以 web/package.json 为准。避免为了“标准化”安装职责重叠或尚未使用的依赖。

## 目录与依赖方向

`src/components/ui/` 为基础组件，`src/lib/` 为项目级通用能力，业务代码随需求按 `features/<name>/` 聚合。当前已有账户、令牌、工具、用量与管理路由；沿用已有聚合方式，不预建空业务目录。UI 不直接耦合 HTTP URL；API 函数处理契约，组件处理展示，Query 管生命周期。

TypeScript strict 开启，禁止 any、无说明 ts-ignore、非空断言掩盖边界错误。组件使用具名 props 类型；只导出被实际使用的 API；不滥用 memo/useMemo/useCallback。

## 状态与请求

- 服务端状态归 Query；局部交互归 useState；可分享的筛选将归 URL。不可把所有状态搬进全局 store。
- 网络请求传 AbortSignal、合理超时；组件卸载取消；显示 loading/error/empty/success。
- 健康检查禁用 HTTP 缓存和自动重试，用户主动重试；即使 navigator 报告离线也要尝试检查本地服务，失败后清除已连接展示；后台查询按业务设置 staleTime，不沿用健康检查设置到所有查询。
- API 原始错误可能包含秘密，用户只看安全可恢复摘要；原始响应/令牌/内部路径不进页面或 console。

## UI 与可访问性

使用用户选择的 AI 装备工坊视觉系统（最新 iOS 风格：中性灰白/黄色/薄荷绿，兼容深色）；具体 token 和布局规则以 DESIGN.md 为准。字号、间距、圆角和焦点统一；className 优先用于布局，颜色走组件变体。按钮用语义 button，链接用 a，不做伪点击 div。表单有 label，异步状态有 live region，图表和状态不能只靠颜色区分。

375/390px 窄屏无页面横向溢出，代码块可自身滚动；长中文/URL 可换行。按钮覆盖 hover/focus/disabled/pending，遵循 prefers-reduced-motion。未实现功能以说明呈现，不出现无行为的“登录/创建令牌”按钮。

## TDD

先写失败测试再实现：成功契约、网络失败、HTTP 错误、非法 JSON/字段、重试和取消。组件测试使用真实 QueryClient（每例独立），只 mock 网络边界；集成测试使用真实 Go 服务。用 user-event 操作、role/name 查询；禁止源码字符串测试、任意 sleep、snapshot 代替关键断言。

执行：`pnpm --dir web test` → `pnpm --dir web typecheck` → `make check`。CI 保留 harness 日志；DOM 测试不作为视觉通过证据。本次工坊重设计用户已明确授权浏览器自动化与保留原画的本地抠图，使用实际浏览器检查响应式、计算样式、破图及页面横向溢出，并人工检查桌面、手机及两个主题；这是本任务授权记录，不修改 AGENTS.md。具体 RED/GREEN 工作流见 HARNESS.md。

## 工坊素材与视觉记录

Web 正文优先自托管 Manrope 与中文系统字体；桌面端优先系统 UI 字体；Caveat 只用于局部手写注释，许可见 docs/third-party/Caveat-OFL.txt。七张 workshop WebP 统一使用透明 alpha；详细尺寸、字节和 token 见 DESIGN.md。大屏主内容最大 1360px，业务表格与代码局部滚动，不能导致页面横向越界。

本轮覆盖 18 条路由、浅深两主题、390/1440/2560px 宽度，共 108 张主截图及 20 张手机底部截图。人工审阅与行为测试证据、未完成的最终检查统一记入 docs/superpowers/plans/2026-09-10-workshop-ui.md。

登录采用最大 1040px 的双栏 auth-card，900px 以下隐藏欢迎插画，仅保留表单。登录卡/面板/按钮圆角分别为 32/16/12px。加载页面提供原创 workshop-loading 插画、语义标题与骨架，加载状态来源于真实请求，遵循 reduced-motion。新增素材生成出处和尺寸见 DESIGN.md；本轮最终验证结果由实施记录保存，旧截图统计不替代新页面回归。

## 页面 head 与应用图标

Web 的 index.html 提供中文初始标题、产品描述、theme-color 与 color-scheme，运行时标题随实际语言更新。favicon 从原创 workshop-mark 本地派生：透明 16/32/48px ICO 和 16/32px PNG；apple-touch-icon 为中性底 180px，manifest 引用透明 192/512px PNG（purpose any）。图标以同一角色与安全留白保持一致；不使用生产站绝对 URL。site.webmanifest 只声明应用展示元数据，未注册 Service Worker，不承诺离线可用。

桌面 index.html 使用“Loadout · 本地接入”初始标题与打包内相对 favicon，不硬编码 dark class；主题由现有设置逻辑决定。两端构建后应检查 HTML/manifest 引用都落在各自 dist 内。


公共首页为 LandingPage，账户概览使用 /overview。品牌专用 Bricolage 字体只用于字标，许可与原始字体来源随仓库保存。偏好控件前置图标必须在可点击 trigger 内；选中、hover、键盘焦点分别表达。桌面侧栏可收起为图标栏，所有链接保留可访问名称；账号退出在右上菜单，失败可重试。左下账号设置入口是导航，不直接执行退出。


## 有数据后的列表规范

主列表采用usePagedList + Pagination，默认10条，可选20/50；基于API真实next_cursor，返回上一页使用Query缓存，当前范围按实际已加载条数计算，不显示未知总数。筛选/容量变化回第一页，失败保留当前页且可重试；空结果不显示无效翻页控件。翻页成功后通过显式listRef滚动至列表开头并转移阅读焦点；令牌团队候选与成员交接候选仍保留已加载选项集合。导出筛选清除时reset mutation，不能沿用旧下载与游标。

数据表桌面保持表格与数字右对齐、表头局部固定，手机将同一语义表格排为带字段名称的卡片，列头保留可访问语义。列表元信息紧凑，语义状态使用颜色加文字；概览真实指标优先于接入指南。当前验收见2026-09-11-data-pagination-polish.md。
