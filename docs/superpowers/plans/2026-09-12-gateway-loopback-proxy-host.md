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

## 验收与限制

- `make check` 完整通过（本地首次补跑 `pnpm install --frozen-lockfile`；golangci-lint 下载遇网络抖动重试成功）。退出码 0。
- 部署验证：修复镜像上线后 CLI `bridge` initialize + `tools/list` 成功（详见 CLI E2E 记录）。
- 附带观察未处理：gateway `mcp.NewServer` 版本串 "0.2.0"、healthz "0.1.0" 与发行版本 0.3.0 不一致，属既有的版本常量漂移，未纳入本次最小修复。
