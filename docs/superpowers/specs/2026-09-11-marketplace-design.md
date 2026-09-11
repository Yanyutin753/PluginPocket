# 插件市场与工具元数据覆盖 — 设计（2026-09-11）

## 目标

1. 服务端提供**插件市场**：浏览目录（服务端精选 + GitHub 热门 MCP 同步）→ 一键安装进预设工具池 → 网关目录立即可用 → 客户端经既有 bridge/apply 直接在 Codex 等客户端使用。
2. HTTP/stdio 上游工具支持**自定义描述与参数描述覆盖**（描述、InputSchema 整体替换），目录展示与下发都以覆盖后为准。

## 核心决策（用户委托设计，已按产品边界选定）

- **插件 = MCP 上游**（http/stdio）。市场是"可浏览的候选目录"，安装 = 在 `tools` 表创建一行（密封配置），与现有网关目录、计量、后台完全复用；**不做**每用户自带 key 的安装（违反"只托管自有预设池凭证"边界）。
- **同步源 = GitHub Search API**（`topic:modelcontextprotocol` 按星排序，可选 `LOADOUT_GITHUB_TOKEN` 提额）。否掉：解析 awesome 列表（脆弱）、第三方 hub API（依赖/条款不明）。同步失败（离线/限流）返回 502，目录保持原样。
- **同步项 transport=unknown**：GitHub 元数据无法可靠判断传输方式与端点，安装时管理员必须显式提供 transport + config；精选（curated）项带已知 endpoint，可默认直装。**不放宽 stdio allowlist**，`ValidateConfig` 沿用 SSRF/allowlist 校验，凭证经 `SealConfig` 密封。
- **客户端不改**：安装发生在服务端池，`loadout apply`/bridge 已把整套池交给 Codex——"直接安装到 Codex"由既有机制完成。

## 数据模型

```sql
-- migration 017
CREATE TABLE marketplace_items (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  slug text NOT NULL UNIQUE,                -- owner-name（GitHub 仓库名 sanitize）
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  source text NOT NULL CHECK(source IN ('curated','github')),
  repo_url text NOT NULL DEFAULT '',
  homepage text NOT NULL DEFAULT '',
  transport text NOT NULL CHECK(transport IN ('http','stdio','unknown')),
  endpoint text NOT NULL DEFAULT '',        -- curated http 默认端点
  package text NOT NULL DEFAULT '',         -- stdio 包名（提示用）
  stars int NOT NULL DEFAULT 0,
  synced_at timestamptz,
  installed_tool_id bigint REFERENCES tools(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
-- curated 种子：deepwiki、context7（公开 http，可直装）；puppeteer、filesystem、memory、everything（官方参考 stdio，需受控环境）

-- migration 018
CREATE TABLE tool_metadata_overrides (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tool_id bigint NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
  remote_name text NOT NULL,                -- 上游原始工具名（未加前缀）
  description text NOT NULL DEFAULT '',
  input_schema jsonb,                       -- NULL=保留上游 schema
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tool_id, remote_name)
);
```

## API（均挂现有 admin 会话鉴权）

| 方法 | 路径 | 行为 |
|---|---|---|
| GET | `/api/v1/admin/marketplace` | 目录列表（curated 在前，github 按星序） |
| POST | `/api/v1/admin/marketplace/sync` | 同步 GitHub；成功 `{synced:N}`，失败 502 `upstream_unavailable` |
| POST | `/api/v1/admin/marketplace/install` | `{slug,key?,name?,units_per_call?,transport?,config}` → 建 tools 行（key 默认 slug，须匹配 `^[a-zA-Z0-9_-]{3,32}$`）+ 回链 installed_tool_id + `Gateway.Invalidate()`；已装 409 `already_installed` |
| POST | `/api/v1/admin/marketplace/uninstall` | `{slug}` → 停用 tools 行并解除回链 |
| GET | `/api/v1/admin/tools/{id}/upstream` | 经网关实时发现该上游工具（含覆盖态） |
| PUT/DELETE | `/api/v1/admin/tools/{id}/metadata[/name]` | 覆盖 upsert / 删除，变更后 `Invalidate()` |

## 网关集成

