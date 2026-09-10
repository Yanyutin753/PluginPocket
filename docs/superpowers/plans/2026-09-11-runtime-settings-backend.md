# 运行时系统配置底层与 Identity 接入

2026-09-11。执行已授权设计 [runtime-settings](../specs/2026-09-11-runtime-settings.md)，按 Ponytail、TDD 与完成前验证完成。本子任务范围为 `settings/**`、迁移014、`identity/**`、`cmd/loadout-server/application.go` 及新增装配行为测试；管理员API和密码注册由另一个子任务实现，前端与完整检查由主任务负责。

## 最小实现与验收映射

- `settings.Manager` 直接从 PostgreSQL 读取单行快照，没有本机缓存。无持久化行时返回环境默认值与revision0；保存后全部采用数据库值。跨manager立即观察更新，版本CAS拒绝陈旧写入。管理员API使用共享 `LockID` 获取初始化/更新锁，提交前重验角色和会话。
- `014_runtime_settings.sql` 保存公开JSON、加密秘密、正整数revision、更新管理员与数据库时间。`Values` 的Client Secret/SMTP Password采用 `json:"-"`，默认JSON编码无法泄漏；秘密单独经现有gateway AES-GCM helper加密后落库。无秘密保存不依赖加密key；非空秘密要求32字节key，错误key返回统一错误，不传出crypto内部错误。没有将秘密写入日志。
- 配置边界校验额度范围、字段长度、启用集成所需公开源和完整参数、SMTP host/port/发件人/配对认证以及GitHub组织名称。部署配置仍保有公开源与本地明文SMTP例外；界面不能放宽明文例外或修改加密主密钥。
- Identity每个HTTP请求先读快照，再构造独立Options与handler；共享HTTP Client保留连接池，既有Options和秘密不被请求间修改。OAuth开关、Client ID/Secret、组织scopes与注册赠额即时生效；SMTP开关/地址/发件人/认证即时生效。数据库读取失败返回503，不回退陈旧默认值。现存邮箱验证链接的消费逻辑保持可用。
- `applicationHandler` 从cfg生成唯一Manager并同时传入app和identity。装配测试通过管理员PATCH，观察同一handler的meta及随后真实注册余额更新，不重启handler。

## RED / GREEN / REFACTOR

仅向测试命令环境传入 `.env` 的 `LOADOUT_TEST_DATABASE_URL` / `LOADOUT_TEST_REDIS_URL`，未打印内容。新增fixture只创建和删除自己记录的唯一schema；本地OAuth和SMTP供应商使用隔离TCP fixture，无真实供应商请求。

1. 底层与校验：先落类型/空实现供协同编译，并写真实行为测试。
   - `go -C server test ./internal/settings -count=1 -v -timeout=60s`
   - **RED** `/tmp/loadout-runtime-settings-red.log`：保存后另一Manager仍返回revision0/默认额度，非法配置全部被接受；均为行为断言失败。
   - 最小迁移、CAS读写、加密与校验后同一命令 **GREEN**，`/tmp/loadout-runtime-settings-green.log`。
2. Identity动态能力：
   - `go -C server test ./internal/identity -run '^TestRuntimeIdentity' -count=1 -v -timeout=60s`
   - **RED** `/tmp/loadout-runtime-identity-red.log`：运行时关闭GitHub仍报告开启，保存SMTP启用未反映在meta。
   - 每请求不可变快照接入后同一命令 **GREEN**，`/tmp/loadout-runtime-identity-green.log`。本地OAuth回调实际创建余额81的账号，本地SMTP实际收到新发件人与一次性验证链接；禁用立即停止新邮件请求。
3. 真实应用装配：
   - `go -C server test ./cmd/loadout-server -run '^TestApplicationReloads' -count=1 -v -timeout=60s`
   - **RED** `/tmp/loadout-runtime-application-red.log`：数据库保存后同一应用仍报告环境GitHub开启。
   - 注入统一Manager后 **GREEN** `/tmp/loadout-runtime-application-green.log`。随后增强为管理员API PATCH保存，秘密不出响应，后续meta关闭与新注册赠额73均通过。
4. 绿灯后整理与相关回归：
   - `go -C server test -race ./internal/settings ./internal/identity -count=1 -v -timeout=120s`：退出0，settings 1.164s、identity 6.343s，`/tmp/loadout-runtime-backend-final.log`。既有会话故障、失败重发/并发、多副本限流/OAuth/邮箱及SMTP TLS边界均通过。
   - `go -C server test -race ./internal/settings ./internal/identity ./cmd/loadout-server -run 'TestSettings|TestRuntimeIdentity|TestApplicationReloads|TestConfiguredDatabase|TestNoDatabase' -count=1 -v -timeout=90s`：退出0，`/tmp/loadout-runtime-backend-focused.log`。
   - `build/tools/golangci-lint-2.13.2/golangci-lint fmt server/internal/settings server/internal/identity server/cmd/loadout-server/application.go server/cmd/loadout-server/runtime_settings_test.go`：退出0。
   - 仓库标准lint最初发现SMTP fixture可改为tagged switch，调整后在server目录运行 `../build/tools/golangci-lint-2.13.2/golangci-lint run ./internal/settings/... ./internal/identity/... ./cmd/loadout-server/...`：退出0，0 issues。
   - 整理后 `go -C server test -race ./internal/identity ./cmd/loadout-server -run 'TestRuntimeIdentity|TestApplicationReloads' -count=1 -v -timeout=60s`：退出0，`/tmp/loadout-runtime-backend-refactor.log`。

## 独立审查协作与边界

接本任务前的app独立审查确认：tokens INSERT的users外键锁等待期间禁用用户，原补丁仍201。临时overlay `/tmp/loadout-late-auth-review-5um8w4l1/overlay.json` 的 `TestIndependentTokenInsertWaitRechecksDisabledUser` 在 `/tmp/loadout-late-auth-review-red.log` RED；app作者在所有受保护事务提交前末次核验后，同一独立测试GREEN（`/tmp/loadout-late-auth-review-green.log`），确认无残留token。没有修改app文件。另提醒注册避免持有事务再向同一满连接池申请配置读取连接，作者已将快照读取提前。

完整 `make check` 由主任务统一执行。未验收真实GitHub、外部SMTP、生产集群或真实桌面/移动端视觉；本地fixture与DOM测试不能代替这些范围。没有提交、推送、重启真实服务、改变客户端配置或写回环境文件。
