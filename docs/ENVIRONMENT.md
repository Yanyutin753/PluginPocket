# 环境变量参考

以 `server/internal/config/config.go`、`scripts/dev.mjs` 和各消费端源码为准。Go当前读取20个应用变量；完整本地示例见 [`.env.example`](../.env.example)，集群运行时示例见 [`runtime.env.example`](../deploy/kubernetes/runtime.env.example)。下面区分**代码缺省值**与示例中的显式值，空字符串通常等同未配置。

## 加载与优先级

| 入口 | 从哪里读取 | 改动何时生效 |
| --- | --- | --- |
| 根目录 `make up/dev/restart/status/logs/down`、对应pnpm脚本 | Node `--env-file-if-exists=.env`；已导出的进程环境优先 | 后端/开发端口等变更用 `make restart`；up对已启动实例不重建 |
| 单独Go二进制 | 仅进程环境，不自动读.env | 重启进程 |
| `make check` / 独立Go测试 | 进程环境，不自动加载根.env；需要测试DB/Redis URL | 下一次运行 |
| 本地Compose | Shell及根.env用于Compose插值，只有compose.yaml中environment项进入Go | 重新创建容器；`docker compose restart`不会更新容器环境 |
| Kubernetes模板 | Secret `loadout-runtime`通过envFrom注入；Deployment内显式ADDR/WEB_DIR优先 | Secret更新后滚动重启；环境不会热更新 |
| CLI / 桌面 | 启动该进程的环境；不读取仓库.env | 下次进程启动 |

Node .env支持引号，Shell `source`还会进行命令/变量展开。`kubectl create secret --from-env-file`使用字面值，不写 `export`、外层Shell引号或 `$OTHER_VAR`。不要把这三种格式当作相同解析器。环境值不要放进浏览器 `VITE_*`、命令参数、Git、日志或渲染出的公开配置。

## Go应用：基础与依赖

| 变量 | 代码缺省值 / 校验 | 部署规则 |
| --- | --- | --- |
| `LOADOUT_ADDR` | `127.0.0.1:8787`；host:port，端口0–65535 | 同主机副本端口不同；0仅测试。容器/K8s模板固定`0.0.0.0:8787`，由Service/防火墙控制入口 |
| `LOADOUT_WEB_DIR` | `web/dist`，相对进程工作目录 | 镜像工作目录/app，资源在/app/web/dist；K8s固定此绝对路径。裸二进制需携带构建后的Web资源 |
| `LOADOUT_PUBLIC_URL` | 空；HTTP(S) origin，不含账号、查询、片段或非根路径 | 生产填最终HTTPS Origin，所有副本一致。影响CSRF、设备授权链接、OAuth回调和Secure Cookie；不支持部署在URL子路径。示例本地值5173不是生产值 |
| `LOADOUT_DATABASE_URL` | 空进入仅健康/静态基建模式；postgres/postgresql URL | 产品必填；所有副本同一个可写主库入口、库及schema。凭据属于Secret；生产验证TLS证书，例如sslmode=verify-full。不得接异步只读副本 |
| `LOADOUT_REDIS_URL` | 空关闭共享缓存；redis/rediss URL，由官方客户端进一步解析 | 集群建议配置同一Redis主入口；故障直接发现，readyz显示degraded。跨不可信网络用rediss。仅支持单端点，不解析Sentinel服务名或Redis Cluster节点列表 |
| `LOADOUT_REDIS_NAMESPACE` | `loadout`；1–64个字母/数字/下划线/连字符 | 同一DB/schema的副本一致，不同环境/租户部署隔离，例如loadout_prod。不要用冒号或随机Pod名 |
| `LOADOUT_ENCRYPTION_KEY` | 空；非空必须Base64解码为32字节 | 保存/解密上游配置时必需。`openssl rand -base64 32`生成一次；所有副本相同，随备份保管。当前没有双密钥轮换/自动重加密，不能直接换新值滚动上线 |
| `LOADOUT_INITIAL_CREDITS` | `1000`；整数0–1000000000000 | 新用户首次赠额，设0关闭；不修改既有余额。同部署统一策略，生产示例明确设0 |
| `LOADOUT_ADMIN_USERNAME` | 空；与密码同时配置或同时留空 | 可选首次管理员引导；同一账号不存在时创建，已存在时不重置密码或自动提权 |
| `LOADOUT_ADMIN_PASSWORD` | 空；与用户名配对，遵循应用密码规则 | Secret；引导完成可同时移除两个引导变量再滚动重启。更改变量不是管理员改密机制 |

