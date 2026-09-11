# 前端同源 API 反代与来源校验

## 范围与原因

用户要求普通前端 `/api` 反代不依赖固定 `LOADOUT_PUBLIC_URL=http://127.0.0.1:5173`。前端本来使用相对URL，但后端设置公开地址时只比较固定Origin；同时Vite 8.3.0的字符串代理配置实际隐含 `changeOrigin:true`，把Host改成后端地址。最初假定Vite保留Host不准确，真实请求及已安装Vite源码证明第二个原因。

先更新PLAN契约，然后修复后端与代理：

- 使用Go标准库 `http.CrossOriginProtection`，保留写请求Origin必填、GET/HEAD/OPTIONS及设备协议例外。`LOADOUT_PUBLIC_URL` 不再作为CSRF白名单；公开链接、OAuth/邮件回调与Secure Cookie配置仍保留它。
- `/api` 代理显式 `changeOrigin:false`，保留Host、Origin、Sec-Fetch-Site，不通过改写Origin绕过校验。
- `.env.example` 的公开地址改为空；本地 `.env` 同步留空，原配置以0600权限备份在 `.loadout/env.before-request-origin`。未变动数据库、管理员或加密密钥。
- README、ENVIRONMENT、DEPLOYMENT、AGENTS同步；没有修改页面交互或无关工作区文件。

## RED / GREEN / REFACTOR

1. `cd server && go test ./internal/app -run '^TestBrowserWritesUseRequestOrigin$' -count=1`：新增处理器行为测试RED，localhost/TLS反代被错误返回403，固定公开地址跨Host却获放行，以及跨站Fetch Metadata被忽略。401断言表示通过来源检查后仍要求登录；403断言表示来源被拒绝。
2. 替换固定Origin判断后同一命令GREEN。覆盖公开地址为空/非空、localhost/IP、TLS终止、现代浏览器代理改Host、外站、不同端口、跨站/同站元数据、缺失或opaque Origin；客户端伪造的X-Forwarded头不授予信任。
3. `cd web && node node_modules/vitest/vitest.mjs run src/PluginProxy.test.ts`：扩展真实Vite代理测试RED，收到的Host为后端127.0.0.1端口，期望前端localhost端口。显式禁用Host改写后同一命令GREEN（两个测试）。测试同时断言跨站Origin与Fetch Metadata原样保留。
4. REFACTOR：`gofmt -w server/internal/app/origin_test.go`、`node node_modules/@biomejs/biome/bin/biome check --write web/src/PluginProxy.test.ts web/vite.config.ts`，仅格式化本次文件。最终针对这两个Web文件的Biome检查、Web `tsc --noEmit`、`git diff --check`退出0。
5. 回归发现 `TestApplicationReloadsPersistedSettingsWithoutRestart` 原fixture的默认Host是example.com，而Origin是https://loadout.test，原白名单掩盖了不一致。聚焦运行确认403；将两条请求URL补全为 `cfg.PublicURL + path` 后，同一测试GREEN，保留全部业务断言。

## 验收与限制

- 环境通过 `node --env-file=.env` 加载。`go test -race ./internal/app ./cmd/loadout-server -count=1` 首次：app全包通过（149.910秒），cmd仅上述旧fixture失败。修正后单独复跑cmd包，结果见下方补记。
- `make restart`退出0。通过真实Vite入口分别访问localhost:5173和127.0.0.1:5173，管理员登录200、账号读取200、注销204；外站Origin请求403。未输出密码/Cookie，验证会话均已注销。
- `make check`退出2，停于已有 `docs/openapi.json` Biome格式错误（`.loadout/origin-make-check.log`），未改动该文件。完整harness没有通过，不能以相关测试代替。
- 请求来源与Vite代理分别进行了独立代码审查，无阻断发现；审查者另行复跑代理测试通过。
- [标准库](https://pkg.go.dev/net/http#CrossOriginProtection)在缺少Sec-Fetch-Site时比较主机与端口，不比较协议，以兼容TLS终止；该边界已记入部署文档。未进行浏览器自动化、桌面/移动视觉检查，也未宣称已验证外部HTTPS部署。

最终补记：`go test -race ./cmd/loadout-server -count=1` 退出0（2.727秒）；`go test ./internal/app -run '^TestBrowserWritesUseRequestOrigin$' -count=1` 再次退出0。`curl -fsS --max-time 5 http://localhost:5173/readyz` 返回 `{"redis":"ready","status":"ready"}`，`make status` 为running，最终 `git diff --check` 退出0。
