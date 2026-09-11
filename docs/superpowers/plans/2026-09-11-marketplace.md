# 插件市场执行记录（2026-09-11）

设计见 `docs/superpowers/specs/2026-09-11-marketplace-design.md`；按用户追加决策收窄为 **http-only 市场 + CLI 本地直连插件管理**（借鉴 cc-switch 的配置管理体验；Codex 等客户端的版本/供应商管理不属于本产品边界，doctor 仍做客户端检测）。

## 变更清单

**服务端**
- 迁移 `017_marketplace.sql`（marketplace_items + 6 条 curated 种子）、`018_tool_metadata.sql`（tool_metadata_overrides）、`019_marketplace_http.sql`（http-only：删除 stdio 精选，新增 microsoft-learn）。
- `internal/marketplace`：GitHub Search（`mcp-server in:name topic:mcp-server`，星标序，30/页）；Upsert 仅覆盖 github 来源行，curated 永不被覆盖；失败目录不变。
- app：`GET /api/v1/admin/marketplace`、`POST sync`（502 upstream_unavailable）、`POST install`（http-only；sealed config；重复 409）、`POST uninstall`（停用工具并解除回链）、`GET /api/v1/admin/tools/{id}/upstream`（实时发现+覆盖态）、`PUT/DELETE metadata`；`GET /api/v1/marketplace`（Bearer/会话，公开字段，无 stdio）。
- 网关：目录构建按 (tool_id, remote_name) 应用覆盖（描述与 InputSchema；无效 schema 整体忽略）；导出 `UpstreamTools`。
- 配置：`LOADOUT_GITHUB_API`、`LOADOUT_GITHUB_TOKEN`（ENVIRONMENT/.env.example 已记录）。

**CLI（Rust）**
- `loadout market` / `loadout install <slug> [--url] [--clients]` / `loadout uninstall <slug>`：公共 HTTP 端点直连写入客户端配置，`loadout-<slug>` 托管条目（独立标记 + 备份 + manifest `<client>:<slug>`），零凭证落盘，与 bridge 条目共存。
- `clients.rs` 的 toml 拼接/渲染辅助参数化（markers/key），bridge 行为不变。

**Web**
- ToolsPage：管理员「插件市场」区块（列表/同步/安装/卸载，http-only 安装器）+ http/stdio 工具的上游工具与描述覆盖编辑器；i18n 中英。

## TDD 证据（命令均在 `server/` 或仓库根执行，`LOADOUT_TEST_DATABASE_URL` 来自 `.env`）

| 步骤 | RED | GREEN |
|---|---|---|
| 目录覆盖 | `go test -run TestCatalogAppliesMetadataOverrides ./internal/gateway/` → `ping override not applied` | 同命令 ok |
| 同步 | `go test ./internal/marketplace/` → `items=0` / `403 must surface` / `count=0` | 同命令 ok |
| 市场 API | `go test -run 'TestMarketplace|TestToolMetadataOverrideEndpoints' ./internal/app/` → 404/403/400 | 同命令 ok |
| 公开端点+http-only | → `token marketplace list 404` | 同命令 ok |
| CLI 插件 | `cargo test --locked --test plugins` → `no method named install_plugin` | 3 passed |
| Web 市场 | `pnpm exec vitest run src/Marketplace.test.tsx` → 3 failed（无 UI） | 3 passed |

UpstreamTools 与 applyOverrides 在同一 GREEN 内实现，行为测试紧随（TestUpstreamToolsReturnsRawDefinitions）——与"先测后码"的偏差在此记录。

## 真实 E2E（本机 127.0.0.1:8787 + 真实外网）

1. `POST /admin/marketplace/sync` → `{"synced":30}`；目录 36 项（curated 在前，github 按星标）。
2. 安装 deepwiki（curated http）→ tools 行 + 密封配置；隔离 HOME bridge `tools/list` 出现 `deepwiki__ask_question/read_wiki_contents/read_wiki_structure`。
3. PUT 覆盖 ask_question（中文描述 + 参数说明）→ bridge 目录即生效（描述与两个参数描述均为覆盖文案）。
4. bridge 真实调用 `deepwiki__ask_question(microsoft/vscode)` → 真实 AI 回答；余额 1000→998（计量生效；此前一次对未索引仓库的失败调用亦按预留扣除，账本一致）。
5. CLI：`loadout market`（33 项）；`loadout install microsoft-learn --clients codex` → 与 bridge 条目共存；`uninstall` 干净移除。
6. 环境注记：本机 DNS 为 fake-IP（198.18.0.0/15 + ULA），网关 SSRF 防护正确拒绝；按 docs/DEPLOYMENT.md 的演示惯例以 `LOADOUT_ALLOW_PRIVATE_UPSTREAMS=true` 验证。生产正常 DNS 不受影响。