生产需要自签CA时，将可信CA只读挂载进容器，并按PostgreSQL连接参数配置sslrootcert；Redis的TLS验证使用容器/Go信任根，可追加受信任CA bundle（例如标准`SSL_CERT_FILE`）。不要把禁用证书验证当成正式配置。URL用户名/密码中的保留字符必须百分号编码；本地Compose密码推荐十六进制以避免拼接歧义。

## Go应用：上游与外部身份

| 变量 | 代码缺省值 / 校验 | 部署规则 |
| --- | --- | --- |
| `LOADOUT_ALLOW_PRIVATE_UPSTREAMS` | `false`，布尔值 | 默认只允许HTTPS公网HTTP上游；true放宽私网/本地HTTP访问，只有受控网络才启用。各副本保持一致，配合出站网络策略 |
| `LOADOUT_STDIO_COMMANDS` | 空，或名称→绝对可执行路径的JSON对象 | `{}`表示禁用；例如`{"search":"/opt/tools/search"}`。所有副本同白名单/运行时；默认scratch镜像无工具程序，需要专用镜像。不是执行任意管理端命令的开关 |
| `LOADOUT_GITHUB_CLIENT_ID` | 空；与secret配对，启用时PUBLIC_URL必填 | OAuth回调固定为`PUBLIC_URL/api/v1/auth/github/callback`；所有副本一致 |
| `LOADOUT_GITHUB_API` | 空；覆盖 GitHub API 根地址（代理/自托管网关用） | 插件市场同步源；空时使用 `https://api.github.com` |
| `LOADOUT_RATE_TOKEN_PER_MINUTE` | 空；每令牌每分钟调用上限 | 默认 60；网关容量调优/压测时按机器能力放大 |
| `LOADOUT_RATE_USER_PER_DAY` | 空；每用户每日调用上限 | 默认 10000；与上同理，所有副本一致 |
| `LOADOUT_GITHUB_TOKEN` | 空；可选 Bearer 令牌 | 提升插件市场同步的 GitHub 搜索限流额度；绝不能是用户令牌 |
| `LOADOUT_GITHUB_CLIENT_SECRET` | 空；与client ID配对 | Secret；未配置则GitHub登录明确不可用 |
| `LOADOUT_GITHUB_ORG` | 空不限组织；非空1–39个字母/数字/连字符 | 仅约束GitHub登录用户组织资格，不关闭独立用户名密码登录 |
| `LOADOUT_SMTP_ADDRESS` | 空；host:port，与FROM及PUBLIC_URL配套 | 邮箱验证可选。服务使用STARTTLS，常用587；不支持把465隐式TLS地址当作STARTTLS端口 |
| `LOADOUT_SMTP_FROM` | 空；有效邮件地址 | 发件地址必须获SMTP服务授权；配置须与ADDRESS配套 |
| `LOADOUT_SMTP_USERNAME` | 空；与PASSWORD配对 | SMTP认证账号；无认证仅适用于明确允许的服务配置 |
| `LOADOUT_SMTP_PASSWORD` | 空；与USERNAME配对 | Secret；未配置SMTP时邮箱验证明确不可用 |
| `LOADOUT_SMTP_ALLOW_LOCAL_INSECURE` | `false`，布尔值 | true仅对回环IP允许无STARTTLS测试；生产保持false，不是允许任意远端明文SMTP |

