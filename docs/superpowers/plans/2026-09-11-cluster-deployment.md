# 集群部署规范与环境契约

用户授权补齐集群部署规范、环境变量与文档。沿用既有无粘性Go副本 + PostgreSQL权威状态 + Redis可降级缓存；本轮不部署到任何集群，不修改实际.env或运行中服务。

- [x] 根据Config.Load、开发启动器、CLI/桌面与harness核对全部变量；补全.env.example作用域说明及Compose传参。
- [x] 提供Kubernetes应用层Deployment/Service/PDB、完整运行时环境示例；镜像/公开Origin/数据库/Redis/密钥需部署者填入，使用现有外部PG/Redis主入口。
- [x] 新增集群规范和环境变量参考，覆盖容量、探针、流量/网络、5秒应用关闭限制与preStop摘流、首次部署、扩缩容、发布回滚、密钥更新、备份恢复和演练。
- [x] 同步README/DEPLOYMENT/HARNESS/ADR链接。验证Compose环境渲染、示例配置真实Config.Load、Kubernetes官方schema、文档变量覆盖与链接，运行完整make check。

第三方声明式配置使用真实解析/官方schema验证；没有新增业务行为，不编造TDD RED。若发现并修改自有业务逻辑，先增加能复现的失败测试。保持无浏览器自动化、不提交/推送、不操作生产资源。


## 校验记录

- Config.Load实际读取20个Go运行时变量；本地.env.example、Kubernetes runtime.env.example和ENVIRONMENT参考无遗漏/多余运行时键。客户端与测试变量另列，不注入集群。
- `docker compose --env-file /dev/null config --format json`使用验证专用密码、true布尔值和非空JSON白名单渲染，三个原缺失参数值均保持正确；空白名单使用空默认，避免`${VAR:-{}}`在非空变量时残留花括号。
- `kubectl --kubeconfig=/dev/null --server=http://127.0.0.1:1 create secret ... --dry-run=client -o json`本地解析生产env示例，未访问集群。补入临时合法32字节密钥后，通过临时Go overlay测试运行真实Config.Load，production/compose两例均通过；没有连接示例外部数据库/Redis，也不把占位符示例说成可直接生产使用。
- 从官方kubernetes/kubernetes v1.37.0下载api/openapi-spec/swagger.json，以本机JSON Schema验证Deployment、Service、PodDisruptionBudget全部通过。转换了官方Swagger的format=int-or-string扩展为JSON Schema整数/字符串联合类型；未跳过资源schema。此为离线结构校验，不代替目标server dry-run/准入策略/运行效果。
- README/部署/环境/harness/ADR本地链接存在，git diff --check通过。
- requesting-code-review独立只读审查覆盖六个核心文件及消费代码，未发现阻断或重要问题；准确保留5秒Shutdown、静态assets混版、Secret非原子轮换、PG/Redis外部HA边界。
- 纯文档与第三方声明式配置补全，没有新增业务逻辑；未编造功能TDD RED，原有完整harness另行复跑。

最终 `make check` 在Node26.8.2、真实隔离PostgreSQL/Redis环境下退出0，覆盖四端静态检查/测试/构建、Linux deb、真实多副本/故障恢复/Web生产与开发/CLI/桌面bridge及进程生命周期。完整日志 `build/cluster-deployment-harness.log`（忽略产物）。本轮没有修改实际.env、没有手动重启日常开发实例、没有对集群执行部署或Secret写入、没有提交/推送。
