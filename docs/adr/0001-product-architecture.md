# ADR 0001：完整产品架构与数据库

日期：2026-09-10。状态：按用户完整实现授权执行。用户确认响应式 Web + Tauri 桌面，不开发原生手机 App；先兑换码/管理员充值，外部支付保留接口。

## 决策

采用模块化 Go 单体、PostgreSQL 18.6、pgx v5.11.0 连接池、官方 MCP Go SDK v1.7.0。React/Vite 深色控制台以响应式布局服务手机、平板、桌面；Rust CLI 与 Tauri 复用本地接入逻辑。功能边界在进程内清楚划分，数据库是余额和授权的唯一事实来源。

## 数据库比较

| 方案 | 适用与代价 | 决策 |
|---|---|---|
| PostgreSQL | 事务、行级并发、约束、索引、JSONB；支持共享钱包与多进程网关；需独立数据库服务 | 从业务第一天采用 |
| SQLite WAL | 单机部署简单；并发写入串行化，团队记账与多实例需要后续迁移 | 不维护第二套 SQL 方言 |
| MySQL/InnoDB | 同样可实现核心事务；本项目无既有 MySQL 约束，增加双库支持没有产品收益 | 不增加兼容层 |

PostgreSQL 当前稳定版为 18.6，19 为 beta。版本必须固定；pgx 直接执行参数化 SQL，余额关键事务清楚可审计，不用 ORM 隐藏锁和事务行为。迁移嵌入 Go 二进制、数据库 advisory lock 串行执行；失败回滚，已应用迁移有记录。

## 性能与一致性

- 连接池有上限，查询继承请求取消和超时；网络调用期间不持有数据库事务或锁。
- 成功计费改为：事务中条件扣减可用余额并记录 pending → 提交事务 → 执行工具 → 事务中完成账单，失败原路退回；账本与余额更新必须同事务。并发测试证明不会透支，失败和重复完成不会多退/多扣。
- 中途崩溃的 pending 有期限和恢复记录，不能悄悄当作成功；上游非幂等工具不自动重放，外部副作用不能宣称 exactly-once。
- token/session 以哈希存库，撤销与成员权限每次鉴权检查。工具定义短 TTL 快照，后台变更主动失效；上游 HTTP 连接复用、限量、超时、配置变化后关闭旧连接。
- 明细使用稳定的 ID 游标分页（默认 50，上限 100），使用 user_id/id、wallet_id/id、工具/时间索引；不加载全量日志到内存。共享钱包只串行该钱包的短事务。
- 静态资源内容哈希长缓存，HTML 重新验证；业务页面路由分包。手机操作不依赖 hover，表单与表格在窄屏保持可操作。
- Go benchmark 与真实 PostgreSQL 并发测试记录硬件、并发度、错误率、延迟和分配；不承诺未经实测的 QPS。指标不得包含 token、用户名等高基数标签。

## 会话与安全边界

网页采用随机 opaque session + HttpOnly/SameSite cookie，服务端可即时注销；生产 Secure cookie，敏感写请求校验同源。保持网页会话与 ldt_ 网关令牌分离。密码使用 Go 官方 x/crypto 的 scrypt；上游凭证在写入时 AES-GCM 加密，密钥由部署环境提供，管理列表不回传秘密。

服务端 HTTP 上游地址限制 HTTPS；回环/私网只有显式开发配置可用，连接时校验解析地址且禁止重定向，避免 DNS 重绑定绕过。stdio 上游默认禁止，开启时只允许部署配置内的命令白名单。公共 HTTP 接口不能任意启动本地程序。

## 多端

320–767px：单列、可展开导航、可触控操作；768–1199px：紧凑导航和双列；1200px 以上：完整侧栏与数据表。Web 保持完整功能，不按设备隐藏业务。Tauri 使用打包的本地 UI，通过有限 Rust commands 调用共享本地库，远程网页不能获得 shell/文件系统能力。CLI 配置写入始终使用临时 HOME 测试，不能修改开发者真实客户端配置。

## 官方依据（本次查验）

- PostgreSQL 发布：https://www.postgresql.org/
- 并发控制：https://www.postgresql.org/docs/18/mvcc.html
- pgx pool：https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
- MCP SDK：https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0

此 ADR 替代原 PLAN 的 SQLite→PostgreSQL、JWT 网页会话、成功后再抢扣额度与明文上游凭证方案。
