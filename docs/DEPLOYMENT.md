# 开发、部署与数据恢复

服务端与 Web 共用一个 Go 进程；数据库为 PostgreSQL 18.6。开发使用 Go + Vite，CLI 和 Tauri 按需运行。变量的作用域、缺省值与优先级见 [环境变量参考](ENVIRONMENT.md)，Go 不自动读取文件。`.env` 与 `.pluginpocket/` 不进入 Git 或容器上下文。

## 本地数据库与源码

复制 `.env.example` 为 `.env`，填入自己生成的 `PLUGINPOCKET_DB_PASSWORD`（建议 `openssl rand -hex 24`，保证可直接放入连接 URL），同时替换两个数据库 URL 中的密码。`PLUGINPOCKET_ENCRYPTION_KEY` 使用 `openssl rand -base64 32` 生成并妥善保存；缺失时仍可用内置工具，但不能保存加密 HTTP/stdio 配置。

```sh
make db-up
make redis-up
# 第一次为 harness 创建独立库；业务库 pluginpocket 由 Compose 初始化。
docker compose exec db createdb -U pluginpocket pluginpocket_test
```

根目录的 `make up/dev/restart/status/logs` 和对应 NPM Scripts 自动读取 `.env`，从 IDE 点击启动也适用；已导出的环境变量优先。本地前端使用相对路径 `/api`，Vite转发后端，`PLUGINPOCKET_PUBLIC_URL` 可留空，localhost与127.0.0.1均可同源访问。修改配置后执行 `make restart`。

来源校验使用Go标准库 `http.CrossOriginProtection`：写请求仍必须带Origin；现代浏览器通过Sec-Fetch-Site判断同源，缺少该头时比较Origin与Host（含端口），不依赖固定公开地址，也不信任客户端的X-Forwarded-Host。反代应保留Host、Origin和Sec-Fetch-Site，不能改写跨站来源以绕过校验。生产HTTPS仍配置 `PLUGINPOCKET_PUBLIC_URL` 以启用Secure Cookie并生成正确的公开链接与身份回调。

