# 浏览器登录态与 AT/RT 执行记录

目标：公共页识别会话、已登录访问登录页自动恢复，AT 过期可透明刷新。

设计：复用 PostgreSQL sessions 作为可撤销的 默认 7 天绝对期限 RT；新增关联的 默认 15 分钟 opaque AT，双方 HttpOnly/SameSite=Lax Cookie，生产遵循 SecureCookies。RT 固定到期，不无限滑动；每次刷新发新 AT，已有未到期 AT 保留以支持多标签并发。旧 7 天 session 在原到期前兼容。注销删除父会话及所有 AT；禁用账号立即拒绝。前端仅在受保护 API 的 unauthorized 上刷新一次，同页并发合并；刷新失败的网络/服务故障保留可重试错误，不误报退出。

计划（按 writing-plans / TDD，在当前工作区执行，不提交无关修改）：
- [x] 公共页和登录页：增加已登录恢复、匿名、网络错误与键盘入口的失败测试，再复用 accountQuery 修复。
- [x] 服务端：增加刷新/到期/撤销/跨来源/并发测试；双轨迁移新增 session_access；auth 共享发放和验证被 app 与 identity 使用。
- [x] 请求层：增加并发 401、刷新失败与仅一次重试测试；实现共享刷新。
- [ ] 回归、代码审查、make check；记录未验证视觉范围。

API：POST /api/v1/auth/refresh，无请求体，Cookie RT（旧 session 仅在原到期前访问兼容，不作为刷新凭据），成功 204 + Set-Cookie；401 unauthorized，403 forbidden_origin，500 internal_error。凭据不返回 JSON、不进 Web storage；真实客户端配置不变。

## 热更新与 K8s 验收范围

用户追加要求：全部支持 K8s 多副本，系统设置支持有效期热更新。复用 runtime_settings revision 写入保护；access_token_seconds 默认900、范围60–86400；refresh_token_seconds 默认604800、范围60–31536000且不短于AT。省略字段保留当前值，显式0拒绝；旧存储缺字段按默认值读取。各副本签发时读取共享PG，旧RT期限不延长，刷新AT不得超过RT绝对期限。GitHub回调使用同一发放实现。初次AT/RT版本升级不兼容旧二进制验AT，需同步切换/维护窗口，见CLUSTER。

## RED / GREEN / REFACTOR 证据

命令使用WSL本地锁定工具链，Go经 `node --env-file=.env .loadout/browser-test.mjs go -C server ...` 安全加载测试环境；不输出凭据。日志在忽略目录 `.loadout/`。

| 验收 | RED 命令与根因 | GREEN / 回归 |
|---|---|---|
| 已有会话恢复、公共页入口 | `pnpm --dir web exec vitest run src/BrowserAuth.test.tsx`；2失败：登录页无账号菜单，公共页没有已登录入口（browser-ui-red.log） | 同命令2通过（browser-ui-green.log）；随后兼容其他任务将已登录plugins嵌入Shell，键盘测试使用概览入口 |
| 双Cookie、过期、并发与撤销 | `go -C server test ./internal/app -run TestBrowser -count=1`；缺RT Cookie、refresh404（browser-server-red.log） | 同命令通过（browser-server-green.log）；独立连接池与race扩展见统一全量 |
| 单次并发刷新、失败归类 | `pnpm --dir web exec vitest run src/BrowserRefresh.test.ts`；3失败：无刷新、无重试、503被误认为401（browser-refresh-red.log） | BrowserRefresh + BrowserAuth 6通过（browser-refresh-green.log） |
| 热更新期限 | `go -C server test ./internal/app -run TestBrowserLifetime -count=1`；PATCH新字段400（browser-ttl-red.log） | TestBrowser通过（browser-ttl-green.log）；跨handler更新立即影响登录/刷新，原RT不延长，非法期限400 |
| 设置键盘编辑 | `pnpm --dir web exec vitest run src/SystemSettings.test.tsx -t lifetimes`；缺有效期输入（browser-ttl-ui-red.log） | 同命令通过（browser-ttl-ui-green.log） |
| 满连接池 | `go -C server test ./internal/app -run TestBrowserLoginWithOneDatabaseConnection -count=1`；单连接事务再取连接导致503超时（browser-pool-red.log） | 修复为复用tx读取配置，恢复后 `go -C server test -race ./internal/app ./internal/identity ./internal/settings ./internal/store -run TestBrowser -count=1` app通过，其余包无匹配测试（browser-restored-go.log） |
| 公共页账号身份隔离 | `pnpm --dir web exec vitest run src/BrowserAuth.test.tsx -t cached`；旧account缓存未清理（browser-cache-red-2.log） | SessionBoundary同时观察public-session并退休旧QueryClient；测试改为重新查询身份变化后重挂载的DOM |

