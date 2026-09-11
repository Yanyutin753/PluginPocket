# 工具管理与规则编辑优化

**Goal:** 工具支持自定义图片/SVG；管理员能按说明配置连接、参数与结算规则；配置在多个服务副本间共享。

**Architecture:** 复用 tools 表与现有 API，新增 icon 字符串（HTTPS 图片或受限 data 图片），不引入本地上传目录。前端复用 SidePanel、React Hook Form、shadcn 和已有结算协议，常用规则由表单生成，高级 JSON 保留。规则试算由服务端同一 Go 判定执行，不在浏览器运行用户脚本。

**约束:** 当前工作区保留已有修改；不提交/推送/发布；不改真实客户端配置；不使用浏览器自动化；中英文；完整 TDD 与 make check。

## 验收与步骤

- [x] 后端：先写跨副本图标创建/读取/清空、非法图标拒绝、PATCH 省略规则保持原规则和内置账号工具可编辑的测试。RED 后实现 icon 双轨迁移、验证与 API；GREEN 后相关回归。
- [x] 规则预览：先测鉴权、默认/业务码/正则/脚本试算与退款结果，复用网关判定，不发上游请求、不扣费。RED/GREEN 记录。
- [x] 前端：先测图标输入/上传/预览/清空、规则表单生成/重开/清除、JSON 错误与重试、规则试算、键盘流程；再按基本信息/连接/参数/计费分区实现，列表统一操作排列与类型信息。
- [ ] 同步 API、PLAN、前端设计规范；完整 make check、设计扫描与最终审查。视觉人工验收若无法执行明确标注。

## 执行记录

初始发现：所有列表图标固定 Wrench；编辑长 JSON 表单缺少结构；PATCH 省略 settlement 会重置策略，启停也受影响；builtin 保存只允许 echo/time_now，使账号内置工具不可编辑。测试需覆盖这些根因。

分布式解释：已向用户询问，默认指共享数据库多副本配置，图标不存本地磁盘。用户追加强调图片上传，文件选择与预览/移除均纳入交付。

### 前端 TDD 与回归

命令均从仓库根通过 WSL 运行，PATH 固定加入 Node26.8.2、cargo、local/bin；使用现有依赖，不下载。

- RED：`pnpm --dir web exec vitest run src/ToolEditor.test.tsx`，5项因缺少扣费条件/更新连接/参数示例控件失败，日志 `.loadout/tool-editor-red.log`。最初 pnpm PATH 缺失是环境准备，不计 RED。
- GREEN：相同命令5项通过；覆盖业务码生成/重开/清除、服务端试算及输入变化清结果、保存错误恢复、HTTP连接保留和更新、参数示例。修正 user-event 字符串输入转义与断言API使用后得到真实结果。
- REFACTOR：分离 ToolEditor/SettlementEditor，ToolsPage 仅管理列表与面板；复用 shadcn 与现有请求/分页，无新增依赖。
- 第二轮 RED：高级 JSON 切换测试观察到字段路径由 result.code 回退为 data.code；修复切换时使用当前表单值生成 JSON。试算期间修改样本的回归直接通过，未对已正确的 Mutation.reset 行为增加逻辑。
- 审查 RED：保存的 `equals:null` 重开变成0。`.loadout/tool-editor-null-red.log` 记录断言失败；改为仅 undefined 使用默认值。
- GREEN：`pnpm --dir web exec vitest run src/ToolEditor.test.tsx src/Operations.test.tsx src/SettlementValidation.test.tsx src/Localization.test.tsx`，53项通过，日志 `.loadout/tool-editor-final-focused.log`。
- 现有 JSON 用例通过新的“高级 JSON”入口操作，保留错误恢复断言；脚本测试用粘贴输入长JSON，取消固定sleep。登录测试fixture改为未登录，以符合工作区当前登录后重定向行为。
- `pnpm --dir web build` 通过（类型检查+Vite生产构建），日志 `.loadout/tool-editor-build.log`。
- Impeccable扫描仅5项字号/圆角 advisory；字号为本页16px列表标题/17px分段/12px元数据，已说明于设计文档；4px焦点框和8px内联试算结果为局部控件，不增加页面卡片层级。

### 后端与图标

精确RED/GREEN和多副本用例见 `2026-09-11-tool-editor-backend.md`；图标上传/粘贴/移除/错误恢复见 `2026-09-11-tool-icon-ui.md`。审查仅发现上述null规则回填问题，已修复并回归。

### 整体检查（持续更新）

`node --env-file=.env .loadout/tool-editor-check.mjs` 实际调用 `make check`。首次在 lint 阶段被当前工作区其他登录/市场改动的8处格式检查阻断，日志 `.loadout/tool-editor-check.log`；未把此结果报告为全量通过。

首次完整Web测试182通过/3失败：两项属于正在修改的市场登录侧栏旅程，另一个脚本输入测试在并行负载下超时；本任务将长JSON输入改为真实粘贴后重新验证。未更改市场侧栏行为。

遵守用户契约不使用浏览器自动化；桌面和手机实际视觉未人工验收，组件测试与生产构建不替代此证据。

### 共享工作区回退与恢复

