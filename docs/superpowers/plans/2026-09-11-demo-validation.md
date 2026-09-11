# 后端演示数据与 MCP 热重载验收

**目标：** 生成可浏览、可调用的本地演示数据，并重新运行全部功能 harness。

**设计：** 在独立 PostgreSQL schema 和本地端口运行现有服务；使用真实 REST API 写入用户、令牌、套餐、兑换码、团队、设备及账本。本地官方 MCP SDK 上游提供成功与失败场景。默认不改变当前 `.env`、业务数据或客户端配置。外部支付保持未接入状态。

**约束：** 复用现有依赖与测试 fixture；当前工作区完成；无浏览器自动化；不提交或推送。

**用户后续明确要求：** “数据库也要加 是真的加上 在5173”。因此隔离验证完成后，已获授权将相同业务数据写入当前 `.env` 数据库；保留原管理员、原密码与原配置，不清空已有数据。具体实查见后文。

- [x] 演示工具：`server/cmd/loadout-demo/`；先测试通过真实服务准备数据后账号、团队、流水可读及余额一致，再实现。重复初始化明确失败，防止意外重复加钱。
- [x] MCP 热重载：`server/cmd/loadout-server/hot_reload_e2e_test.go`；保持两个服务副本与同一个 Rust bridge 存活，测试添加、替换地址/工具 schema、改价、禁用、重新启用及旧调用完成。已存在行为首次通过如实标记为回归覆盖；发现缺陷再 RED/GREEN 修复。
- [x] 聚焦回归、`make check`、运行演示环境并核验 REST/MCP，补录实际命令与未验证范围。

## 验证记录

测试命令在 `server/` 下执行，使用从 `.env` 加载的隔离测试 PostgreSQL/Redis URL。产品测试同时设置绝对路径 `LOADOUT_SERVER_BINARY` 与 `LOADOUT_CLI_BINARY`；没有读取或写入真实客户端配置。

| 阶段 | 实际命令与结果 | 证据（忽略目录） |
|---|---|---|
| RED 基础数据 | `go test ./cmd/loadout-server -run TestProductJourneyDemoData -count=1 -v`：空壳实现下 `demo accounts=0 want=4` | `build/demo-red.log` |
| GREEN 基础数据 | 同命令通过；钱包余额等于账本汇总 | `build/demo-green.log` |
| RED MCP mock | 同命令，404 fixture 导致 `demo MCP scenario demo__success failed` | `build/demo-upstream-red.log` |
| GREEN MCP mock | 使用官方 SDK HTTP server；成功扣费、失败退款有真实流水 | `build/demo-hot-reload-green.log` |
| RED 命令入口 | `go test ./cmd/loadout-server -run TestProductJourneyDemoCommand -count=1 -v`：空入口不报告数据就绪 | `build/demo-command-red.log` |
| GREEN 命令入口 | 同命令通过，真实网页 API 可登录并读取团队 | `build/demo-command-green.log` |
| RED 管理员 | `TestProductJourneyDemoData`：管理员有效令牌数为 0 | `build/demo-admin-red.log` |
| RED 大批量 | 同测试：`demo accounts=4 want=60` | `build/demo-volume-red.log` |
| GREEN 大批量 | `go test -race ./cmd/loadout-server -run TestProductJourneyDemoData -count=1 -v` 通过，24.52s | `build/demo-volume-green.log` |
| RED 失败分布 | 同测试：实际退款错误 1 条，期望至少 100 条 | `build/demo-errors-red.log` |
| GREEN/REFACTOR | `go test -race ./cmd/loadout-server -run 'TestProductJourney(MCPHotReload\|Demo)' -count=1 -v` 三项通过，61.463s；复用请求/清理、格式化，无新依赖 | `build/demo-final-focused.log` |
| RED 取消清理 | `go test ./internal/demo -count=1 -v`：已取消仍发 69 次 logout，登录失败总计 70 次请求 | `build/demo-cleanup-red.log` |
| GREEN 清理修复 | `go test -race ./internal/demo -count=1 -v` 两项通过；只清理已有 Cookie 会话，遵循原 context，单次 1 秒上限，失败立即退出循环 | `build/demo-cleanup-green.log` |

热重载测试最初对低层 `Server.AddTool` 的自动入参验证作了错误假设，出现 `replacement did not update required input schema`。核对锁定 SDK 源码后改用具备入参验证的 `mcp.AddTool` fixture，并直接断言返回的 required schema。此失败属于测试 fixture 修正，**不算生产网关缺陷 RED**。修正后现有网关通过双副本、同一存活 bridge、替换地址/schema、旧请求完成、禁用/启用及新旧价格验证，没有修改网关生产逻辑。

用户追加“admin也是”“多来点数据”后，扩充为 60 普通用户和 1 管理员；管理员有独立资金、60 令牌、11 个运营团队。`-more` 用于将本轮已启动的 4 用户数据扩充一次，重复运行拒绝。普通用户中的 5 个批量账号和 `demo_disabled` 停用，第三个设备令牌吊销，每四次调用包含一次真实无效入参错误；错误正常退款。

