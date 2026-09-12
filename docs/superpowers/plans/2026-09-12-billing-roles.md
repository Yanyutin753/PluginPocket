# 计费角色机制实施计划（billing roles）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans（本仓已授权 inline 执行）。

**Goal:** 引入计费角色（billing role）：每个角色一个消耗倍率，调用在预留事务内按用户角色折算扣费；用量与用户管理全链路显示角色。

**Architecture:** 计费角色独立于鉴权角色（users.role: user/admin）与团队角色（owner/member）。`billing_roles(name, multiplier_bp, description)` 以基点整数存倍率（10000=1.0×），种子 default/member/vip；`users.billing_role` 默认 default。倍率在 store 的 `reserve` 事务内读取应用（`ceil(cost×bp/10000)`），`usage_logs` 记录实际生效的 `billing_role`/`multiplier_bp`；结算中间件语义不变（仍是对预留额的扣/退）。PG 与 SQLite 双实现同步改。

**Spec:** docs/PLAN.md §9.4/§12（本计划随附修订）；资金路径约束见 AGENTS.md 工程边界。

## 设计决定（假设已固化，执行中不再摇摆）

1. **倍率用基点整数**（multiplier_bp，1bp=0.01%），杜绝浮点进钱的路径；上限 1,000,000（100×）。
2. **向上取整**：`cost=(cost*bp+9999)/10000`——对运营方安全（不欠收）；bp=0 允许（免费角色）。
3. 角色集合**种子三档**（default 1.0×、member 0.8×、vip 0.5×），管理员可改倍率与描述，**不做增删角色**（增删牵连 FK 与 UI 面，YAGNI，记录在案）。
4. 幂等重放比较的是**折算后** requested_cost（同请求键重算同值，语义自洽）。
5. 历史调用行补 default/10000（历史即按工具原价计费，语义准确）。
6. 错误码全部复用现有注册表（not_found/invalid_request/last_admin 等），不新增 wire code。

## 文件与任务

### Task 1: 文档先行
- PLAN.md：顶部变更记录 + §9.4 计量公式补角色倍率 + §12.1 说明；API 表加 admin/billing-roles。
- 本计划入库。

### Task 2: 迁移 031 双轨
- `server/internal/store/migrations/031_billing_roles.sql`（PG）与 `migrations_sqlite/031_billing_roles.sql` 一一对应：billing_roles 表 + 种子三行 + users.billing_role（FK, NOT NULL DEFAULT 'default'）+ usage_logs.billing_role/multiplier_bp（NOT NULL DEFAULT）。
- 跑既有迁移等价测试（`TestSQLiteMigrationTrackBuildsEquivalentSchema` 等）确认双轨一致。

### Task 3: store 层倍率（RED→GREEN，双方言）
- RED（PG `store_test.go`）：用户设 billing_role='vip' 后 ReserveTool 按工具价 5×0.5=2.5→ceil 3 扣费；usage_logs 行带 vip/5000；账本 balance_after 一致；bp=0 → 扣 0。
- RED（`store_sqlite_test.go`）：同断言镜像。
- GREEN：`store.go` reserve 事务内 `SELECT r.multiplier_bp,r.name FROM billing_roles r JOIN users u ON u.billing_role=r.name WHERE u.id=$1`，折算 cost，INSERT usage_logs 带两列；`store_sqlite.go` 同步（SQL 仅方言参数差异）。
- 回归：默认角色下全部既有账本/团队/app 测试不变。

### Task 4: 用量与用户管理 API（RED→GREEN）
- admin.go:365 列表 SQL 与 usage_detail.go:41 详情 SQL 增加两列；对应 Item 结构（admin.go/teams/account 的 usage item 定义处）与 JSON 输出加 `billing_role`、`multiplier_bp`。
- 新增 `GET /api/v1/admin/billing-roles`（items: name/multiplier_bp/description）与 `PATCH /api/v1/admin/billing-roles/{name}`（multiplier_bp 0..1000000，description ≤200）。
- admin 用户列表 SELECT 与 PATCH（adminUserUpdate 模式扩展）支持 billing_role（白名单取自 billing_roles 表，防 FK 破坏）。
- RED：app 层 httptest 断言新字段与新端点（404/400 边界：改不存在角色、非法倍率、未知 billing_role 赋值）。