## 回归

- `go test -race -count=1 ./...`（服务端全包）通过；期间修正 7 处"内置工具=2"的偶然计数断言（boundary/hardening/metadata_validation/catalog_revision/redis/distributed）。
- `cargo test --manifest-path cli/Cargo.toml --locked` 全部通过（28 用例，含既有 clients/bridge/doctor）。
- `pnpm exec vitest run` 114+3 用例通过；`make lint`（biome+tsc+golangci）通过。
- `make check` 结果见下。

## make check

命令：`set -a; source .env; set +a; make check`（2026-09-11 本机，日志 `/tmp/loadout-check3.log`）
结果：**退出码 0**，10 个子目标全部进入并完成（database/redis/skills 检查、lint 含 biome+tsc+golangci+clippy+fmt、test 三端、test-desktop、test-e2e、test-process 4/4、test-dev 6/6、integration.test.mjs）。首次运行因 biome 格式化与残留探针目录失败，修复后完整重跑通过。

## 残留

- 演示库中 deepwiki 已安装且 ask_question 带一条中文覆盖（可经后台清除）；验证用户 `codex_plugin_e2e2` 及其令牌仍在本地 demo 库。
- `/tmp/loadout-e2e/` 为隔离验证目录，可删。

## 修订 2 执行记录：skills 与装备组（同日晚）

- 迁移 `020_marketplace_kinds.sql`：`kind`（mcp/skill/bundle）+ `spec`（inline 文件 / github repo+path / includes）；精选种子 2 条内联技能（commit-style、verify-before-done）。
- 服务端：`POST /admin/marketplace/skills`（inline / github 导入，github 即时解析校验，失败 502 不落库）、`POST /admin/marketplace/bundles`（成员存在且非 bundle）、`GET /api/v1/marketplace/{slug}/files`（技能文件唯一出口，GitHub 由服务端代取）；install 守卫非 mcp 条目 400。
- CLI：`install`/`uninstall` 按 kind 分发；技能写入 codex/claude skills 目录（cursor 明确不支持），清单登记文件集，外来文件拒绝整目录卸载；bundle 递归成员安装/卸载；market 输出 kind 列。
- Web：市场列表 kind 标签（MCP/技能/装备组），技能与装备组提示用 CLI 安装。
- TDD：app `TestMarketplaceSkillsBundlesAndFiles`（404 RED→GREEN，含遍历拒绝/嵌套拒绝/权限 403/files 端点）；CLI `skill_install_writes_managed_directories_and_uninstall_refuses_foreign_files`、`bundle_install_replicates_expert_setup_in_one_command`（no-method RED→GREEN）。
- 真实 E2E：管理员建 `expert-pack`（deepwiki + 两技能）→ `loadout install expert-pack --clients codex` 一条命令写入 deepwiki 直连条目 + 2 个技能目录（SKILL.md 内容正确）→ uninstall 反向清除；market 显示 mcp/skill/bundle 三类。
- 回归：`go test -race ./...`、CLI 全套、Web 114 用例、`make lint` 通过；`make check` 见下。

### make check（修订 2 后）

`set -a; source .env; set +a; make check` → 退出码 0（日志 `/tmp/loadout-check4.log`，10 个子目标全部完成）。

## 修订 3：结算中间件、压测与内存安全（同日深夜）

### 结算中间件（自定义成功判定）

现状澄清：`store.finish` 对非 ok 状态**本来就全额退款**（钱包加回 + ledger refund + cost=0），协议层 `isError` 已"失败不扣费"。本修订补的是**业务体判定**：上游返回 isError=false 但 body 携带失败语义（`{"code":500}`）时按管理员规则判失败。

