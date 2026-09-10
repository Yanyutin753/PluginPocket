# 产品实现与验收账本

2026-09-10。范围来自完整产品设计与用户确认：响应式 Web + 文档中的 Tauri；兑换码/管理员充值先行，外部支付只保留接口。原生手机 App、供应商市场、私人第三方 OAuth 代管不在本轮。完整检查的最终结果在文末更新。

## 已实现且有行为证据

| 功能 | 实现 | 主要验证与记录 |
|---|---|---|
| PostgreSQL迁移、钱包、不可变账本、恢复 | `server/internal/store` | 真DB并发防透支、幂等退款/调账、重开连接恢复、账本触发器；store-execution |
| 注册、密码、Cookie会话、独立网关令牌 | `server/internal/auth`、`app/account.go` | 初始赠额、对象隔离、注销/撤销/禁用、密码工作量上限、真实verify |
| 官方MCP网关、HTTP/受控stdio | `server/internal/gateway` | 官方客户端、真实HTTP和stdio子进程、schema/命名空间、禁用/变更释放、慢上游隔离、失败退款不重放 |
| 管理用户、工具、价格、额度、审计 | `app/admin.go`、`billing.go`、`reports.go` | 未登录/越权/坏参数、秘密加密不回传、一次性凭证、过滤/分页/CSV公式保护 |
| 套餐、兑换、充值流水、支付边界 | `app/billing.go` | 同码并发仅一人到账、幂等调账，外部支付明确503且不制造订单 |
| 团队、邀请、席位、owner、共享钱包 | `app/teams.go`、store | 同事务转入、一次性邀请、owner转让/成员移除/令牌创建锁等待竞态回归 |
| device-code、邮箱/GitHub | `app/devices.go`、`identity`、`cli/src/device.rs` | 真实设备批准链路；本地SMTP/OAuth HTTP fixture、PKCE/state/一次性/过期/禁用保护 |
| Rust CLI / 三客户端配置 / bridge | `cli` | 25项黑盒测试；临时HOME、0600、冲突/用户改动保护、部分写入恢复、stdout纯协议、凭证轮换；cli-execution |
| 中文深色响应式用户/运营/团队Web | `web` | 55项DOM行为测试；TypeScript/Biome/build；跨账号秘密/缓存与跨团队状态隔离；web-execution |
| Tauri本地UI、有限invoke、托盘 | `desktop` | UI5项、Rust commands与native bridge、Linux deb及解包二进制；desktop-execution |
| 可观测性、部署、API、harness | `observability`、Makefile、CI、Compose | request ID/Prometheus、readiness、OpenAPI校验、真实跨进程旅程、Compose语法校验 |

## TDD 与独立审查

真实 RED/GREEN、工具准备错误与追加集成验证分别记录，未把所有首次通过的补充测试伪称RED。细节：

- [数据库/运营记录](2026-09-10-store-execution.md)
- [网关/身份/跨进程记录](2026-09-10-gateway-execution.md)
- [CLI记录](2026-09-10-cli-execution.md)
- [Web记录](2026-09-10-web-execution.md)
- [桌面记录](2026-09-10-desktop-execution.md)

跨模块独立审查实证并关闭：旧owner锁等待权限、团队token跨成员移除窗口、钱包锁等待成员权限、OAuth无界状态、禁用邮箱虚假成功、上游连接退休、禁用调用漏审计、跨账号旧令牌/Query缓存、跨团队邀请码、Codex用户修改保护和多文件中断恢复。评审用临时overlay复现的缺陷随后均转为仓库正式回归；修复后由原评审者独立复测。

## 真实调用链

`make test-product`：PostgreSQL + 刚构建的Go进程 + Rust CLI + 官方Go MCP客户端。完成注册/令牌/临时HOME配置/bridge调用/余额账本/撤销，再覆盖兑换与重放、团队幂等转入与凭证轮换、CLI设备码与用户明确批准，最后remove/logout清理。真实旅程约5.6秒（不含编译）。不读取或修改开发者真实客户端配置。

## 性能证据

Go基准为真实PostgreSQL、同一钱包 reserve+finish(true)，不是内存mock。环境Linux amd64、AMD Ryzen 7 9700X、Go1.27.1、GOMAXPROCS16、PostgreSQL18.6、同步持久化开启，2秒/轮、3轮。事务锁只覆盖数据库写入，不跨上游网络。最终3轮serial为7.25–8.49ms/op、3576–3583B/op、65–66allocs/op；parallel为5.36–5.83ms/op、6930–7426B/op、77–78allocs/op。全部调用成功。共享开发主机，期间有构建/检查进程竞争资源，不视为独占机器容量基准；并行ns/op是整体吞吐折算，不能当作单请求p95或公网SLA。

报表针对大量历史与其他用户当期记录检查实际HTTP使用的SQL及EXPLAIN ANALYZE BUFFERS，统计结果必须保持UTC边界一致。新增011迁移管理3个时间索引与2个联合MCV统计，绑定确定UTC边界。暖缓存概览1771→3页、团队1780→5页、全局430→3页；执行计划前后数据见store-execution，避免只看小样本API耗时。前端使用路由分包，所有明细/CSV均有界分页，不全量加载日志。

## 完整验证状态

最终独立审查修复与报表优化后，本机 `make check` **退出0，全部通过**；`make benchmark` **退出0，3轮serial+parallel均成功**。原始输出保存在仓库忽略的 `build/product-harness.log`、`build/product-benchmark.txt`。该入口强制真实测试数据库，包含四端静态检查、行为测试、生产构建、Linux deb、产品调用链、4个进程测试、5个后台生命周期测试、4个HTTP/CLI测试；任一步失败立即停止。

## 明确未验证的交付环境

- 不使用Playwright或任何浏览器自动化。DOM测试、CSS审查和构建不等于手机/平板真实排版或桌面托盘人工验收。
- Tauri Linux deb实际构建并解包执行bridge；macOS/Windows桌面、签名发行与商店没有在本机验证。CLI跨平台CI已配置，远程运行未触发。
- Docker daemon socket权限拒绝，容器镜像未在本机构建、Compose未实际启动；Compose配置校验通过，Go/Web原生产产物已构建，CI包含容器构建。
- GitHub/SMTP使用本地真实协议fixture；外部服务需自己的客户端/邮件配置。正式支付按用户确认排除，预留接口明确不可用。
- 未提交、推送或部署本轮产品修改。此前用户授权的基建提交为6481c6a；当前工作区保留所有后续变更。

补充验证：OpenAPI spec validator0.9.0返回OK；Compose `config -q`退出0；`git diff --check`通过。最终Linux deb再次临时解包，使用包内二进制运行native_bridge通过；未安装到系统或操作真实客户端。
