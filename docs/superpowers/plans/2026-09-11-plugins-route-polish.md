# 插件目录接通与全站精修

**Goal:** 5173 与生产入口都可匿名访问插件目录、详情与市场源；首页和工作空间可发现入口，全站保持统一工坊风格。
**Architecture:** 最终按用户纠正采用公共 React 路由 `/plugins` 与详情，复用 PublicHeader、Heading、列表、按钮与主题/i18n；Go 提供匿名安全元数据 API，Vite 仅代理 API 与 Git。Git 历史使用 PG 持久共享，不增加依赖。
**Spec:** PRODUCT.md、DESIGN.md、docs/PLAN.md；当前用户明确授权实现，在当前工作区完成。

- [x] 路由：真实 Vite HTTP 转发公开API/git、404；首页和工作空间入口。
- [x] 页面：最终公共 React 目录/详情，搜索、URL分类筛选、安装命令、错误重试、空结果；会话过期不强制公开页面登录。匿名 API 仅公开字段白名单。
- [x] 全站：精修 styles.css 与 LandingPage.css 的公共页面标题、表单、表格、导航、空态和主题细节；保持原功能与专属插画。
- [x] 自动检查：聚焦测试、Web 回归、设计扫描、独立审查、make check；现有开发服务热重载后 HTTP 验证 5173。
- [ ] 人工验收：按要求不使用浏览器自动化，桌面/手机视觉仍待人工检查；未执行生产集群部署。

## 执行证据

命令在仓库根使用固定 Node 26.8.2/pnpm，PG命令继承 `.env` 的测试库/Redis；日志均位于 `.loadout/`，不含凭证。

| 验收 | RED | GREEN / 重构 |
|---|---|---|
| 入口与开发代理 | `pnpm --dir web exec vitest run src/PublicHome.test.tsx src/Product.test.tsx` 缺少市场入口；代理初次测试为URL准备错误，修正后 `src/PluginProxy.test.ts` 得到SPA HTML而非上游（plugins-proxy-red.log） | plugins-green.log；最终代理改测公开API，plugins-react-green.log |
| 公共 React 目录/详情、过滤、重试 | `pnpm --dir web exec vitest run src/PluginsPage.test.tsx` 三条缺少页面行为（plugins-react-red.log） | 同命令通过（plugins-react-green.log） |
| 匿名API | `go test ./internal/app -run TestAnonymousPluginAPI -count=1` 返回404 | 与TestPublicPluginDirectoryJSON合跑通过；追加公开字段白名单断言 |
| 旧会话过期仍匿名浏览 | PluginsPage新增用例，旧逻辑跳转login（plugins-session-red.log） | PluginsPage、SessionExpiry、SessionIsolation、PublicHome 9项通过（plugins-session-green.log） |
| Git跨副本与重启 | `go test ./internal/marketplace -run TestGitRegistrySharesChangesAndHistoryAcrossReplicas -count=1` 暖副本旧refs（plugins-distributed-red.log） | `go test -race ./internal/marketplace ./internal/store -run 'TestGitRegistry|TestSQLite' -count=1` 通过（plugins-distributed-green.log） |
| SQLite共享失效 | `go test ./internal/store -run TestSQLiteMarketChangeInvalidatesSharedGit -count=1` 缺少共享状态表 | 同上GREEN；覆盖insert/update/delete原子失效与对象字节存取 |
| 全部页面无粘性 | 既有旅程扩展26个页面（逐个请求两次）、品牌资源、公共API、真实git clone/pull、节点退出 | `LOADOUT_SERVER_BINARY=... go test -race ./cmd/loadout-server -run TestProductJourneyWithoutStickySessionsSurvivesReplicaExit -count=1` 通过（plugins-route-e2e.log） |

初版SSR代理/模板曾完成RED→GREEN；用户明确改为统一React页面后删除SSR模板及对应模板测试，不把废弃设计列作最终功能。保留Git提交构造器，删除本机失效通知，市场写入通过数据库触发器生效。首页共用PublicHeader后，其组件测试补齐MemoryRouter消费环境。

独立审查：初版与用户变更后的第二轮均完成；第二轮会话过期P2已通过上述RED/GREEN修复，分布式未发现其他有证据P1/P2。设计扫描70条均为字号登记等advisory，不代表视觉验收。

## 全API追加验收

用户继续明确所有 API 必须支持分布式。按路由组审查 app、identity、settings、auth、gateway：账号/角色/会话/令牌、钱包/账本/兑换、团队/邀请/转账、设备授权、运行配置、OAuth/邮箱验证、公共限流均以 PG 为权威；Redis与本机目录仅用于优化，写后修订号检测与事务权限检查仍执行。

