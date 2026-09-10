# 本地页面 404 与环境配置修复

## 根因和实际处理

截图中的“记录已不存在”来自 `GET /api/v1/account/me` 的真实 `404 {error:not_found}`。当时根目录没有 `.env`，服务进程也没有数据库配置。`applicationHandler` 因此只注册健康基建路由，Web 账号页面无法工作。

- 根 Make 的 up/down/status/logs 与 pnpm dev 使用 Node 原生 `--env-file-if-exists=.env`，已导出的变量优先。移除 Make 默认导出的 `LOADOUT_RUN_DIR`，避免它遮盖文件配置。
- 日志命令复用后台脚本的 `logPath`，使自定义运行目录与启动/关闭保持一致。
- 创建 Git 忽略、权限 0600 的本地 `.env`，使用随机数据库密码、加密密钥和管理员密码；不在日志记录秘密。
- Docker 无权限且 sudo 需要密码，因此使用本机已有 PostgreSQL 18.6（127.0.0.1:44035），新建独立 `loadout_dev` 角色和业务库；测试仍使用 `loadout_test`。未重置已有角色或删除测试数据。
- 本机 PostgreSQL 二进制和数据目前位于 `/tmp/loadout-pg18`、`/tmp/loadout-pgdata`，属于临时开发实例；正式保存数据应按 DEPLOYMENT 的持久卷与备份流程迁移。本次没有将数据库部署为系统服务。

## RED / GREEN / REFACTOR

所有 Node 验证使用本机固定 Node 26.8.2。

- RED：`node --test --test-name-pattern='local env file' tests/dev.test.mjs`。测试使用临时工作目录和 `.env` 指向真实的不可达控制 socket；原命令忽略文件，错误地输出 stopped，因此 `Missing expected rejection`。
- GREEN：Make 命令加载 `.env` 后，同一测试通过。
- 审查补充 RED：扩展同一测试，要求 `make logs` 输出自定义目录中的日志；原命令跟随默认目录空文件，两秒后超时。
- GREEN：logs 通过同一环境加载入口并复用 `logPath` 后通过。
- REFACTOR：`pnpm exec biome check --write scripts/dev.mjs tests/dev.test.mjs`；随后 `node --test tests/dev.test.mjs tests/process.test.mjs` 十项全部通过，包括前台退出、后台重启和测试数据库配置隔离。
- 独立复查确认日志目录问题已解决。

## 实际验证与限制

- `pnpm run restart` 从 `.env` 启动成功，未额外 export 数据库变量。
- `/readyz` 返回 200；匿名 `/api/v1/account/me` 返回 401；使用生成的管理员通过 Vite 代理登录返回 200，查询账号返回 200 且 role=admin；验证会话已注销（204）。密码和 Cookie 未输出。
- `pnpm exec biome check scripts/dev.mjs tests/dev.test.mjs package.json`、`git diff --check` 通过。
- 加载 `.env` 后运行真实 `make check`，静态检查通过，但 Go 网关 `TestCanceledCallDoesNotCloseAnotherCallOnSharedSession` 失败：`unrelated call failed after the other request was canceled`。这是工作区其他网关变更的测试，本次未修改网关。完整 harness 未通过，日志 `/tmp/loadout-env-check.log`。
- 本次修复没有修改页面 UI；未运行浏览器自动化或声称桌面/移动视觉验收通过。
