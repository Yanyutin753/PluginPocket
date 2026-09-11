# 全站 UI 与工程质量检查

目标：沿用现有 PluginPocket 视觉和业务契约，检查 Web、桌面、服务端、CLI 与文档，修复本轮有证据的问题。当前工作区起点 e1e75b0，无已有未提交修改。

## 执行与验收

- [x] 通用 SidePanel：关闭全屏面板后重新打开恢复普通尺寸，键盘焦点回到入口，锁定期间不误关。
- [x] 桌面客户端：刷新中或客户端读取失败禁止写入配置，恢复后允许原选择继续操作；清除过期成功提示。
- [x] 界面精修：窄屏输入字号、市场筛选的中英文空间、面板标题与滚动区域；保留主题和 reduced-motion。实际视觉验收未执行。
- [x] 文档核对：修正过时定位、冲突布局和验收说明，区分历史证据与当前检查。
- [x] 运行聚焦 RED/GREEN、相关回归、静态检查、构建和 make check，记录受限项。

行为改动先失败测试再实现。不更换依赖、不推送、不修改真实客户端配置、不使用浏览器自动化；DOM 测试不证明桌面/手机视觉通过。

## 基线

- `pnpm exec biome check --error-on-warnings .`：通过。
- `pnpm --dir web typecheck`：通过。
- `node --env-file-if-exists=.env --run check`：发布预检通过；缺少 `PLUGINPOCKET_TEST_DATABASE_URL`，在 database-check 停止，后续未执行。
- WSL 默认 shell 未加载项目 Node/pnpm/Go 路径；后续命令显式使用已安装的固定工具链。

## 顺序证据

记录本轮实际执行结果，不引用旧检查作为通过证据。

- SidePanel RED：`pnpm --dir web exec vitest run src/components/SidePanel.test.tsx`，1失败/2通过；重新打开仍为全屏，找不到普通「全屏」按钮。GREEN：相同文件加 `src/SidePanel.test.tsx`，4项通过；覆盖关闭锁与 Escape。
- 桌面刷新 RED：`pnpm --dir desktop/ui exec vitest run src/App.test.tsx`，1失败/7通过；刷新请求未完成时配置按钮仍 enabled。GREEN：同文件8项通过，覆盖失败禁用、恢复与保留选择。
- 桌面反馈 RED：`pnpm --dir desktop/ui exec vitest run src/App.test.tsx -t replaces`，旧「服务可达」提示在失败后仍存在。GREEN：`pnpm --dir desktop/ui test`，全部9项通过；四个 mutation 共用清除反馈入口。
- REFACTOR/相关回归：`pnpm --dir web test`，33文件231项通过；`pnpm --dir desktop/ui test`，9项通过。`pnpm --dir web build` 和 `pnpm --dir desktop/ui build` 均通过，包括 TypeScript。
- CLI 独立审查发现并修复嵌套技能卸载及附件清单变化更新错误，交叉审查保留外来文件/目录/链接保护；证据见 `2026-09-12-cli-skill-lifecycle.md`。
- 本轮 UI diff 已经独立代码复核；未发现额外可操作正确性问题。组件测试不证明排版、移动输入体验或真实 Tauri GUI。
- 本地 `.env` 使用旧品牌变量名。首先仅将两个专用测试连接临时映射给检查子进程；确认日常启动也受影响后，将 `.env` 行首 `LOADOUT_` 替换为 `PLUGINPOCKET_`，原值保持不变。原文件以0600权限备份到忽略目录 `build/env-before-quality-review.backup`，未纳入版本控制。未重启日常服务；产品不增加旧变量兼容路径。
- 单独 `make test-server` 与全量检查短时重叠；单独运行的 app 测试在约145秒时为诊断被中断，栈位于新用例初始化迁移，不能算通过或据此认定死锁。最终结果以随后不中断的 `make check` 为准。
- README中英文、PRODUCT、DESIGN、PLAN、HARNESS、FRONTEND和desktop README的本地Markdown链接检查未发现不存在的目标（不验证外部URL和标题锚点）。

## 最终验证

`node build/quality-check.mjs check` 调用真实 `make check`，最终退出0。显式使用本机已安装的 Node 26.8.2、pnpm 12.3.4、Go/Rust 仓库声明工具链，专用 PostgreSQL 与 Redis 连接来自本地配置。

- 发布预检、skills-check、Biome、TypeScript、Go格式/lint/tidy、Rust格式/Clippy均通过。
- `go test -race -count=1 ./...` 全包通过；app集成154.898秒，包含真实数据库，不以跳过集成冒充通过；store同时执行SQLite轨道。
- CLI全部43项、Web全部231项、桌面UI全部9项通过；桌面Rust受限命令和无GUI原生bridge通过。
- Web/Go/Rust生产构建、Linux Tauri deb打包通过。
- `make test-e2e` 真实产品旅程通过（86.241秒），包括生产/开发Web、真实CLI与桌面bridge、跨副本故障切换、Redis故障降级、并发不超扣、退款与崩溃恢复。
- `make test-process` 4项、`make test-dev` 7项、最终HTTP/CLI集成4项通过。
- 最终 `git diff --check` 通过，无提交、推送或发布。

未验证：本轮未进行浏览器自动化、真实桌面/手机视觉和GUI/托盘交互；Windows/macOS安装与签名未运行；未做新的吞吐压测。沿用品牌并修复本轮发现的问题，不承诺项目不存在尚未发现的缺陷。
