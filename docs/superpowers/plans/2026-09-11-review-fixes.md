# 审查问题修复执行计划

**Goal:** 修复本次鉴权、边界与标准 lint 审查确认的问题，并使完整 harness 可重复运行。

**Architecture:** 沿用事务内授权校验、官方 MCP SDK、TanStack Query 分页及标准检查工具。仅修复已确认根因，保留当前工作区其他改动。

**Tech Stack:** Go / PostgreSQL / Redis、React / TypeScript / Biome、Node test runner、Rust Clippy；golangci-lint 固定 2.13.2。

**Spec:** `docs/PLAN.md`、`docs/API.md`；本次审查原始证据为 `/tmp/loadout-server-auth-review.md`、`/tmp/loadout-server-boundary-review.md`、`/tmp/loadout-language-lint-review.md`。

## 实施与验收

- [x] 网关：将工具状态/价格重验放到钱包锁后；无歧义工具名称；完整有界目录发现；数据库故障保留可重试错误。`gateway_fixes` 负责 gateway/store，RED/GREEN 见独立执行记录。
- [x] 账号：管理员/团队锁后会话授权；工具 Schema 拒绝；省略 config 保留当前凭证；匿名按请求族与调用方限流；设备签发时过期检查。`app_fixes` 负责 app/auth，RED/GREEN 见独立执行记录。
- [x] 邮件身份：数据库故障返回 5xx；重发失败保留旧链接且并发安全；复用匿名限流。`identity_fixes` 负责 identity，RED/GREEN 见独立执行记录。
- [x] 汇总：在 `web/src/Operations.test.tsx` 添加第二页失败后键盘重试、保留第一页、切换时间范围重置分页的行为测试；先运行 `pnpm --dir web exec vitest run src/Operations.test.tsx -t 'summary pages'` 确认 RED，再让 `UsageSummary` 复用 `listOptions` / `useInfiniteQuery` 和既有分页控件，GREEN 后运行相关 Web 回归。同步 API/OpenAPI 的游标契约与套餐公开范围。
- [x] 开发进程：现有 `tests/dev.test.mjs` 的 unexpected launcher exit 是复现入口；控制 socket 返回真正子进程 PID，测试用本 fixture socket 的 PID 注入故障，替代依赖 `ps` 标题。先通过测试断言缺失 PID 确认 RED，再实现，验证服务和控制状态均清理。
- [x] 标准检查：根据现有 golangci 诊断处理错误返回值与代码规范；导入排序及 tidy 用标准工具生成。添加固定版本工具入口和 golangci 配置，接入 `make lint` / `make format` / CI；不关闭规则掩盖问题。
- [x] 最终复核：检查各修复 diff，运行聚焦回归、`make lint`、`make check`，保存真实日志。测试环境只使用隔离测试数据库/Redis；不修改真实客户端配置、不提交或推送。

## 验证记录

命令结果随执行追加。格式整理、类型错误修复和工具配置使用真实检查器验证，不为它们编写镜像实现测试；业务行为保持先 RED 后 GREEN。

### 主代理 RED / GREEN

- 汇总：`pnpm --dir web exec vitest run src/UsageSummary.test.tsx` RED（2失败，真实组件缺少“加载更多”入口）；复用 listOptions/useInfiniteQuery 与 More 后同命令 GREEN（2通过，团队/全局第二页失败→键盘重试→无丢行→切日期重置）。后台原有分页行为未改，增加51分组消费契约回归。
- 启动器：`node --test --test-name-pattern='unexpected launcher exit' tests/dev.test.mjs` RED（控制状态缺少实际 launcher PID）；控制socket输出已持有的child.pid后同命令 GREEN（1通过，SIGKILL之后服务端口关闭且状态stopped）。
- TypeScript：原 `pnpm --dir web typecheck` 报 TS2769；只将不支持的 exact 参数替换为精确正则 name，不改变键盘操作断言。
- 聚焦回归：`pnpm --dir web exec vitest run src/UsageSummary.test.tsx src/Operations.test.tsx src/Product.test.tsx src/Localization.test.tsx` GREEN（4文件59项通过）。
- 配置：通过 `golangci-lint config verify`；Make目标实际从固定版本官方脚本安装2.13.2成功；go.mod/go.sum已由 `go mod tidy` 整理。已有lint诊断作为工具配置和格式修改的验证依据，不将编译/lint失败伪称业务TDD RED。

