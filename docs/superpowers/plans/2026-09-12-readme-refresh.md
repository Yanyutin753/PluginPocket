# README 中英文门面更新 · 2026-09-12

## 范围与设计

用户要求更美观的开源 README，插图与前端一致，默认中文并提供英文。README.md 为默认入口，README.en.md 提供对应英文内容与双向语言切换。复用首页实际使用的 workshop-welcome.webp，以及市场与连接场景的透明 WebP；不修改图片或前端功能，不增加依赖。徽章与 Mermaid 架构图沿用黄色、薄荷绿及中性底色。

内容按产品价值、市场类型、客户端接入、架构、自部署、状态和贡献组织。详细运行规则指向现有部署与 Harness 文档；保留 README.md#快速开始 锚点供贡献指南引用。英文版明确深层工程文档目前为中文。用户进一步允许调整名称；候选 MCPocket / Toolnest 已询问，未确定前保留现有品牌与命令。

## 验证与验收映射

- 本次只改文档，没有新增可执行行为；RED / GREEN / REFACTOR 行为测试不适用，不编造 RED 证据。
- Python 标准库检查两版 README：每版 29 个本地引用均存在，代码围栏和 details 成对，各引用 3 张原有产品插图；语言切换为双向。结果通过。
- 人工查看 welcome、marketplace、connect 原始图片，并核对 LandingPage.tsx 的实际首页素材与 DESIGN.md 配色。没有使用浏览器自动化，没有宣称 GitHub 桌面/手机渲染已验收。
- `make check`：发布预检 4 项通过；database-check 因未设置 LOADOUT_TEST_DATABASE_URL 退出，后续完整测试和构建未运行。不修改当前真实数据库或导出已有私密配置来绕过前置条件。
- `git diff --check`：初次发现 PowerShell 写入末行 CRLF；统一两版 UTF-8 / LF 后复验。
- 当前工作区已有 SQLite 相关修改；本任务未修改其实现、计划或 ADR。

## 后续维护

变更产品介绍、安装命令或状态时同步两个 README。插图直接引用 web/public/images 中现有文件，移动素材时同步链接。品牌命名需要同时明确展示名与 CLI 兼容范围。

后续用户确定品牌为 PluginPocket（插件口袋），要求全量更名且不保留兼容。最终实现与验证见 [更名执行记录](2026-09-12-pluginpocket-brand.md)。
