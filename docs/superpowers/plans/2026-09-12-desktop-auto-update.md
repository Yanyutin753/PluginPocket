# 桌面自动更新 + v0.3.0 发版执行记录（2026-09-12）

设计见 `../specs/2026-09-12-desktop-auto-update-design.md`。本记录保留 RED/GREEN 顺序证据；CI 证明最终状态。

## TDD 证据

### 1. updater manifest 生成脚本（node）

- RED：`node --test tests/updater-manifest.test.mjs` → `not ok 1..3`（桩 `buildUpdaterManifest` 返回空 `{platforms:{}}`，失败来自平台映射/缺失抛错/版本一致性断言）。初次 GREEN 过程中修正两处测试自身断言（darwin 平台报错顺序、缺产物错误信息不含 signature 字样），实现未变。
- GREEN：`node --test tests/updater-manifest.test.mjs` → `# pass 3 / # fail 0`。
- 回归：`node --test tests/release-preflight.test.mjs tests/updater-manifest.test.mjs tests/integration.test.mjs` → `# pass 12 / # fail 0`。

### 2. release-preflight updater 配置校验

- RED：桩 `validateUpdater(){}` + 新用例 → `not ok 5 - updater config requires reachable endpoints and a minisign pubkey`。
- GREEN（实现 endpoints https / minisign pubkey base64 / installMode passive 校验并接入 CLI 读取 `desktop/tauri.conf.json`）→ `# pass 5 / # fail 0`。
- 集成验证：`RELEASE_TAG=v0.3.0 TAURI_SIGNING_PRIVATE_KEY=dry node scripts/release-preflight.mjs` → `Release version and signing configuration present`。

### 3. 桌面配置防漂移（Rust）

- RED：`cargo test --locked --test updater_config` → `updater_config_is_release_ready` FAILED（`plugins.updater configured` panic；当时 tauri.conf.json 无 updater 块），`tauri_config_version_matches_cargo_manifest` pass。
- GREEN（tauri.conf.json 增加 endpoints/pubkey/passive；release 覆盖配置仅留 `createUpdaterArtifacts`；依赖 `tauri-plugin-updater =2.11.0`、`tauri-plugin-process =2.3.1` 按 optional + native feature 接入；main.rs 注册插件；capabilities 增加 `updater:default`、`process:allow-restart`）→ `2 passed`。
- 全量：`cargo test --manifest-path desktop/Cargo.toml --locked` → 13 passed / 0 failed（commands 10、native_bridge 1、updater_config 2）。

### 4. UI UpdatePanel（vitest）

- RED：`pnpm --dir desktop/ui exec vitest run src/UpdatePanel.test.tsx` → `8 failed`（组件桩返回 null，按钮/文案查询全部落空）。
- GREEN：`8 passed`。期间两处测试修正（队列式 check mock 区分启动静默检查与手动检查；downloadAndInstall 改为测试手动 resolve 的 pending promise 以观察下载中状态），组件实现未因测试而变。
- 回归：`pnpm --dir desktop/ui exec vitest run` → `28 passed (28)`（App 5、Workbench 15、UpdatePanel 8）。
- 构建：`pnpm --dir desktop/ui build`（tsc --noEmit + vite build）通过。

### 5. 版本统一 v0.3.0（7 处）

cli/Cargo.toml、desktop/Cargo.toml、desktop/ui/package.json、desktop/tauri.conf.json、web/package.json、docs/openapi.json、App.tsx 侧栏（改运行时 `getVersion()`，预览回退 package.json）。两个 Cargo.lock 经 `cargo update -p <pkg>@0.1.0` 更新且无依赖漂移；`cargo metadata --locked` 双工作区一致。
随 bump 修正 `cli/tests/doctor.rs` 硬编码 `0.1.0` → `env!("CARGO_PKG_VERSION")`（防漂移语义保留）。`cargo test --manifest-path cli/Cargo.toml --locked` 全绿（doctor 6/6）。

## 静态检查

- `pnpm exec biome check --error-on-warnings <changed files>` → 清洁（自动整理 7 个文件的 import 后复检通过）。
- `cargo fmt --manifest-path desktop/Cargo.toml --all -- --check` → OK。
- `cargo clippy --manifest-path desktop/Cargo.toml --locked --all-targets -- -D warnings` → 0 error。
- `.github/workflows/release.yml` YAML 解析通过；新增 `updater-manifest` job（needs desktop+verify-signatures，产物合并后生成 latest.json 并上传草稿），`publish` needs 增补该 job；verify-signatures 公钥读取源改为 `desktop/tauri.conf.json`。

## 未在本地执行的检查（环境受限）

`make check` 完整链路（Go 需 `PLUGINPOCKET_TEST_DATABASE_URL` 真实 PostgreSQL 与 Redis，本机未配置）、web vitest（仅版本字段变更）、`make lint` 的 golangci-lint 段（server 无代码变更）、跨平台桌面构建与 Tauri 签名产物（由 Release workflow 五平台 runner 验证）。CI 为最终门禁。

## 验收映射

- 设计 §组件 1 配置 → 测试 2/3；§组件 2 Rust → 测试 3 + clippy；§组件 3 UI → 测试 4；§组件 4 CI manifest → 测试 1 + workflow 语法与 job 编排。
- 安全（验签失败拒绝安装、失败侧退款式保守路径）→ updater 插件内置验签 + manifest 仅在 verify-signatures 之后生成；发布门禁链 `publish needs updater-manifest`。
