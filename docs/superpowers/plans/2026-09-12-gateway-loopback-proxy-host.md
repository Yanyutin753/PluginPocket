# 网关回环反代公网 Host 修复（CLI E2E 发现）

## 范围与原因

用户要求用 v0.3.0 Release CLI 对生产 https://pluginpocket.lambchat.com 做全流程测试。`pluginpocket login/doctor/status/market/install/update` 全部通过，但 `bridge` 的 `tools/list` 失败：`gateway connection failed`。直接 `curl /mcp` 返回 `Forbidden: invalid Host header "pluginpocket.lambchat.com"`。

系统排障定位根因：go-sdk v1.7.0 `StreamableHTTPHandler` 自动启用 DNS-rebinding 防护——监听地址为回环（openresty 同机反代到 `127.0.0.1:30978`）而 Host 头为公网域名时直接 403。该防护面向无鉴权本地 MCP 服务器；本网关入口先做 Bearer 鉴权，外层已有 `http.NewCrossOriginProtection()`，生产形态就是同机反代 + 公网 Host，属于误伤。DEPLOYMENT.md 明确"外部访问经自己的 HTTPS 反向代理"是受支持拓扑。

现有测试盲区：`httptest.NewServer` 的 Host 也是 127.0.0.1，从未覆盖"回环监听 + 公网 Host"组合，故 CI 全绿而生产不可用。

## RED / GREEN / REFACTOR

1. RED：新增 `TestGatewayServesPublicHostBehindLoopbackListener`（gateway_test.go）。自定义 RoundTripper 把 Host 头改写为 `pluginpocket.example.com` 复现反代形态，官方 MCP 客户端 initialize 即被拒。命令与失败：`go test -race -run 'TestGatewayServesPublicHostBehindLoopbackListener' ./internal/gateway` → `public Host behind loopback listener rejected connection: Forbidden`（与生产症状一致，失败来自待修行为）。
2. GREEN：`gateway.go` 的 `StreamableHTTPOptions` 增加 `DisableLocalhostProtection: true`（SDK 官方逐处理器选项，非已废弃的全局兼容变量），并注释安全依据。同一命令通过（0.39s）。
3. 回归：`go test -race ./internal/gateway` 全包 37.6s 通过；`go test -race ./...`（服务端全量，真实 PG 18.6 + Redis，本地 5433/6381）通过。

## 顺带修复：市场 git 导出测试的 umask 依赖

全量跑测时 `TestSkillManifestExportAndGitClone` 在本机失败：克隆出的附件权限 0o775 ≠ 断言 0o755。根因是 git checkout 权限 = 0777 & ~umask，本机 umask 0002、CI 为 022，属测试环境依赖而非产品缺陷。修复：`gitClone` helper 改经 `sh -c 'umask 022 && exec git clone …'` 固定子进程 umask，不放宽 0o755 断言、不改进程全局 umask。两种 umask 下复跑均通过。

## 顺带修复：dev 热重载构建的 VCS 探测

`make check` 首跑在 `test-dev`（tests/reload.test.mjs）失败：fixture 在 /tmp 构建报 `error obtaining VCS status: exit status 128`。根因：本机 /tmp 为独立文件系统，git 在无仓库目录探测到文件系统边界即退 128，Go buildvcs 视为失败（CI 的 /tmp 与 / 同文件系统故未暴露）。修复：`.air.toml` 构建命令加 `-buildvcs=false`（dev 热重载二进制本就不需要 VCS 模写），单测复跑通过。

## 验收与限制（第一轮）

- `make check` 完整通过（本地首次补跑 `pnpm install --frozen-lockfile`；golangci-lint 下载遇网络抖动重试成功）。退出码 0。
- 部署验证：修复镜像上线后 CLI `bridge` initialize + `tools/list` 成功（详见 CLI E2E 记录）。

## 追加：版本单一事实源与 Release CLI 校验（同日第二轮）

CLI E2E 收尾观察的三项遗留全部处理：

1. **版本串漂移**：healthz 报 0.1.0、gateway MCP serverInfo/上游 client 标识报 0.2.0，与统一发行版本 0.3.0 不一致。RED：`tests/integration.test.mjs` healthz 深断言改 0.3.0，`make build-server` 后运行确证失败（actual 0.1.0）。GREEN：新增 `server/internal/version`（`const Version = "0.3.0"`），main.go 默认值与 gateway 四处（gateway.go / metadata_cache.go / upstream.go / http_pool.go）统一引用；CLI doctor 回显断言与 dev.test.mjs mock 同步 0.3.0，integration 4/4 通过。`scripts/release-preflight.mjs` 的版本收集加入该 Go 常量，今后 tag 漂移在 preflight 即被拦截。
2. **Release CLI 校验**：release.yml CLI 打包步骤同时生成并上传 `<name>-SHA256SUMS`（Linux/macOS tar.gz 与 Windows zip 一致处理，sha256sum 缺失时回退 shasum）；verify-signatures 步骤只下载 desktop-* 产物，不受影响。
3. **生产测试令牌回收**：`DELETE /api/v1/account/tokens/3` 返回 204，旧 Bearer 复验 401。测试账号 `cli_e2e_test` 保留供复跑（余额 999 测试额度，无有效令牌）。

第二轮验收：`make check` 通过（preflight 5/5、integration 4/4），CI 双 workflow 绿，`main-021d022` 部署 yang 后 healthz/serverInfo 均为 0.3.0。release.yml 的 CLI SHA256SUMS 属 CI 变更，由下次 tag Release 运行证明；未发布新 tag。

## 追加：注册失败的细分错误码与自动填充兜底（同日第三轮）

用户报告生产站"注册不了"。排障证据：openresty 访问日志显示用户浏览器（Edge）18:07:30 的 `POST /api/v1/auth/register` 返回 400 `invalid_request`（此前 18:07:26 登录 401）；数据库无对应行、序列号存在缺口（两次重复用户名回滚）。API 直连 curl 与完整浏览器头模拟均 201，服务端正常。真实浏览器自动化复现：jsdom/自动填充不触发原生 tooShort 校验（dirty flag 机制），短密码会被直接提交并收到笼统的"提交内容不符合要求"。

修复（TDD）：

1. RED：`go test ./internal/app -run TestRegisterValidationReportsSpecificCodes`——期望 `invalid_password`/`invalid_username`，实际 `invalid_request`。
2. GREEN：account.go 注册校验拆分为两个具体码；`codes.go` 注册（契约金样测试同步 `error-codes.json`）；`i18n/errors.ts` 增加双语文案；PLAN.md API 表同步。
3. RED：`vitest run src/Product.test.tsx -t 'blocks autofilled invalid register input'`——短密码提交真的发出请求并显示无关错误。
4. GREEN：AuthPage onSubmit 兜底校验用户名正则与密码长度，经 `ApiError(400, code)` 走既有 ErrorNotice 渲染，不发请求。

## 验收与限制（第三轮）

- 服务端 app/httpapi 全绿（-race，226s/1.1s）；web vitest 34 文件 235 用例全绿；`make check` 通过（首次因 biome 格式失败，--write 修复后复跑）。退出码 0。
- 部署后真实浏览器复验：短密码提交显示"密码长度需在 12 到 1024 个字符之间"且无请求发出；合格输入注册成功（见部署记录）。
