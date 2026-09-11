# 开发 Harness

Harness 是同一套可执行的本地/CI 验证入口。所有命令从仓库根运行；工具链按 `.node-version`、`server/go.mod`、`rust-toolchain.toml` 和 `packageManager` 固定。

## 工作循环

公共插件路由增加 `PluginsPage.test.tsx` 匿名浏览/筛选/重试与 `PluginProxy.test.ts` 真实 Vite 代理；分布式进程旅程覆盖全部 React 页面直达、资源、公开目录与真实 Git clone/pull，再停掉一个副本复验。Git registry 跨副本历史、原子发布及 SQLite 失效触发器各有回归测试。

1. 读 PLAN / API 与任务设计，写最小行为验收。
2. 添加测试。实现不存在时先用可编译空壳建立测试入口，再运行到具体断言失败；编译/依赖错误仅为准备阶段，不算有效 RED。
3. 记录 RED 命令、断言和失败原因，确认失败对应缺失行为。
4. 最小实现，运行同一测试 GREEN；此时再提取重复或调整结构，测试必须持续绿灯。
5. 运行相关模块测试，最后 `make check`；记录实际结果，不能以先前测试或其他人的结论代替本次验证。

```sh
# Go
cd server && go test ./internal/httpapi -run TestHealth -count=1
# Rust（仓库根）
cargo test --manifest-path cli/Cargo.toml --test doctor
# React（仓库根）
pnpm --dir web test
```

## 入口

