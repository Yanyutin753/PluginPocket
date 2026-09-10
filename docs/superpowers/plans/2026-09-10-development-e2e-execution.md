# 开发服务与真实 E2E 修复记录

## 根因与修复

页面账号请求曾得到404的配置根因及本机环境恢复见 [本地环境记录](2026-09-10-local-env-execution.md)。此轮检查确认真实管理员可经5173代理登录、读取账号并注销，数据库readyz返回200；不输出账号密码或Cookie。

另发现 Vite 只代理 `/api` 和 `/healthz`，访问 `/readyz`、`/mcp` 被SPA回退成200 HTML；生产Go分别正确返回实际就绪状态和MCP鉴权错误。开发地址因此不能作为MCP接入地址，且就绪路径误报。

- RED：Node26.8.2执行 `node --test --test-name-pattern='terminal interruption' tests/process.test.mjs`，隔离无业务数据库服务的 `/readyz` 代理返回200，Go直接返回503，断言 `200 !== 503`。
- GREEN：为Vite添加 `/readyz` 与 `/mcp` 代理。同一进程用例通过；`node --test tests/process.test.mjs tests/dev.test.mjs` 10项通过，包括后台重复启动、异常退出清理和端口冲突保护。
- 原生配置变更，无新依赖、无手写MCP消息或浏览器自动化。

## 真实产品测试

新增隔离schema/Go进程fixture和 `resilience_e2e_test.go`。以下用例首次即绿，明确属于补足现有行为覆盖，不是虚构RED：

- Go重启后会话、撤销令牌和账本保持；管理员幂等调账重放不多记账；注销后重放旧Cookie仍401。
- 两个真实Go进程共享一个私有schema，24个并发SDK调用竞争10额度，恰好10次成功、14次拒绝、余额0、账本消费10。
- 上游执行中SIGKILL Go，持久pending经SQL推进超过恢复阈值后重启退款；第二次重启不重复退款，上游只执行一次。
- 管理员配置真实HTTP上游，Rust bridge调用错误退款且不重放；修改单价后按新价格扣费，禁用后不调用上游。
- 原生Tauri二进制bridge经Go/真实数据库成功调用并扣费。

Web测试详见 [Web执行记录](2026-09-10-web-e2e-execution.md)。根fixture扩展为生产Go和真正 `make up` 的Vite开发服务各执行3条DOM→HTTP旅程；另用官方SDK经Vite地址调用MCP并核对真实余额。聚焦命令：

```sh
LOADOUT_SERVER_BINARY=$PWD/build/loadout-server LOADOUT_WEB_E2E=1 \
  go -C server test -race ./cmd/loadout-server \
  -run 'TestProductJourney(DevelopmentMCP|Web)' -count=1 -v
```

已通过，输出记录 `/tmp/loadout-development-e2e.log`。运行前导出隔离测试数据库URL；该临时日志不作为仓库永久测试证据。

## Harness 与独立审查

新增 `make test-e2e` / `pnpm test:e2e`，构建四端并提供所有必需二进制/真实数据库/显式Web标志；`make check` 调用此入口，不能只跑缺少产品环境的单元检查就声称产品通过。

独立只读审查后强化四处测试：保存并重放注销前Cookie；开发fixture显式清空SMTP/GitHub/stdio等配置，避免根 `.env` 注入；Web超时杀本次pnpm进程组并限制管道等待；schema清理失败报告测试错误。测试不停止日常 `.loadout` 服务，不使用真实用户HOME、客户端配置或业务数据。

完整检查仍须在当前所有修改落定后重新运行。第一次完整检查发现本轮新增断言的Biome格式问题，已格式化；第二次发现工作区正在新增的本地化测试尚未格式化，未覆盖该并发工作。聚焦通过不代替最终 `make check`。

审查后动态复验：6项进程/故障旅程通过（`/tmp/loadout-process-e2e-review.log`）；Web在并发国际化改写期间曾两次找不到团队转入成功文案，另一次因字典语法错误未执行任何测试。保留原精确文案与余额断言并添加窄化失败诊断（仅转入面板和余额），源码修复后两种模式6条全绿（`/tmp/loadout-web-e2e-diagnostic.log`）。旧日志DOM被截断，不将该中间态失败推断成后端转账缺陷或确认的测试时序问题。当前前端75项组件测试与typecheck通过；`make test-cli test-desktop lint-desktop` 通过。

日常实例还以现有配置进行了一次管理员只读验证：登录200，账号/用户列表/工具/套餐/兑换码列表/用量/汇总/审计均200 JSON，测试会话注销204。未输出密码、Cookie或业务内容；测试不读取真实客户端配置，也不调整实际用户额度。

