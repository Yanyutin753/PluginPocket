# Web 首个产品模块执行记录

日期：2026-09-10。范围：账号登录/注册/退出、概览、个人令牌、个人用量游标分页、管理员用户列表/幂等调账、响应式应用壳。其余产品模块由后续实现扩展；此记录不代表完整产品验收。

## 已实现行为与验收映射

| 需求 | 实现和验证 |
|---|---|
| 登录、注册、退出 | Cookie same-origin 请求；真实 QueryClient；键盘填表提交；退出成功清理缓存；退出失败保留页面并可再次操作 |
| 401 | Query/Mutation 缓存事件统一处理过期会话，清除缓存并返回登录；错误密码安全提示 |
| 概览 | 校验 account/me 数据，显示真实余额与 summary；错误不显示虚构值并提供重试 |
| 令牌 | 创建/命名/一次明文/复制/隐藏/离页清理；明文不进入 Query 或 Mutation 缓存；列表分页；撤销需明确二次确认 |
| 用量 | limit=50，使用服务器 next_cursor，追加而非覆盖；失败保留已有记录，可重新加载；成功/失败/处理中/拒绝/恢复文字状态 |
| 管理员 | 隐藏普通用户入口且直接访问显示无权限；服务端仍是权限边界；列表分页；调账金额/备注；不确定失败保留同一 idempotency_key 与请求内容 |
| 响应式 | 320–767 单列/展开导航，768–1199 紧凑导航/双列，1200+ 侧栏；触控最小 44px；表格局部横向滚动及键盘焦点；所有功能保留 |
| 路由和状态页 | React Router；页面 React.lazy 分包；原健康检查移到 /health，保留超时、取消、离线手动重试与所有原回归 |
| 错误边界 | Zod 校验 API unknown、新令牌前缀与时间格式；原始响应、内部错误细节不显示到页面 |

## 技能与官方核验

已读取项目 PRODUCT.md / DESIGN.md / FRONTEND.md、产品 spec / ADR，使用 Ponytail full、TDD、Impeccable new-work / craft-floor。Impeccable context 在此工作开始时执行一次，沿用用户确认的中文深色开发者工具系统；不重新索要已授权设计确认。

- `pnpm view react-router version --registry=https://registry.npmjs.org` 返回 8.3.1，已精确固定并更新锁文件。
- `pnpm --dir web exec shadcn docs input button label`、`docs field`，并读取官方文档。
- 用仓库固定的 shadcn CLI 加入 Input / Label / Field；原 Separator 未覆盖；消费现有 `cn`，没有新建重复工具。
- React Router 官方安装：https://reactrouter.com/start/declarative/installation
- shadcn：https://ui.shadcn.com/docs/components/radix/field 、https://ui.shadcn.com/docs/components/radix/input

## RED → GREEN → REFACTOR 顺序证据

使用 Node 26.8.2 / pnpm 12.3.4，命令前将 `/home/yangyang/.nvm/versions/node/v26.8.2/bin` 加入 PATH。

1. **RED（22:00:25）**：`pnpm --dir web test src/Product.test.tsx`，8/8 失败。现有页面仅健康检查，找不到登录标题、令牌操作、用量记录与管理调账行，属于未实现用户行为。初次编写的管理员用例缺少断言，运行正式 RED 前已补全；没有将该错误计为有效证据。
2. **GREEN（22:03:21）**：实现 API 边界、路由、页面后同一命令，8/8 通过。退出缓存断言最初对 undefined 使用 not.toContain 导致断言 API 类型错误，改为更直接的 toBeUndefined 后通过；未弱化需要清空缓存的要求。
3. **RED（22:04:14）**：增加恢复/导航消费行为测试后，14 例中 1 失败：创建接口给出无 ldt_ 前缀的字符串时页面仍将其显示为可用令牌，缺少期望的安全错误。其余 5 例是已有行为的额外消费回归，不声称它们经历新的 RED。
4. **GREEN（22:04:28）**：创建令牌响应 schema 增加 ldt_ 格式校验；同一命令 14/14 通过。
5. **REFACTOR（22:05:41）**：表单改为官方 FieldGroup / Field / FieldLabel 消费，保持交互和错误行为；同一测试 14/14、类型检查通过。
6. **RED（22:06:00）**：新增非法时间戳用户用例，15 例中 1 失败，并复现 `Intl.DateTimeFormat.format` 的 Invalid time value。根因是列表日期字段仅 z.string，API 信任边界不完整，页面无法提供恢复入口。
7. **GREEN（22:06:12）**：令牌及用量共享日期契约使用 Zod ISO datetime（接受时区偏移），在 API 解析阶段拒绝错误数据；同一测试 15/15 通过，无未处理异常。

