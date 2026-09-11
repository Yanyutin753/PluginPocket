# 工具列表图标与排版

用户要求参考其他页面生成好看的图标并美化截图中的列表。沿用工坊风格，默认/失效图片显示原创工具箱，名称与状态同行，成本和操作分列，窄屏换行。保留自定义图标优先、参数抽屉、编辑和启停。

## 验证

- RED：`cd web && pnpm exec vitest run src/ToolIcon.test.tsx`，日志 `/tmp/tool-list-red.log`。默认插图与失效回退两条断言因仍显示 Wrench 失败；同轮另有正在改动的上传编辑器三条失败，不作为本任务RED证据。
- GREEN：同命令加 `-t "falls back|workshop illustration"`，2项通过。
- REFACTOR：移除无调用的旧列表样式，Biome格式化；`pnpm --dir web exec vitest run src/ToolIcon.test.tsx src/ToolEditor.test.tsx src/PanelFlows.test.tsx`，39项通过，覆盖自定义图片恢复、上传错误、编辑保存、抽屉键盘焦点与Escape。
- `pnpm --dir web typecheck` 通过；`pnpm --dir web build` 通过。
- `make check` 已执行，发行预检4项通过，随后 database-check 因缺少 `LOADOUT_TEST_DATABASE_URL` 退出，完整检查未通过。
- 按仓库约束没有浏览器自动化；桌面/手机浅深主题实际视觉未验收，不以组件测试替代。

## 素材

内置ImageGen参考 `workshop-mark.webp` 生成透明PNG，原件 `exec-6e2ffcde-77e4-4bf2-8978-19d6e8f546f3.png`；应用文件 `web/public/images/workshop-tool-icon.png`。保留原件；仅缩小部署尺寸，不改变画面内容。未改真实数据库图标，未发布。