审查新增 P2：createSkill 在事务外读版本、计算、UPSERT，两个副本会发布不同内容但相同版本。`go test ./internal/app -run TestDistributedSkillPublicationSerializesVersions -count=1` 使用真实数据库锁与两个独立连接池确定性交错，RED 两份回复均1.0.1（plugins-publish-red.log）。改为同slug共享事务锁（覆盖首发）+旧行锁+事务内读取响应后提交；`go test -race ./internal/app -run 'TestDistributedSkillPublicationSerializesVersions|TestMarketplaceFormalVersioning' -count=1` GREEN（plugins-publish-green.log）。

新增 `TestDistributedAPIRouteMatrix`：两套独立 App / pool / Gateway，对比26类读取路径的返回及no-store，验证管理权限一致；覆盖跨副本工具创建/更新、上游读取、metadata增删、市场同步/安装/卸载冲突、技能/装备组、套餐、支付未配置的真实503、兑换一次性、调账幂等、团队更新、令牌撤销和账号禁用。`go test -race ./internal/app -run TestDistributedAPIRouteMatrix -count=1` 通过（plugins-api-matrix.log）。其余OAuth/邮箱、设备消费、团队邀请/资金/成员权限并发与共享限流由既有 app/distributed_test.go、identity/distribution_test.go 和真实无粘性进程旅程覆盖，纳入最终 make check。

完整检查的前几次运行被同时进行的抽屉/发布/热重载改动及格式问题阻断；保留其他修改。最终结果在下方追加。禁止浏览器自动化，未完成桌面/手机人工视觉检查，也未部署真实生产集群或演练PG/Redis HA切换。

## 追加：全部路由的多副本部署与负载均衡

用户确认“分布式”指多副本部署。既有 PG 会话、限流、账本与设备授权继续通过现有无粘性进程旅程验证；新增全部 React 路径及公共目录/详情/静态资源在两副本直达与故障切换的检查。

发现 GitRegistry 以本机 lastTip 和内存 files 为权威：副本独立构建提交，更新/重启会丢失彼此对象。修复采用 PostgreSQL 共享 git 状态与对象表、数据库行锁串行发布、市场表触发器原子失效；同内容不产生空提交，历史对象保留用于进行中的克隆和已有仓库更新。SQLite 同步建表与触发器验证。保持单一 PG 权威，不新增 Redis 持久化或粘性会话。

- [x] 跨独立 registry 的更新/历史对象/重启回归，真实 PG RED。
- [x] 024_shared_marketplace_git 双轨迁移，registry 原子发布，HTTP 按对象读取；GREEN 与 SQLite 行为验证。
- [x] 双进程无粘性旅程覆盖所有页面、静态资源、公共目录、市场 git 与节点退出；文档明确滚动发布资产保留要求。

## 最终复验

- `pnpm --dir web exec vitest run src/PluginsPage.test.tsx src/PublicHome.test.tsx src/features/LandingPage.test.tsx src/PluginProxy.test.ts src/SessionExpiry.test.tsx`：5个文件、15项通过（plugins-final-focused.log），包含当前工作区追加的目录分页行为。
- `go test -race ./internal/app ./internal/marketplace ./internal/store ./internal/identity -run 'TestDistributed|TestGitRegistry|TestAnonymousPluginAPI|TestPublicPluginDirectoryJSON|TestSQLite|TestRuntimeSettingsPersist' -count=1`：app、marketplace、store通过（plugins-final-api.log）；identity没有匹配测试，不将其计为已验证。
- 实际5173入口HTTP复验：目录与详情返回React HTML，匿名API返回41个真实条目及deepwiki详情，Git HEAD与readyz均200。现有前台开发进程自行热重载；`make restart`因已有前台进程占用8787失败，未终止用户进程。
- `git diff --check`通过。全量检查仍独立记录，聚焦通过不替代完整harness。
- 最终 `node --env-file=.env .loadout/check-plugins.mjs`（内部完整执行 `make check`）退出0，日志 `plugins-check-final-7.log`。通过格式/静态检查、Go全包race、CLI、Web 144项、桌面UI/Rust、真实产品旅程（含Redis降级及无粘性双副本退出）、进程与开发热重载、生产静态资源/API集成检查。此前第6次被并行任务的UsageDetail ARIA错误阻断，该任务修复后第7次完整通过。