- `Gateway.UpstreamTools(ctx, toolID)`：导出给 app 层做实时发现（复用 remoteTools 与 Redis 缓存）。
- 目录构建（`tools()` 内 upstream 分支）：remoteTools 返回后按 `(tool_id, remote_name)` 查覆盖表，替换 Description 与 InputSchema（schema 须过 `ValidateToolSchema`，无效则忽略该覆盖并在目录构建日志可见错误）。覆盖在缓存之后应用 → 改覆盖只需失效目录（既有 revision 机制），不触碰上游缓存。

## Web 控制台（ToolsPage 扩展，仅管理员）

- 「插件市场」区块：条目卡（名称/描述/来源/星标/传输/已装态）、「同步 GitHub」按钮、安装表单（transport 决定字段：url+headers 或 command+args+env + 额度）、卸载。
- 工具条目展开区：http/stdio 工具显示「上游工具与描述」编辑器（实时发现 + 每工具描述/参数 schema 覆盖保存/清除）。
- 全部文案进 i18n（中/英）；键盘可达、错误态与加载态按现有模式。

## 测试策略（TDD）

- 同步：httptest 伪造 GitHub API（含 token 头断言、限流 403 路径、离线失败目录不变）。
- 安装/卸载/覆盖 API：app 层真实 DB 测试（隔离 schema）；安装产物可被网关目录发现。
- 网关覆盖应用：真实 fixture 上游 + 覆盖行 → 目录描述/schema 替换、调用仍路由原上游名。
- Web：组件测试（市场渲染/安装提交/覆盖编辑表单校验）。
- E2E（真实验收）：真实 GitHub 同步 + 安装 deepwiki（公开 MCP）→ 隔离 HOME bridge `tools/list` 出现 `deepwiki__*` → 覆盖其工具描述后目录更新。

## 范围外（明确不做）

CLI 改动；普通用户自助安装/带私钥；GitHub 项自动探测端点；市场评分/评论；支付分成。

## 修订（2026-09-11 晚）：http-only 市场 + CLI 本地直连

用户追加决策：**服务端一般只走 HTTP MCP；插件可在本地客户端安装，管理体验借鉴 cc-switch**。据此收窄：

- 市场安装接口拒绝非 http 传输（`transport_not_supported`）；stdio 仍属部署者受控能力，不进入市场（迁移 019 移除 stdio 精选、公开列表过滤 stdio）。
- 新增 `GET /api/v1/marketplace`（用户令牌可访问的公开目录）。
- 新增 CLI 本地插件管理：`loadout market` / `loadout install <slug> [--url]` / `loadout uninstall <slug>`——公共 HTTP 端点直连写入客户端配置（`loadout-<slug>` 托管条目、独立标记与备份、零凭证），与 bridge（服务端池计量通道）并存，构成"池内计量 / 本地直连"双通道。
- Codex 等客户端的版本/供应商切换管理不属于 Loadout 产品边界（PLAN 定位为 MCP 订阅网关），`loadout doctor` 维持客户端检测职责。

## 修订 2（2026-09-11 晚）：skills 与装备组 —— 从网关到生态

痛点：普通用户看到专家的 Codex/Claude 配置（MCP 服务 + skills + 提示词）想整套复刻，没有可信的一键途径。
产品回答：市场条目从单一 MCP 扩展为三种 kind，"Loadout"即"出战配置"的字面落地：

- **mcp**：HTTP MCP 插件（服务端池计量安装，或 CLI 本地直连安装），即前述能力。
- **skill**：Agent Skill（SKILL.md 及配套文件，写入 `~/.codex/skills/<slug>/`、`~/.claude/skills/<slug>/`）。
  - 来源一：inline —— 管理员在目录里直接维护文件内容（DB spec.files）；
  - 来源二：github —— (repo, path)，由**服务端**经 GitHub contents API 解析为文件集（复用可选 `LOADOUT_GITHUB_TOKEN`），CLI 永远只连 Loadout 服务（`GET /api/v1/marketplace/{slug}/files`），不吃 GitHub 限流。
- **bundle（装备组）**：一组 mcp/skill 引用；`loadout install <bundle-slug>` 递归安装全部成员——普通用户一条命令复刻专家效果。bundle 不允许嵌套 bundle。

边界与安全：技能文件仅允许相对安全路径（拒绝 `..`/绝对路径/控制字符），条数与大小有限；CLI 写入前校验，卸载只清理清单登记过的文件且目录无外来文件才整目录移除；GitHub 解析失败返回 502 且不落库脏数据；生态发布（管理员导入）与消费（用户安装）分离，用户不能自行发布（与"只托管运营方预设"一致，后续再做共建流程）。
