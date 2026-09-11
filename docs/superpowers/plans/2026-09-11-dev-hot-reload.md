# 开发启动与 Go 自动重载执行记录

范围：用户要求关闭旧开发服务，用 IDE 的 pnpm dev 启动，并支持后端热更新。保留当前工作区其他改动，不提交、不推送。

## 根因与实现

- 原有 `.loadout/dev.sock` 主管运行默认 5173/8787，`.loadout/demo/dev.sock` 主管运行演示 5174/8788；IDE 的 web/dev 又绑定 5173 导致冲突。
- 通过各自主管的 down 关闭两套开发服务；关闭已确认的演示 mock PID 2008657。数据库、Redis、客户端 bridge 不属于本次旧开发服务清理。
- 根目录 `pnpm run dev` 继续复用 concurrently；Go 的 `make dev-server` 改用 Air 1.67.4，独立工具安装到忽略的 build/tools。版本核对官方 release API；配置参考 https://github.com/air-verse/air/blob/v1.67.4/air_example.toml。
- `.air.toml` 监听 server 的 Go/SQL/mod/sum，排除测试和数据目录，编译失败停止旧程序；SIGINT 退出与 2 秒宽限使用 Air 原生能力。
- `.vscode/tasks.json` 的 Loadout: dev 固定工作目录为仓库根目录；无需重复启动 web/dev。`.env` 仍由根目录 Node 启动命令读取，修改后重启整个任务。
- 工具安装在 setup、up、test-process、test-dev 的前置依赖中完成，避免首次安装消耗行为测试和后台就绪超时预算。

## RED / GREEN / REFACTOR

1. RED：`node --test tests/reload.test.mjs`，退出 1；真实临时 Go HTTP 服务初始返回 before，修改源码后仍返回 before，断言期望 after。不是编译或依赖失败。
2. GREEN：同一命令，退出 0；验证 before → after、编译错误后不可访问、修复恢复 recovered、SIGINT 后不可访问。
3. REFACTOR：Biome 格式化测试；按独立审查意见将 Air 安装移到测试前置依赖，再检查启动前置依赖。
4. 回归：`make test-process test-dev`，4 个进程测试和 7 个开发测试通过；覆盖前台 Ctrl+C 清理、子进程失败联动、后台重复启动/重启/停止、占用端口、失联 socket、主管异常退出、错误代理不能掩盖后端启动失败、源码重载。
5. 格式：`pnpm exec biome check --error-on-warnings tests/reload.test.mjs .vscode/tasks.json` 与 `git diff --check` 通过。

## 完整检查与现场验收

- 裸 `make check` 因未导出 LOADOUT_TEST_DATABASE_URL 停止。确认本地 .env 已有测试 PG/Redis 设置后，使用 `node --env-file-if-exists=.env -e 'const {spawnSync}=require("node:child_process"); process.exit(spawnSync("make",["check"],{stdio:"inherit"}).status ?? 1)'` 重跑。
- 重跑通过数据库/Redis环境检查、技能检查、Biome 和 Web typecheck，随后在 `golangci-lint fmt --diff` 因既有 `server/internal/app/directory.go` 导入分组格式退出 2。保留这处无关修改；后续完整 harness 未运行，不宣称全量通过。
- 完整检查日志保存在 `.loadout/verification/hot-reload-full-check.log`。
- 以根目录 `pnpm run dev` 实际运行，5173 `/api/v1/health` 返回 loadout/ok，8787 `/readyz` 返回 ready（Redis ready）。IDE 任务配置已加入；未通过 IDE UI 点击运行。未使用浏览器自动化。
- pnpm 的 native binary 未安装提示仍存在，但不阻止成功启动；本次未改全局 pnpm 安装。