### Task 5: Web（RED→GREEN，双语）
- `features/account/api.ts`：usageSchema/adminUserSchema 加字段；billingRoleSchema + query。
- AdminPage 用户表：计费角色列 + 下拉编辑（保存走 PATCH）。
- PlansPage（套餐页）新增"计费角色"分区：列表 + 倍率/描述编辑。
- UsagePage（个人）/UsageSummary（团队/管理员）行与 UsageDetail：角色徽标 + 倍率展示（如 `vip ×0.5`）。
- i18n 双语新文案；组件测试覆盖显示与编辑路径（键盘可操作、错误态）。

### Task 6: 契约与收尾
- docs/API.md + docs/openapi.json 同步；`make check` 全绿；提交（仅本特性文件，避开并行会话在途文件）；push；远程 CI 绿。

## 验收映射

- 管理员改 vip 倍率为 5000 → 指定用户 vip → 该用户调用 5 价工具扣 3，个人/团队/管理员用量行显示 vip ×0.5；账本与余额精确一致（并发不透支既有测试继续通过）。
- 未赋角色用户行为与现状完全一致（default 1.0×）。

## 增补（2026-09-12 用户澄清）：工具级角色门槛（对标 new-api 分组开放）

- `tools.allowed_roles`（jsonb/TEXT，NOT NULL DEFAULT '[]'，并入未发布的迁移 031 双轨）：空数组对所有计费角色开放；非空时仅列出的角色可调用。
- store 预留事务：读取工具价同时读取 allowed_roles，角色判定通过后（ roleName 已取得）若名单非空且不含当前角色 → 返回新哨兵 `ErrRoleNotAllowed`，不产生扣费；网关 `call()` 对该哨兵返回工具错误"当前计费角色无权使用该工具"并 `recordDenied` 留痕（与限频/下线工具同型）。
- 不新增 wire 错误码：网关错误走 MCP 工具结果而非 HTTP 错误信封；REST 侧 saveTool 校验沿用 invalid_request/not_found。
- 管理端：`GET /api/v1/tools`/`admin/tools` 返回 `allowed_roles`；`POST/PATCH /admin/tools` 接受 `allowed_roles`（省略保持原值，名字必须存在于 billing_roles）；ToolEditor 提供角色复选，工具列表显示限制徽标。
- 测试：PG/SQLite 各加门槛用例（默认用户被拒、vip 放行且按倍率计费、空名单全开放）；app 层 saveTool/listTools 往返；Web 编辑与显示。

## 执行记录（2026-09-12，inline 会话）

- **RED（store 双方言）**：`TestBillingRoleMultiplierAppliesAtReserve` / `TestSQLiteBillingRoleMultiplierAppliesAtReserve` 失败于"balance=5"（未折算）；`TestToolRoleGating` / `TestSQLiteToolRoleGating` 先失败于哨兵未定义，补哨兵后失败于"must be gated: <nil>"（门槛未生效）。
- **RED（app）**：`TestBillingRolesAdminListUpdateAssignAndUsageDisplay` 404（端点不存在）；`TestToolAllowedRolesRoundTrip` 失败于 allowed_roles 缺失。
- **RED（web）**：`src/BillingRoles.test.tsx` 3 项（用量徽标/管理指派/倍率编辑）。
- **GREEN 过程中的真实缺陷**：①角色读取初版置于授权校验前，伪造用户返回 no-rows 而非 ErrUnauthorized——重排为授权后读取；②`allowedRolesRaw` 声明在 toolID 分支内导致作用域编译错——提升至函数级；③Operations 测试的 PATCH 字段白名单需同步 `allowed_roles`（编辑器必须始终发送该字段以支持清空门槛）。
- **最终聚焦**：store 全量（PG+SQLite，race）ok；app 全量 ok（58s）；web 34 文件 234 测试通过（含 4 项新测试）。
- **回归保障**：默认角色 10000bp 下既有账本/团队/并发测试全部不变（倍率恒等）。
- 全量 `make check` 结果见 `.loadout/billing-roles-check.log`。
- 未验证范围：浏览器自动化按项目要求未使用；管理端编辑/用量徽标的桌面与移动端视觉人工检查待用户执行；`recordDenied` 的角色列读取失败时回退 default/10000（不阻塞拒绝留痕）。

## CI 验证（2026-09-12 终态）

- `make check` 退出 0（`.loadout/billing-roles-check.log`）；提交 `052bb99` 推送后经 CI 全量绿：
  run 34671545802（Server -race / Desktop×3 / CLI×3 / SQLite 双方言迁移 / Vitest / Lint / 容器 / E2E 全部通过）。
- 期间两次 CI 抖动均已闭环：PluginProxy 的 Vite deps_temp 清理竞态（并行会话重试修复）、Windows 桌面作业 pnpm verify-deps（workspace yaml 正确修法）。
