# 开发 Harness

Harness 是同一套可执行的本地/CI 验证入口。所有命令从仓库根运行；工具链按 `.node-version`、`server/go.mod`、`rust-toolchain.toml` 和 `packageManager` 固定。

## 工作循环

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

| 命令 | 检查 |
|---|---|
| `make setup` | 按锁文件安装 JS 与 Rust 依赖 |
| `make dev` | Go + Vite；退出时停止组合进程 |
| `make test` | Go race 测试、Rust 黑盒测试、React 行为测试 |
| `make lint` | Biome、TS、gofmt、go vet、rustfmt、clippy；只检查不改文件 |
| `make format` | 显式自动格式化 |
| `make build` | React 生产资源、Go 二进制、Rust release 二进制 |
| `make test-process` | 真正的进程退出码、SIGTERM、开发服务清理 |
| `make integration` | 构建后用 Node 内置 test runner 验证真实 Go HTTP、生产静态资源与 Rust CLI |
| `make check` | lint → test → build → process → integration，全流程失败即停止 |
| `make skills-check` | 检查三个技能入口及 Impeccable launcher 可执行性 |

集成与进程测试使用操作系统分配的临时端口，启动本次构建的真实服务；测试退出清理自己创建的进程，不操作用户真实 HOME、客户端配置或凭证。请求、子进程及测试有独立超时，避免待测代码失去超时后拖死 harness。

正常测试不下载包；依赖变更后显式 `pnpm install`，避免 RED 阶段意外开始安装。pnpm 保留默认供应链检查，最新稳定版的精确发布等待期例外记录在 `pnpm-workspace.yaml`。

## 失败定位与产物

- 单元/组件失败：先复现聚焦测试；网络下载失败不能解释成代码断言失败。
- 集成失败：查看 Node 内置测试运行器的断言和进程日志；聚焦运行 `node --test tests/integration.test.mjs` 或 `tests/process.test.mjs`（先构建）。
- 前端使用 Vitest + Testing Library + user-event，在 DOM 中检查状态、错误恢复和键盘操作；不引入浏览器自动化验证。桌面/移动视觉效果由人工检查，组件和 HTTP 测试不证明实际排版或无溢出。
- CI 上传 `harness.log`（完整检查日志）。Windows/macOS 执行 Rust CLI 测试与构建；Linux 跑完整 harness，进程组测试仅在 Linux 验证。
- CI 可证明当前代码状态，无法证明 RED 先于实现。任务记录和 PR 模板保留真实开发顺序；禁止编造 RED 记录。

## 后续业务必须增加的测试

M0 账号/令牌/网关/bridge 各按独立计划 TDD：对象级鉴权、令牌撤销、并发额度与账单事务、上游超时、stdio 干净输出、临时 HOME 下配置幂等与冲突保护。不能用当前健康检查冒烟替代完整 M0 验收。