约21:11，外部stash操作临时移走全部tracked修改；本任务没有执行reset/restore/stash。本轮ToolEditor接线、icon字段、preview路由与其他任务代码均暂时消失。用户明确答复“其他任务误覆盖，请恢复本轮改动”。与插件市场/鉴权任务协调后，外部恢复操作已把完整快照写回；固定快照对象 `5c4df696e72ee6658139255a4bdb668c6b184e81` 可用于核对，本任务没有并行apply。

恢复后只读核验ToolEditor接线、icon模型、preview路由和两份i18n均存在。再次运行 `pnpm --dir web exec vitest run src/ToolEditor.test.tsx src/ToolIcon.test.tsx src/SettlementValidation.test.tsx src/Operations.test.tsx src/Localization.test.tsx src/PanelFlows.test.tsx src/Marketplace.test.tsx`：94项全部通过，日志 `.loadout/tool-editor-restored-web.log`。

图标最终22项通过，上传入口在地址/SVG输入之前；上传后显示文件名，重开显示“已上传图片”，不显示base64。外部视觉任务补充的工坊默认图标实现已保留。

整库检查由本任务统一执行，其他任务不再并行运行重负载全量。此前运行分别遇到同时修改的auth/市场格式及暂时的类型错误，不能作为最终结果；安全格式整理不更改其功能逻辑。等待鉴权任务完成最后跨副本刷新断言与gofmt后执行最终检查。

### 追加：开发页面504修复

用户反馈 ToolEditor 动态导入报 `504 Outdated Optimize Dep`。通过真实HTTP复现：`/src/features/operations/ToolEditor.tsx` 200，但该模块返回的 `react-hook-form.js?v=35b5edf9` 504，因此不是仅浏览器旧页面缓存。发现 PluginProxy 测试直接createServer及多进程e2e Vite均共用 `web/node_modules/.vite`，其他副本可覆盖当前进程预构建元数据。

- RED：`pnpm --dir web exec vitest run src/PluginProxy.test.ts -t isolated`，两个运行目录解析为同一个缓存目录，断言失败。日志 `.loadout/tool-vite-cache-red.log`。
- GREEN：Vite按现有 `LOADOUT_RUN_DIR` 使用独立vite-cache；代理测试用mkdtemp设置运行目录并清理。`pnpm --dir web exec vitest run src/PluginProxy.test.ts` 2/2通过，日志 `.loadout/tool-vite-cache-green.log`。
- 修复配置触发当前Vite自动重启；`node .loadout/tool-vite-probe.mjs` 验证工具模块、新依赖地址和用户提供的旧hash依赖地址均200。未关闭用户开发进程，未删除node_modules，未改锁文件。
- 同步ENVIRONMENT与.env.example，后续真实开发e2e使用其已有隔离运行目录，不再覆盖日常5173缓存。

- 进程测试的两个前台 Vite 实例也改用临时 LOADOUT_RUN_DIR，结束后清理，避免继承日常 .env 中的运行目录。沿用已有真实启动/退出测试验证，不改变生产进程行为。
- 完整检查曾在进程测试第4项30秒超时；未增加超时或删断言，原测试文件再次运行4/4通过（.loadout/tool-process-diagnostic.log），继续执行最终完整检查。

### 最终验证（2026-09-12 会话恢复后）

- 前一轮 `.loadout/tool-editor-check-complete.log` 中 Go race、192项Web、桌面、真实 TestProductJourney（含多副本）通过，随后进程测试超时，不能称完整check通过。下一轮 `.loadout/tool-editor-check-final.log` 因会话中断收到Hangup。
- 本次 `node --env-file=.env .loadout/tool-editor-check.mjs`（内部make check）重新运行：格式/类型/Go lint/CLI与桌面静态检查通过；真实PG测试因127.0.0.1:44035连接拒绝失败，日志 `.loadout/tool-editor-check-resumed.log`。原开发环境文档中的 `/tmp/loadout-pg18` 与 `/tmp/loadout-pgdata` 已不存在。未替换业务库、未初始化假数据、未改.env；当前5173服务也已停止。
- 本次 `make test-web test-process test-dev integration` exit 0：193项Web、4项进程、7项后台/热重载、4项生产HTTP/CLI集成通过，生产Web/Go/CLI构建通过，日志 `.loadout/tool-editor-local-final.log`。
- 本次 `cd server && go test -run TestSQLite ./internal/store` exit 0。
- 504修复之前已用真实HTTP确认新旧依赖地址200；本次Vite隔离配置与真实开发进程测试通过。未使用浏览器自动化；桌面/移动视觉仍未人工验收。
- 为运行完整检查，仅对新错误码字典 `web/src/i18n/error-codes.json` 做Biome格式整理。共享工作区其他任务的变更保留。

交付：上传PNG/JPEG/WebP/静态SVG（64KiB）、HTTPS图标/粘贴SVG、预览/移除；共享数据库跨副本保存；分区连接与规则编辑、样例与真实服务端试算；相关API/迁移/中英文及设计文档已同步。完整make check的当前未通过原因明确为缺失临时PostgreSQL环境；不将历史通过或局部检查拼接成全量成功。