恢复后聚焦：`pnpm --dir web exec vitest run src/BrowserAuth.test.tsx src/BrowserRefresh.test.ts src/SystemSettings.test.tsx src/SessionIsolation.test.tsx` 4文件17测试通过（browser-restored-green.log）。Go相关全包首次回归在并行任务与stash中断时出现旧OAuth expiry/settings默认值断言失败；已同步新AT存储与默认值，不能把该次记为通过。工具编辑器旧icon失败由对应任务跟进。

独立审查发现事务中二次取连接问题，补RED后修复；其他范围未发现明确问题。Impeccable detect三个改动界面输出空数组（browser-design-scan.json）。未使用浏览器自动化；DOM/HTTP不作为本任务桌面/手机视觉通过证据。

21:11外部stash收起tracked改动，经用户确认由外部恢复；本任务未执行reset/restore/stash，保留其他任务修改，恢复后重新验证，不沿用已被覆盖状态的通过结论。

真实5173 HTTP：`/plugins?page=3` 200；无RT的POST `/api/v1/auth/refresh` 401。真实双进程旅程增加：签发节点退出→强制AT到期401→另一副本刷新204→账号200；注销后旧AT/RT在重启副本均401。最终验收由共享工作区唯一完整make check统一执行，避免多份重负载测试互扰；结果下方追加。



## 全量运行与最终待验项

共享工作区唯一全量由工具编辑任务执行：`node --env-file=.env .loadout/tool-editor-check.mjs`（内部运行 `make check`），日志 `.loadout/tool-editor-check-complete.log`。通过lint/TS/Go格式、Go全包race、CLI、Web192项、桌面UI/Rust与构建、真实产品旅程77.931s；其中 `TestProductJourneyWithoutStickySessionsSurvivesReplicaExit` 0.99s通过，生产/开发Web旅程均通过。最后在 `tests/process.test.mjs:150` 的开发子进程失败清理用例超时，退出1，故此次make check不能计为全部通过；对应任务正在排查。SessionBoundary新增逻辑经独立复核未发现新问题。未经实际K8s集群部署/HA演练，未进行本任务桌面/移动人工视觉验收。

### 2026-09-12 最终验证状态更新

后续完整复跑 `.loadout/tool-editor-check-final.log` 在 Go race 阶段收到 Hangup，未完成。再次复跑 `.loadout/tool-editor-check-resumed.log` 在数据库测试阶段因 `127.0.0.1:44035` 连接拒绝失败，make退出2；协调任务确认临时PG18实例目录已不存在。没有改用或修改用户现有PG17.6实例，也没有以跳过数据库测试代替验收。此前通过的鉴权/热更新/真实双副本证据保留，但最终完整 `make check` 未通过，需恢复隔离的PG18测试环境后重跑。

协调任务随后完成 `make test-web test-process test-dev integration`，退出0（`.loadout/tool-editor-local-final.log`）：193项Web、4项进程、7项开发服务、4项生产HTTP/CLI集成通过；`cd server && go test -run TestSQLite ./internal/store` 通过。原进程退出超时已通过独立重跑验证。上述补跑不替代依赖PG18的完整make check；当前开发服务5173已停止，不将前日HTTP200描述为当前在线状态。
