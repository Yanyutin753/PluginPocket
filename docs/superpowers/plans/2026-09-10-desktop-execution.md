# Tauri 本地端实现与验证记录

2026-09-10。对应 product.md Task 6，用户已授权当前工作区连续实现；不提交/推送，不写真实 HOME。

## 实施决定

- 桌面本地独立 React/Vite 面板；保留 DESIGN 深色 tokens、中文系统字体与常规表单布局。主任务为登录→选择客户端→配置，附实时余额/检测、移除和退出登录。窄屏自然单列，桌面账号与客户端两区，错误留在操作附近并可恢复。遵循 Impeccable Operate、现有 shadcn Button/Input/Alert，不引入营销图或远程内容。
- Rust 仅 `local_command(command)`，serde 枚举约束动作，拒绝未知字段；命令在 spawn_blocking 中调用 CLI 共享库。JS 不能传任意 home、路径、命令或 shell；秘密只通过内存 invoke，不进日志和浏览器持久存储。
- 同一已安装桌面可执行文件支持内部 `bridge` 参数，直接启动共享 MCP bridge，不初始化 GUI；配置中的 current_exe 始终可用，不依赖 PATH 或开发目录。
- 托盘菜单“显示 Loadout / 退出”，窗口关闭隐藏，退出动作结束应用。远程导航拒绝，本地 CSP 仅允许 IPC，外部窗口无权限。

## 计划与验收顺序

1. desktop/src/lib.rs 可编译空壳与真实临时目录/HTTP命令测试，验证登录、配置、状态、移除、注销；未知字段/客户端/动作拒绝。
2. 最小实现 execute，再运行无 GUI 的 Rust 测试 GREEN；native入口/托盘与实际桥接子进程测试。
3. desktop/ui 组件测试先 RED：登录、秘密清空、客户端选择与apply/remove、失败重试、注销、键盘与禁重复提交；Zod解析invoke响应。
4. 构建本地UI与Tauri；fmt/clippy/test、TypeScript/Vitest/Biome/生产构建；记录真实命令和平台限制，不将DOM测试等同视觉人工检查。

## 官方版本核验

crates.io：tauri 2.11.5、tauri-build 2.6.3。npm官方registry：@tauri-apps/api 2.11.1、@tauri-apps/cli 2.11.4。React/Vite/Query/Zod/测试库复用现有web精确版本。无额外Tauri shell/fs插件。

当前系统 pkg-config 实际检测 GTK 3.24.41、WebKitGTK 2.52.6；随后尝试真实原生编译与包构建。

## RED / GREEN / REFACTOR 证据

- `cargo test --manifest-path desktop/Cargo.toml --no-default-features --test commands` **RED**：login返回`not implemented`；额外路径字段的退出动作意外被serde unit variant接受。实现execute真实共享库调用，并将无参数动作改为严格空对象变体后，同命令 **GREEN**（2项）。测试覆盖真实verify请求、秘密不出返回、绝对desktop bridge路径、apply/离线客户端/实时状态/remove/logout、未知动作/客户端/路径字段拒绝。
- `cargo test --manifest-path desktop/Cargo.toml --test native_bridge` **RED**：可编译空main未提供stdio initialize响应。加入GUI之前的共享bridge分支后 **GREEN**。测试移除DISPLAY并使用临时HOME，SDK初始化成功、缺凭证返回isError、stdin关闭后正常退出，8秒总watchdog。
- `pnpm --dir desktop/ui test` **RED**：4个行为测试找不到登录字段和状态入口。实现React+Query+Zod+有限invoke后 **GREEN**（4项）：登录/秘密清空、选择apply/remove、doctor、退出、失败安全文案与Enter重试、请求中禁重复提交、畸形native响应拒绝。
- 复核新增“撤销后刷新不能继续展示旧余额”测试，`pnpm --dir desktop/ui test -- -t 'removes stale account'` **RED**（该脚本实际运行全suite，4通过1失败），失败仍显示旧账号；错误状态屏蔽旧数据后 `pnpm --dir desktop/ui test` **GREEN**（5项）。
- 在绿灯下格式化；clippy发现可折叠if，合并条件后重跑clippy与完整Rust测试通过。Biome发现图标源SVG缺少title，补充可访问名称后单独检查通过。上述样式/配置修正未改变业务断言。