### 完整检查第一轮

`make check` 使用Node26.8.2与隔离测试PG/Redis，exit2，日志 `/tmp/loadout-final-check-first.log`。标准lint（0 issues）与桌面lint通过；Go所有业务包通过，但cache重连fixture在已显式Close后又执行检查式defer Close，第二次Close的已关闭错误导致失败。保留正文的Close错误及watch退出断言，defer仅作容错清理，不改变生产cache语义。

### 并行界面改动后的检查

完整检查在新引入的Select替换期间两次于Biome/TS停止（日志second/third），之后新增Landing测试格式导致第四轮停止；均未跳过检查或回滚其他任务内容。使用标准Biome整理，并移除Testing Library不支持的ByRoleOptions.exact（字符串name本身精确匹配），修订原生select断言以操作实际Radix弹层并验证aria-selected和关闭后标签。此处属于消费方组件迁移测试适配。

Web全量测试出现冷懒加载仍显示骨架时的一秒默认查询超时；Product文件单独15项通过，降低并发仍在冷加载超时。把Testing Library条件等待预算调整至3秒（仍在Vitest单测试期限内），不添加固定sleep或删除断言，继续全量验证。

### 固定窗口测试的时间边界

第五轮完整检查：Go app/auth/cache/gateway/settings/store全部通过，identity的 `TestGitHubStartBudgetAndExpiredStateCleanup` 因121次调用跨过整分钟最终302，失败日志 `/tmp/loadout-final-check-fifth.log`。该预算按PG固定窗口刷新，原测试错误地假定循环期间时钟不会越界。保留真实handler、真实PG及121次请求，首次302后将此私有schema的预算行窗口固定为未来窗口，逐次断言前120次302、第121次429；窗口前进/回退另由store回归覆盖。聚焦 `go test -race ./internal/identity -run TestGitHubStartBudgetAndExpiredStateCleanup -count=1` 退出0，2.119s。未改动生产限流规则。

等待预算调整后的Web全量为97通过，只有另一任务尚未实现的PublicHome用例RED；产品、系统配置和所有既有回归已通过。待其他任务路由改动稳定后继续完整检查，不把移动中的工作区结果写为最终绿灯。

### 会话监听器订阅空窗（追加根因修复）

3秒条件等待仍发现已有登录组件加载后的401未跳转，不能归为冷加载。新增 `SessionExpiry.test.tsx` 在App挂载前由真实QueryClient执行失败401查询，并预置私有流水缓存；要求进入登录页、清除缓存与保护操作。

- RED：`pnpm --dir web exec vitest run src/SessionExpiry.test.tsx` 退出1，仍在/tokens受保护页面，日志 `/tmp/loadout-session-expiry-red.log`。
- 最小修复：SessionEvents订阅Query/Mutation后立即检查已记录的错误；已有401执行相同清理/导航逻辑，首次命中后停止扫描，invalid_credentials继续保留表单错误。
- GREEN：`pnpm --dir web exec vitest run src/SessionExpiry.test.tsx src/Product.test.tsx src/SessionIsolation.test.tsx` 退出0，3文件19项通过，日志 `/tmp/loadout-session-expiry-green.log`。

### 登录页迟到401保留回跳

第六轮full check通过所有Go竞态及Rust CLI测试，但Web设备回跳用例失败。登录页出现后的第二个401会把location.state.from覆盖成/login，登录后错误回overview。增强既有Operations设备用例：先进入登录页，再由真实QueryClient返回迟到401，再登录，断言设备码保持且未自动批准。`pnpm --dir web exec vitest run src/Operations.test.tsx -t 'returns to a requested device'` RED退出1（日志 `/tmp/loadout-session-return-red.log`），SessionEvents只在当前非/login时执行导航、已有登录页保留from，同命令GREEN退出0（`/tmp/loadout-session-return-green.log`）。原会话扫描再加effect局部handled防同批重复处理，19项相关回归通过。

完整Web回归 `pnpm --dir web test` 退出0，15文件99项通过，日志 `/tmp/loadout-web-final-after-session.log`；独立会话检查确认订阅需要补读现有state、clear发removed不会直接递归，账号隔离4项独立通过。

### 支付能力异步边界（真实服务E2E发现）

