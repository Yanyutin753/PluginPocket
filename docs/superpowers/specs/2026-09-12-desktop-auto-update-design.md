# 桌面客户端自动更新设计（2026-09-12）

## 目标

PluginPocket 桌面应用在 v0.3.0 起内置自动更新：启动时静默检查新版本，用户确认后下载、校验签名并重启安装。不引入自建更新服务器，签名体系完全复用现有发布 Minisign 密钥。

## 方案选型

对比过三条路线：

1. **Tauri 官方 `tauri-plugin-updater` + `tauri-plugin-process` + GitHub Releases 静态 `latest.json`**（选定）：官方开源组件、原生支持 Minisign/dsa 验签、断点与代理由系统 HTTP 栈处理、`createUpdaterArtifacts` 已在发布配置启用、五平台产物与 `.sig` 已由 Release workflow 生成。零自建基础设施。
2. 自建更新服务（PluginPocket 服务端下发清单）：需要每个自托管实例托管安装包，成本高且安装包只在 GitHub Releases 存在，被否决。
3. 只提示新版本、跳转浏览器下载（无自动安装）：实现最小，但"比较完美"的目标要求一键完成，被否决。

关键约束：更新二进制的来源必须是 GitHub Releases（安装包唯一发布地）；中国网络可达性后续可通过 updater 的多端点回退追加镜像，本期不做。

## 组件

### 1. 客户端配置（`desktop/tauri.conf.json`）

`plugins.updater` 增加（pubkey 从 `tauri.release.conf.json` 移入，单一来源）：

- `endpoints`: `https://github.com/Yanyutin753/PluginPocket/releases/latest/download/latest.json`
- `pubkey`: 现有发布公钥（Base64 Minisign 公钥）
- `windows.installMode`: `passive`（无交互进度条，安装后自动运行）

`tauri.release.conf.json` 仅保留 `bundle.createUpdaterArtifacts: true`。`scripts/release-preflight.mjs` 与 workflow 验签步骤改从基础配置读公钥。

### 2. 桌面 Rust（`desktop/`）

- 依赖：`tauri-plugin-updater`、`tauri-plugin-process`（精确版本，随锁文件提交）。
- `main.rs` 注册两个插件；更新逻辑不写自定义 Tauri command，UI 直接使用官方 JS API（`@tauri-apps/plugin-updater` / `plugin-process`），保持与现有 zod 校验分层一致。
- `capabilities/main.json` 增加 `updater:default`、`process:allow-restart`。
- 防漂移测试：`desktop/tests/updater_config.rs` 断言 updater 端点/pubkey/installMode 存在且 `tauri.conf.json` 版本与 Cargo 清单一致。

### 3. 桌面 UI（`desktop/ui/src`）

- `updater.ts`：官方插件的薄封装，导出 `currentVersion()`、`checkForUpdate()`、`downloadAndInstall(onProgress)`、`relaunch()`，返回值经 zod 校验；测试 mock 该模块。
- `UpdatePanel.tsx`（仅 Tauri 环境渲染，挂概览页底部）：状态机 idle → checking → available/uptodate → downloading → ready，error 可重试；启动静默检查失败不打扰用户（仅手动检查显示错误）；下载显示进度（无 contentLength 时显示不确定态）；安装完成提供「立即重启」（Windows 由 NSIS passive 安装器负责重启，relaunch 失败提示手动重开）。键盘可操作、`role="status"`/`role="alert"` 语义、动效尊重 `prefers-reduced-motion`。
- 侧边栏版本号改为运行时 `getVersion()`（浏览器预览回退 package.json），消除硬编码。

### 4. Release workflow（`.github/workflows/release.yml`）

- 新增 `scripts/build-updater-manifest.mjs`：从验签后的 `desktop-*` 产物目录构建 `latest.json`（version/pub_date/notes + 五平台 `{url, signature}`，签名取产物同名 `.sig` 内容）；任何平台缺失或签名为空即失败。纯函数可测，CLI 仅做参数解析。
- 新 job `updater-manifest`：在 `desktop` 与 `verify-signatures` 之后下载合并产物、生成并校验 `latest.json`、上传草稿 Release。
- `publish` job 的 `needs` 加入 `updater-manifest`：清单未生成不发布。

## 数据流

```
CI: tag push → 五平台构建签名 → verify-signatures（Minisign 全量验签）
    → build-updater-manifest 生成 latest.json → 上传草稿 → publish
客户端: 启动/手动 → GET releases/latest/download/latest.json
    → semver 比较 → 用户确认 → 下载安装包 → 内置 pubkey Minisign 验签
    → 安装 → relaunch（Windows 由安装器处理）
```

## 安全与失败模式

- 安装包 URL 指向本仓库 Release 资产；updater 先验 Minisign 签名再落盘执行，公钥编译进应用，端点走 HTTPS。
- `latest.json` 生成于全量验签之后：签名不通过的产物到不了清单；发布顺序上 publish 等待清单与全部平台成功。
- 失败侧安全：检查失败静默（不阻塞使用）、下载失败保留原版本并允许重试、验签失败拒绝安装。
- 密钥边界：私钥仅存在于 GitHub Secret 与所有者备份；`latest.json`、pubkey 均为公开信息。

## 测试

- Node：`tests/updater-manifest.test.mjs`（平台映射、URL 形态、缺签名/缺平台报错、版本与 tag 一致）；`release-preflight` 增加 updater 配置校验用例。
- Rust：`desktop/tests/updater_config.rs` 配置防漂移。
- UI：`UpdatePanel.test.tsx` 覆盖状态机、键盘操作、错误重试、进度与静默启动检查；`App.test.tsx` 保持通过（预览态不渲染更新面板）。
- CI：`make check` 全量；Release workflow 五平台真实构建 + 验签 + 清单校验为最终门禁。

## 范围外

CLI 自更新（cargo/脚本分发，建议后续用版本提示 + 文档指引）、更新渠道/灰度、企业代理鉴权、中国镜像端点、Sparkle 风格差量更新。