## 实际完成验证

- `cargo fmt --manifest-path desktop/Cargo.toml --check`：通过。
- `cargo test --manifest-path desktop/Cargo.toml --locked`：通过，commands 2、native_bridge 1，无skip。
- `cargo clippy --manifest-path desktop/Cargo.toml --locked --all-targets -- -D warnings`：通过。
- `pnpm --dir desktop/ui test`：通过，5项组件行为测试。
- `pnpm --dir desktop/ui build`：TypeScript和Vite生产构建通过；JS 382.92 kB（gzip119.92 kB），CSS18.04 kB。
- `pnpm exec biome check desktop`：通过，15文件，无修正。
- `pnpm --dir desktop/ui tauri build --bundles deb`：**真实release编译和Debian安装包构建通过**；产物 `desktop/target/release/bundle/deb/Loadout_0.1.0_amd64.deb`。
- 在Python TemporaryDirectory中运行`dpkg-deb --extract`，令`LOADOUT_DESKTOP_TEST_BIN`指向提取出的`usr/bin/loadout-desktop`，再运行`cargo test --manifest-path desktop/Cargo.toml --locked --test native_bridge`：通过。未安装软件包、未修改系统目录或真实客户端配置。
- `git diff --check -- cli desktop pnpm-workspace.yaml pnpm-lock.yaml`：通过。

构建过程的命令修正：直接在desktop/ui调用Tauri找不到父层配置，已将UI的tauri脚本设为先进入desktop目录，当前文档命令实际通过；图标CLI的多个尺寸需要重复`--png`选项。两次工具配置错误属于构建准备，不计有效RED。

## 未验证范围

未使用浏览器自动化。组件测试验证标准表单、键盘Enter、错误恢复、pending与真实调用边界，不证明桌面/窄屏真实视觉；图形窗口、托盘显示/点击/关闭隐藏未做人工交互验收。Linux GTK/WebKit依赖可用且原生可编译，不等于GUI交互已验证。macOS/Windows、签名发行、商店打包未运行；这些平台需要对应图标/系统构建环境。未使用真实服务器凭证或真实用户配置。完整仓库 `make check` 仍由主任务集成执行。

已有PRODUCT.md仍有M0.0旧状态文本，由主任务统一更新；本面板颜色/字体沿用现有DESIGN，未修改共享设计系统。图标PNG来源为同目录icon.svg，经官方Tauri icon CLI转换；未使用外部位图或AI生成图片。

共享bridge跨语言错误重复前缀修复后，桌面Rust测试/clippy与Debian包亦重新构建，确保最终产物消费新共享库；详见cli-execution的集成回归记录。

## 桌面 Harness 集成

Makefile 新增 `build-desktop-ui`、`lint-desktop`、`test-desktop`、`build-desktop`。最后一次共享 CLI 修改后，使用 Node 26.8.2 执行 `make lint-desktop test-desktop build-desktop` **通过**：UI 5 项、Rust no-default-features 命令 2 项、native 命令 2 项及 bridge 1 项，fmt/clippy/TypeScript/Vite 均通过；`tauri build --bundles deb -- --locked` 真实生成新版 `Loadout_0.1.0_amd64.deb`。全仓库 check 串联和 CI 系统依赖由主任务整合，本记录不代替最终整仓库验证。

CLI 配置归属交叉审查修复后再次执行 `make lint-desktop test-desktop build-desktop` 通过；最新 deb 在临时目录解包并通过 `native_bridge`。未改变此前 GUI 人工交互未验证的限制。