开发命令输出临时保存在 `/tmp/loadout-web-red.log`、`/tmp/loadout-web-red2.log`、`/tmp/loadout-web-red3.log`（临时文件不是持久 CI 工件）；上方记录保存实际时间、失败根因和验收映射。

## 验证与限制

本模块最后执行的独立检查：

- `pnpm --dir web test src/Product.test.tsx`：15 个业务组件测试通过。
- `pnpm --dir web test`：2 个测试文件、27 个测试通过，包括 12 个原健康 API 消费回归。
- `pnpm --dir web typecheck`：通过。
- `pnpm exec biome check web/src web/package.json`：通过。
- `pnpm --dir web build`：通过；输出独立 Auth/Overview/Tokens/Usage/Admin/Health 页面 chunk。
- `impeccable detect --json web/src/App.tsx web/src/HealthPage.tsx web/src/features/account web/src/styles.css`：执行一次，2 条字号 advisory；主标题已改回 DESIGN.md 的 1.875rem，1.125rem 小标题与 DESIGN.md 正文描述一致而保留。检测器是源码检查，不是屏幕渲染。

组件测试只替换 fetch 网络边界，真实 QueryClient / Mutation / Router；user-event 提供键盘、剪贴板消费行为。未使用 Playwright 或任何浏览器自动化。**未进行桌面、平板、手机真实屏幕人工视觉检查**，DOM 测试不能作为视觉通过证据。

本子任务不启动正在并行开发的后端，不声明真实 Cookie/数据库端到端已通过。根代理在所有模块汇合后执行完整 `make check` 与真实 HTTP 集成；该最终结果需记录在 product-execution.md。本模块未提交、推送、发布或修改真实客户端配置。

## 独立审查后状态隔离修复（22:33–22:35）

后续完整运营模块见总产品执行记录。本次由 CLI 代理只读审查 Web 后，使用真实 QueryClient / App / Router、仅替换 fetch 的测试正式重现两项缺陷：另一标签页切换 Cookie 后，当前页面刷新 account/me 为 Bob，但仍保留 Alice 一次性令牌及列表；直接在两个团队详情路由之间切换，旧团队邀请码继续显示。

- **RED 22:33:59**：`pnpm --dir web exec vitest run src/SessionIsolation.test.tsx`，2/2 失败，分别断言发现 `ldt_alice_secret` 与 `invite_for_team_one` 仍在 DOM。
- **GREEN 22:34:16**：同一命令 2/2 通过。SessionBoundary 订阅账号身份，身份变化时替换 QueryClient 并重新挂载路由树；旧请求回调只能访问已退休客户端。团队详情使用团队 ID 作为 React key，清理旧邀请码及表单状态。
- **REFACTOR / 扩展回归 22:34:44**：同一测试 3/3 通过；加入 StrictMode，并验证旧账号在途团队充值响应晚到后不能回填新账号页面。第三例为架构消费回归，不声称独立 RED。
- **最终本地检查 22:34:55**：`pnpm --dir web test` 4 文件 55/55；`pnpm --dir web typecheck`、`pnpm exec biome check web`、`pnpm --dir web build`、`git diff --check -- web` 均通过。

本轮仍没有浏览器自动化或真实桌面/移动屏幕视觉验证；完整 make check 由根代理汇合时执行。