外部支付当前只预留接口，沒有可生效的支付商户环境变量。Web/CLI/Tauri使用相同后端API，不添加独立设备类型或副本ID变量。

## 本地Compose与开发启动器

这些不是Go应用参数，不能仅把它们写入Pod Secret后期待应用读取。

| 变量 | 消费者 / 缺省 | 用途 |
| --- | --- | --- |
| `LOADOUT_DEV_PORT` | Vite与dev.mjs，5173 | 本机Web端口；PUBLIC_URL、API_ORIGIN按实际地址匹配 |
| `LOADOUT_API_ORIGIN` | Vite默认http://127.0.0.1:8787；dev.mjs未设置时从ADDR推导 | Vite代理Go地址；与公开PUBLIC_URL不同，不应指回Vite自身 |
| `LOADOUT_RUN_DIR` | dev.mjs，`.loadout` | 开发主管socket/日志及Vite依赖缓存目录；up/down/status/logs必须使用同一个值；并行开发/集成测试用不同目录，避免预构建缓存互相覆盖 |
| `LOADOUT_PORT` | Compose，8787 | 应用发布到主机的回环端口，容器内仍8787 |
| `LOADOUT_DB_PORT` | Compose，5432 | PG发布到主机的回环端口，容器内仍5432 |
| `LOADOUT_DB_PASSWORD` | Compose，必填 | 初始化PG密码，同时插入容器内应用DB URL；只对新数据库卷执行初始化。修改变量不会更改既有PG用户密码 |
| `LOADOUT_REDIS_PORT` | Compose，6379 | Redis发布到主机的回环端口，容器内仍6379 |

Compose固定使用内部`db:5432`与`redis:6379`，不会将根.env里的应用DATABASE_URL/REDIS_URL直接传入容器；这两个URL供源码开发。ADDR由模板固定，WEB_DIR使用镜像缺省；其余18个Go配置均通过environment传入。连接外部生产数据库使用集群模板或独立二进制配置，不以本地Compose的回环端口或默认密码作为生产方案。

## CLI / 桌面与测试

| 变量 | 消费者 | 含义 |
| --- | --- | --- |
| `LOADOUT_CONFIG` | Rust CLI及复用其库的Tauri | 本地凭证文件路径；缺省`$HOME/.loadout/config.json`（Windows用USERPROFILE）。覆盖路径不会改变客户端配置所依赖的用户主目录 |
| `LOADOUT_TOKEN` | 生成的Codex direct模式配置所引用的外部客户端 | 可选Bearer令牌；Go和loadout CLI不会用它自动登录。默认bridge从凭证文件取令牌，无需此变量 |
| `HOME` / `USERPROFILE` | Rust本地目录解析 | 测试使用临时目录；不在服务Pod里设置用户的真实客户端目录 |
| `LOADOUT_TEST_DATABASE_URL` | Go集成、make database-check | 必填测试PG，允许创建/删除私有schema；不得指向生产库 |
| `LOADOUT_TEST_REDIS_URL` | Redis集成、make redis-check | 必填测试Redis；随机namespace隔离，不FLUSHDB；连接重连测试需要对自己连接执行CLIENT LIST/KILL，推荐独立测试实例 |
| `LOADOUT_SERVER_BINARY` | Go进程E2E | 构建后Go可执行文件绝对路径，由Make传入 |
| `LOADOUT_CLI_BINARY` | Go产品/bridge E2E | Rust release绝对路径，由Make传入 |
| `LOADOUT_DESKTOP_BINARY` | Go产品桌面bridge E2E | 原生桌面release绝对路径，由Make传入 |
| `LOADOUT_DESKTOP_TEST_BIN` | Rust桌面native_bridge测试 | 可选覆盖待测二进制，例如临时解包的deb；通常无需设置 |
| `LOADOUT_WEB_E2E` | Go产品fixture | 仅值1启动Web E2E，make test-e2e自动设置 |
| `LOADOUT_E2E_ORIGIN` | Web E2E配置 | fixture生成的临时真实HTTP地址；普通web test不需要 |
| `LOADOUT_E2E_ADMIN_USERNAME` / `LOADOUT_E2E_ADMIN_PASSWORD` | Web E2E | fixture创建的临时管理员；不要填写实际生产管理员 |