- 迁移 `021_settlement.sql`：`tools.settlement jsonb`，默认 `{}`（保持现状）。
- 规则：`{"content":{"path":"code","equals":0}}`（JSON 点路径判等，**equals 支持数组任一命中**，覆盖多成功码上游）或 `{"content":{"pattern":"^OK"}}`（拼接文本正则）。判定失败 → `Finish(false)` 走既有退款路径，返回内容保留上游文本并加 `[loadout] 结算检查未通过，本次不扣费` 前缀；配置了 path 但内容非 JSON → 判失败（严格侧）。
- 管理端：saveTool 接受 settlement（校验非法 400），listTools 返回；Web 工具编辑器新增"结算策略（JSON，可选）"。
- TDD：gateway `TestSettlement*` 三用例（404/策略缺失 RED→GREEN）+ `TestSettlementEqualsAnyOf`；app `TestSaveToolSettlementPolicy`。
- 真机 E2E：deepwiki 配 pattern 正/反两例——匹配扣费（997→996）、不匹配退款（余额不动，内容带前缀）；规则已恢复默认。

### 压测与容量（`make load-test`，16 核 9700X 单副本 PG+Redis）

| 场景 | 结果 |
|---|---|
| store 同钱包串行/并行（既有基准） | 111 / 164 QPS（钱包行锁物理上限） |
| store 64 钱包并发（新增 BenchmarkManyWallets） | **1416 QPS**（0.71ms/op） |
| HTTP tools/call 完整计量链路（32 并发 20s） | **727 QPS**，p50=42ms p99=144ms，0 失败 |
| HTTP tools/list 目录缓存路径（16 并发 10s） | **2653 QPS**，p99=9.5ms，0 失败 |
| 计量一致性 | usage_logs ok=14552 = 请求数+预热，逐笔对上 |

支撑改动：限流可配置（`gateway.Options.TokenPerMinute/UserPerDay` + `LOADOUT_RATE_TOKEN_PER_MINUTE/LOADOUT_RATE_USER_PER_DAY`，默认不变；`TestAdmitUsesConfiguredLimits` RED→GREEN）。
瓶颈注记：727 vs 1416 的差值主要是 `/mcp` 每请求重建 MCP server + AddTool 循环与 admit/auth 的 DB 往返；单副本足够小团队场景，更大 QPS 走水平扩副本（多实例已有验收）。

### 慢上游 × 高 QPS（TestLoadGatewaySlowUpstreams）

| 阶段 | 结果 |
|---|---|
| A 混合：12 路慢调用(8s)+48 路快调用 30s | 快调用 **673 QPS 0 失败**（vs 纯快 727——慢调用几乎不拖快路径，无队头阻塞）；慢调用 48/48 成功 |
| B 80 路并发 8s 慢调用（超每上游 32 租约槽） | **24s 全部完成 0 失败**（80/32≈2.5 批 × 8s，排队不拒不死锁） |
| C 单次 35s（超上游 30s 超时） | 干净 isError + usage error + cost=0（退款） |
| 收尾 | goroutines=57、无 pending 残留 |

### 内存安全（TestLoadGatewayMemorySafety）

64 并发 60s 持续负载，每 5s 采 `/metrics`：goroutines 基线 13 → 负载 ~180-240 → **空闲回落 12**（连接 goroutine 由 IdleTimeout=60s/客户端关闭归因验证，非泄露）；RSS 22→34→35MB 稳态，负载后半段无单调增长（mid/last <15% 断言）。压测子进程通过 `/metrics` 采样 `go_goroutines`/`process_resident_memory_bytes`（prometheus 文本科学计数法注意 ParseFloat）。

### SQLite 单机部署

未实施——评估与四阶段路径见 `docs/adr/0002-sqlite-single-node.md`（数据层 100+ 条 PG 专属 SQL，双方言=测试矩阵翻倍，需专门工作流；Redis 分布式侧已就绪无需重做）。

### 修订 4：多方言库调研 + SQLite 可行性 Spike（同日）

- 调研：one-api/new-api 以 GORM v2 实现三库切换；候选 GORM/bun/ent/sqlc 均覆盖 pg/mysql/sqlite。**选定 bun**（SQL-first 贴现有 pgx 风格）+ modernc.org/sqlite（纯 Go）+ golang-migrate 双轨迁移。
- Spike（throwaway，未入仓库）：bun+SQLite 复刻账本事务，同钱包 **5884 QPS**、64 钱包 **8620 QPS**、12800 笔零超扣、退款幂等；方言差异实证清单（_txlock=immediate 替代行锁、触发器拆分、多语句 Exec 拆分、synchronous 取舍）。详见 ADR 0002。
- 上轮遗留说明：自定义 JS 结算脚本（goja）已完成选型与安全设计（沙箱纯函数/200ms 中断/内存上限/编译缓存），因本主题插入未实施，go.mod 已 tidy 还原无残留。

