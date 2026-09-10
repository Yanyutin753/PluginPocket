# Redis 客户端与运行环境执行记录

范围：internal/cache、config 的 Redis URL/namespace、go.mod/go.sum、Compose、.env.example、CI Redis 服务及任务专用本机 Redis。gateway 缓存策略、应用生命周期、根 Make/harness 与进程 E2E 由主代理和 gateway 代理负责。共享工作区内修改，无提交、推送或生产服务操作。

## 精确版本与本机实例

- Redis Open Source [8.10.1 官方 release](https://github.com/redis/redis/releases/tag/8.10.1)。官方 download.redis.io tar 请求返回 HTTP 403，改从同一官方 GitHub tag archive 下载并编译；没有把 GitHub archive 冒称通过 redis-hashes 中另一份发布 tar 的 SHA256 校验。
- 官方 [go-redis v9.22.0 release](https://github.com/redis/go-redis/releases/tag/v9.22.0) 已核实并通过 `go get github.com/redis/go-redis/v9@v9.22.0` 固定；锁文件同时记录依赖 go.uber.org/atomic v1.11.0。
- 本机源码与配置：`/tmp/loadout-redis810/redis-8.10.1`、`/tmp/loadout-redis810/redis.conf`。编译命令 `make -j4 BUILD_TLS=no MALLOC=libc redis-server redis-cli` 成功，`redis-server --version` 确认 8.10.1。当前实例仅监听 127.0.0.1:52363，maxmemory 64mb、allkeys-lru、无 AOF/RDB 持久化，PING 返回 PONG。
- `/tmp/loadout-redis.env` 权限 0600，包含任务实例的 LOADOUT_REDIS_URL、LOADOUT_TEST_REDIS_URL、LOADOUT_REDIS_NAMESPACE。未修改既有 PostgreSQL 或其他真实服务。

## 最小缓存接口

`Open(rawURL,namespace)` 只解析，不要求 Redis 可用。Get 未命中返回 nil,nil。Get/Set/Publish/Ping 每次操作有 250ms 总超时，并显式限制拨号、读写及池等待时间；官方连接池最多 8 个命令连接，客户端最多持有一个 PubSub 订阅连接。namespace 只允许 1–64 个字母、数字、下划线或连字符，数据 key 固定加 `namespace:data:`，失效 channel 固定为 `namespace:invalidate`。Set 必须有正 TTL。

Redis 操作错误返回固定 ErrUnavailable，URL 解析错误也不包含原始 URL/凭证。调用者据此降级；不在 Redis 中保存账户、资金、会话权限、限流额度或数据库 revision。Watch 使用官方客户端重连与重新订阅能力，ctx 取消或 Close 均停止订阅；不实现自制 Redis 协议。

Config 新增 LOADOUT_REDIS_URL（可空）和 LOADOUT_REDIS_NAMESPACE（默认 loadout）。Compose 与 CI 固定 redis:8.10.1-alpine；Compose 缓存限制 128mb、无持久化、宿主端口只绑定 loopback。应用仅要求 Redis service_started，运行时可按缓存失败降级。配置使用者应为独立部署设置不同 namespace，同一部署所有副本共享一个 namespace。

## TDD 证据

工作目录 server：

```sh
go test ./internal/cache ./internal/config -run 'TestOpen|TestCacheCommand|TestRedisConfiguration' -count=1 -v
```

可编译空壳 RED：无效 Redis URL / namespace 被接受，不可用 Redis Ping 和无响应服务器 Get 返回假成功，Config namespace 默认值为空。实现解析、固定错误、实际官方客户端调用与限时后，同一命令 GREEN。无响应测试使用真实 TCP listener，只验证截止时间，不用假 Redis 协议返回业务成功。

```sh
source /tmp/loadout-redis.env
go test ./internal/cache -run 'TestSharedCache|TestInvalidation' -count=1 -v
```

真实 Redis RED：Set/Publish/Watch 的未实现空壳使第二个客户端读不到共享值，另一实例收不到失效消息。实现真实写入、TTL 与命名空间订阅后同一命令 GREEN。每例随机 namespace，不使用 FLUSHDB；测试缓存键均短 TTL 自然清理。

补充真实连接生命周期回归 `TestWatchReconnectsAfterItsRedisConnectionIsLost`：用唯一 client_name 标识测试订阅，通过 CLIENT LIST 定位该连接后仅 CLIENT KILL ID 这一连接；消息在断连前与重连后均收到，Close 使阻塞 Watch 退出。没有关闭共享 Redis 服务或踢掉其他测试/部署连接。

## 完成前验证

```sh
source /tmp/loadout-redis.env
go test -race ./internal/cache ./internal/config -count=1
go vet ./internal/cache ./internal/config
```

最终 race 通过：cache 1.496s、config 1.015s；vet 通过。覆盖 URL 脱敏、不可用/无响应 Redis、共享读取、TTL、key/channel namespace 隔离、订阅取消/关闭及真实重连。

根目录：

```sh
LOADOUT_DB_PASSWORD=validation-only docker compose --env-file /dev/null config --quiet
```

通过；compose.yaml 与 CI workflow 也通过 PyYAML 解析。尝试使用项目未安装的 prettier 时发现命令不存在，没有新增格式化依赖；Compose/CI 的实际语法由上述检查验证。没有 Docker daemon 权限，因此未宣称本机运行过 Compose 容器或 GitHub CI 服务；本机 Redis 运行验证使用官方源码编译实例。完整 make check / gateway 降级与进程 E2E 由主代理统一执行记录。

主代理最终验收（2026-09-11）：完整 `make check` 退出0，含真实双进程共享缓存/冷启动Redis不可用时业务降级；运行中的开发实例 `/readyz` 返回redis ready，实际loadout_dev失效channel有1个网关订阅。结果和根接线RED/GREEN见 [开发服务与E2E记录](2026-09-10-development-e2e-execution.md)。本机仍为单个Redis/PG，不宣称已部署生产HA。
