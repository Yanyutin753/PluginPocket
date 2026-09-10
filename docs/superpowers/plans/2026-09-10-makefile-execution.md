# Makefile 开发入口执行记录

日期：2026-09-10。范围：当前工作区的 Makefile、Node 标准库后台开发管理、真实进程测试与使用文档。保留原有前台开发和三端 harness，不新增依赖、不提交或推送。

## 验收映射

- `make` 默认显示帮助；`setup` 安装锁定依赖。
- `ready` 顺序执行 `check` 后 `up`；`test-all` 复用完整 `check`。
- `up/start` 后台启动已有 Go + Vite 开发命令并等待本地 Go、Vite 代理与页面响应；重复启动保留实例。
- `down/stop` 关闭本次主管进程创建的进程组；`restart` 先停后启；`status/logs` 提供状态、地址和日志。
- 端口占用不杀其他进程；异常启动器退出清理子进程；无法连接的控制 socket 不自动覆盖。
- Compose 的启动、关闭和日志使用原生 Compose 命令，独立于源码开发实例。

## RED / GREEN / REFACTOR

验证时先将 `/home/yangyang/.nvm/versions/node/v26.8.2/bin` 加入 PATH，使用仓库固定 Node 版本；未修改全局 Node 配置。

| 验收 | RED 命令与实际失败 | GREEN |
|---|---|---|
| 后台生命周期、占用端口 | `node --test tests/dev.test.mjs`：缺失 Make 规则仅算准备阶段；增加空 `up/down/status` 入口后重跑，状态输出为空、端口冲突仍返回 0，两个行为断言失败 | 实现后台入口后，同一命令两例通过，验证真实 API、Web、日志、重复启动和停止后端口释放 |
| 残留控制 socket 的边界 | `node --test --test-name-pattern='unreachable control' tests/dev.test.mjs`：原实现自动替换不可达 socket 并启动，预期失败却返回 0 | 删除自动 unlink，仅以 Unix socket 的独占监听防重复；同文件回归通过 |
| 启动器意外退出 | `node --test --test-name-pattern='unexpected launcher' tests/dev.test.mjs`：SIGKILL 本测试专属 `pnpm dev` 后等待 10 秒仍显示 running | 在子进程 exit/error 触发清理，close 仅用于等待管道排空；同文件回归通过 |
| 本地 Go 的真实就绪 | `node --test --test-name-pattern='healthy proxy' tests/dev.test.mjs`：代理目标正常但本地 Go 编译延迟失败，原实现约 0.6 秒返回启动成功 | 增加本地 Go 健康检查；同文件回归通过，编译失败返回非零且关闭 Vite |

REFACTOR：`pnpm exec biome check --write scripts/dev.mjs tests/dev.test.mjs` 格式化，Make 别名单独声明以便帮助发现。随后 `node --test tests/dev.test.mjs` 五例通过；生命周期测试同时验证了 `restart`。新增测试接入 `make check`。

## 验证记录

- 初轮 `make check` 退出 0；独立审查后修复上述两项进程问题，五例聚焦回归已通过。
- 独立审查实际验证 30 秒启动超时后返回失败，Go/Vite 端口释放，状态 stopped；修复后的定向复查无阻塞问题。
- `git diff --check`、`make help`、`docker compose config --quiet` 通过。
- 共享工作区的最终 `make ready` 在内部 `make check` 的 gofmt 步骤失败：当时并行任务的 `server/internal/auth/{auth.go,auth_test.go}` 和 `server/internal/store/{store.go,store_test.go}` 尚未格式化。本任务没有改这些文件；失败后未启动后台服务，验证了检查失败的阻断行为。日志：`/tmp/loadout-make-ready.log`。
- 为隔离并行修改，使用基线 `6481c6a` 的临时 clone，仅复制本任务文件。依赖先用 `pnpm install --frozen-lockfile --offline` 从本地缓存安装，没有禁用 pnpm 的依赖校验。完整 `make ready` 退出 0：内部 `make check` 的静态检查、三端测试、构建、前台进程测试、五项后台测试与 HTTP/CLI 集成全部通过，然后实际后台启动成功。日志：`/tmp/loadout-make-isolated-ready.log`。
- 该隔离实例的 `make status` 显示实际地址；`make logs` 能输出日志，发送 Ctrl+C 后 Web 代理健康接口仍返回 200；`make down` 退出 0 且两个 TCP 端口均已释放。验证结束后清理临时 clone，保留日志。

## 范围说明

进程测试使用临时端口与临时 `LOADOUT_RUN_DIR`；不操作日常开发实例。后台命令在 Linux 验证；未执行 Windows/macOS 运行、Docker 镜像构建/容器运行或浏览器视觉检查。本任务没有修改 UI。工作期间出现的其他业务文件不属于本任务，保持原样。

## 追加：IDE NPM Scripts 入口

用户要求从 IDE 的 NPM Scripts 面板发现 Make 命令。根 `package.json` 增加启动、关闭、测试、构建、数据库与容器入口，直接转发已有 Make 目标；`lint/format` 同步转发三端 Make 命令。`dev` 保留原有 concurrently 命令，避免 `make dev → pnpm dev → make dev` 递归。

本轮仅修改工具配置和使用说明，没有新增自有运行逻辑，不另造业务 RED 记录。消费方验证：`pnpm run help`、`npm run help`、`pnpm exec biome check package.json` 均退出 0。按契约运行 `make check`，因当前 shell 未设置 `LOADOUT_TEST_DATABASE_URL` 在数据库前置检查处退出 2，后续检查未执行；未声称完整通过。
