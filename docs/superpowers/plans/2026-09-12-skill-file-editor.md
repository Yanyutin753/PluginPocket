# 技能文件编辑体验

用户选择文件编辑器：左侧技能文件列表，右侧编辑选中文件。当前工作区完成，保留既有文件存储工作。

采用宽侧面板，默认 SKILL.md；附属文本按需读取并保留草稿，二进制显示文件信息而非解码编辑。支持新建文本文件和现有附件上传/删除，保留执行位。基本信息按需展开，新建时展开。已有 GitHub 技能默认编辑已发布快照，设置内显式选择重新同步；同步会替换发布内容，界面明确提示。手机文件列表在编辑区上方。

验收：切换文件保留修改；读取失败可重试且不写空内容；二进制不显示文本框；新增路径校验；保存真实 files_v2 并保留执行位、未修改附件。组件测试覆盖键盘与错误恢复；不使用浏览器自动化，视觉验收边界明示。

## 执行记录

### 追加：所有面板全屏

用户追加“全部”“全部修复”：全屏切换推广至所有SidePanel，移除技能专用开关，统一“全屏/退出全屏”文案，继续保留锁定与Escape还原顺序。

- RED：`pnpm --dir web exec vitest run src/SidePanel.test.tsx`，默认普通面板缺全屏按钮失败（`/tmp/loadout-all-fullscreen-red.log`）。
- GREEN：`pnpm --dir web exec vitest run src/SidePanel.test.tsx src/SkillFileEditor.test.tsx src/Marketplace.test.tsx`，3文件20项通过；`pnpm --dir web build`通过，修改文件Biome通过。日志 `/tmp/loadout-all-fullscreen-green.log`、`/tmp/loadout-all-fullscreen-build.log`。
- 已通知全量负责人补完整Web测试/build/Biome覆盖最终共享面板改动，等待最终结果。
- 最终验证完成：统一负责人运行 `node --env-file=.env .loadout/tool-editor-check.mjs`（固定PATH、内部执行 `make check`）退出0，日志 `.loadout/file-storage-check.log`；覆盖Go race、CLI、桌面测试/Linux deb、真实服务e2e及开发进程/HTTP集成。最新UI另补全仓Biome、完整Web测试、Web生产构建和 `git diff --check`，整个命令退出0；本任务实际读取 `.loadout/file-final-web.log` 确认133文件Biome通过、32文件220项测试通过、生产build通过。桌面/手机人工视觉与Windows原生仍未验收。

### 追加：全屏编辑

用户要求可全屏。标题栏切换全屏/还原，保留选中文件和草稿；Escape先退出全屏，再次Escape按既有行为关闭。仅技能启用，不使用浏览器Fullscreen API。

- RED：`pnpm --dir web exec vitest run src/SkillFileEditor.test.tsx -t fullscreen`，因不存在全屏入口失败（`/tmp/loadout-fullscreen-red.log`）。
- GREEN/回归：`pnpm --dir web exec vitest run src/SkillFileEditor.test.tsx src/SidePanel.test.tsx src/Marketplace.test.tsx`，实际匹配2文件19项通过（仓库无SidePanel.test.tsx）；覆盖键盘切换、还原按钮、草稿和Escape顺序。`pnpm --dir web build`通过。日志 `/tmp/loadout-fullscreen-green.log`、`/tmp/loadout-fullscreen-build.log`。
- 全量仍由文件存储任务统一运行，已通知取本次最新UI；未做桌面/手机人工视觉验收。

### 文件编辑器

- RED：`pnpm --dir web exec vitest run src/SkillFileEditor.test.tsx`，初始3项失败：GitHub模式未提供正文、缺少文件选择和新建入口。日志 `/tmp/loadout-skill-red.log`。
- 第二轮RED：同命令，清空SKILL.md后切换文件，保存按钮仍可用；1失败/4通过。日志 `/tmp/loadout-skill-red2.log`。
- GREEN：文件切换、GitHub已发布快照编辑、执行位、二进制/读取重试、新建路径和键盘焦点、主文件必填、切换来源保留上传删除全部通过。
- REFACTOR：附件状态由编辑器统一持有，附件读取/错误阻塞时禁止文件编辑，避免旧异步读取覆盖新草稿；未加入编辑器依赖。
- 回归：`pnpm --dir web exec vitest run src/SkillFileEditor.test.tsx src/SkillAttachments.test.tsx src/Marketplace.test.tsx src/Localization.test.tsx`：37项通过；`pnpm --dir web build`通过（TypeScript + Vite）。末次附件/文件编辑聚焦9项通过，包含上传前数量/大小预检。日志 `/tmp/loadout-skill-regression.log`、`/tmp/loadout-skill-final.log`。
- `make check`由同工作区通用文件存储任务统一运行，避免并发构建与数据库harness；结果待回传。
- 首轮统一 `make check`实际停在Go导入格式lint（pgxpool导入分组）；日志 `.loadout/file-storage-check.log`，由文件存储任务修复后重跑。
- 第二轮统一检查停在另一个管理员界面任务正在修改的 `Marketplace.test.tsx` 格式/非空断言lint；全量负责人等待该任务冻结再重跑。本任务交付时全量尚未完成，不宣称全量通过。
- 未运行浏览器自动化；桌面/手机实际视觉效果尚未人工验收。组件键盘和错误恢复测试不能替代视觉验收。
- 独立review发现SKILL.md自身执行位与新建路径大小写/目录冲突：先加行为测试，`pnpm --dir web exec vitest run src/SkillFileEditor.test.tsx` 实际2失败/4通过（`/tmp/loadout-skill-review-red.log`）；修复后上述4文件回归38项通过、生产构建再次通过（`/tmp/loadout-skill-reviewed.log`、`/tmp/loadout-skill-build.log`）。