标准库旧浏览器回退只比较主机和端口，不比较HTTP/HTTPS协议，以兼容TLS终止反代；生产入口应强制HTTPS并配置HSTS。参见 [Go标准库说明](https://pkg.go.dev/net/http#CrossOriginProtection)。

单独运行 Go 二进制或完整 `make check` 时仍需显式导出配置。仅对自己检查过的 shell 兼容配置执行 `set -a; . ./.env; set +a`，值包含空格时必须加引号；也可逐项 export。

```sh
make setup
make up
make status
# 浏览器 http://127.0.0.1:5173
make down
```

已有 PostgreSQL 18 时直接提供连接 URL，不依赖 Docker。测试账号须能创建 schema；每例创建并删除独立 schema，不清空整个数据库。`PLUGINPOCKET_DATABASE_URL` 用于应用，`PLUGINPOCKET_TEST_DATABASE_URL` 仅用于测试。未提供应用数据库时只有健康基建模式，不生成虚构业务响应。

管理员使用 `PLUGINPOCKET_ADMIN_USERNAME` + `PLUGINPOCKET_ADMIN_PASSWORD` 首次引导创建；已存在同名账号不会重置密码或被自动提权。`PLUGINPOCKET_INITIAL_CREDITS` 默认1000，设0可禁用新用户赠额；用户名与 GitHub 注册遵循同一规则，既有账户不补发。

## Linux 桌面依赖与验证

需要本机 C/C++ 工具链与下面的开发库（Ubuntu/Debian 包名）：

```sh
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev librsvg2-dev patchelf
make check
make benchmark
```

`make check` 要求 `PLUGINPOCKET_TEST_DATABASE_URL` 和 `PLUGINPOCKET_TEST_REDIS_URL`，包含真实 PostgreSQL / Redis 集成和 Linux Tauri deb 构建；缺少测试数据库时直接失败。CLI release 在 `cli/target/release/pluginpocket`，桌面安装包在 `desktop/target/release/bundle/deb/`。无 DISPLAY 的原生 bridge 测试不代表 GUI、托盘、签名或其他操作系统验收完成。

## 容器与生产配置

设置 `.env` 的 `PLUGINPOCKET_PUBLIC_URL` 为最终公开 HTTPS Origin（本地单容器预览用 `http://localhost:8787`），然后 `make docker-up`。Compose 绑定主机回环端口；外部访问经自己的 HTTPS 反向代理。服务使用非 root、只读根文件系统，数据库写入独立持久卷。

PostgreSQL 18 官方镜像的卷挂载点是 `/var/lib/postgresql`，见 [官方镜像说明](https://hub.docker.com/_/postgres)。`make docker-down` 保留卷，不要在升级时使用 `down -v`。自动迁移在事务与 advisory lock 下执行；失败阻止产品服务启动，不跳过失败迁移。

生产数据库账号应限于应用库；公网部署使用数据库 TLS、独立凭证、定期备份及自己的资源限额。`/healthz` 是进程存活，`/readyz` 实测数据库连接，并报告Redis的 ready/degraded/disabled 状态；Prometheus `/metrics` 提供低基数 HTTP/Go/连接池指标，反代仅向监控网络开放该路径。日志不记录密码、Cookie、令牌或上游原始错误。上游请求默认30秒，数据库连接池每副本20，HTTP会话每副本每上游最多32（新旧配置共享上限），目录发现并发8、每上游2秒。远端目录TTL5秒；数据库工具变更通过共享revision在每次使用缓存前校验，不等待其他副本TTL到期。

## 多副本部署

正式操作规范、Kubernetes模板、Secret示例、容量/摘流/升级/回滚与故障演练见 [集群部署规范](CLUSTER.md)。本节保留独立进程的最小启动方式。

应用可运行两个以上独立Go进程，负载均衡不需要粘性会话。各实例使用相同 `PLUGINPOCKET_DATABASE_URL`（同一可写主库/HA入口与schema）、`PLUGINPOCKET_PUBLIC_URL`（负载均衡对外Origin）、`PLUGINPOCKET_ENCRYPTION_KEY` 以及OAuth/SMTP配置；仅监听地址各自不同。不要给不同副本生成不同加密密钥，也不要将鉴权和资金请求路由到异步只读库。

例如先在两个终端加载同一套应用环境，再分别运行：

```sh
# 实例 A
PLUGINPOCKET_ADDR=127.0.0.1:8788 ./build/pluginpocket-server
# 实例 B（另一个终端/主机；跨主机时监听受保护的内网地址）
PLUGINPOCKET_ADDR=127.0.0.1:8789 ./build/pluginpocket-server
```

让现有HTTPS负载均衡将流量分配到两个实例，探测 `/readyz`，保留Host/Origin/Cookie/Authorization，允许流式HTTP，超时至少覆盖30秒上游预算与结算；禁用POST和MCP工具调用自动重试。滚动更新先停止向待下线节点分配新流量，再终止该节点。并发迁移和管理员bootstrap可重复执行，不重复创建账号或钱包；过期pending恢复可由多个副本运行，数据库保证只退款一次。

默认 `compose.yaml` 是本地单实例便利配置，包含固定主机端口；不能直接 `--scale pluginpocket=3` 后宣称已获得集群。容器集群应为副本移除固定主机端口，由自己的Service/负载均衡统一暴露入口，并采用独立持久化PostgreSQL/HA服务。连接容量至少按副本数×20评估；HTTP会话/CPU限制是每副本资源限制，集群令牌与账户配额则由PG统一执行。stdio默认关闭，启用后各副本须具备相同受限运行时，或把有共享状态的上游独立部署为HTTP服务。

分布式状态、时钟及故障语义见 [ADR 0002](adr/0002-distributed-backend.md)。`make test-e2e` 包含真实无粘性双进程与节点退出旅程；本机测试未部署PostgreSQL HA，也不证明外部负载均衡或数据库主库切换已经演练。

HTTP 上游仅允许 HTTPS 公网地址，检查 DNS 解析并禁重定向。`PLUGINPOCKET_ALLOW_PRIVATE_UPSTREAMS=true` 仅用于可信开发服务。stdio 默认关闭；`PLUGINPOCKET_STDIO_COMMANDS` 是部署者提供的名称到绝对可执行路径 JSON，例如 `{"search":"/opt/tools/search"}`。管理页面只能引用已有名称。默认 scratch 镜像不携带工具运行时；需 stdio 时由运营构建受限工具镜像，并配置网络、进程和资源隔离。

## 外部身份配置

GitHub OAuth 需要 client ID、secret、公开 Origin；回调地址为 `/api/v1/auth/github/callback`。使用 PKCE、与浏览器绑定的一次性 state，GitHub token 不存库。`PLUGINPOCKET_GITHUB_ORG` 限制该 OAuth 登录的组织成员资格；用户名密码登录仍独立可用，不应把这个选项解释成全站仅组织可访问。

邮件需 `PLUGINPOCKET_SMTP_ADDRESS`、`PLUGINPOCKET_SMTP_FROM`、公开 Origin；认证提供 username/password。默认必须 STARTTLS，`PLUGINPOCKET_SMTP_ALLOW_LOCAL_INSECURE=true` 仅允许回环 IP 测试 SMTP。验证链接30分钟过期、一次性消费。未配置时 Web 明示不可用。

支付按当前范围预留，`POST /account/orders` 返回 `payment_unavailable`；实际充值使用管理员幂等调账和一次性兑换码。正式付款、域名/TLS、真实邮件/OAuth与桌面签名均需自己的配置，不附带演示成功状态。

## 备份与恢复

使用 PostgreSQL 18 客户端；备份保存到受保护目录，另行保管加密密钥。两者缺一会导致上游凭证不可恢复。

```sh
# Shell 需已导出连接 URL；文件名可自行调整。
pg_dump --dbname="$PLUGINPOCKET_DATABASE_URL" --format=custom --file=pluginpocket.backup
# 在新的空数据库执行恢复，先验证，不覆盖运行中的库。
pg_restore --dbname="$PLUGINPOCKET_RESTORE_DATABASE_URL" --no-owner --exit-on-error pluginpocket.backup
```

恢复后对新实例执行 readiness、登录、账本核对与一个可重复的测试工具调用，再决定切换连接。未知结果的非幂等上游不能自动重放；超过5分钟的 pending 由每分钟恢复任务退款并标记 recovered，保留审计记录。

## Redis部署与降级

`make redis-up` 启动固定版本 Redis 8.10.1；配置 `PLUGINPOCKET_REDIS_URL`，同一应用各副本使用相同 `PLUGINPOCKET_REDIS_NAMESPACE`（默认pluginpocket）。不同环境和不同schema使用独立namespace。harness另外设置 `PLUGINPOCKET_TEST_REDIS_URL`，每例生成唯一namespace，不执行FLUSHDB。

Redis用于5秒HTTP上游工具目录缓存和Pub/Sub失效广播；不用它存凭证、权限、余额或会话。配置变更仍以PostgreSQL revision为准。Redis错误在有界超时后降级直接发现，上游调用与PG计费继续；readyz仍为200并返回 `redis: degraded`。未配置Redis返回disabled。

生产使用内网受保护的Redis主入口；跨不可信网络使用 `rediss://` TLS和独立ACL凭证，禁止公开暴露端口。当前客户端支持单端点连接；Redis Cluster分片和Sentinel发现不在本次实现范围，HA由托管服务/稳定主入口提供。缓存可丢弃，无需业务备份；可按目录规模设置maxmemory及allkeys-lru。所有数据恢复仍以PostgreSQL与加密密钥为准。

本地目录缓存与Redis元数据缓存各5秒TTL；上游自行改变工具定义、但未修改数据库配置时，两层叠加最多接近10秒才发现变化。数据库中的启用、定价和连接配置提交仍按revision立即失效，不受这两个TTL限制。