### 修订 5：JS 结算脚本落地 + 前后端双重校验 + 加载动效（同日）

**JS 结算脚本（goja 沙箱）**
- `{"script":"…"}` 第三种结算模式：脚本为接收 `result={isError,text}` 的函数体（IIFE 包装，支持 return/多语句/循环）；无任何宿主绑定（零 I/O）、200ms `Interrupt` 中断、编译产物按源码缓存（sync.Map）；异常/超时/非真值一律判失败走退款。
- TDD：`TestSettlementScript`（业务码正反例+非 JSON 异常）、`TestSettlementScriptTruthyAndInfiniteLoop`（truthy 放行；`while(true){}` 在中断预算内判定失败且 <2s）；app 层保存校验（语法错/超 8KB/脚本与规则互斥均 400）。
- 真机 E2E：deepwiki 配 `t.length>40` 脚本 → 真实回答扣费（996→995）；配死循环脚本 → 200ms 中断、退款、上游内容保留；规则已还原默认。

**前后端双重校验**
- 后端：`invalid_settlement` 专属错误码（原来混入 invalid_request）。
- 前端：提交前 `new Function('result', script)` 仅编译不执行做语法预检，坏脚本不出本机；`invalid_settlement`/`invalid_settlement_script` 友好文案入 messages 映射。
- 组件测试 ×2：语法错误拦截（无网络请求发出）/合法脚本正常提交（RED→GREEN）。

**UI 加载动效（impeccable 质量底线）**
- 主操作按钮 pending spinner（市场同步/安装到工具池/卸载/保存工具/保存覆盖）。
- 列表条目鱼贯而入：rise-in 240ms `cubic-bezier(0.16,1,0.3,1)` + 40ms stagger（全页唯一 authored moment）。
- editor/market/metadata 面板展开 180ms 浮现；`prefers-reduced-motion` 全局兜底既有。
- 桌面/移动端视觉人工检查按契约留待用户确认（未用浏览器自动化）。

**回归**：go -race 全套（后台退出码 0）、Web 116 用例、make lint、`make check` EXIT=0（`/tmp/loadout-check7.log`）。

**多方言工作流状态**：盘点完成（生产 SQL：store 20 + app 28 = 48 条；迁移 17 文件），选型 bun + modernc/sqlite + golang-migrate 已定，Spike 数据见 ADR 0002；实施为独立工作流待续。

### 修订 6：SQLite 迁移轨道落地 + AGENTS.md 长期维护版（同日深夜）

- **SQLite 迁移轨道**：`server/internal/store/migrations_sqlite/` 17 文件与 PG 轨道一一对应；转写要点——IDENTITY→AUTOINCREMENT、正则 CHECK 降为长度/枚举（应用层负责）、plpgsql 触发器拆事件、013 约束演进并入 001 基线、`ADD COLUMN UNIQUE` 改独立唯一索引、STATISTICS 跳过、时间统一 ISO8601 UTC 文本。
- **行为验证**（`TestSQLite*` 三用例，纯 Go 零外部依赖）：23 表建齐；种子数量与 JSON 合法；账本 UPDATE/DELETE 双双 ABORT；tools 增删使目录修订 +2；钱包归属触发器拒绝无主/双主。
- **AGENTS.md 重写为长期维护版**：产品速查 + 模块地图、测试分层表（含 SQLite 轨道与 load tag）、结算即钱路径/市场 http-only/技能路径安全等新增边界、文档同步义务表、本文自身更新要求。
- 依赖：`modernc.org/sqlite v1.58.0`（纯 Go）。store 层 bun 移植与 app 层 28 条查询仍为待续清单（ADR 0002 状态已更新）。

### 修订 7：Codex 官方插件市场导出（产品方向定锚）

- 调研：OpenAI 已发布 Codex 插件体系（skills+MCP+apps，`codex plugin marketplace add` 支持任意 git/本地源）——生态位在第三方源；awesome-codex-skills 1.3 万 star 验证策展需求。
- 实现（TDD）：`internal/marketplace/exporter.go`——`ExportCodexMarketplace`（纯渲染，RED 断言清单结构/插件树/skills/mcp.json/跳过原因）+ `LoadExportInputs`（库→导出输入，内联直取、github 服务端解析、bundle 成员装配，失败跳过不产坏件）；命令 `cmd/loadout-export`。
- **真机全链路验证（Codex CLI 0.153.4）**：dev 库导出 36 条→6 插件（3 MCP+2 技能+expert-pack 装备组，31 个无端点 GitHub 项正确跳过）→ `codex plugin marketplace add /tmp/loadout-market` 成功 → `plugin list` 6 项全解析 → `plugin add expert-pack` 安装 enabled 并自动注册 `[plugins."expert-pack@loadout"]`；验证后已清理。
- 方向校准：PLAN §9.3a 定锚"官方机制为地基的策展源"定位；README/AGENTS 模块地图同步。

