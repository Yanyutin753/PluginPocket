# 公共插件市场卡片与分页

范围：现有公共 React 目录；参考 Apify 的搜索、分类、卡片层级，保留 Loadout tokens。不增加依赖或后端接口。默认每页 12 项，先搜索/分类再分页；URL 页码、边界禁用、筛选复位、非法页码收敛；匿名详情与安装命令继续可用。

验收：PluginsPage 用户行为测试覆盖分页、键盘、URL、筛选复位与页码边界；既有测试覆盖匿名访问、详情和错误重试。最终 Web 回归/构建和 make check。按契约不使用浏览器自动化；桌面和移动视觉须独立人工验收。

## 执行证据

命令均从仓库根在 WSL 执行，使用固定 Node 26.8.2/pnpm。日志保存在 `.loadout/`。

| 阶段 | 命令与结果 |
|---|---|
| RED | `pnpm --dir web exec vitest run src/PluginsPage.test.tsx`：6 failed / 4 passed。旧页面仍展示第13项且没有分页，筛选不移除页码；`catalog-red.log`。此前一次 shell PATH 转义失败不计 RED。 |
| GREEN | 同命令：10 passed，包含首末页、键盘 Enter、URL 共享、筛选复位和4种异常页码；`catalog-green.log`。 |
| REFACTOR | 精简标题选择、删除冗余可选链，Biome 格式化；同一聚焦命令仍10 passed。 |
| Web 回归 | `pnpm --dir web test`：23 files / 141 tests passed；`catalog-web.log`。 |
| 构建 | `pnpm --dir web build` 最终通过；`catalog-build.log`。首次发现测试查询参数 `exact` 不属于 ByRoleOptions，删除后重新通过聚焦测试和构建。字符串 name 本身已精确匹配。 |
| 完整 harness | 加载 `.env` 后 `make check`：发布预检、数据库/Redis/skills 检查通过；最终停在工作区其他改动 `web/src/UsageDetails.test.tsx` 的 Biome 格式错误，未修改该文件。后续阶段未完成，不能声称全量通过；`catalog-check.log`。 |
| 静态设计扫描 | `impeccable detect --json web/src/features/PluginsPage.tsx web/src/features/PluginsPage.css`：仅字号规范提示，本轮目录字号层级已明确写入 DESIGN.md；`catalog-design.json`。context 误将 `/plugins` 当文件路径解析，实际上下文直接读取现有 PRODUCT.md、DESIGN.md、FRONTEND.md。 |
| 独立审查 | 只读 reviewer 检查分页/URL/焦点/样式级联，未发现阻塞问题。 |
| 开发入口 | `Invoke-WebRequest http://127.0.0.1:5173/plugins` 返回200；不作为视觉验证。 |

未验证：桌面/移动真实视觉、浏览器排版与溢出；按用户契约不运行浏览器自动化。当前分页为客户端处理完整公开目录，未改变 API 的全量传输方式。
