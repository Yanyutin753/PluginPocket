# CI 工作流重构执行记录（2026-09-12）

**目标**（用户要求）：GitHub Actions 命名更专业；补 SQL 双方言、前后端 lint/报错、e2e 等更多可见检测。

**事实前提**：这些检测原本已全部包含在 `make check` 中，由 ci.yml 单个 `harness` job 单步执行——命令等价但 UI 不可见、失败定位慢。本次重构将其拆为按类别命名的并行 job，命令本身与 `make check` 各步一一对应（未改任何 make 目标）。

## 变更

### `.github/workflows/ci.yml`（重构）

| job | 覆盖的 `make check` 步骤 | 服务依赖 |
|---|---|---|
| `lint`（Lint & static analysis） | `skills-check`、`lint`、`lint-desktop` | 无（需 GTK apt 依赖供桌面 clippy） |
| `sqlite-track`（SQL migrations dual-dialect） | `test-server` 中的 SQLite 轨道（`go test -race -run TestSQLite ./internal/store`） | 无，纯 Go |
| `server`（Server tests, PostgreSQL + Redis, -race） | `test-server` 全量 | PG 18.6 + Redis 8.10.1 services |
| `web`（Web component tests） | `test-web` | 无 |
| `desktop`（Desktop tests, 三平台矩阵） | `test-desktop`（Linux 走 `make test-desktop` 同款底层命令展开，因 Windows runner 无 make；mac/win 与 release.yml 已验证命令集一致） | 无（仅 Linux 需 GTK apt 依赖） |
| `cli-portability`（CLI cross-platform, 矩阵含 os） | `test-cli`（Linux 腿并入矩阵）+ fmt/clippy/build | 无 |
| `container`（Container image build） | 原 harness 的 `docker build` 步骤 | Docker |
| `e2e`（End-to-end product journey） | release 预检 node 测试、`test-e2e`、`test-process`、`test-dev`、`integration`；上传 `e2e-evidence`（旅程日志 + deb） | PG + Redis services |

- `harness.log`/`harness-results` 产物更名为 `e2e-journey.log`/`e2e-evidence`（日志只覆盖 e2e job 范围，不再声称完整 harness 日志）。
- 全部 step 补齐祈使句命名；矩阵 job 显示矩阵维度（如 `CLI cross-platform (ubuntu-latest)`）。

### `.github/workflows/build-and-push.yml`（命名）

- 工作流名 `Build & Push` → `Build & Publish`；job 加显示名（Docker image (ghcr.io) / CLI binaries (${{ matrix.name }})）；全部 step 命名。触发条件、矩阵、构建命令零改动。

### `.github/workflows/release.yml`（命名）

- job 加显示名（Release preflight / Create draft release / CLI binaries / Docker image / Desktop installers / Verify installer signatures / Publish release）；裸 step 补名。依赖图、权限、构建与验签逻辑零改动。

### 文档

- `docs/HARNESS.md` 失败定位章节的 CI 描述改为新并行结构，注明"各 job 合计覆盖 make check 全部步骤；新增检查时同步补进对应 job"。

## 验证证据（本地，工作区含并行会话 billing-roles 在途改动）

| 命令 | 结果 |
|---|---|
| `python3` yaml.safe_load 三工作流 | 全部解析 OK（CI 8 jobs / Build & Publish 2 / Release 7） |
| `actionlint 1.7.7`（安装至 build/tools/，未入库未进 CI） | ci.yml、build-and-push.yml 零告警；release.yml 唯一报 `macos-15-intel` 未知标签——为既有 label（GitHub 2025 新增，actionlint 标签库滞后），仓库历史 release 均以该矩阵跑绿，不改 |
| `cd server && go test -race -count=1 -run TestSQLite ./internal/store` | `ok ... 25.443s` |
| `make skills-check` | 通过 |
| `node --test tests/release-preflight.test.mjs` | fail 0 |
| `make lint` | biome 141 文件无问题、tsc 无错误、golangci fmt --diff 空、run 0 issues、go mod tidy -diff 空、cargo fmt/clippy 干净 |
| `pnpm --dir web test` | 34 文件 234 用例全过（22.67s） |
| `make lint-desktop` | desktop/ui 构建成功、cargo fmt 干净、clippy `-D warnings` 通过 |

## 远端首跑证据

- run 34670426804（desktop 三平台矩阵版）：11 job 中 10 绿——三平台 Desktop（mac 5m25s / linux 3m40s / win 7m33s）、E2E 10m13s、Server 7m13s、Lint 5m2s、CLI 三平台、SQLite 轨道 1m23s、容器 1m10s 全过；唯一失败为 Web job 的 `PluginProxy.test.ts` teardown `ENOTEMPTY`（Vite 冷缓存依赖优化器与递归 rm 的 TOCTOU 竞态，与断言无关，旧 harness 同样可能偶发）。
- 修复 1：rm 增加 Node 内置 `maxRetries/retryDelay`（ENOTEMPTY 为官方覆盖重试码），本地全量 234 用例过；commit `c55632f`。
- run 34671051168：Web 修复生效，但 `Desktop tests (windows-latest)` 41s 失败：`ERR_PNPM_VERIFY_DEPS_BEFORE_RUN × Cannot check whether dependencies are outdated`。同一命令上一轮 Windows 绿——pnpm 状态文件（`node_modules/.pnpm-workspace-state-v1.json`）在 Windows 偶发不可读，叠加仓库 `verifyDepsBeforeRun: error` 策略成为误杀。
- 根因复现（本地，确定性）：移除该状态文件 → `pnpm --dir web run typecheck` 报同样错误。逐项验证覆盖通道：`npm_config_verify_deps_before_run` env（无效）、`--config.verifyDepsBeforeRun`（无效）、`.npmrc` 追加（无效，workspace yaml 优先）、`--config.verify-deps-before-run=false`（有效）、`--config.verify-deps-before-run=install`（有效且自愈——自动补装 235ms 并重建状态文件，exit 0）。
- 修复 2：ci.yml（web/desktop job）与 release.yml（desktop job 的 UI 测试与 tauri build）直接调用 pnpm 脚本处统一加 `--config.verify-deps-before-run=install`；CI 内 frozen install 刚完成、策略为 install 仅在状态不可读时补装，不削弱本地 error 策略；make 内部调用的 pnpm 无法加参数，但相关 job 均为 ubuntu，未观察到该偶发。

## 未验证范围（如实说明）

- 新工作流未在 GitHub runner 上实际执行过：远端 CI 首跑是最终证明。各 job 命令与原 `make check`/旧工作流步骤逐字相同或为其直接子集，风险集中在 job 环境拼装（已按旧 harness 的 services/apt/工具链步骤镜像）。
- `server`（PG+Redis 全量）、`desktop`、`e2e`、`container`、CLI 三平台矩阵未在本地重跑（重命令本地耗时且依赖外部服务/平台，行为与 `make check` 内部步骤一致）；本地已验证其快速核心（SQLite 轨道、lint、web、预检）。
- 若仓库配置了分支保护必需状态检查（如 `harness`），job 更名后需同步更新；当前仓库直推 main，未见此配置。
