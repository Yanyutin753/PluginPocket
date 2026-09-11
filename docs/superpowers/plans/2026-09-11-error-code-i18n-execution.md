# 错误码标准化 + 全站 i18n 执行记录（2026-09-11）

设计文档：`docs/superpowers/specs/2026-09-11-error-code-i18n-design.md`。任务：后端错误码注册表 + 机器契约 + 前端按码双语字典 + i18n 缺口填补。

## RED / GREEN / REFACTOR 证据

### 后端（Go）

1. **RED**：`go test ./internal/httpapi -run TestErrorCodes` → 编译失败（`Codes`/`statusesFor`/`AllowedStatus` 未定义）——待实现注册表 API 的有效失败。
   **GREEN**：实现 `server/internal/httpapi/codes.go`（53 码注册表 + `Fail` 写出器）后，`TestErrorCodesRegistry` 通过。
2. **RED（真实缺口）**：`TestEmittedCodesRegistered` 抓到 `username_taken` 未注册——扫描门在第一次运行就发现人为遗漏。
   **GREEN**：补登记后通过；含 `http.Status*` 常量形式的调用（settings.go/settingsFailure、account.go）。
3. **RED**：`TestNoPlainTextHTTPErrors` 列出 gateway.go 5 处 `http.Error` 与 application.go 的 `http.NotFound`/`http.Error`（git 路由）。
   **GREEN**：全部改走 `httpapi.Fail`（新增码 `marketplace_unavailable`）；`app.fail`/`identity.failure` 变为 `httpapi.Fail` 薄封装。
   行为锁定：`gateway_test.go` 的 `TestGatewayRequiresToken` 增加断言——401 响应体必须精确等于 `{"error":"unauthorized"}\n` 且 Content-Type 为 application/json。
4. 契约金样：`go test ./internal/httpapi -run TestErrorCodesContract -update` 生成 `web/src/i18n/error-codes.json`（53 码），复跑 `go test ./internal/httpapi` 全绿。
5. 回归：`go test ./internal/identity ./internal/marketplace ./internal/store ./internal/httpapi ./internal/gateway ./internal/app ./cmd/loadout-server` 全部 ok；`internal/app` 的 DB 门控用例无 `LOADOUT_TEST_DATABASE_URL` 时按设计跳过（146 RUN / 0 FAIL）。

### 前端（vitest）

1. **RED**：`src/i18n/errors.test.ts` → `./errors` 模块不存在（transform 失败）。
   **GREEN**：新建 `src/i18n/errors.ts`（53 后端码 + 前端本地码 `invalid_json`/`unknown`，双语），5 用例通过：契约覆盖、幻影码为零、双 locale 回退、按 locale 解析。
2. `api.ts`：删除旧 `messages` 硬编码映射与幻影码 `invalid_settlement_script`、`seat_limit`；`ApiError.message` = `errorText(code, 'zh-CN')`。
3. `ErrorNotice`（shared.tsx）：ApiError 按 `error.code` 渲染 `errorText(code, locale)`；非 ApiError 保持 `t()`。
4. **行为改进带来的期望更新**（有意变更，非断言删除）：
   - `Localization.test.tsx`：`internal_error` 现有专属文案，断言由回退文案改为 'The service hit an internal error'；新增非 JSON 错误体 → `unknown` 回退的集成用例（桩需前缀匹配带查询串的列表 URL）。
   - `Product.test.tsx`/`ToolEditor.test.tsx`：`internal_error` 桩对应断言改为 '服务内部错误，请稍后重试'；4 处 schema 解析失败（unknown 路径）断言保持回退文案。
5. 字典清理：33 个孤儿错误文案键删除（共 57 条含跨文件重复）；重复键去重 42 条（保留合并顺序赢家，渲染语义不变，全量 204 用例通过）；残留重复 0。
6. i18n 缺口：页脚标语 'Your AI, fully loaded.' ×4 与 OverviewPage 装饰句进 `t()`（新键 '你的 AI，准备就绪。'、'工欲善其事，'、'必先利其器。'——工坊主题）。
7. 回归：`pnpm exec vitest run` → **29 文件 / 204 用例全过**；`pnpm exec tsc --noEmit` → 0 错误。

### 契约三方一致性

`web/src/i18n/error-codes.json` ↔ `httpapi/codes.go` 注册表 ↔ `docs/openapi.json` Error enum：53 码完全一致（脚本核对 `contract==registry: True`、`enum==contract keys: True`）；`docs/API.md` 错误码注册表 53 行人工同步。