第八轮make check通过标准lint、所有Go竞态、Rust、Web99项与桌面测试，但真实Web production E2E发现套餐已加载而meta尚未返回时购买按钮短暂可用。原判断只有payments===false禁用，undefined被误当可购买。新增Operations测试以未完成Promise固定meta加载阶段，再503失败、重试false，逐阶段断言禁用与错误恢复。

- RED：`pnpm --dir web exec vitest run src/Operations.test.tsx -t 'keeps purchases disabled'` 退出1，按钮在meta待定时未禁用；日志 `/tmp/loadout-payment-capability-red.log`。
- GREEN：仅在meta成功且payments=true时允许购买，并复用ErrorNotice提供meta失败重试；相同命令退出0，日志 `/tmp/loadout-payment-capability-green.log`。
- 既有下单503回归fixture显式声明payments=true，仍验证能力公告后供应商失效不能返回虚构成功；通用fixture提供真实meta形状，避免错误的空列表响应。

### 同类限流测试时间边界收敛

第九轮检查在app清理限流行测试跨分钟时触发400而非429（`/tmp/loadout-final-check-ninth.log`），同根因不只存在于OAuth单例。检查所有120次预算测试后，为app与identity各增加一个私有fixture窗口固定helper：首次真实请求创建计数后才固定窗口，继续保留全部请求次数、跨副本/重启、不同peer/flow、错误method及过期行清理断言。生产rate算法未变。聚焦app四项预算回归 `go test -race ./internal/app -run 'TestAuthenticationBudget|TestAuthenticationEndpointsRejectExcessiveAttempts|TestDistributedAuthenticationBudget' -count=1` 通过（4.521s）；identity及store时钟边界 `go test -race ./internal/identity ./internal/store -run 'TestIdentityBudget|TestGitHubStartBudget|TestRate' -count=1` 通过（4.878s/1.089s）。

### 用户实际系统配置页404

用户截图显示通用请求错误。只读实测API8787与Vite5173代理的 `/api/v1/admin/settings` 均404；运行PID1338333仍持有00:01启动的旧Go可执行文件（已被新构建替换但进程未重启）。按用户已授权修复，执行本地 `make restart`，未改.env或保存配置值。之后匿名请求401；使用现有本地管理员凭证真实登录200、GET系统配置200、公开字段形状正确且两个secret原文字段不存在，最后退出该验证会话204。没有输出密码/Cookie/秘密值。补充README与环境文档说明前端HMR与Go程序升级的区别。

## 最终验收

使用Node26.8.2、仅向测试进程传入隔离PG/Redis变量，最终 `make check` **退出0**，完整日志 `/tmp/loadout-final-check.log`。标准检查包括Biome/TypeScript、golangci-lint 2.13.2（0 issues）、gofmt/goimports、go mod tidy -diff、Rust fmt/Clippy；所有Go包-race、Rust CLI与原生桌面测试、Web103项、桌面组件7项、生产/开发各3项真实Web E2E、MCP/CLI/Redis故障/多副本/恢复/桌面bridge链路、前台进程4项、后台进程6项及最终HTTP/CLI集成4项均通过。没有跳过完整检查阶段。

最终界面增加侧栏与账户菜单两处用户名后，原会话隔离用例全局findByText变得不唯一；将两处定位限定到实际账号菜单，保留旧账户秘密/缓存和迟到mutation不可污染新账户的原断言。聚焦3项通过，随后上述完整检查通过。菜单交互同步到真实E2E的登录/退出帮助函数，生产/开发均实际验证成功。

本轮不执行浏览器自动化；未做桌面/手机人工视觉验收，也未连接真实GitHub/外部SMTP供应商（使用本地OAuth/SMTP fixture）。本地开发服务已因用户截图中的旧后端404执行一次make restart，并验证管理员配置GET200。未改写真实.env、未通过配置页保存业务值、未提交或推送。

完整检查结束后，另一任务继续替换账户头像图片并改动样式；保留其内容，标准化两处格式，再对该前端变化追加 `pnpm --dir web test`（15文件103项通过，`/tmp/loadout-post-check-web.log`）和 `make lint build-web`（退出0，`/tmp/loadout-post-check-lint-build.log`）。`git diff --check`退出0；本地make status仍running。完整harness日志对应头像替换之前的运行；头像替换之后的验证范围为上述lint、Web组件与构建，不冒称再次执行全部后端/原生链路。
