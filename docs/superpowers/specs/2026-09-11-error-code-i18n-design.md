# 错误码标准化 + 全站 i18n 契约设计（2026-09-11）

## 背景与现状

- 前端 i18n 体系成熟（zh 键 = 中文串，`web/src/i18n/*.ts` 英文字典，`t()` 渲染），覆盖率 ~99%。真实缺口：api.ts 错误码→文案映射中 `invalid_settlement` 缺英文；`invalid_settlement_script`、`seat_limit` 为后端从不发射的幻影码；页脚标语 "Your AI, fully loaded."（×4）与 OverviewPage 装饰句硬编码；43 个字典重复键。
- 后端错误响应已是事实契约 `{"error":"snake_case_code"}`：app 包 `fail()` 331 处、identity `failure()` 45 处、httpapi 路由级 3 处，全部内联字符串字面量，无注册表、无完整性保证。网关 `/mcp` 4 处与 git 路由 2 处为纯文本错误。openapi.json 已承诺 `{"error":"stable_code"}`。
- 前端 `ApiError` 解析 `{error: code}` 后经硬编码中文 messages 映射，再靠 `t()` 转 en——双跳间接、按码覆盖无测试保证。

## 决策

**单一错误码注册表 + 机器契约 + 前端按码双语字典。** 错误码是钱的路径之外的稳定 wire 契约，后端是唯一事实源，前端字典按契约测试锁定覆盖。

### 1. 后端注册表（`server/internal/httpapi/codes.go`）

不新建包——httpapi 已是共享 HTTP 助手模块（cmd 已导入）。新增：

- `Fail(w, status, code)`：唯一 JSON 错误写出器（`{"error":code}`、`application/json`、`no-store`）。app.`fail`、identity.`failure` 改为薄封装；gateway 4 处 `http.Error` 与 cmd git 路由 2 处纯文本全部改走 `Fail`。
- `statusByCode`：码 → 允许的 HTTP 状态集合。多状态码如实登记（`upstream_unavailable` 502/503、`email_unavailable` 409/503）。
- `Codes()`：排序码表，用于契约发射。
- 网关/git 路由统一后新增码：`gateway_unavailable`、`method_not_allowed`、`marketplace_unavailable`；`/readyz` 保持 K8s 惯例 `{"status":"unavailable"}` 不变。

### 2. 强制门（防漂移，零调用点改动）

- **源扫描测试**（httpapi 包）：正则提取 app/identity/httpapi/gateway/cmd 全部 `fail|failure|Fail(w, N, "code")` 与 `http.Error(w, "code", N)` 字面量，断言码已注册且状态合法。码表不齐或拼错即 RED。
- **契约金样测试**：注册表序列化（`[{code, status:[…]}]` 按码排序）== `web/src/i18n/error-codes.json`；`go test … -update` 再生成。

### 3. 前端（`web/src/i18n/errors.ts` + `error-codes.json`）

- `error-codes.json`：后端契约（生成物，双端唯一共享文件）。
- `errors.ts`：`Record<code, {zh, en}>`，覆盖全部契约码 + 前端本地码 `unknown`、`invalid_json`；`errorText(code, locale)` 未知码回退双语通用文案。删除幻影码 `invalid_settlement_script`、`seat_limit`；api.ts 旧 messages 映射移除。
- `ErrorNotice` 改为按 `error.code` 渲染 `errorText(code, locale)`（单跳、按码可测）；`ApiError.message` 保留 zh 兼容既有消费者。
- 覆盖测试（vitest）：契约码 ⊆ 字典且 zh/en 非空。

### 4. i18n 缺口填补

- 页脚标语 ×4 与 OverviewPage 装饰句进 `t()`（zh 键 + shell 字典 en 词条）。
- 43 个字典重复键同值合并（仅无歧义项）。

### 5. 文档同步

- `docs/API.md`：完整错误码表（码/状态/含义），移除"可能为纯文本"表述（统一 JSON 后仅 MCP JSON-RPC 协议层错误另有形状）。
- `docs/openapi.json`：Error schema 增加 enum 全码表。
- `docs/PLAN.md`：API 表补注册表与契约文件入口。
- `AGENTS.md`：模块地图 HTTP API 与 Web 控制台行各补一句。

## 否决的备选

- **类型化常量改 376 个调用点**：编译期强制同等保证，但巨型机械 diff 与在途工作区修改冲突；源扫描测试以一个测试文件达到同等 wire 保证。
- **错误体增加 `message` 字段**：服务端文案与前端字典双事实源，违背"码为准"的 i18n 契约；保持 `{error}` 单字段。
- **新建 `internal/errcode` 包**：httpapi 已是共享 HTTP 助手的家，无需新模块。

## 测试计划（TDD）

后端：注册表完整性测试 → 源扫描测试 → 契约金样测试 → 网关错误 envelope 测试（先断言 JSON 体，RED 后改 gateway）。前端：errors 覆盖测试 → Localization 扩展（en 错误码渲染、unknown 回退）→ 受影响字面断言测试更新。最终 `make check`（DB 不可用项明示）。
