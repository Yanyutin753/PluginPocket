# 全流程实测：打包装 → CLI/市场双路径安装 → Codex 实际使用

目标：以运营方+最终用户双视角跑通"登录即武装"闭环（mock 数据），收集并修复过程中的真实缺陷。工作区起点 8b13d8b（此前在途的质量检查改动已先行单独提交）。

## 环境

- dev 服务 `make restart`（Air 在 rebrand 提交前启动，持有旧 `loadout-server` 构建配置导致 API 起不来；Air 不热载自身配置，重启即愈，非代码缺陷）。
- mock MCP 上游 `build/pluginpocket-demo -listen 127.0.0.1:8790`；demo 种子 `-origin http://127.0.0.1:8787`（一次性，账号 demo_owner 等）。
- 自打包三种市场条目（管理员 API）：`demo-mcp`（gateway，7 credits/次）、`demo-reviewer`（inline 技能，SKILL.md + 嵌套附件 references/checklist.md）、`demo-expert-kit`（bundle）。
- Codex CLI 0.153.4（npm 全局）；mock LLM 为本地 Responses API SSE 服务器（`wire_api="chat"` 已被 0.153 移除，必须用 responses 协议；经 `-c model_providers.*` 注入，不改用户 config）。

## 旅程验证（全部实机）

1. `pluginpocket login/doctor/status`：demo_owner 5890 credits，三客户端 detected。
2. `pluginpocket apply --clients codex` → config.toml 标记块正确。
3. `pluginpocket install demo-expert-kit` → `~/.codex/skills/`、`~/.claude/skills/` 写入含嵌套附件；`uninstall` 后重装回路完好。
4. `git ls-remote /marketplace.git` 正常；`codex plugin marketplace add` → `codex plugin list` 见三个自制包 → `codex plugin add demo-mcp@pluginpocket` 成功。
5. `codex exec`（mock 供应商）经 `mcp__pluginpocket`（apply 路径）调用 `demo__success`：返回 "DEMO: success"，余额 5890→5883（-7，精确）。
6. Bug#1 修复 + `codex plugin marketplace upgrade pluginpocket` + 插件重装后，`codex mcp list` 出现 `demo-mcp` 服务器；再经 `mcp__demo_mcp`（插件路径）调用：5883→5876（-7）。
7. 用量 API（会话 cookie）两条 `demo__success cost=7 ok` 记录；`/api/v1/plugins` 41 条含三个自制包（`/plugins` 为客户端渲染目录页）。

## Bug#1：导出插件声明 MCP 缺失（server/internal/marketplace/exporter.go）

根因：Codex 插件的 MCP 服务器必须声明在 `.codex-plugin/plugin.json` 的 `mcpServers` 字段（内联对象或 `./.mcp.json` 路径）；导出器只写根目录 `mcp.json`，Codex 静默忽略——插件的 bridge 服务器从未加载（`codex mcp list` 无 demo-mcp、工具命名空间缺失）。次要错误：http 端点类型写成 `"streamable-http"`，Codex 配置枚举为 `"http"`。

- RED：`cd server && go test ./internal/marketplace/ -run 'TestExport'`——三项断言失败（plugin.json mcpServers 为空 map[]）。
- GREEN：mcp/bundle 分支内联 `mcpServers` 进 plugin.json、停写根 mcp.json、http 类型改 `"http"` 后，`go test ./internal/marketplace/` 全部通过；首轮全量 `make check` 暴露需测试库环境的 `gitrepo_test.go` 克隆断言仍指向 `plugins/<slug>/mcp.json`，已随同契约改为 plugin.json 并以 `PLUGINPOCKET_TEST_DATABASE_URL=... go test -count=1 ./internal/marketplace/` 验证。
- 实机验收：升级市场快照重装插件后 `codex mcp list` 出现 `demo-mcp`，`codex exec` 经插件服务器调用真实扣费 7。

## Bug#2：apply/remove 被 codex 追加段污染后失效（cli/src/clients.rs）

根因：块位于 config.toml 末尾时，`codex plugin marketplace add`/`plugin add` 会把 `[marketplaces.*]`/`[plugins.*]` 段追加进托管块内部，块内容不再匹配管理记录，apply/remove 报 "unmanaged or modified"。install 路径已有 `heal_polluted_block`，apply 路径未用。

- RED：`cargo test --manifest-path cli/Cargo.toml --locked --test clients codex_marketplace_sections`——复现实测错误字符串失败。
- GREEN：`heal_polluted_block` 提升为 `pub(crate)`，clients.rs 的 codex 读取点（states 检测 + 写入）先治愈再比对/切除；同一命令 11/11 通过（含既有"用户手改不得覆盖""字符串内标记不得授权"保护）。
- 实机验收：对已污染的真实 config 重跑 `apply --clients codex` 成功，外部段移出托管块零丢失，二次 apply 幂等。

## 非产品问题的处置

- 用户 4 个 bigmodel HTTP MCP 服务器在 codex 内报 `missing-content-type`：上游对 `notifications/initialized` 返回无 Content-Type 的空 200，rmcp 严格校验判死；属第三方服务器与 codex 的兼容问题，产品侧不修。
- 用户 codex/claude/cursor 配置中的旧品牌（loadout）残留条目：产品明确不做旧名兼容（质量检查记录），已按用户授权手动清理（备份于 `build/../.loadout/*-before-cleanup.json`、`.loadout/codex-config-before-e2e.toml`），用户自有 MCP 服务器全部保留。
- `install` 对 gateway 成员自动确保 bridge 时写全部检测到的客户端：与 apply 默认行为一致（自动检测全部），属设计行为。

## 最终验证

`node build/quality-check.mjs check`（真实 `make check`，日志 `.loadout/e2e-make-check-3.log`）退出 0：发布预检、skills-check、Biome、TypeScript、Go fmt/lint、Rust fmt/Clippy、`go test -race` 全包（含真实测试库的 app 与 marketplace）、CLI 43 项、Web 全量、桌面 UI、生产构建与 `make test-e2e`（真实 Web/CLI/桌面 bridge 的 TestProductJourney 全过）。

## 未验证范围

- 桌面 Tauri GUI、Windows/macOS 安装包、真实 TUI 交互式 codex 会话（本次以 `codex exec` 非交互验证协议与计费链路）；移动端视觉。
- mock LLM 不代表真实模型工具选择行为，仅验证协议/路由/计费正确性。