桌面发行预检 `node --test tests/release-preflight.test.mjs` 已进入 `make check`，覆盖非法 tag、版本漂移和缺失签名配置。发行 workflow 使用原生多平台 runner，运行 desktop/UI 测试、解包 bridge 验证及独立 Minisign 验签；远程 CI 未运行时不得宣称跨平台已验收。详见 [桌面说明](../desktop/README.md#桌面发行)。

| 命令 | 检查 |
|---|---|
| `make help` / `make` | 列出可用命令 |
| `make setup` | 按锁文件安装 JS、Rust、Go 依赖 |
| `make dev` | Air 自动编译重启 Go + Vite；退出时停止组合进程 |
| `make up` / `make down` | 后台启动并等待就绪 / 关闭当前实例 |
| `make restart` / `make status` / `make logs` | 顺序重启 / 状态与地址 / 跟随日志 |
| `make ready` | 完整 `check` 成功后再 `up` |
| `make test` | Go race 测试、Rust 黑盒测试、React 行为测试 |
| `make lint` | Biome、TS、golangci-lint（含 go vet/staticcheck/errcheck）、gofmt/goimports、go mod tidy -diff、rustfmt、clippy；只检查不改文件 |
| `make format` | 显式自动格式化 |
| `make build` | React 生产资源、Go 二进制、Rust release 二进制 |
| `make test-process` | 真正的进程退出码、SIGTERM、开发服务清理 |
| `make test-dev` | 真实后台 Make 命令：就绪、重复启动、重启、停止、端口冲突、启动器异常退出与控制 socket 边界；隔离 Go fixture 验证源码重载、编译失败恢复与端口释放 |
| `make integration` | 构建后用 Node 内置 test runner 验证真实 Go HTTP、生产静态资源与 Rust CLI |
| `make test-e2e` / `pnpm test:e2e` | 构建四端后执行真实数据库、Web 生产/开发旅程、Rust/Tauri bridge、重启/并发/崩溃恢复 |
| `make check` / `make test-all` | 数据库检查 → 四端 lint/test/build → 真实产品链路 → process/dev/HTTP integration，全流程失败即停止 |
| `make skills-check` | 检查三个技能入口及 Impeccable launcher 可执行性 |

集成与进程测试使用操作系统分配的临时端口，启动本次构建的真实服务；测试退出清理自己创建的进程，不操作用户真实 HOME、客户端配置或凭证。请求、子进程及测试有独立超时，避免待测代码失去超时后拖死 harness。

后台测试还使用临时 `PLUGINPOCKET_RUN_DIR`，不操作日常 `.pluginpocket/` 实例。后台启动最多等待 30 秒就绪，关闭先发送 SIGTERM，6 秒后仍未退出的本次进程组使用 SIGKILL；这些超时不影响已有前台 `make dev`。本地状态来自主管进程，实际 HTTP 就绪由 `up` 检查；`status` 是进程状态，不是持续健康监控。

正常测试不下载包；依赖变更后显式 `pnpm install`，避免 RED 阶段意外开始安装。pnpm 保留默认供应链检查，最新稳定版的精确发布等待期例外记录在 `pnpm-workspace.yaml`。

Web Vitest最多4个工作进程，避免与Go race或并行开发任务争抢CPU导致交互测试超时；保持默认测试超时和全部断言，不用延长超时掩盖失败。

浏览器单连接回归先预热连接池，再断言登录/注册没有额外等待连接；请求10秒截止用于拦截死锁，真实scrypt计算不采用1秒性能断言。

## 失败定位与产物

- 单元/组件失败：先复现聚焦测试；网络下载失败不能解释成代码断言失败。
- 集成失败：查看 Node 内置测试运行器的断言和进程日志；聚焦运行 `node --test tests/integration.test.mjs` 或 `tests/process.test.mjs`（先构建）。
- 前端使用 Vitest + Testing Library + user-event，在 DOM 中检查状态、错误恢复和键盘操作；不引入浏览器自动化验证。桌面/移动视觉效果由人工检查，组件和 HTTP 测试不证明实际排版或无溢出。
- CI 上传 `harness.log`（完整检查日志）。Windows/macOS 执行 Rust CLI 测试与构建；Linux 跑完整 harness，进程组测试仅在 Linux 验证。
- CI 可证明当前代码状态，无法证明 RED 先于实现。任务记录和 PR 模板保留真实开发顺序；禁止编造 RED 记录。

## 产品与数据库测试

`make check` 强制要求 `PLUGINPOCKET_TEST_DATABASE_URL` 和 `PLUGINPOCKET_TEST_REDIS_URL`，未设置直接非零退出。Go 测试每例创建唯一 schema，结束删除该 schema；不要使用生产数据库。独立的 `go test ./...` 允许没有数据库时跳过集成部分，不能据此报告产品验收通过。

`make test-product` 启动刚构建的 Go 与 Rust，验证注册、一次性令牌、三客户端 apply、官方 SDK → Rust bridge → HTTP 网关、余额/账单、撤销、兑换幂等、团队转入与调用、设备码明确批准、注销清理。所有本地凭证在临时 HOME，基建进程测试强制清空业务数据库和管理员环境变量。

`make test-e2e` 是 `make check` 的完整产品入口，包含上述旅程及 Web/Tauri，要求真实 PostgreSQL 和 Redis，不返回模拟业务响应。Web 使用 Vitest + Testing Library 挂载真实 App，经 HTTP/Cookie 访问隔离 Go；分别验证生产服务和真实 `make up` 启动的 Vite 代理。开发地址还必须通过官方 MCP SDK 完成调用和实际扣费。进程旅程验证登录/撤销/账本跨重启保持、多实例并发余额不透支、上游错误退款且不重放、崩溃后 pending 恢复幂等、管理员定价与禁用生效、Tauri 原生 bridge 接入。Web 子命令 `pnpm --dir web test:e2e` 由 Go fixture 传入临时地址和测试账号，日常使用根入口自动准备依赖和隔离环境。

`make test-desktop` 检查 React UI、受限 Rust commands 与无 DISPLAY 的原生 bridge；`make build-desktop` 生成 Linux deb。安装包路径 `desktop/target/release/bundle/deb/`。这些检查不证明真实 GUI/托盘交互。Linux 系统依赖见 [部署说明](DEPLOYMENT.md)。

`make benchmark` 对真实 PostgreSQL 同一钱包运行 serial/parallel reserve+finish，输出 ns/op、B/op 和 allocs/op。它测事务吞吐，不能把并行 ns/op 当成请求 p95；不包括公网、上游或 UI 延迟。基准记录附机器和运行条件，不编造 SLA。

分布式回归还包括：两个真实Go进程经无粘性HTTP负载均衡轮询，关闭一个副本后授权/会话/记账继续，重新加入后注销不复活；另一个副本的暖目录必须立即观察已提交工具配置。独立Store/app/identity实例覆盖共享限流、并发初始化/退款恢复、OAuth/email重放、设备与团队操作。synctest只模拟节点时钟，数据库和HTTP保持真实；有效期、限流、报表和恢复不得依赖偏差节点时钟。详见 [分布式计划](superpowers/plans/2026-09-10-distributed-backend.md)。

OpenAPI 契约见 [openapi.json](openapi.json)，可用 `uvx --from openapi-spec-validator==0.9.0 openapi-spec-validator docs/openapi.json` 验证。完整 RED/GREEN 和评审修复证据见 [产品执行记录](superpowers/plans/2026-09-10-product-execution.md)。

Redis测试连接通过 `PLUGINPOCKET_TEST_REDIS_URL` 指定。每例隔离namespace，TTL自动淘汰，不清空整库。真实进程旅程验证两个Go副本只进行一次上游发现，以及新实例无法连接Redis时仍返回可用目录、完成上游调用并按PG账本扣费；就绪接口明确报告degraded。Pub/Sub、过期、命名空间和连接取消另有真实Redis模块测试。

## 部署契约验证

集群模板在 `deploy/kubernetes/`，变量全集与加载优先级见 [ENVIRONMENT](ENVIRONMENT.md)。变更配置时验证Compose实际渲染值、生产示例经Go Config.Load解析、Kubernetes官方版本schema，再由部署者在目标集群执行server dry-run。不要把静态schema验证写成真实HA切换演练。集群完整操作步骤见 [CLUSTER](CLUSTER.md)。

Go lint 固定 golangci-lint 2.13.2；`make setup` 或首次 `make lint` 使用该版本官方安装脚本及校验后的发布二进制，安装到忽略目录 `build/tools/`。配置 `.golangci.yml` 同时用于本地与 CI，不忽略测试中的标准诊断。`make format` 使用同一版本 gofmt/goimports；`go mod tidy` 仅在依赖变更或整理时显式运行，lint 只检查差异。

浏览器鉴权回归：BrowserAuth/BrowserRefresh 测试覆盖已有登录态、共享刷新与错误恢复；app/browser_auth_test.go 覆盖 AT/RT 期限、跨副本并发、撤销、单连接池、运行时热更新，SQLite 验证关联凭据级联撤销。证据见 2026-09-11-browser-auth 执行记录。
