# PluginPocket 桌面端

独立 Tauri 本地应用，以概览、我的装备、运行日志、连接诊断四页管理本机接入。账号、余额、客户端配置、装备管理与诊断调用 `../cli` 的共享 Rust 库。关闭窗口隐藏到托盘，托盘菜单可显示或退出。

生产界面随二进制打包，不能导航到远程网页。前端只可调用受限 `local_command`；不提供 shell 或任意路径文件系统能力。日志导出由原生命令校验所选记录后写入固定的 `.pluginpocket/exports/` 下文件，并返回实际保存路径。用户的令牌只在内存中交给 Rust，并由共享库原子保存到本地凭证文件。

## 本机装备工作台

- **概览**：登录/退出、真实余额、客户端选择与配置；状态刷新或客户端读取失败时禁用配置写入。
- **我的装备**：分开展示 PluginPocket MCP 网关与本地安装装备。网关组从真实账号状态读取服务端已启用工具目录与本机已配置 bridge 的客户端；这些 MCP 服务在服务端运行，本机只保存 bridge 配置，无需逐项安装。目录不保证每项调用已获授权，实际调用仍受权限和额度约束；“管理网关接入”返回概览。本地组从托管清单读取直连 MCP 与 Skill，按类型、客户端和 slug 搜索筛选；网关组同样参与类型、客户端和工具名筛选。同 slug 的不同类型分别成行；版本未保存时显示“版本未记录”。装备组已展开为成员，不补造装备组来源。更新与卸载作用于本地装备行列出的实际安装目标，客户端筛选只筛选显示。
- **运行日志**：持久保存在 `.pluginpocket/operations.json`，最多保留最近 200 条配置、登录、装备操作与 bridge 失败记录。仅记录固定安全文案，不包含令牌、请求响应内容或用户输入的 URL；支持搜索、级别筛选、复制和导出当前筛选记录。导出成功显示原生写入后的实际路径，失败可重试。
- **连接诊断**：主动分别检查服务健康、账号凭证、三个客户端配置与 bridge 可执行文件，单项失败不隐藏其他结果。文件可读或服务健康不代表实际 MCP 工具调用成功；不会通过诊断执行工具调用。

卸载需行内确认。MCP 沿用配置冲突保护；Skill 卸载核对托管文件名与目录结构，阻止外来文件和符号链接，但不会比较已安装文件的内容，手动编辑过的托管文件也会删除。请先备份需要保留的技能内容，再确认卸载。

## 源码运行

先在仓库根 `pnpm install --frozen-lockfile`。Linux 需要 GTK 3、WebKitGTK 4.1 与 AppIndicator 开发库。

```sh
# 终端 1，从仓库根启动本地 UI
pnpm --dir desktop/ui dev

# 终端 2，启动原生应用
pnpm --dir desktop/ui tauri dev
```

桌面应用使用真实用户凭证与配置。开发/测试需隔离时，在启动命令上指定临时 `HOME` / `USERPROFILE` 与 `PLUGINPOCKET_CONFIG`；不要对日常客户端目录运行测试。

## 测试与构建

```sh
# 无 GUI 的共享命令行为测试
cargo test --manifest-path desktop/Cargo.toml --locked --no-default-features
# 本地组件、键盘、错误恢复、响应校验
pnpm --dir desktop/ui test
pnpm --dir desktop/ui build
# 原生全量测试（有系统 WebKit 依赖；bridge 测试不启动窗口）
cargo test --manifest-path desktop/Cargo.toml --locked
# Linux 安装包
pnpm --dir desktop/ui tauri build --bundles deb
```

产物在 `desktop/target/release/bundle/`。完整分发配置见下方；本机验收与远程平台验收分开记录。

## 桌面发行

`.github/workflows/release.yml` 使用原生平台 runner：

| 平台 | 架构 | 安装包 |
|---|---|---|
| Windows | x64 | MSI、NSIS exe |
| macOS | Apple Silicon、Intel | DMG、app.tar.gz |
| Linux | x64、ARM64 | deb、rpm、AppImage |