独立只读代码审查发现并复现了取消清理问题，修复后复审确认关闭，无遗留高/中优先问题。

## 实际演示环境

配置：`.loadout/demo.env`（0600）；schema：`loadout_demo_20260911`；独立 Redis namespace 同名。Web `5174`、API `8788`、mock MCP `8790`。原 `.env` 和 `5173` 实例未改。

通过真实 Vite 代理运行 seed 与一次 `-more` 后，PG 直接查询得到：61 用户、234 令牌、12 团队、24 成员关系、12 套餐、60 兑换码、704 用量、987 账本。532 条成功总扣 538；169 条错误与 3 条拒绝均 cost=0；钱包/账本不一致数为 0。查询输出：`build/demo-live-counts.log`。

管理员通过真实 HTTP 登录，个人与管理端共 14 个入口返回 200；令牌、用户、兑换码、用量、账本的第二页均有实际数据，注销 204。记录：`build/demo-admin-live.log`。支付订单为空符合尚未接入支付的契约。

### 5173 当前数据库实际写入

用户明确要求后，先通过 5173 登录原管理员，检查到原库只有 1 个用户、没有 `demo_*` 账号；`LOADOUT_ALLOW_PRIVATE_UPSTREAMS=false`。没有更改该配置或重启服务。

实际执行 `make build-demo`，然后以 Node 26 从 `.env` 加载原管理员凭据运行 `./build/loadout-demo -origin http://127.0.0.1:5173`，退出 0。全部写操作经现有 REST/MCP，不直接修改钱包或伪造用量。

之后用 `.env` 中 `LOADOUT_DATABASE_URL` 直接连接 PostgreSQL 核对：61 用户、234 令牌、12 团队、24 成员关系、12 套餐、60 兑换码、702 调用、984 账本；531 成功扣 531，168 错误和 3 拒绝均不扣费；钱包/账本不一致数 0。记录 `build/demo-5173-seed.log`、`build/demo-5173-counts.log`。

原管理员通过 5173 的 14 个个人/管理 API 均 200，令牌/用户/兑换码/用量/账本第二页均有数据，注销 204，记录 `build/demo-5173-admin.log`。原管理员的 60 个令牌、11 个运营团队、20 条本人调用可见；原密码没有修改。私网上游不启用，因此该库比隔离 mock 环境少 2 次远端 mock 调用及其对应 3 条账本记录。

## 完整检查

第一轮从仓库根运行（Node 26.8.2 加载 `.env` 中测试 URL）`make check` 退出 0，日志 `build/demo-harness.log`，包含真实 Go/PG/Redis、Web/桌面组件、Rust CLI、Linux deb、产品 E2E、进程/开发生命周期和 HTTP 集成。

取消清理修复后再次 `make check`，同期前端正在修改的 `App.tsx`、`AuthPage.tsx`、`styles.css` 格式检查失败，退出 2，记录 `build/demo-harness-final.log`。随后仅对这些文件、`web/src/features/account/shared.tsx` 和 `desktop/ui/src/styles.css` 执行 `biome format --write`，保留已有界面内容；没有将其他人的前端实现计入本任务成果。Biome 随后通过，继续重新执行完整入口。

格式整理按项目要求读取 Impeccable/产品/设计/前端规范，并执行一次机械 detector。它报告既有字体/字号等设计建议（含 DESIGN.md 正文已记录的 Bricolage），这些不是格式化新增问题，也不是本次数据任务的视觉验收；未据此重做界面。记录 `build/demo-format-detector.json`。

### 最终结果

最终根目录 **`make check` 退出 0**，完整日志 `build/demo-harness-complete.log`。实际启动命令使用 Node 26 从 `.env` 加载测试 URL，再执行 `make check`；所有数据测试均使用独立 schema/Redis namespace，当前 5173 数据仅由上述显式 seed 写入。

通过项包含四端 lint/格式/typecheck、Go 全包 race（含清理回归）、React 与桌面组件、Rust CLI/桌面原生测试、生产构建与 Linux deb、批量演示命令及真实 mock 进程退出、完整数据账本断言、双副本 MCP 热重载、生产/开发 Web E2E、Redis/多副本/崩溃恢复、进程生命周期、6 条开发管理测试及 4 条 HTTP/CLI 集成。

完整入口运行期间，另一个前端任务继续增加插画类型与 Heading 参数；对 `shared.tsx` / `ToolsPage.tsx` 额外做了纯格式化，保留内容。追加 `pnpm exec biome check --error-on-warnings .` 通过，`git diff --check` 通过。前端并行实现不属于本任务的改动成果，实际视觉仍由其任务验收。

未提交、推送或修改真实客户端配置。5173 原管理员凭据不变，当前数据库里的演示数据已经持久化，普通重启不会丢失。

未做浏览器自动化、真实桌面/移动视觉或原生 GUI/托盘验收，未连接外部 OAuth/SMTP/支付供应商，也未执行其他 OS 测试。mock/组件/HTTP 测试不能证明这些环境通过。
