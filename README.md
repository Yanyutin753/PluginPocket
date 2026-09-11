# Loadout

> **Your AI, fully loaded.** — 登录即武装的开源自托管 MCP 装备网关（全仓库 MIT）

[![Status](https://img.shields.io/badge/status-implemented-blue)](docs/PLAN.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 这是什么

用 Codex / Claude Code / Cursor 的开发者，想用 MCP 工具得自己找 server、申请一堆 API key、手改各客户端配置文件 —— 换台机器全部重来。

**Loadout 把这些压缩成两步**：

```bash
# 1. 网页注册，复制一个令牌
# 2. 本地两条命令：
loadout login --device --server https://api.example.com
loadout apply --clients codex,claude,cursor
```

重启客户端，整套预设好的 MCP 工具直接可用。所有调用经过统一网关：**鉴权、计量、扣额度，后台看得见每一次调用**。

服务端提供**插件市场**（只收录 HTTP MCP）：管理员在后台同步 GitHub 热门 MCP 仓库、一键装入预设池并可覆盖工具描述与参数说明；本地也能把公共插件直连装进客户端——

```bash
loadout market                       # 浏览市场：MCP 插件 / 技能 / 装备组
loadout install expert-pack          # 一条命令复刻专家配置：MCP 直连条目 + 技能目录
loadout uninstall expert-pack
```

市场条目三种：**mcp**（HTTP 插件，服务端池计量或本地直连零密钥）、**skill**（Agent Skill 写入 `~/.codex/skills/`、`~/.claude/skills/`，可从 GitHub 仓库导入由服务端代取）、**bundle** 装备组（一键复刻专家整套配置）。**服务端本身就是 Codex 插件市场源**：运营方在后台定制插件（网关独享 MCP + 技能 + 装备组，独享工具凭证密封、按次计量），服务端在 `/marketplace.git` 实时渲染官方插件格式并提供免登录 React 目录页 `/plugins`；用户 `codex plugin marketplace add <服务地址>/marketplace.git` 一行接入，独享工具以本地 bridge 形态进插件、走鉴权计量——登录即武装的完整闭环。

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

工具管理支持图片自动缩小压缩/SVG 上传及共享图标、分区连接配置、常用结算规则表单与服务端试算；图标压缩后存数据库（最多64 KiB），多副本共用，无需额外对象存储；试算不调用上游、不扣费。

用量明细支持按次查看网关传入参数和返回内容，个人、团队与管理员按各自权限读取；新调用原样记录，超出每方向 64 KiB 会标注截断，历史未保存内容显示“未记录”。

已实现账号与令牌、MCP 网关、事务额度账本、运营后台、套餐/兑换码、团队共享额度、设备授权、Rust CLI 和 Tauri 桌面端；插件市场收录 HTTP MCP（GitHub 热门同步 + 精选直装 + 上游工具描述/参数覆盖 + CLI 本地直连安装）。React 控制台采用 AI 装备工坊风格，支持中英文与浅色/深色/跟随系统，手机、平板和桌面共用 API 与功能。GitHub/邮箱通过部署配置启用；外部支付按本次范围仅预留接口，不产生虚假支付成功。

项目定位为**开源项目**（全仓库 MIT，2026-09-12 确定）：优先服务自部署与团队自治理——额度、兑换码与计量是治理能力而非商业前置；商业化与外部支付不再是路线图关键路径。

管理员可在“系统配置”页热更新注册赠送额度、GitHub登录与SMTP邮件参数；配置版本化保存在数据库，密钥加密且不回显，多个副本的新请求同时生效。部署连接和安全边界参数仍由环境变量管理，详见 [环境配置](docs/ENVIRONMENT.md)。

数据库选用 PostgreSQL 18.6，Redis 8.10.1 提供跨副本工具元数据缓存与刷新通知（故障可降级），Go 模块化单体通过官方 MCP SDK 接入 HTTP 或受控 stdio 上游；Rust CLI 与桌面共享接入逻辑。性能依据和完整验收状态见 [产品执行记录](docs/superpowers/plans/2026-09-10-product-execution.md)，架构取舍见 [ADR](docs/adr/0001-product-architecture.md)。

2026-09-11 本机完整 `make check` 通过：真实 PostgreSQL / Redis、多实例与故障恢复、Web79项组件及生产/开发真实HTTP旅程、CLI与桌面bridge、Linux deb构建。新增分布式与Redis验收见 [执行记录](docs/superpowers/plans/2026-09-10-development-e2e-execution.md)；此前数据库基准结果见产品执行记录。真实屏幕/托盘、macOS/Windows 桌面和外部 OAuth/邮件服务仍需在对应环境验收；CI 配置不等于远程 CI 已通过。

## 快速开始

桌面打包已配置 Windows x64、macOS 双架构及 Linux 双架构，使用独立 Loadout 发布签名并在公开前验签；当前本机已产出 Linux x64 包，其他平台尚未远程验收。下载与证书边界见 [桌面发行说明](desktop/README.md#桌面发行)。

工具链固定为搭建时最新稳定版：**Node 26.8.2、pnpm 12.3.4、Go 1.27.1、Rust 1.98.1**。Go 可自动下载模块要求的工具链；Rust 由 rustup 读取 `rust-toolchain.toml` 安装。Rust HTTPS 依赖需要本机 C/C++ 编译环境；完整 harness 在 Linux 运行，CLI 同时配置 macOS/Windows CI。

```bash
git clone https://github.com/Yanyutin753/loadout.git
cd loadout
# 使用 nvm 时：nvm install 26.8.2 && nvm use 26.8.2
npm install -g pnpm@12.3.4
make setup
```

集群部署见 [部署规范与Kubernetes模板](docs/CLUSTER.md)，全部参数见 [环境变量参考](docs/ENVIRONMENT.md)。

先完成 [数据库与部署配置](docs/DEPLOYMENT.md)，配置 `LOADOUT_DATABASE_URL` 和管理员。本地前端通过相对路径 `/api` 反代，`LOADOUT_PUBLIC_URL` 可留空，无需固定localhost或127.0.0.1。Linux 桌面构建还需要 GTK/WebKit 开发库，具体命令见该文档。未配置数据库只提供基建健康检查，无法使用账号业务。配置完成后执行 `make up`；若已用旧配置启动，执行 `make restart`。

打开 **http://127.0.0.1:5173**，页面通过 Vite 代理访问 **http://127.0.0.1:8787** 的 Go 服务。使用 `make down` 关闭后台服务；偏好前台日志时在**仓库根目录**运行 `pnpm run dev`（或 `make dev`），VS Code 选择任务 **Loadout: dev**，Ctrl+C 关闭前后端。不要同时启动后台实例或 `web` 目录的 dev 脚本，否则会占用相同端口。Go 开发服务使用固定版本 Air 自动编译重启，监听 `server/` 的 Go、SQL、go.mod/go.sum；编译失败停止旧程序，修复保存后恢复。首次 `make setup` / `make dev-server` 安装 Air 需要网络。

首页和工作空间可直接进入 **http://127.0.0.1:5173/plugins** 公共插件市场；目录与详情共用前端样式、通过免鉴权 API 读取；开发代理转发 API 与 `/marketplace.git`，与生产地址一致。

公共市场采用响应式插件卡片，支持搜索、类型筛选和每页 12 项分页；筛选与页码可通过 URL 分享。

管理员从侧栏 **市场管理**（`/admin/marketplace`）创建或编辑技能与装备组。技能支持自定义 Markdown 和 GitHub `owner/repo` + 目录导入；“精选 GitHub 技能”提供逐项及一键全部同步，失败条目可重试。装备组勾选可安装的 MCP 与技能组成，保存后在公共市场展示。GitHub 同步保存已验证文件，重复同步内容变化才升级版本；技能更新也更新引用它的装备组版本。

技能编辑默认打开 `SKILL.md`，支持目录与文件标签、行号和语法高亮、查找与撤销、Markdown分栏预览、图片预览和全屏编辑；可新建文本与上传附件。已有 GitHub 技能可直接修改已发布文件；重新同步作为独立选项，会替换已发布的文件。

日常命令（直接运行 `make` 或 `make help` 查看全部入口）：

常用入口也已注册在根 `package.json`，可以从 IDE 的 **NPM Scripts** 面板点击运行，或执行 `pnpm run ready`、`pnpm run up`、`pnpm run down`、`pnpm run check`。`npm run <名称>` 同样可用；依赖仍统一用 pnpm 安装。分组命令使用冒号，例如 `pnpm run test:server`、`pnpm run docker:up`，内部复用 Makefile。

| 命令 | 用途 |
|---|---|
| `make ready` | 一键完整检查，成功后后台启动全部开发服务 |
| `make up` / `make start` | 后台启动 Go + Vite，等待 HTTP 就绪；已启动时保持现有实例 |
| `make down` / `make stop` | 关闭后台开发进程，保留日志 |
| `make restart` | 先关闭再启动，重新构建 Go；`.env` 等启动配置变更后使用 |
| `make status` | 查看后台运行状态和地址 |
| `make logs` | 跟随最近 100 行及新日志；Ctrl+C 仅退出日志查看 |
| `make dev` / 根目录 `pnpm run dev` | 前台运行 Go 自动重载 + Vite，Ctrl+C 停止 |
| `make test` | Go / Rust / React 三端测试 |
| `make check` / `make test-all` | 真实测试数据库、四端检查/构建、跨进程产品流程；需 Linux 桌面依赖 |
| `make db-up` / `make redis-up` | 启动 PostgreSQL / Redis，供源码开发 |
| `make test-product` / `make benchmark` | 真实产品调用链 / 同钱包事务基准 |
| `make build-desktop` | 构建 Linux Tauri deb |
| `make build` | 构建三端生产产物 |
| `make docker-up` / `make docker-down` / `make docker-logs` | Compose 容器的后台启动、关闭和日志 |

后台命令在 Linux 验证，使用 Unix socket 和进程组（Windows 请用 WSL）。日志和控制 socket 存在 `.loadout/`，已被 Git 忽略；`make down` 仅管理 `make up` 启动的实例。`make dev` 的前台实例用 Ctrl+C 停止，容器用 `make docker-down` 停止。Rust CLI 按需运行，无需常驻进程。

`make up` 启动失败返回非零退出码，并清理本次创建的进程；详细日志见 `.loadout/dev.log`。若主管进程被强制终止导致 socket 无法连接，先检查旧服务是否仍在运行，确认停止后再删除 `.loadout/dev.sock` 并重新 `make up`。不要在服务运行时删除控制文件。

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
make docker-up
```

`.env.example` 列出当前支持的配置。根目录的 `make up/dev/restart/status/logs` 及对应 NPM Scripts 自动读取 `.env`，已导出的环境变量优先。Go 二进制本身不加载 `.env`；单独运行 Go 或 `make check` 时需显式导出配置。Compose 也会读取 `.env`。开发端口冲突时设置 `LOADOUT_ADDR`、`LOADOUT_API_ORIGIN`、`LOADOUT_DEV_PORT`；集成测试自动分配临时端口。

```sh
# 后台模式在未设置 LOADOUT_API_ORIGIN 时，会从 LOADOUT_ADDR 推导代理地址
LOADOUT_ADDR=127.0.0.1:8887 LOADOUT_DEV_PORT=5183 make up
make status
make down
```

需要隔离多套开发实例时设置不同的 `LOADOUT_RUN_DIR` 和端口；后续 `down/status/logs` 使用同一 `LOADOUT_RUN_DIR`。`make ready` 与 `make restart` 在单个目标内部保证顺序；请勿用 `make -j check up` 或 `make -j down up` 代替它们。

## 开发规范

本地完整演示数据、管理员账号和 MCP 热重载验证见 [演示与测试说明](docs/DEMO.md)。演示工具使用真实 API 写入指定本地数据库；本次按用户要求已填充 5173 当前数据库，MCP 外部上游另在隔离环境验证。

- **[AGENTS.md](AGENTS.md)**：强制 Superpowers + Ponytail + Impeccable，以及 RED → GREEN → REFACTOR → 完整验证。
- **[docs/HARNESS.md](docs/HARNESS.md)**：测试分层、可执行命令、隔离、证据与失败排查。
- **[docs/FRONTEND.md](docs/FRONTEND.md)**：React、shadcn/ui、Tailwind、Query、Zod、无障碍与前端测试规范。
- **[PRODUCT.md](PRODUCT.md) / [DESIGN.md](DESIGN.md)**：产品事实、用户选定的装备工坊风格与设计 token。
- **[.agents/README.md](.agents/README.md)**：三套技能的安装位置、官方来源、固定提交和许可证；随仓库共享。

方案与实现不一致时，先改 [docs/PLAN.md](docs/PLAN.md) 再改代码。

## License

[MIT](LICENSE) © Loadout contributors —— 贡献方式见 [CONTRIBUTING.md](CONTRIBUTING.md)。

浏览器登录使用 HttpOnly AT/RT，公共页面自动识别登录态；管理员可在系统配置热更新有效期，凭据与配置由共享 PostgreSQL 支持无粘性多副本。首次升级的版本兼容边界见 [集群规范](docs/CLUSTER.md#浏览器-atrt-与有效期热更新)。


技能支持二进制附件与可执行权限，上传/同步后以SHA256清单发布；CLI校验后按字节安装。小文件存共享数据库，大文件及Git大对象通过官方AWS SDK接入S3兼容存储，配置参见 [文件存储](docs/FILE_STORAGE.md)。

市场管理的“同步 GitHub”同时同步 MCP 与精选/已导入的 GitHub 技能；显示分类型成功数与失败项，可重试。自定义技能不会被批量覆盖。
