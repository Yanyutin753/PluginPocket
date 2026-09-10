# Loadout Foundation Implementation Plan

> **For agentic workers:** 使用 superpowers:executing-plans 按任务执行；对行为坚持 RED → GREEN → REFACTOR。当前工作区完成，保留已有修改。

**Goal:** 交付 React / Go / Rust 可运行基建，以及每次开发都能复用的 TDD harness。

**Architecture:** 三个独立构建单元；Go 托管健康 API 和生产 Web；Vite 代理开发 API；Rust doctor 使用相同健康契约。测试从单元到真实进程端到端。

**Tech Stack:** React、TypeScript、Vite、Vitest、Testing Library、Node 内置 test runner、Biome、Go net/http、Rust clap/reqwest/serde、Make、GitHub Actions。

## Task 1：工具与规范

- [x] 安装 `.agents/skills/` 三套技能，记录上游 SHA、许可证和执行权限。
- [x] 同步 `docs/PLAN.md` 的技术选型、目录和客户端命令；保留 M0 业务验收。
- [x] 写 `AGENTS.md`、`PRODUCT.md`、`DESIGN.md`、`docs/HARNESS.md`、PR 模板；明确 TDD 证据及技能分工。

## Task 2：Go 服务（测试先行）

- [x] 创建 `server/go.mod` 和 `internal/httpapi/server_test.go`：GET/HEAD 健康契约、错误 method、未知 API 的 JSON 404、SPA 路由、缺失静态资源不回退、健康响应不缓存。
- [x] 创建 `internal/config/config_test.go`：默认值、覆盖、无效地址。
- [x] 运行 `cd server && go test ./...`，确认可编译空壳因行为缺失而断言失败。
- [x] 实现 `internal/httpapi/server.go`、`internal/config/config.go`、`cmd/loadout-server/main.go`；通过 `go test -race ./...` 与 `go vet ./...`。

## Task 3：Rust CLI（测试先行）

- [x] 创建 `cli/Cargo.toml`、`cli/tests/doctor.rs`：真实临时 HTTP server，测试 doctor 成功、不可达、非 Loadout 响应、HTTP 错误、无效 URL、帮助与版本。
- [x] 运行 `cargo test --manifest-path cli/Cargo.toml`，确认命令行为测试失败。
- [x] 实现 `cli/src/main.rs`，请求有超时、不跟随重定向、错误摘要不泄露 URL/响应体；通过 cargo test / clippy / fmt。

## Task 4：React 界面（测试先行）

- [x] 建包管理、TypeScript、Vite、Vitest、Biome 配置与最小挂载入口。
- [x] 创建 `web/src/App.test.tsx`，断言 loading、真实契约成功、网络/错误契约失败和用户重试。
- [x] `pnpm --dir web test` 观察预期失败，再实现 `web/src/App.tsx`、`api.ts`、`styles.css`。
- [x] 运行组件测试、类型检查与构建；设计规则落实为语义结构、可访问状态、响应式 tokens。

## Task 5：可执行 harness

- [x] 添加 `tests/integration.test.mjs` 与 `tests/process.test.mjs`，由 Node 标准库验证真实 Go 静态服务/API、Rust doctor 与进程生命周期；既有行为在迁移验证工具时保留契约。
- [x] 添加根 `package.json`、`Makefile`、`.github/workflows/ci.yml`；统一 test / lint / format / build / integration / check。
- [x] 添加 Dockerfile / compose.yaml / .env.example，使用真实构建产物。
- [x] `make check`；检查 Linux 全流程、可用的容器校验；桌面/移动视觉人工检查单独标明范围。

## Task 6：收尾

- [x] correctness 与 Ponytail 复杂度审查；Impeccable 界面检查。所有修复先补失败回归再实现。
- [x] 更新 README quickstart、执行记录和实际限制；明确 M0.0 完成与后续 M0 功能边界。

执行证据与限制见 [foundation-execution](2026-09-10-foundation-execution.md)。本清单完成表示已交付基建及明确验证范围；容器运行、远程 CI 和最终桌面/移动视觉检查尚未执行，不标记为通过。
