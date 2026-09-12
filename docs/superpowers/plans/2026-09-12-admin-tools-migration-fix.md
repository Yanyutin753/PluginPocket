# 管理员工具列表 500：031 旧库补偿迁移

## 根因与方案

2026-09-12 实际开发 PostgreSQL 已记录 031_billing_roles.sql，但该迁移执行后才追加 tools.allowed_roles，数据库缺列（SQLSTATE 42703）。迁移按名称跳过已执行文件，重启不能补列。新增 032 补偿迁移：缺列时以非空空数组补齐，已有列及限制原样保留。不改历史迁移，不重置数据。

SQLite 没有运行中服务受此旧 PG 状态影响，031 已定义该列；对应 032 为显式无操作，保持轨道编号与最终结构一致，不重建工具表。测试覆盖 SQLite 031 升级、角色限制保留和重复迁移。

## 验收与执行

- RED：加载 .env 后，`cd server && go test ./internal/store -run "TestToolAllowedRolesUpgrade|TestSQLiteToolAllowedRolesUpgrade" -count=1`。original_031 用例失败：upgraded tools must be readable / column allowed_roles does not exist (42703)；已有列和 SQLite 用例通过。
- GREEN / 回归 / make check：待运行。
- 验收：旧库升级后工具可读取且名称/价格保留；完整 031 已有 vip 限制不变；重复迁移安全；真实开发 API 不再返回 500。

- GREEN：`cd server && go test -race ./internal/store -run "TestToolAllowedRolesUpgrade|TestSQLite" -count=1` 通过（25.005s），包含双方言升级/重复执行及 SQLite 全轨道回归。
- 真实服务：Air 观察到 032 后自动重建启动；使用 .env 管理员登录，经 Vite `http://127.0.0.1:5173` 请求 `/api/v1/admin/tools?limit=10&cursor=` 与 `/api/v1/tools?limit=10&cursor=`，均 HTTP 200，各 8 个工具，allowed_roles 均为数组。凭证及工具内容未输出。未使用浏览器自动化。
- 独立静态代码审查：未发现具体正确性问题。
- REFACTOR：最小 SQL 补偿，无额外生产代码重构。
- 相关回归：`cd server && go test -race ./internal/store ./internal/app -count=1` 通过（store 32.834s、app 167.344s）。