本地 `.env.example` 中测试二进制/CLI项以注释列出，避免覆盖默认路径或把测试开关带入日常运行。CI和根Make入口负责注入内部测试变量。

## 固定预算，不是环境开关

| 项目 | 当前值 / 作用域 |
| --- | --- |
| PG池 | 每Go副本最多20连接；connect timeout 5秒、statement_timeout 15秒，代码覆盖同名URL参数 |
| Redis池 | 每Go副本最多8命令连接和1订阅连接；命令总预算250毫秒 |
| HTTP上游SDK会话 | 每副本每provider最多32个，新旧配置共用上限；不等同供应商集群配额 |
| 目录 | 按ID每批128行遍历全部启用配置，并发发现8；每上游2秒、目录构建5秒，整体超时不发布截断目录；两层各5秒缓存，纯远端变更最坏接近10秒 |
| 网关调用 | 上游默认30秒；成功/失败结算独立5秒；不自动重放外部调用 |
| HTTP服务 | 读头5秒、读请求15秒、写响应40秒、空闲60秒、请求头最大1MiB |
| 进程终止 | 收到SIGTERM后HTTP Shutdown等待5秒；增加容器grace本身不会改变它 |
| 启动/恢复 | 打开DB并迁移预算15秒；恢复每分钟执行，PG时钟判定超过5分钟pending，执行预算10秒 |

这些是当前实现的容量与超时契约；`LOADOUT_DB_POOL_SIZE`、`LOADOUT_SHUTDOWN_TIMEOUT`等未实现变量不会生效。调整先更改实现与测试，不能只在环境文件中添加名称。滚动升级如何容纳这些预算见 [集群部署规范](CLUSTER.md)。

## 管理员系统配置与热更新

`/admin/settings` 支持注册赠送额度、GitHub登录开关/Client ID/Secret/组织限制、SMTP开关/地址/发件人/用户名/密码。对应环境变量只在数据库尚未保存系统配置时提供默认值；保存后以PostgreSQL记录为准，重启和其他服务副本均读取相同记录。页面更新不会改写 `.env`、进程环境或现有用户余额。

生产服务更新代码仍需重新构建并重启；本地根目录 `pnpm run dev` / `make up` 通过 Air 自动编译重启 Go。修改 `.env` 仍需重启整个开发任务（后台实例使用 `make restart`）。运行新版本后，通过系统配置页保存的值才会在后续请求热生效。

新请求使用最新配置；已开始的请求保持其快照。配置读取失败返回可重试错误，不静默回退。GitHub与邮件启用需要已有公开访问源；密钥加密持久化使用LOADOUT_ENCRYPTION_KEY，网页从不回显原文。未配置加密主密钥时只能保存不包含秘密的配置。

数据库/Redis连接、命名空间、监听/公开地址、加密主密钥、bootstrap管理员、stdio执行白名单、允许私网上游与允许SMTP本地明文仍为部署参数，不在网页可编辑，修改须重启。不要通过页面放宽部署信任边界。

## 桌面发行专用配置

GitHub Actions Secrets：`TAURI_SIGNING_PRIVATE_KEY`（独立 Loadout Minisign 私钥或本地私钥路径）、`TAURI_SIGNING_PRIVATE_KEY_PASSWORD`（可选密码）。仅用于 `desktop/tauri.release.conf.json` 发行构建，不传入服务端、前端或安装包；公钥随仓库版本管理。详见 [桌面发行](../desktop/README.md#桌面发行)。

浏览器 AT/RT 秒数在管理端系统配置中热更新（默认900/604800），不是进程环境变量；各副本从共享PG读取，旧RT期限不变。修改DB/Redis地址、PUBLIC_URL与加密主密钥仍需重启/滚动副本。