手机端不在本次范围。Windows ARM 目前只有 x64 兼容运行路径，未提供原生 ARM 包；不把它算作已验证架构。Linux 包在 Ubuntu 24.04 构建，目标系统仍需相容的 glibc/GTK/WebKit，AppImage 不代表所有 Linux 发行版均兼容。

发行使用独立 PluginPocket Minisign 密钥，公钥在 `desktop/tauri.conf.json`（`plugins.updater.pubkey`，单一来源）。所有安装包有同密钥生成的 `.sig` 和 SHA256SUMS；CI 将全部平台产物下载至独立任务，校验哈希并用 Minisign 验签，失败不发布。Tauri 的公私钥不匹配警告不能代替这个门禁。签名文件是 Tauri 使用的 Base64 编码 Minisign 格式，验签前先解码；公钥亦先从 JSON 的 `plugins.updater.pubkey` 解码。

应用内置自动更新（Tauri 官方 updater 插件）：端点为 GitHub Releases 的 `latest.json`，由 Release workflow 在验签通过后从五平台产物生成并上传草稿，`publish` 等待清单生成后才公开。客户端启动静默检查并支持手动「检查更新」，下载安装包后先以内置公钥 Minisign 验签再安装重启（Windows 由 NSIS passive 安装器完成重启）。验签失败拒绝安装，下载失败保留原版本可重试。macOS 使用 ad-hoc 应用封印并执行 `codesign --verify --deep --strict`，没有 Apple Developer ID 公证；Windows 未配置 Authenticode。发布签名保证文件来源和完整性，不消除 Gatekeeper/SmartScreen 提示。需要免提示发行时，必须使用相应受信任平台证书。

GitHub Actions Secret `TAURI_SIGNING_PRIVATE_KEY` 已配置独立私钥，`TAURI_SIGNING_PRIVATE_KEY_PASSWORD` 对当前无密码密钥可不设置。私钥备份由所有者保管，目录应为 0700、文件为 0600，禁止提交或随包上传；仓库更名不移动本机私钥。下面的文件路径须替换为实际备份位置，GitHub Secret 无法读回。更换密钥必须同步公钥，不要每次构建重新生成。

本地签名构建（仓库根）：

```sh
TAURI_SIGNING_PRIVATE_KEY="/absolute/path/to/release.key" \
APPIMAGE_EXTRACT_AND_RUN=1 \
pnpm --dir desktop/ui tauri build --ci --bundles deb,rpm,appimage \
  --config tauri.release.conf.json -- --locked
```

Tauri 自动签名 AppImage 更新包；CI 额外签名 deb、rpm、DMG 等首装文件。普通 `make build-desktop` 不需要发行私钥。构建钩子总是重新构建 UI，避免把旧前端打入新包。

发行 tag 必须为 `vMAJOR.MINOR.PATCH`，且与 CLI Cargo、desktop Cargo、desktop UI 和 Tauri 版本一致。手动触发可选择打包分支，并填写与 manifests 一致的 `release_tag`（如 `v0.3.0`）；仅生成 Actions 安装包产物并验签，不创建 Release、tag 或推送镜像。只有 tag push 发行流程先创建草稿，CLI/desktop/container、验签和 updater 清单全部成功后才公开。平台安装包包含 bridge 验证门禁，图形窗口/托盘与目标机器首次安装仍须实际验收。真实命令和限制见 [执行记录](../docs/superpowers/plans/2026-09-11-desktop-release.md)。

客户端配置中的可执行文件是当前桌面程序的绝对路径，参数固定为 `bridge`。这种模式在 GUI 初始化之前启动共享 MCP bridge，桌面窗口不需要保持打开。删除/移动应用后应重新配置客户端；退出登录会移除本地凭证，客户端配置保留以便下一次登录继续使用。

