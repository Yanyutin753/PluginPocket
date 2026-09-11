# 工具图标组件执行记录

对应主计划：`2026-09-11-tool-editor.md`。本子任务仅新增 ToolIcon / ToolIconEditor、其样式、翻译字典与组件测试；主任务负责页面/API/翻译注册集成。

## TDD 证据

环境：WSL，Node `/home/yangyang/.nvm/versions/node/v26.8.2/bin`；使用显式 PATH 加 `/usr/local/bin:/usr/bin:/bin`。默认 WSL PATH 无 pnpm 的尝试是环境失败，不计入 RED。

- RED（2026-09-11 20:58）：`cd web && pnpm exec vitest run src/ToolIcon.test.tsx`，可编译空组件 stub 下 16 个测试失败，原因是缺少用户可访问的编辑/预览控件。日志 `/tmp/tool-icon-red.log`。
- GREEN（20:59）：同一命令，16 个测试通过。
- REFACTOR：`pnpm exec biome check --write web/src/features/operations/ToolIcon.tsx web/src/features/operations/ToolIcon.css web/src/i18n/tool-icons.ts web/src/ToolIcon.test.tsx`，仅格式化；随后同一聚焦测试 16/16 通过，四文件 Biome 无修改。
- 类型：`pnpm --dir web typecheck` 退出 0。

## 验收映射

- HTTPS 地址预览、键盘 Enter 移除；无凭证/2048 字符上限。
- SVG 粘贴按 UTF-8 base64 编码，仅通过 img 呈现；拒绝脚本、事件、foreignObject、外部 href、不完整 XML。
- 原生文件上传、SVG 预览；错误类型、超过 64 KiB 与危险 SVG 保留原图，并可重选恢复。
- 失效图片显示 Wrench，替换 URL 后重新尝试；保存禁用状态覆盖所有输入操作。
- 错误显示 role=alert、aria-invalid，并用原生 customValidity 阻止父表单误保存。
- 样式复用 Field/Input/Button 和语义颜色；40px 等比预览，上传/操作区域允许窄屏换行。

未执行浏览器自动化（遵循用户约束）；桌面/手机视觉人工验收未执行。完整 make check 与集成回归由主任务统一执行，不以组件通过替代全量验收。

## 上传审查修复

- 根因：FileReader 开始读取时没有设置表单 invalid，因此父表单可保存旧图标；移除按钮仅参考 value/draft，没有考虑空图标的错误状态。
- RED（21:03）：同一聚焦命令，新增真实 FileReader 读取期间原生有效性、空图标错误移除恢复测试，2 failed / 16 passed；日志 `/tmp/tool-icon-upload-red.log`。
- GREEN（21:04）：开始读取时设置 customValidity 与状态提示，完成/失败或输入/移除时替换读取状态；移除按钮允许清理 error/reading。18/18 通过。
- REFACTOR：仅格式化所属文件，然后聚焦 18/18 与 web typecheck 均通过。上传大小契约仍为 64 KiB。
- 新增 `web/src/i18n/tool-editor.ts`：覆盖 ToolEditor、SettlementEditor 的标签、说明、校验错误以及工具列表连接/结算文案；主任务负责注册。

## 上传优先与地址栏精简

- RED（21:09）：新增键盘首次聚焦上传、上传后显示文件名而非 base64、重开已有 data 图片保留预览且地址栏空白测试。同一聚焦命令 3 failed / 18 passed；日志 `/tmp/tool-icon-upload-first-red.log`。
- GREEN：上传/预览/移除行移至地址栏前；已存 data 图片初始化空 draft，成功上传清空 draft 并显示文件名，重开显示本地化“已上传图片”；SVG 原文粘贴保持可编辑。新行为断言全部通过。
- 同时检测到外部协作修改同一测试文件，新增工坊图片默认/失败回退断言；初次合跑 2 failed / 20 passed，失败为 `falls back when an image fails and retries a changed icon` 和 `shows the workshop illustration when no custom icon is configured`，未改动这些外部断言。外部实现随后落地，21:10 最新完整 `pnpm exec vitest run src/ToolIcon.test.tsx` 已 22/22 通过，日志 `/tmp/tool-icon-final.log`。
- 本轮所属文件 Biome 已格式化；最新全 Web typecheck 在并发修改的 `src/App.tsx:141` 报 useQuery 泛型不兼容，已通知主任务，未据此声称聚合检查通过。
- 64 KiB 大小约束不变；本轮未进行浏览器自动化或视觉人工验收。
