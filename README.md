# Loadout

> **Your AI, fully loaded.** — 登录即武装的 MCP 订阅网关

[![Status](https://img.shields.io/badge/status-foundation-blue)](docs/PLAN.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 这是什么

用 Codex / Claude Code / Cursor 的开发者，想用 MCP 工具得自己找 server、申请一堆 API key、手改各客户端配置文件 —— 换台机器全部重来。

**Loadout 把这些压缩成两步**：

```bash
# 1. 网页注册，复制一个令牌
# 2. 本地两条命令：
loadout login --server https://api.example.com
loadout apply        # 自动检测 Codex / Claude Code / Cursor 并写入配置
```

重启客户端，整套预设好的 MCP 工具直接可用。所有调用经过统一网关：**鉴权、计量、扣额度，后台看得见每一次调用**。

## 架构一览

```
Codex / Claude Code / Cursor
        │ stdio
   loadout bridge（本地转发，自动注入令牌）
        │ streamable HTTP + Bearer ldt_xxx
   Loadout 服务端 ── 账号 / 令牌 / 网关 / 计量扣费 / 管理后台
        │
   预设 MCP 上游池（builtin / http / stdio）
```

三个核心设计：

1. **bridge 为默认** —— 客户端配置里不落任何密钥，令牌轮换不触碰客户端配置；
2. **网关无状态** —— 每请求独立实例，可水平扩展；
3. **只托管自有预设池凭证** —— 不碰用户私人第三方账号的 OAuth token。

## 项目状态

当前已搭建 **M0.0 工程基建**：React 深色控制台、Go 健康 API、Rust `doctor`，配套完整 TDD harness。上面的 `login / apply / bridge` 是后续 M0 目标，目前尚未实现。

本地验证与未执行范围见 [基建执行记录](docs/superpowers/plans/2026-09-10-foundation-execution.md)。

完整产品与技术方案见 **[docs/PLAN.md](docs/PLAN.md)**。Loadout 不托管用户第三方账号的 OAuth token，上游凭证仅属于运营方自有预设池。

| 里程碑 | 内容 | 状态 |
|---|---|---|
| M0.0 | React / Go / Rust 基建、技能、TDD harness 与 CI | ✅ 本地 harness 通过 |
| M0 | 端到端骨架：网关 + 控制台 + CLI，冒烟全绿 | ⏳ 后续业务开发 |
| M1 | 可运营：上游管理、限流防滥用、Docker 部署 | ⏳ |
| M2 | 商业闭环：支付、套餐、兑换码 | ⏳ |
| M3 | 团队版：席位、共享额度、审计 | ⏳ |

## 快速开始

工具链固定为搭建时最新稳定版：**Node 26.8.2、pnpm 12.3.4、Go 1.27.1、Rust 1.98.1**。Go 可自动下载模块要求的工具链；Rust 由 rustup 读取 `rust-toolchain.toml` 安装。Rust HTTPS 依赖需要本机 C/C++ 编译环境；完整 harness 在 Linux 运行，CLI 同时配置 macOS/Windows CI。

```bash
git clone https://github.com/Yanyutin753/loadout.git
cd loadout
# 使用 nvm 时：nvm install 26.8.2 && nvm use 26.8.2
npm install -g pnpm@12.3.4
make setup
make dev
```

打开 **http://127.0.0.1:5173**，页面通过 Vite 代理访问 **http://127.0.0.1:8787** 的 Go 服务。Ctrl+C 停止开发进程。

```sh
# 完整检查：静态检查、三端测试、构建、进程生命周期与真实 HTTP/CLI 集成
make check

# 原生 CLI 检查运行中的服务
cargo run --manifest-path cli/Cargo.toml -- doctor --server http://127.0.0.1:8787

# 构建后只运行 Go，即可访问 API 和生产前端
make build
./build/loadout-server
# 打开 http://127.0.0.1:8787

# 或使用容器
docker compose up --build
```

`.env.example` 列出当前支持的配置。Go 从环境变量读取，**不自动加载 `.env`**；Compose 自动读取 `.env` 中的端口配置。开发端口冲突时设置 `LOADOUT_ADDR`、`LOADOUT_API_ORIGIN`、`LOADOUT_DEV_PORT`；集成测试自动分配临时端口。

## 开发规范

- **[AGENTS.md](AGENTS.md)**：强制 Superpowers + Ponytail + Impeccable，以及 RED → GREEN → REFACTOR → 完整验证。
- **[docs/HARNESS.md](docs/HARNESS.md)**：测试分层、可执行命令、隔离、证据与失败排查。
- **[docs/FRONTEND.md](docs/FRONTEND.md)**：React、shadcn/ui、Tailwind、Query、Zod、无障碍与前端测试规范。
- **[PRODUCT.md](PRODUCT.md) / [DESIGN.md](DESIGN.md)**：产品事实、用户选定的深色风格与设计 token。
- **[.agents/README.md](.agents/README.md)**：三套技能的安装位置、官方来源、固定提交和许可证；随仓库共享。

方案与实现不一致时，先改 [docs/PLAN.md](docs/PLAN.md) 再改代码。

## License

[MIT](LICENSE) © Loadout contributors
