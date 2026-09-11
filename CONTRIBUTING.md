# 贡献指南

PluginPocket 是开源自托管项目（全仓库 MIT）。本文只做入口，行为契约以 [AGENTS.md](AGENTS.md) 为唯一事实源，冲突时以它为准。

## 开发环境

工具链版本与启动命令见 [README 快速开始](README.md#快速开始)：`make setup` 安装固定版本依赖，`make up` 后台启动开发服务（Go 自动重载 + Vite），或根目录 `pnpm run dev` 前台运行。

服务端测试需要真实 PostgreSQL 与 Redis（`PLUGINPOCKET_TEST_DATABASE_URL`、`PLUGINPOCKET_TEST_REDIS_URL`，见 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) 与 [docs/HARNESS.md](docs/HARNESS.md)）；本地可用 `make db-up` / `make redis-up` 起容器。

## 行为契约（摘要，全文见 AGENTS.md）

- **完整 TDD**：先写用户可观察行为的最小测试并确认 RED，再实现最少逻辑到 GREEN；不删失败断言、不以源码字符串断言代替行为。
- **文档同步**：改产品能力先改 `docs/PLAN.md`，改环境变量同步 `docs/ENVIRONMENT.md` + `.env.example`，改 API 同步 OpenAPI 与 web Zod 模型，改任一数据库迁移轨道必须同步另一轨道。
- **工程边界**：结算是钱的路径，预留-结算-退款必须走 store 事务；市场只收 HTTP MCP；客户端配置零密钥；不绕过 append-only 账本触发器。

## 提交前

```bash
make check   # 静态检查 + 三端测试 + 构建 + 进程/HTTP 集成；任一步失败即停
```

环境受限无法运行的检查必须在 PR 里明确说明，不能写“全部通过”。

## Pull Request

- 使用仓库 PR 模板；一次 PR 聚焦一件事。
- 行为改动附 RED / GREEN / 回归证据（真实命令与结果）；缺陷修复先提交能重现根因的失败测试。
- 执行记录追加到 `docs/superpowers/plans/`（命令、失败原因、验收映射、未验证范围）。
- 不要求签署 CLA；提交即表示同意以 MIT 许可贡献。
