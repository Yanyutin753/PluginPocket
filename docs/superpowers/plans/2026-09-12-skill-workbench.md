# 技能工作区优化实施计划

用户明确要求以截图中的技能文件区域为目标，美化、支持全屏和文件预览，交互接近VS Code。沿用产品主题与现有API，不改后端，不提交/推送，当前工作区保留其他修改。

## 设计与边界

- 紧凑资源管理器：文件夹层级、类型图标、新建/上传就近入口；点击文件形成顶部标签，关闭标签不删除文件或草稿。
- CodeMirror 6编辑：行号、语法高亮、撤销、查找、自动换行、只读状态，Markdown/JavaScript/TypeScript/JSON按扩展名选语法。其他文本照常编辑。按官方npm源锁定稳定版本。
- Markdown编辑/预览/分栏，预览直接反映草稿；使用react-markdown与GFM，禁原始HTML，图片限制为技能包内文件，不执行脚本。常见图片直接预览，二进制显示大小与下载。
- 工作区工具栏提供全屏/还原，与外层SidePanel共享状态；全屏时编辑区利用剩余高度，保留保存按钮，Escape还原。桌面左右分栏，手机预览上下排、资源管理器可折叠。
- 展示文件类型、字节数和未保存状态；错误可重试，执行位、路径和容量边界继续有效。

## 顺序

1. 先写预览安全、图片、标签草稿、目录、新建与工作区全屏行为测试，运行确认RED。
2. 锁定依赖、接CodeMirror组件与预览组件；原编辑/附件行为全部继续验证，升级相关测试为真实contenteditable用户操作，不mock编辑器。
3. 实现目录/标签/工具栏/共享全屏，重做CSS，分离文件预览与编辑器职责；同步PLAN/DESIGN/FRONTEND。
4. 聚焦测试GREEN，相关回归、独立review与修复、生产构建和make check。记录真实未验证范围，不把DOM当视觉证据。

## 文件

- SkillFileEditor.tsx/css：工作区与文件会话。
- SkillCodeEditor.tsx：CodeMirror React生命周期与语言、只读配置。
- SkillFilePreview.tsx：安全Markdown/图片/二进制预览。
- SidePanel.tsx：共享全屏上下文与编辑工作区布局。
- SkillFileEditor.test.tsx、SidePanel.test.tsx、Marketplace.test.tsx：新行为与旧编辑回归。

## 验证记录

### RED → GREEN

命令均在 `web` 下执行，Node 26.8.2，真实CodeMirror，不mock编辑器。

- `pnpm exec vitest run src/SkillWorkbench.test.tsx`：初始3项RED，尚无预览、标签与工作区全屏；接入后GREEN。
- 同一命令追加缺陷测试：已发布文件覆盖当前草稿下载、已删除图片仍参与预览、frontmatter混入正文3项RED；优先草稿、过滤已删文件与折叠元数据后GREEN。
- `pnpm exec vitest run src/SkillWorkbench.test.tsx -t 'undo'`：图片预览切回文本后撤销丢失RED；父级按文件缓存EditorState、重建时重新配置回调与扩展后GREEN。
- `pnpm exec vitest run src/SkillWorkbench.test.tsx -t 'Escape'`：查找框Escape先触发外层面板RED；局部Escape处理优先关闭查找/补全后GREEN，覆盖普通与全屏。
- `pnpm exec vitest run src/SkillWorkbench.test.tsx -t 'accessible embedded image'`：图片失败提示在Markdown段落内生成嵌套p，React报错RED；改为行内可访问状态后GREEN，并验证替换图片恢复。
- 最终聚焦回归：`pnpm exec vitest run src/SkillWorkbench.test.tsx src/SkillFileEditor.test.tsx src/Marketplace.test.tsx src/SidePanel.test.tsx src/SkillAttachments.test.tsx`，退出0，5文件33测试通过。覆盖新建/上传/删除、保存边界、草稿/下载、目录/标签、预览安全、键盘与错误恢复。

### REFACTOR 与审查

- 将目录、CodeMirror和安全预览分为独立组件，编辑器/预览器/语言包按需加载；生产构建通过，无大块警告。语言加载失败仍能编辑，可重试。
- 官方CodeMirror示例与react-markdown仓库用于核对接入方式；依赖按npm官方稳定版本精确锁定，见package.json与pnpm-lock.yaml。
- JSDOM缺少Range几何接口，测试补最小接口以运行CodeMirror；文本输入使用真实键盘选中和剪贴板粘贴，避免user.type模拟原生contenteditable不完整造成的字符丢失。
- 独立代码审查发现撤销与Escape两项，均按上述RED/GREEN修复；第二次静态复核通过，无新增阻断。
- 全量 `make check` 退出0（通过 `node --env-file=.env .loadout/tool-editor-check.mjs` 加载已有本地测试环境）。日志：`.loadout/skill-workbench-check.log`。包含Biome 139文件、类型检查、Go race与真实PG/Redis、CLI、Web 33文件230测试、桌面7测试与Linux deb构建、生产/开发产品链路、进程与开发启停以及最终HTTP集成4测试。首次启动因固定PATH缺少本机Go路径失败；加入 `/home/yangyang/.local/go/bin` 后重新执行，未修改产品代码规避检查。
- 最终 `git diff --check` 退出0。共享工作区含其他任务的存储与后台修改；本次全量检查针对当前整合状态，不将其归为本次UI新增功能。

### 未验证范围

按项目约定未使用浏览器自动化。桌面与移动端实际视觉、浏览器原生图片解码及手动操作尚未人工验收；组件测试和构建不替代视觉证据。未提交、推送或发布。
