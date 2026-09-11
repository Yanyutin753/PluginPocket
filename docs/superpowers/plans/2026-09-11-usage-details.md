# 用量输入输出详情执行记录

目标：个人、团队与管理员用量列表按需查看单次调用传入参数、返回内容。

设计：复用 SidePanel；详情独立 GET，继承原列表权限，列表和 CSV 不携带载荷。usage_logs 新增 nullable 输入/输出文本及截断标记，PG/SQLite 同步迁移。完成调用时与结算原子保存载荷，拒绝调用保存可用数据；历史/恢复记录缺失时明确“未记录”。用户明确要求输入输出不脱敏，原内容保存；每方向最多 64 KiB，按 UTF-8 边界截断；不额外采集 HTTP 头或配置凭据。

步骤：
- [x] 服务端行为 RED：详情权限、网关输入输出、SQLite 迁移。
- [x] 最小迁移、采集及 GET 实现并 GREEN。
- [x] 页面行为 RED：按需打开、键盘关闭、未记录、失败重试；实现详情面板和双语。
- [x] 聚焦回归、审查和 make check，记录结果及未验证范围。

记录边界：input_data 是网关收到的工具参数；output_data 是网关返回客户端的 MCP 结果（保留既有错误摘要和结算注记），不是原始 HTTP 报文。两者不额外脱敏，前端直接渲染字符串以保留大整数、转义和原始内容。

验证证据：
- RED：`cd server && go test ./internal/app -run TestUsageDetailAuthorizationAndLegacyData -count=1`，本人详情 404，预期 200。
- RED：`cd server && go test ./internal/gateway -run TestGatewayRecordsOriginalInputOutput -count=1`，成功/错误/超长/余额不足记录均没有载荷。
- RED：`cd server && go test ./internal/store -run TestSQLiteUsageDataRoundTrip -count=1`，迁移缺少 input_data。首次 fixture 漏 status 的失败不作为 RED。
- RED：`pnpm --dir web exec vitest run src/UsageDetails.test.tsx`，缺少“查看详情”按钮。环境 PATH 引号错误不作为 RED。
- GREEN：服务端上述三项聚焦联合命令及 `-race` 均退出 0；扩充本人原文响应、列表不携带载荷、团队可见与非成员拒绝验证后通过。
- 界面 GREEN：UsageDetails、ExportFilterReset、SidePanel 三个文件 7 项通过；`pnpm --dir web typecheck` 退出 0。测试文案匹配兼容项目既有中文句末标点归一化。
- 新缺陷 RED/GREEN：JSON.parse 美化会丢失大整数精度；加入 9007199254740993 原文和 HTML 字面量测试确认失败后去掉解析、美化，直接安全文本渲染，3 项用量详情测试通过。
- 独立只读评审：以网关输入/返回结果为记录边界，无可操作缺陷；后续补充团队与并发回归。
- REFACTOR/回归：`go test -race ./internal/store -run TestFinishWithDataConcurrentRefundAndRollback -count=1` 退出 0；数据库触发器令退款失败时，载荷和结算一起回滚；12 个并发完成仅一笔退款，后来的完成不能覆盖结果。
- Impeccable detector 对 UsageDetail.tsx / UsagePage.tsx 返回 `[]`；OpenAPI 0.9.0 验证 `docs/openapi.json: OK`。这些检查不代替实际屏幕视觉验收。

完整验证：通过 WSL 使用固定 Node 26.8.2 工具链，显式加载本地 `.env` 中测试数据库与 Redis 设置后执行 `make check`，退出 0。包括全部 Go race 测试（app 136.693s、gateway 28.824s、store 18.006s）、Web 24 文件 144 项、CLI、桌面 UI/原生 bridge、Linux deb 构建、生产/开发真实 Web 与 MCP 旅程、多进程/重启/故障恢复、开发生命周期和 HTTP 集成。日志 `.loadout/usage-details-check.log`。本地 5173 与 8787 新详情接口未登录访问均返回 401。

`make load-test` 退出 0（30.836s）：真实进程、隔离 schema；32 并发 echo 20 秒共 11765 次、失败 0，587 QPS，p95 79.20ms；目录 18298 次、失败 0。计量校验通过（11766 条成功记录包含预热调用）。这是本机本轮测量，不作为生产容量承诺。日志 `.loadout/usage-details-load.log`。

禁止浏览器自动化；桌面/手机视觉未人工验收，其他系统桌面包未在本轮验证。`git diff --check` 通过；没有提交或推送，保留原有无关修改。