## 验收映射

- 后端错误码唯一事实源 + 防漂移：codes.go + TestEmittedCodesRegistered + TestNoPlainTextHTTPErrors。
- 前端按码 i18n：errors.ts + errors.test.ts 覆盖锁定 + ErrorNotice 改造 + Localization 集成断言。
- 全站 JSON 错误 envelope：gateway/git 路由统一 `{"error":code}`，`/readyz` 保持 K8s 惯例。
- 文档同步：API.md（注册表 + 移除"可能为纯文本"）、PLAN.md 第 8 章、openapi enum、AGENTS.md 模块地图两行。

## 未验证范围

- `LOADOUT_TEST_DATABASE_URL` 未配置（本地 PG 凭据不可得）：`internal/app` DB 门控行为用例、`make check` 的 database-check/test-e2e/test-product 未跑；改动仅重接线 `fail`（envelope 字节不变），风险由扫描门与无 DB 套件覆盖。
- 视觉验收（桌面/移动端页脚标语双语渲染）按契约需人工检查，未做浏览器自动化（用户要求）。

## 修复记录（验证过程中发现）

- biome 将 Go 生成的 `error-codes.json` 重排格式导致金样字节比对失败：契约测试改为语义比对（排序后 DeepEqual），并在根 `biome.json` `files.includes` 排除该生成物；`-update` 重新生成后 race 与 biome 双通过。
- python 重写 `docs/openapi.json` 改变数组排版：交回 biome 统一格式（语义不变，enum 53 码完好）。

## 完善阶段（2026-09-12，"全部优化 完善"）

1. **DB 门控套件补齐**：从根目录 `.env` 取得 `LOADOUT_TEST_DATABASE_URL`，`go test -race -count=1 ./internal/app ./internal/gateway ./internal/identity ./cmd/loadout-server` —— gateway/identity/cmd ok；app 首跑 `TestGitHubSkillSyncPublishesStableSnapshot` 一次失败（502 upstream_unavailable），隔离 `-count=3` 3/3 通过、全包复跑 ok（215s）：确认负载型 flaky，路径与本任务 diff 无交集（marketplace 为在途修改文件），非回归。
2. **desktop 锁修复**：`desktop/Cargo.lock` 与 Cargo.toml 失配为 HEAD 既有状态；最小 `cargo update`（仅 smallvec 1.16.0→1.16.1、toml 1.1.5→1.1.6 两个补丁版）后 `make test-desktop` 全绿（UI 7/7 + 双特性 cargo 全过）。
3. **message 直通审计**：全部 `error instanceof Error ? error.message` 直通点（SettlementEditor×2、ToolEditor×1）的抛错源均为项目中文 Error（JSON.parse 已被 settlementValue/jsonObject 包装），en 词条零缺失——无原生英文泄漏，无需改动。
4. **openapi /mcp 描述修正**：HTTP 层（401/405/503）描述改为"返回统一 REST Error 结构"；混合层（400/403/413）说明协议错误除外；enum 53 码完好。
5. `make redis-check`（带 `LOADOUT_TEST_REDIS_URL`）、`make lint-desktop` 通过。
6. `make test-e2e`（真实服务 + Web + CLI + 桌面 bridge 全链路，含 web e2e tsx）见下表。

## 最终验证（汇总）

| 命令 | 结果 |
|---|---|
| `go test -race ./...`（无 DB，跳过门控） | 全部 ok |
| `go test -race -count=1 ./internal/app ./internal/gateway ./internal/identity ./cmd/loadout-server`（真实 PG） | 全部 ok（app 含一次负载 flaky，复跑绿） |
| `cargo test --manifest-path cli/Cargo.toml --locked` | 全部通过 |
| `pnpm --dir web exec vitest run` / `tsc --noEmit` | 204/204；0 错误 |
| `make lint` / `make lint-desktop` / `make skills-check` / `make redis-check` / `make database-check` | 通过（后两者带 .env 测试实例） |
| `make test-process` / `make test-dev` / `tests/integration.test.mjs` / `tests/release-preflight.test.mjs` | 全部通过 |
| `make test-desktop` | 全绿（锁修复后） |
| `make test-e2e` | 通过（143s）：真实 Web e2e、RealCLI、Redis 共享目录与故障演练、重启会话与记账保持、双实例不超扣、崩溃恢复不重放、管理员定价退款、桌面 bridge 真实服务全过 |

`make check` 各子目标至此全部执行通过（database-check/redis-check 需从 `.env` 导入测试实例变量）。