原生应用与托盘图标直接复用 Web 的 `web/public/icon-512.png` 薄荷绿工具箱角色，PNG、ICO、ICNS 使用官方 Tauri icon 命令派生，不维护另一套字标。生成命令为 `pnpm --dir desktop/ui tauri icon ../web/public/icon-512.png --output /tmp/pluginpocket-icons`；只更新配置引用的32/128/256px PNG、ICO和ICNS（256px来自128x128@2x.png），不引入未使用的平台资源。

## 工坊界面

桌面操作页与 Web 使用一致的 iOS 风格中性浅深色、黄色主操作和清晰输入边界，优先使用系统字体，Manrope 为本地后备。卡片圆角 16px、按钮圆角 12px，以轻柔阴影区分层次。顶部“外观”复用共享 Select，默认跟随系统，可通过键盘选择浅色/深色；手动选择只保留在当前窗口生命周期内，不写入本地配置。工作台侧栏宽 244px，1100px 以下缩为 216px 并将概览改为单栏；760px 以下导航移到顶部四列。主内容最大 1440px，页面标题 28px、窄窗口 24px，列表动作与筛选按可用宽度换行。

品牌标记和概览插画从 `web/public/images/workshop-mark.webp`、`workshop-desktop.webp` 导入，字体从 `web/public/fonts/` 引用，由 Vite 打包为本地资源；不复制界面插画；原生应用/托盘图标复用 Web 工具箱图标。插画仅装饰，不承载操作文字。

2026-09-11 初版界面历史顺序证据（不代表当前工作台验收）：

- RED：`pnpm --dir desktop/ui test -t appearance`，失败于无法找到名为“外观”的 combobox；不是依赖、导入或语法失败。
- GREEN：添加主题选择后运行同一命令，1 项通过（其余测试由聚焦过滤排除）。验证浅色/深色/系统的浏览器原生色彩方案以及不触发本地命令。
- REFACTOR：统一工坊 token、字体、装饰素材和响应式布局；`pnpm --dir desktop/ui test` 6 项通过，涵盖键盘重试、待处理禁用、令牌清除、配置/移除/退出和过期状态恢复；`pnpm --dir desktop/ui build` 通过，插画/字体随生产产物输出。
- `pnpm exec biome check desktop/ui/src/App.tsx desktop/ui/src/App.test.tsx desktop/ui/src/styles.css` 检查格式与静态规则；不用于证明 TDD 顺序或视觉验收。

启动 Vite 后可在 `http://127.0.0.1:1420/` 实时预览并接收 HMR。普通浏览器明确显示“浏览器预览”及原生能力不可用说明；可以切换页面、筛选和外观，不读取本地装备/日志，不提供模拟账号、数据或配置成功。当前工作台验证见 [执行记录](../docs/superpowers/plans/2026-09-12-desktop-workbench.md)。按项目约定不使用浏览器自动化；本次文档同步未进行实际宽窄窗口、浅深主题或原生 GUI 视觉验收，组件测试和构建不能替代这些检查。

### iOS 细节与首次加载

按钮、输入、外观选择和客户端选项提供悬停、按下、键盘焦点与禁用反馈；禁用按钮不缩放，减少动态效果偏好会关闭过渡与骨架呼吸动画。账号和客户端首次读取分别显示有无障碍状态说明的骨架；各自完成后替换为真实内容，后台刷新保留已有内容。

- RED：`pnpm --dir desktop/ui test -t 'initial loading'`，失败于找不到“正在读取本地状态”的 status；确认是缺少用户可观察的加载状态。
- GREEN：增加独立骨架与区域 `aria-busy` 后，同一命令 1 项通过。覆盖两个请求分别完成、真实账号与客户端替换骨架，以及初始配置按钮禁用。
- REFACTOR：统一浅深色、系统字体、圆角和交互细节后，`pnpm --dir desktop/ui test` 7 项通过，`pnpm --dir desktop/ui build` 通过（包含 TypeScript 检查）。
- `impeccable detect` 已检查组件与样式；现有字号阶梯、系统等宽字体和轻阴影提示为设计规范差异，未作为行为验证。此次不新增依赖、不改变本地命令或真实客户端配置。