未执行真实浏览器布局/桌面GUI、远端SMTP/GitHub供应商、其他操作系统或容器运行；这些不能从组件与HTTP测试推断通过。


## 分布式与Redis追加

- 共享目录版本：真实两个Go进程，A管理员禁用工具后B暖缓存仍广告该工具的RED（`/tmp/loadout-catalog-distribution-red.log`）；数据库revision触发器与每请求校验后GREEN。无粘性轮询LB的会话、团队、设备和节点退出/重入旅程通过（`/tmp/loadout-distributed-green.log`）。
- 恢复时钟：Go synctest把worker时钟固定在2000年、真实PG pending已过期，原进程时间实现回收0条的RED；采用DB statement_timestamp确定阈值后回收1条并退款一次GREEN（`/tmp/loadout-recovery-clock-{red,green}.log`）。
- Redis进程接线：`TestProductJourneyRedisSharedCatalogAndOutage` 在旧二进制实测两个进程发现2次、期望共享后1次RED；application接官方缓存客户端与gateway后，重建相同测试GREEN 2.554s。第二进程重启到不可达Redis端点，readyz200/redis degraded，上游实际执行并在PG扣7。该例验证冷启动降级，PubSub运行中断连/重连由真实Redis客户端测试单独覆盖。
- 本机 `.env` 仅追加Redis URL、独立loadout_dev namespace及测试URL，保持原数据库与凭据；测试进程使用私有schema名称作为Redis namespace。旧CLI旅程改用cleanProductEnv，避免继承实际配置。
- 独立审查发现基础process/dev/integration fixtures继承非法Redis环境会提前配置失败；注入非法URL和namespace实跑integration RED（配置验证阻止启动），显式清空Redis字段后相同环境4项GREEN。日志 `/tmp/loadout-redis-env-{red,green}.log`；保持业务验证断言。
- `make check` 新增强制测试Redis环境，Compose/CI新增Redis服务。完整检查第一次通过后端race与Rust，但Web首次lazy加载在1秒组件等待内未完成；保留失败证据 `/tmp/loadout-final-check.log`，修复或排除后必须重跑完整入口。

审查追加复验：基础process/dev测试在非法继承Redis URL/namespace环境下10项全绿（9.649s），证明fixtures不受该用户配置污染。前端首个lazy等待失败之后，保持原断言的聚焦与全套79项连续两次GREEN；无法从现有证据确认根因，未添加延时或改业务逻辑。并发样式修改曾触发noDescendingSpecificity，当前源码修正后Biome通过，随后重跑完整make check。

完整入口额外发现并修复日志测试分块假设：顶层直接node通过，但make check→make test-dev的嵌套MAKELEVEL使GNU Make先输出Entering directory，测试原先只断言第一个stdout chunk。`MAKELEVEL=1 node --test --test-name-pattern="Make commands read" tests/dev.test.mjs` 稳定RED。改用Node标准库events.on在原2秒预算内累积实际输出，直到预期日志出现；保留相同文本断言，不过滤业务输出。同命令GREEN（1项，0.180s），证据 `/tmp/loadout-make-logs-{red,green}.log`。该次检查的真实产品旅程已经全部通过，修复后仍重新执行完整入口。

2026-09-11零点前后开发实例已执行 `make restart`，使用现有.env与新二进制成功启动。真实5173入口 `/readyz` 返回 `200 {"redis":"ready","status":"ready"}`；Redis `PUBSUB NUMSUB loadout_dev:invalidate` 为1，确认实际网关订阅。管理员登录后8个账号/管理只读入口均200 JSON，测试会话注销204；匿名账号与MCP入口仍401。检查没有打印密码/Cookie或改动业务余额。


## 最终验收（2026-09-11）

导出隔离PostgreSQL / Redis测试URL，使用Node26.8.2与锁定工具链，根目录 `make check` **退出0**。包括四端静态检查、所有Go包race、Web79条、桌面5条组件及Rust原生命令、CLI、生产构建和Linux deb、真实产品旅程（含生产/开发各3条Web E2E、双实例/无粘性/Redis/恢复）、4条进程、6条后台生命周期、4条HTTP/CLI集成。完整输出保存在忽略目录 `build/distributed-harness.log`。

`git diff --check` 通过；OpenAPI validator0.9.0返回OK；Compose配置静态校验退出0。开发实例实际重启和readyz Redis状态已验证如上。所有复现缺陷保留RED/GREEN证据，没有浏览器自动化，没有提交或推送；真实GUI/其他OS/供应商和生产PG/Redis HA部署仍是明确环境边界。
