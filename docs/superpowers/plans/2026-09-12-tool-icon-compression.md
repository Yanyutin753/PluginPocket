# 工具图标自动压缩

用户允许 S3 或压缩后数据库存储，由实现选择。沿用共享数据库，避免为小图标增加对象存储配置；不新增依赖或修改已有图片。

- PNG/JPEG/WebP 原文件最多 5 MiB；原生解码后限制 2000 万像素，最长 256 px，编码 WebP（不支持则回退 PNG），逐步缩小至 128/64 px，目标 16 KiB、硬上限 64 KiB。
- SVG 保持静态安全校验和 64 KiB 上限；HTTPS 地址保持原有行为。
- 处理期间阻止保存；失败保留旧图标；移除/新选择不被过期结果覆盖；显示实际保存大小。
- 验证组件行为、错误恢复和相关回归，最终运行 make check 并报告环境限制；不做浏览器自动化。

## TDD

RED：`pnpm --dir web exec vitest run src/ToolIcon.test.tsx`，大图上传仍被64KiB旧限制拒绝，损坏位图未经过解码导致缺少处理失败反馈。日志 `.loadout/tool-compression-red.log`。

GREEN：同一组件及 ToolEditor/Localization 回归53项通过（图标27项），日志 `.loadout/tool-compression-green.log`。额外覆盖待处理移除不被旧结果覆盖、体积过大继续缩小、PNG编码回退、损坏图片失败后SVG恢复；原生解码/Canvas用测试替身覆盖接口行为，未声称实际浏览器编码或视觉已经人工验收。

第二个RED：新增损坏的历史data地址移除测试，保存体积显示因缺少逗号崩溃。日志 `.loadout/tool-compression-malformed-red.log`；仅为带数据部分的地址显示体积后，同一测试通过并纳入上述53项。

REFACTOR：复用原有图片校验、预览、错误状态和revision取消机制；只在同一文件增加原生解码/Canvas压缩，无新增依赖、接口或数据库迁移。压缩失败不修改旧图标，bitmap在finally释放。只读审查未发现实质问题。

`pnpm --dir web build` 类型检查与生产构建通过，日志 `.loadout/tool-compression-build.log`。`node --env-file=.env .loadout/tool-editor-check.mjs` 实际运行make check，当前在其他任务翻译文件的4项格式错误处失败，日志 `.loadout/tool-compression-check.log`；保留无关修改，不报告全量通过。原临时PG环境缺失的背景见上一轮执行记录，未修改业务数据库或.env。

后续统一验证：2026-09-12 文件存储任务的完整 make check 已退出0（.loadout/file-storage-check.log），最新UI完整测试/构建/Biome也退出0（.loadout/file-final-web.log），覆盖本图标改动。详见同日file-storage执行记录。
