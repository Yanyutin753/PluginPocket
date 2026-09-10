# M0.0 基建执行记录

日期：2026-09-10。工作区：当前 Loadout checkout；没有提交、推送或改写真实客户端配置。用户追加要求已落实：中文深色开发者界面、成熟库与固定最新稳定版本、完整 TDD、标准测试工具。验证工具切换为 Vitest / Testing Library、Go testing / httptest、cargo test、Node 内置 test runner。

## 交付范围

- React 状态页读取真实健康 API；包括等待、成功、超时、失败、手动恢复、卸载取消与键盘操作。
- Go 标准库服务提供健康契约、生产静态资源与 SPA 路由边界；支持配置校验、结构化日志与优雅退出。
- Rust `doctor` 调用同一 HTTP 契约，验证服务身份，限制 URL、重定向、超时和错误输出。
- 三套官方技能随项目提供、固定来源及许可证；AGENTS、产品/设计上下文、前端规范、API、harness、PR 模板、CI、容器定义齐备。
- `login / apply / bridge`、账号、网关协议与计费尚未实现；本记录不覆盖 M0 业务验收。

## RED → GREEN → REFACTOR 证据

| 行为 | RED 命令与实际失败 | GREEN 与重构 |
|---|---|---|
| Go 配置、HTTP、启动/退出 | `cd server && go test -count=1 ./...`：先前缺失声明引起的编译失败不算 RED；建立可编译空壳后重新运行，出现默认配置为空、无效地址被接受、健康接口 404、占用地址未报错和服务未就绪的断言失败 | 最小实现后 `go test -race -count=1 ./...` 三个包通过；配置、HTTP、进程职责分开，go vet 通过 |
| Rust doctor | `cargo test --manifest-path cli/Cargo.toml --test doctor`：可编译空 main 对成功输出、错误退出码和命令行为断言失败 | 实现 clap 命令与 reqwest 请求后通过；保留六个黑盒测试，标准库 fixture 支持超时与清理 |
| React 状态与恢复 | `pnpm --dir web test`：状态与错误/重试行为缺失时失败 | 实现真实 QueryClient + Zod 边界，组件组合官方 shadcn 组件；样式提取为语义 token |
| 离线时手动检查 | `pnpm --dir web test -- -t 'reports a failed manual check'`（实际运行整个文件）：期望错误，实际仍显示旧“服务已连接” | Query 使用 `networkMode: always`，禁止恢复网络后自动重试；同文件测试通过，错误隐藏旧版本，再次手动检查可恢复 |
| CLI 版本中的终端控制字符 | `cargo test --manifest-path cli/Cargo.toml --locked --test doctor doctor_rejects_error_and_unrelated_servers_without_exposing_response`：接受包含 ESC/换行的响应 | 拒绝控制字符；同一测试及完整六例通过。给 CLI 测试调用另加 10 秒进程看门狗，防止产品超时失效拖死测试 |
| Web 与 CLI 契约一致 | `pnpm --dir web exec vitest run -t 'does not mistake'`：含控制字符的版本仍显示已连接 | Zod 拒绝 Unicode Cc；完整组件测试通过 |

验证工具迁移没有改变既有 HTTP/CLI 产品契约。新增 Node 集成测试直接验证已有构建产物并通过，属于替换集成验证入口，不声称这一步曾有产品行为 RED。请求超时与键盘覆盖也作为标准组件回归保留；不把新的测试工具配置包装成业务 TDD 证据。

## 最终本地检查

`make check` 全流程通过：

- 三套技能入口与 launcher 权限。
- Biome（warnings 视为失败）、TypeScript、gofmt、go vet、rustfmt、clippy。
- Go race 测试：三个包通过。
- Rust 黑盒测试：6 例通过，包括不响应服务器的 5 秒超时。
- Vitest + Testing Library：12 例通过，包括真实取消信号的 5 秒超时、离线检查与键盘操作。
- Web 生产构建、Go 二进制、Rust release 构建。
- Node 进程测试：4 例通过，覆盖绑定失败、SIGTERM、开发 Ctrl+C、子进程失败清理。
- Node HTTP/CLI 集成：4 例通过，覆盖健康契约、路由边界、生产 HTML/JS/CSS、真实 Rust → Go。

审查发现的离线状态、终端控制字符、测试自身看门狗及请求超时已修复。CI 明确使用 `shell: bash`，确保日志经 tee 输出时仍传播 harness 失败；CI 上传 harness.log。

## 未验证范围

- `docker compose config --quiet` 通过；本机 Docker socket 权限不足，未执行镜像构建和容器运行。CI 配置包含镜像构建，但尚未推送触发远程 CI。
- macOS / Windows Rust 检查已写入 CI，尚未在这些系统运行。
- 按用户要求，最终验证不运行浏览器自动化。DOM 行为检查和生产资源 HTTP 检查均已通过；最终桌面/移动真实排版、滚动与视觉效果需要人工检查，未声称视觉验收通过。
- 工具链与直接依赖以本次官方发布/包注册表核验结果锁定；后续升级重新核验并运行相同 harness，不能把本次版本永远描述为最新。