### 修订 8：服务端即插件市场源（终态闭环，同日）

- 迁移 022（双轨）：`transport` 增加 `gateway`（网关独享组件，无公共端点，指向池内工具）。SQLite 并入 017 基线。
- `POST /admin/marketplace/tools`：池内工具一键发布为插件组件（TDD：发布/404/重复 409/越权 403）。
- 导出器：gateway 成员渲染为 **stdio bridge 条目**（`{"command":"loadout","args":["bridge"],"env":{...}}`）——公共端点直连、独享走网关的双语义（TDD）。
- `internal/marketplace/gitrepo.go`：纯标准库 git 对象模型（blob/tree/commit + 松散对象 + 哑协议元数据）；修复对象路径 2/38 hex 布局后**真实 git clone 通过**、重建后内容新鲜（闭包捕获陷阱为测试 bug）。`registry.go`：懒构建 + TTL + 变更失效；`/marketplace.git/` 挂载，市场 mutation 即时刷新。
- `/plugins` + `/plugins/{slug}`：服务端直出 SEO 目录/详情页（meta/OG + 一行安装命令）。
- CLI：bundle 内 gateway 成员安装=确保 bridge（不落直连），卸载不动 bridge（TDD）。
- **真机终态验证（Codex CLI 0.153.4）**：发布"Pro 视频问答"（池内 deepwiki→gateway）+ 视频创作包 → SEO 页渲染 → `codex plugin marketplace add http://127.0.0.1:8787/marketplace.git`（服务端源直连成功）→ `codex plugin add video-pack` → **stdio bridge mcp.json 被接受**、skills 落位 → 独享工具经 bridge 真实调用扣费（995→994）。验证后已清理 codex 侧。

### 修订 9：全链路同步机制（Codex 侧 + 客户端侧，同日）

同步模型（三层）：
1. **池内网关工具**：天然实时（bridge 每次连接拉最新目录，服务端 5s TTL + 变更失效）。
2. **Codex 插件侧**：`codex plugin marketplace upgrade loadout` → `codex plugin add <slug>`。为此修两个真缺陷（真机实测暴露）：①裸仓每次全新根提交被 upgrade 判"已是最新"→ registry 维护**提交链**（parent）+ 对象累积合并（哑协议要求整链可达）；②版本恒 1.0.0 重装不刷新 → plugin.json 版本改为**内容指纹** `1.<hash8>`（同内容幂等、内容变必更新）。另补 createSkill **upsert**（同 slug = 内容迭代，原先唯一键冲突 500 静默吞掉运营更新）。
3. **客户端本地插件**：新增 `loadout update [--clients]`——按 manifest 收集托管 slug 逐个幂等重装；并实现**污染自愈**（实测 codex plugin add 会把 [plugins.*]/[marketplaces.*] 段插进托管标记块中间，导致块校验失败）：更新时把外部段搬出块外、内容零丢失、TOML 保持可解析。真机验证：codex 侧 v1→v3 经 upgrade+add 全程跟随（版本 1.e644f824）；客户端 `loadout update` 在真机污染 config 上自愈成功。

TDD：registry 链式提交（clone 驱动断言）、contentVersion 指纹、createSkill upsert、update 命令、污染自愈（构造 codex 插段注入块中）共 4 个新用例；全套 CLI 8 用例绿。

### 修订 10：正式发版（版本号运营方掌控，同日）

- 迁移 023（双轨）：`marketplace_items.version`（semver，默认 1.0.0）+ `content_hash`。
- 版本策略（`formalVersion`）：内容不变 → 版本不动（幂等）；内容变 + 显式 `version` → 用指定版本（**正式发版**）；内容变 + 未指定 → 自动 patch +1（静默修复）；非法版本 400。
- 导出：plugin.json 优先用正式版本，空则回退内容指纹；`/plugins/{slug}` 详情页展示版本号；API item 回显 version。
- 真机发版验证：update-probe 指定 2.0.0 → upgrade + add → cache 目录 `2.0.0/`、内容"正式发版 2.0.0"。
