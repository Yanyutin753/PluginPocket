# 网关、身份、服务接线与跨进程执行记录

日期：2026-09-10。依循 Superpowers TDD/审查/完成前验证及 Ponytail；真实 PostgreSQL 18.6，每例隔离 schema。不使用浏览器自动化。下述 RED 是实际断言失败；工具路径、依赖与编译准备失败不计 RED。

## 服务与网关 RED → GREEN

- `go test ./internal/config -run TestProductConfigurationValidation -count=1`：错误数据库URL、公开Origin、加密key、管理员配对与bool起初被接受，RED；实现严格配置验证后GREEN。`TestIdentityConfigurationRequiresCompleteCredentials` 同样先复现不完整OAuth/SMTP参数被接受，再验证修复。
- `go test -race ./cmd/loadout-server -run TestConfiguredDatabaseServesProductRoutes -count=1`：配置真实DB后注册仍走旧健康handler，返回405而非201；连接store/app/gateway/readyz与资源清理后GREEN。
- `TestGatewayRequiresToken`：空路由返回404，RED；官方MCP端点加独立Bearer鉴权后401、GREEN。
- `TestOfficialMCPClientAndLedger`：使用官方Go客户端经过实际HTTP初始化、列工具、调用；两次成功耗尽2额度，第三次拒绝，真实余额0、3条用量、总cost2；撤销后下一次请求失败。SQL保留字修正属于测试准备，不算RED。
- `transport_test.go`：不安全URL、缺少必要上游header、跟随重定向、未验证的dial行为先RED；实现HTTPS/解析IP检查、禁redirect、有限HTTP连接和超时后GREEN。`secrets_test.go`：明文/重复密文nonce先RED，AES-GCM随机nonce与错key拒绝后GREEN。
- `TestHTTPUpstreamSchemaNamespaceAndReplacement`：真实官方HTTP上游Schema与命名空间；改配置后缓存曾保留2个session，RED；关闭旧连接后1个、GREEN。后续独立审查复现禁用仍留1个session，正式断言RED；目录刷新退休不再启用的会话后0、GREEN。网络关闭移出mutex属于绿灯重构。
- `TestSlowUpstreamDoesNotHideHealthyTools`：300ms目录上下文被首个慢上游消耗，健康工具缺失，RED；并行上限8、每上游独立2秒上下文后GREEN。
- `TestDisabledCallIsAudited`：禁用工具拒绝后usage_logs为0、RED；统一记录0-cost denied后1条、GREEN。
- `TestRateLimitIsSharedAndAtomic`：两个Gateway实例共享同一PostgreSQL，100并发仅60次通过。此为既有功能集成覆盖，不伪称每项补充断言均经历缺实现RED。
- `TestUpstreamFailuresRefundWithoutReplay`：真实HTTP MCP返回isError与超时，恰好执行2次，不暴露提供方原始错误；余额9不变，两条error/cost0。`TestAllowlistedStdioUpstreamUsesOfficialProtocol` 实际启动本次Go测试二进制作为受控stdio服务，验证禁用默认、允许后命名空间与调用，关闭会话清理子进程。这两项为追加集成验证。

## 身份与指标

- `TestUnconfiguredIdentityDoesNotPretendToWork`：未配置GitHub路由404→明确503；`TestEmailVerificationIsAuthenticatedOneTimeAndExpires`：初始路由404→已登录发信、真实传递链接、一次性验证、重放拒绝。初版测试名称中的expires不等于已单独覆盖每个时钟边界。
- `TestGitHubOAuthBindsStateAndCreatesOneAccount`：start404、RED；本地真实provider HTTP fixture验证PKCE、state绑定、一次性回调、哈希session后GREEN。无外部GitHub请求或真实邮件发送。
- SMTP本地TCP协议fixture：原空实现声称成功、RED；默认拒绝无TLS服务，仅显式回环开发模式能发送，GREEN。
- `TestInitialCreditsConfiguration`：默认0、覆盖42未生效、非法值被接受，RED；0–1e12整数验证、默认1000后GREEN。OAuth同一配置测试曾得到1000且balance_after NULL，RED；与用户名注册共享部署数值且账本记录真实余额后42/42/42、GREEN。
- 指标测试：空实现缺request ID/计数，RED；官方Prometheus低基数labels、Go/进程/池指标与安全headers后GREEN。
- 独立审查追加并修复：匿名OAuth开始请求不受预算限制、过期state不清理；禁用用户邮箱验证虚假200。具体RED/GREEN见store-execution末尾。

## 真实产品旅程

`make test-product` 是标准Go testing + 官方MCP SDK + 子进程，无浏览器自动化；使用构建后的Go/Rust和真实PostgreSQL。准备阶段曾误用旧Rust binary产生unrecognized login，这不计RED。完成构建后发现实际注册余额为0，与文档默认1000冲突；由账号模块先正式赠额测试RED，再同事务修复GREEN。

最终旅程串起：注册 → 令牌 → CLI verify/login → 临时HOME三客户端apply → 官方Go客户端通过Rust stdio bridge调用真实Go网关 → 余额998/两条用量 → 撤销拒绝 → 管理员创建兑换码 → 用户兑换、重复409 → 团队幂等转入10 → 原bridge轮换团队token后调用、余额9 → CLI device码由已登录用户明确approve → 自动领取后个人钱包调用 → remove/logout清理。该扩展旅程已通过（约5.6秒，不含编译）；最终全仓检查结果在product-execution记录。

## 部署验证边界

PostgreSQL 18.6 从官方发布源构建后实际运行；Docker daemon socket当前用户permission denied，因此没有宣称容器镜像已构建或Compose服务已运行。`LOADOUT_DB_PASSWORD=configuration-validation-only docker compose config -q` 返回0，验证配置插值与语法。原生Go/Web生产资源与Tauri deb在本机实际构建；Docker构建留CI执行，远程CI尚未触发。

## 汇合验收补充

- `TestNoDatabaseCannotReportProductReady` 正式RED：无数据库时 `/readyz` 原被SPA回退为200 HTML；增加明确503 `database_unconfigured` 后同测试GREEN。
- config测试在已导出管理员环境下复现失败：测试本想验证缺密码，却继承真实环境另一半配置。测试fixture清理其继承的LOADOUT环境后同命令GREEN；产品读取配置行为不变。Node基建进程测试同时显式清空产品DB/admin环境，避免日常开发配置影响临时健康实例。
- 独立交叉审查还关闭了：团队owner锁等待权限快照、团队token创建跨移除撤销窗口、Reserve钱包锁等待后的成员权限、跨账号Query缓存/一次性令牌残留、跨团队邀请码残留、CLI Codex用户修改保护、多文件写入中断归属恢复。对应正式RED/GREEN分别存store/web/cli执行记录。
