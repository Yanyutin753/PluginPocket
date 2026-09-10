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

新增路由时统一评估成熟路由库；首次复杂表单引入表单库。当前只有一个状态页，不安装未使用的路由、全局状态、图表和表单依赖。避免为了“标准化”同时安装多个职责重叠的库。

## 目录与依赖方向

`src/components/ui/` 为基础组件，`src/lib/` 为项目级通用能力，业务代码随需求按 `features/<name>/` 聚合。当前 App 和健康 API 文件足以承载单页，不预建空业务目录。UI 不直接耦合 HTTP URL；API 函数处理契约，组件处理展示，Query 管生命周期。

TypeScript strict 开启，禁止 any、无说明 ts-ignore、非空断言掩盖边界错误。组件使用具名 props 类型；只导出被实际使用的 API；不滥用 memo/useMemo/useCallback。

## 状态与请求

- 服务端状态归 Query；局部交互归 useState；可分享的筛选将归 URL。不可把所有状态搬进全局 store。
- 网络请求传 AbortSignal、合理超时；组件卸载取消；显示 loading/error/empty/success。
- 健康检查禁用 HTTP 缓存和自动重试，用户主动重试；即使 navigator 报告离线也要尝试检查本地服务，失败后清除已连接展示；后台查询按业务设置 staleTime，不沿用健康检查设置到所有查询。
- API 原始错误可能包含秘密，用户只看安全可恢复摘要；原始响应/令牌/内部路径不进页面或 console。

## UI 与可访问性

使用用户选择的深色开发者工具风格；具体 token 和布局规则以 DESIGN.md 为准。字号、间距、圆角和焦点统一；className 优先用于布局，颜色走组件变体。按钮用语义 button，链接用 a，不做伪点击 div。表单有 label，异步状态有 live region，图表和状态不能只靠颜色区分。

375/390px 窄屏无页面横向溢出，代码块可自身滚动；长中文/URL 可换行。按钮覆盖 hover/focus/disabled/pending，遵循 prefers-reduced-motion。未实现功能以说明呈现，不出现无行为的“登录/创建令牌”按钮。

## TDD

先写失败测试再实现：成功契约、网络失败、HTTP 错误、非法 JSON/字段、重试和取消。组件测试使用真实 QueryClient（每例独立），只 mock 网络边界；集成测试使用真实 Go 服务。用 user-event 操作、role/name 查询；禁止源码字符串测试、任意 sleep、snapshot 代替关键断言。

执行：`pnpm --dir web test` → `pnpm --dir web typecheck` → `make check`。CI 保留 harness 日志，不使用浏览器自动化验证；DOM 测试不作为视觉通过证据。具体 RED/GREEN 工作流见 HARNESS.md。
