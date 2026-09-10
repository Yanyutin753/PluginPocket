# 2026-09-11 系统配置 API 与注册额度热更新

设计依据：[runtime-settings.md](../specs/2026-09-11-runtime-settings.md)。用户已授权本轮实现，当前工作区修改，不提交/推送/改写真实环境配置。复用项目鉴权、同源保护、JSON解码和 settings.Manager，不新增依赖。

## 实现与验收映射

- `GET /api/v1/admin/settings`：管理员 Cookie 读取；响应公开配置、revision、两个 secret 是否已设置、secret_writes_available；不回传secret原文或对应原文字段。
- `PATCH /api/v1/admin/settings`：revision和八个公开配置字段完整提交，两个secret省略保留、空字符串清除；Validate合并后的配置，启用集成时要求字段完整。
- 写事务按 advisory LockID → Read → 版本比对 → merge/Validate → Write → 最终管理员会话/账号/角色重新验证 → Commit 顺序执行。最终授权检查位于 updated_by 外键可能等待的写入之后。
- 无Runtime旧实例返回503 settings_unavailable；版本冲突409 settings_conflict、非法输入400 invalid_request、不可加密503 settings_encryption_unavailable，其余settings读写故障503 temporarily_unavailable。共享currentUserQuery的鉴权数据库错误沿用既有500行为，未改变其他接口。
- 密码注册在参数校验后、Hash/Begin之前读取一次Runtime快照，从中获取InitialCredits；Manager缺失沿用原Options；读取失败返回503且不创建用户。提前读取避免持有数据库事务后再向同池借第二条连接。
- 配置存储、加密、校验、迁移以及identity/应用装配由并行任务实现；本任务验证API对真实Manager和PostgreSQL的消费行为，不使用stub成功作为验收。

## RED / GREEN / REFACTOR

所有数据库命令仅从根.env加载LOADOUT_TEST_DATABASE_URL、LOADOUT_TEST_REDIS_URL到测试进程，不打印秘密。沿用setup每个测试新建唯一schema且只删除自己创建名称的隔离方式。

1. 在添加路由/处理器和注册读取逻辑之前，新增真实行为测试：

```sh
cd server
go test ./internal/app -run 'TestRuntimeSettingsPersistAcrossHandlersAndChangeRegistrationCredits|TestRegistrationReadsRuntimeDefaults|TestRuntimeSettingsRequireAdministratorAndOrigin' -count=1 -v -timeout=60s
```

RED退出1：系统配置路由404；Runtime默认7额度的新用户实际获1000；匿名路由404而非401。日志 `/tmp/loadout-runtime-api-red.log`。底层Manager的stub随后已由负责者替换，并提供独立真实PG RED/GREEN证据。

2. 完成最小实现后：

```sh
go test ./internal/app -run 'TestRuntimeSettings|TestRegistrationReadsRuntimeDefaults' -count=1 -v -timeout=90s
```

GREEN退出0，日志 `/tmp/loadout-runtime-api-green.log`。覆盖两handler/新Manager实例读取持久化配置、新注册实际余额43、旧revision冲突、secret省略保留/显式清除/响应不回显、匿名/普通用户/异源/过期会话拒绝、非法输入、无加密主密钥时拒绝secret但允许无secret写入、GET与事务PATCH读取故障无fallback、写锁等待期间停用管理员必须回滚。

3. 绿灯下加强会话等待回归：advisory等待期间注销/过期，updated_by外键等待期间禁用/降级，共四种情况，均拒绝提交且revision仍0。重复上面的聚焦命令退出0，日志 `/tmp/loadout-runtime-api-refactor.log`。

4. 代码复核识别注册二次借连接风险。使用真实MaxConns=1连接池新增行为测试，在保留旧顺序（Begin/INSERT user后Read(nil)）时运行：

```sh
go test ./internal/app -run TestRuntimeRegistrationUsesSingleDatabaseConnection -count=1 -v -timeout=30s
```

RED退出1：两秒请求期限耗尽，注册返回503 temporarily_unavailable。日志 `/tmp/loadout-runtime-pool-red.log`。随后把配置快照读取整体提前Hash/Begin之前，同一命令GREEN退出0，注册返回201且balance=7，0.30s完成。日志 `/tmp/loadout-runtime-pool-green.log`。

5. 使用仓库固定工具整理本任务文件，再运行相关回归和静态检查：

```sh
../build/tools/golangci-lint-2.13.2/golangci-lint fmt internal/app/settings.go internal/app/settings_test.go internal/app/app.go internal/app/account.go
go test -race -count=1 ./internal/app
../build/tools/golangci-lint-2.13.2/golangci-lint run ./internal/app/...
```

相关回归退出0：`internal/app` 110.561s，日志 `/tmp/loadout-runtime-api-regression.log`。固定版本golangci检查退出0，0 issues。完整make check、前端组件与人工验证由主任务汇总；本记录不把本地数据库/HTTP handler测试描述为生产部署或外部GitHub/SMTP验证。
