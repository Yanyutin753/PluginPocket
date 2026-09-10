# Loadout 桌面端

独立 Tauri 本地应用。界面用中文显示账号、余额、连接状态和本机客户端；登录、配置、移除和注销调用 `../cli` 的共享 Rust 库。关闭窗口隐藏到托盘，托盘菜单可显示或退出。

生产界面随二进制打包，不能导航到远程网页。前端只可调用受限 `local_command`；不提供 shell、任意路径或文件系统能力。用户的令牌只在内存中交给 Rust，并由共享库原子保存到本地凭证文件。

## 源码运行

先在仓库根 `pnpm install --frozen-lockfile`。Linux 需要 GTK 3、WebKitGTK 4.1 与 AppIndicator 开发库。

```sh
# 终端 1，从仓库根启动本地 UI
pnpm --dir desktop/ui dev

# 终端 2，启动原生应用
pnpm --dir desktop/ui tauri dev
```

桌面应用使用真实用户凭证与配置。开发/测试需隔离时，在启动命令上指定临时 `HOME` / `USERPROFILE` 与 `LOADOUT_CONFIG`；不要对日常客户端目录运行测试。

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

产物在 `desktop/target/release/bundle/deb/`。macOS / Windows 构建可传 `--bundles dmg` / `--bundles nsis` 并提供对应平台图标及打包依赖；本次只验证 Linux，不宣称跨平台签名发行已完成。

客户端配置中的可执行文件是当前桌面程序的绝对路径，参数固定为 `bridge`。这种模式在 GUI 初始化之前启动共享 MCP bridge，桌面窗口不需要保持打开。删除/移动应用后应重新配置客户端；退出登录会移除本地凭证，客户端配置保留以便下一次登录继续使用。

`icon.svg` 为仓库内几何盾牌/L 标记，沿用 Loadout 颜色；PNG 使用官方 Tauri icon 命令从该 SVG 生成，未使用生成式位图素材。

## 工坊界面（2026-09-11）

桌面操作页与 Web 使用一致的 iOS 风格中性浅深色、黄色主操作和清晰输入边界，优先使用系统字体，Manrope 为本地后备。卡片圆角 16px、按钮圆角 12px，以轻柔阴影区分层次。顶部提供原生“外观”选择，默认跟随系统，可通过键盘选择浅色/深色；手动选择只保留在当前窗口生命周期内，不写入本地配置。大屏内容最大宽度 96rem，宽窗口双栏，700px 以下按账号、客户端顺序单栏排列。

品牌标记和工具插画从 `web/public/images/workshop-mark.webp`、`workshop-tools.webp` 导入，字体从 `web/public/fonts/` 引用，由 Vite 打包为本地资源；不复制素材，不更改原生应用/托盘图标。插画仅装饰，不承载操作文字。

本轮顺序证据：

- RED：`pnpm --dir desktop/ui test -t appearance`，失败于无法找到名为“外观”的 combobox；不是依赖、导入或语法失败。
- GREEN：添加主题选择后运行同一命令，1 项通过（其余测试由聚焦过滤排除）。验证浅色/深色/系统的浏览器原生色彩方案以及不触发本地命令。
- REFACTOR：统一工坊 token、字体、装饰素材和响应式布局；`pnpm --dir desktop/ui test` 6 项通过，涵盖键盘重试、待处理禁用、令牌清除、配置/移除/退出和过期状态恢复；`pnpm --dir desktop/ui build` 通过，插画/字体随生产产物输出。
- `pnpm exec biome check desktop/ui/src/App.tsx desktop/ui/src/App.test.tsx desktop/ui/src/styles.css` 检查格式与静态规则；不用于证明 TDD 顺序或视觉验收。

可在 `http://127.0.0.1:1420/` 检查界面；普通浏览器没有 Tauri 本地命令能力，会显示实际读取/连接错误，不提供模拟账号或配置成功。完整 `make check`、宽窄窗口视觉检查和原生窗口平台验收由本轮整体任务记录汇总；组件测试与构建不代表这些检查已完成。

### iOS 细节与首次加载

按钮、输入、外观选择和客户端选项提供悬停、按下、键盘焦点与禁用反馈；禁用按钮不缩放，减少动态效果偏好会关闭过渡与骨架呼吸动画。账号和客户端首次读取分别显示有无障碍状态说明的骨架；各自完成后替换为真实内容，后台刷新保留已有内容。

- RED：`pnpm --dir desktop/ui test -t 'initial loading'`，失败于找不到“正在读取本地状态”的 status；确认是缺少用户可观察的加载状态。
- GREEN：增加独立骨架与区域 `aria-busy` 后，同一命令 1 项通过。覆盖两个请求分别完成、真实账号与客户端替换骨架，以及初始配置按钮禁用。
- REFACTOR：统一浅深色、系统字体、圆角和交互细节后，`pnpm --dir desktop/ui test` 7 项通过，`pnpm --dir desktop/ui build` 通过（包含 TypeScript 检查）。
- `impeccable detect` 已检查组件与样式；现有字号阶梯、系统等宽字体和轻阴影提示为设计规范差异，未作为行为验证。此次不新增依赖、不改变本地命令或真实客户端配置。
