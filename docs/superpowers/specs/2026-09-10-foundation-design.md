# Loadout 基建设计

依据：用户指定 React 前端、Go 后端，允许 Rust 客户端；要求 Superpowers、Ponytail、Impeccable 三套技能和完整 TDD / harness。

## 产品与范围

Loadout 把运营方预设 MCP 工具通过单一网关提供给开发者。网页管理账号、令牌与额度；Rust CLI 负责本地接入，后续 bridge 让客户端配置不保存密钥。本次交付 M0.0 工程基建，不宣称完成 M0 的商业或 MCP 链路。

## 决策

- 保持 `web/`、`server/`、`cli/` 单仓库。React + TypeScript + Vite + Tailwind CSS + shadcn/ui + TanStack Query + Zod + Lucide 从第一天开始；Go 使用 `net/http`、`slog` 和标准库测试；Rust 使用 clap、reqwest、serde，提供原生二进制。
- 基建实现 Go 健康 API、React 服务状态页、Rust `doctor --server URL`。三者通过同一健康响应契约联通。doctor 检查服务身份，拒绝无效 URL、错误 HTTP 状态和不符契约的响应；诊断只输出安全摘要。
- 生产 Go 服务托管构建后的 React 静态文件。开发 Vite 代理 API；健康检查不依赖数据库。未知 API 保持 JSON 404，不能被 SPA 回退成 HTML。
- 监听地址、静态资源目录从环境变量读取；优雅退出、启动错误非零退出。未实现的账号、MCP、计费接口不提供模拟成功响应。
- SQLite 持久化、认证、官方 Go MCP SDK、Rust rmcp bridge 在下一阶段按真实业务测试引入，不预装未使用的运行依赖。未来桌面端可复用 Rust 库，当前不建 Tauri 空壳。
- 前端是中文、深色、清晰操作优先的接入状态页，显示真实 loading / success / error，提供可重试检查。记录 PRODUCT.md、DESIGN.md 和 CSS tokens。无虚构余额、调用量和可点击的空功能。

## Harness 与 TDD

每个可执行行为必须先有失败测试并观察预期失败，之后最小实现到绿，重构后回归。配置和文档通过真实工具消费验证，不能用“文件包含字符串”冒充行为测试。

- `make test`：Go HTTP/配置测试、Rust CLI 集成测试、React 组件测试。
- `make check`：非修改式格式/静态检查、类型检查、三端测试、构建、Node 内置测试运行器的真实 HTTP + 生产资源 + Rust CLI 集成。
- `make dev`：启动 Go + Vite，任一异常导致组合进程退出，退出时清理子进程。
- `make build`：Web 资源、Go 服务端、Rust CLI 可独立构建。
- Node 内置 test runner 启动临时端口的独立服务，验证真实 HTTP 契约、生产 HTML/资源、doctor 成功、SPA/API 路由边界与进程清理；组件测试验证状态、重试、取消与键盘操作。按用户要求不使用浏览器自动化；桌面/移动视觉单独人工检查。
- CI 在 Linux 执行完整 harness，并在 Windows/macOS 验证 Rust CLI；依赖及工具链锁定，禁止 CI 自动修复格式。
- PR 模板要求填写 RED 命令与失败原因、GREEN 命令、重构与最终验证证据。CI 只能证明当前绿灯，不能证明测试曾经先失败，开发记录补充顺序证据。

## 验收

新 clone 可从 README 安装依赖、启动和运行检查；技能随仓库提供、固定上游 SHA 和许可证；`make check` 通过；构建产物可启动并通过真实 HTTP 与 CLI 检测。Dockerfile / Compose 提供单服务运行方式；若本机 Docker 权限限制，明确区分配置校验与实际容器运行。
