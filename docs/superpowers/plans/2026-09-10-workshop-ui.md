# AI 装备工坊实施记录

用户选择第一张实际视觉稿 `exec-80fb9574-9c14-4a44-83f4-13203ab72d18.png`。参考 awesome-design-md 的 PostHog 工程手册风格，使用原创薄荷绿工具箱插画、奶油底色、黄色动作、手写注释和终端细节，替换旧视觉。

## 范围与授权

- 概览突出创建网关令牌入口、真实有序接入步骤、可复制命令及真实账号数据。
- 保留 API、权限、语言/主题设置、错误恢复和全部路由；统一业务表单与运营页面。
- 用户随后明确授权浏览器自动化验证，以及保留原画的本地脚本抠图。当前任务使用这些授权；未修改 AGENTS.md。
- 内置 ImageGen 的两次透明编辑输出均为 RGB 棋盘格，并非 alpha，未作为生产素材。改从原始插画进行本地边界连通抠图与边缘去底色，保留眼睛、高光和铅笔阴影。
- 五张生产素材均为 RGBA WebP。Hero 以 alpha > 8 的画迹边界重构约 3% 最小留白，保持 1200×900，未裁掉插画。字体、资产实测清单见 DESIGN.md。
- 大屏主内容最大 1360px；客户端名称、长文本与各断点适配随最终代码验证。

## RED / GREEN / REFACTOR

| 验收行为 | RED 证据 | GREEN / 重构证据 |
|---|---|---|
| 概览主要入口、键盘 Enter、语义有序的两步接入 | TemplateUI.test.tsx 新增行为测试先运行：缺少目标 h1，1 failed / 3 passed；失败来自待实现页面行为 | 最小实现后同一聚焦测试 4 passed；随后重构共享视觉 CSS |
| 额度审计筛选在浏览器采用预期响应式网格 | 真实浏览器计算样式断言：expected grid, got flex | 修复 grid className 后同一断言 GREEN |
| 相关 Web 行为回归 | 以上行为已有有效 RED | Web 全量 7 files / 79 tests passed |

聚焦 RED/GREEN 使用同一命令：`pnpm --dir web test src/TemplateUI.test.tsx`。额度审计断言通过 `mcp__cua_repl` 的 `page.playwright.evaluate` 读取 `[data-slot="field-group"]` 的 `display`，结果不等于 `grid` 时抛错（`RED expected grid...got flex`）；修复后再次读取为 `grid`。这是工具调用证据，没有另造 shell 日志。测试数量不代替先后顺序，lint 不证明测试先于实现。

## 视觉与浏览器证据

- 18 routes × 2 themes × 3 widths（390 / 1440 / 2560）= 108 张主截图；另有 20 张手机底部截图。
- 截图目录：`C:\Users\yangyang\.codex\visualizations\2026\09\10\01a08bfd-0b27-7442-828c-49b57a546d35`。
- 实际浏览器检查所有覆盖页面无页面级水平越界、无破图；表格和长命令的局部横向滚动属于预期。
- 浅深两主题分别人工审阅桌面和手机截图；当前审阅未发现 P1/P2 重叠、裁切、对比或素材问题。概览与选定稿对照后补充手写注释、终端三色圆点，并放大插画画迹。
- 本记录只覆盖本轮截图状态和已有行为用例，不将截图声明为所有网络、权限、错误组合已穷尽。

## 最终验证

- 2026-09-11，载入本地 `.env` 并设置 Node / Go / Rust PATH 后运行 `make check`，最终退出码 0；完整日志 `.loadout/workshop-client-final-check.log`。包含格式/类型、Web 79 项、客户端 6 项、构建、Go/Rust、真实服务/CLI/bridge/浏览器集成检查。
- 更早一轮遇到 Go 服务关闭的 5 秒超时，恢复用例独立复跑可间歇复现；本轮未修改后端，之后两次完整检查成功。保留失败日志 `.loadout/workshop-check.log`，不抹去失败历史。
- `pnpm exec biome check web/src desktop/ui/src`、`git diff --check` 均退出 0；`impeccable detect --json` 返回 `[]`。
- 浏览器额外实测：手机导航 Enter 展开、工具参数 Enter 展开、取消添加工具、接入命令复制成功反馈；未提交管理数据。
- 客户端复用 WebP 和字体，浅/深/系统主题可键盘切换；1440/2560/390 CSS 像素逐一截图，实际窄窗口截图为 380×822。三尺寸双主题复核无 P1/P2，另查看底部。客户端 RED/GREEN 见 desktop/README.md。
- 浏览器仅验证客户端 Web 界面；无 Tauri 原生桥时显示真实错误。登录、配置等状态由组件和原生集成测试覆盖，不声称已在真实用户凭证环境下手动验证或制作平台安装包。未穷尽所有团队详情/权限/网络组合。
- 本轮视觉记录与逐页截图入口见根目录 design-qa.md。
