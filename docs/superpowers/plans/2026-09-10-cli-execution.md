# Rust CLI / bridge 执行记录

2026-09-10；当前共享工作区，未提交/推送，未修改真实 HOME 或客户端配置。

## 依赖核验

crates.io 官方 API 与 docs.rs rmcp 3.2.0 核验：rmcp 3.2.0、tokio 1.53.1、toml_edit 0.25.13+spec-1.1.0、serde_json 1.0.151、tempfile 3.27.0、rpassword 7.5.4、测试服务器 axum 0.8.9。Cargo.toml 精确固定，Cargo.lock 保留解析结果。

## TDD 证据

1. 新增可编译共享库空壳及 `accounts` 真实 TCP / 临时目录测试。`cargo test --manifest-path cli/Cargo.toml --test accounts` **RED**：有效 login 返回 `not implemented`；CLI login 未识别，退出码 2。下载和编译仅准备阶段，不计 RED。
2. `cargo test --manifest-path cli/Cargo.toml --test clients` **RED**：检测后 apply 与 direct 配置生成返回 `not implemented`。测试通过 TOML/JSON 消费实际文件，断言注释/其他服务器保留、绝对执行路径及路径转义。
3. 最小实现 config/local API/client writers/CLI。`cargo test --manifest-path cli/Cargo.toml --test accounts --test clients --test doctor` **GREEN**：accounts 4、clients 5、doctor 6。原 doctor URL、错误/重定向/超时/秘密保护回归保留，帮助测试改为完整已实现命令集。
4. 在绿灯下 `cargo fmt --manifest-path cli/Cargo.toml`，bridge 官方 SDK 行为测试继续 RED/GREEN。

## 验收映射

- accounts：真实 verify 请求/Bearer、失败保留旧凭证、0600、原子替换、退出清理、实时余额、stdin 秘密不回显、登录不修改客户端文件、目标/父目录符号链接拒绝。
- clients：三客户端、自动检测/未知客户端、bridge/direct、幂等 apply/remove、已有内容与 TOML 注释、JSON 手写冲突/TOML 无标记冲突、坏格式/坏标记、备份、路径转义、符号链接拒绝。
- doctor：既有 HTTP 健康、安全 URL、超时和不跟随重定向。

## 设计边界

JSON 客户端的归属快照单独存于 `~/.loadout/managed-clients.json`（0600），避免向客户端格式添加自定义字段；用户修改已管理条目后拒绝覆盖。TOML 按方案使用明确标记块；所有目标先解析/检查再开始写入，各文件独立原子替换，不声称跨多个文件的事务性。客户端备份为 `.loadout.bak`，每次实际修改备份此前内容，重复 apply 不更新备份。

`LocalClient` 同步共享 API 可供 Tauri spawn_blocking 使用；`client_states()` 可离线查看。所有返回给 UI 的结构均不包含令牌。bridge 协议、完整验证结果随后补充。

## bridge 与复核 TDD

5. `cargo test --manifest-path cli/Cargo.toml --test bridge` **RED**：依赖/API 适配完成后，官方 SDK 子进程连接在 initialize 返回 `ConnectionClosed("initialize response")`，来自尚未实现的 bridge 入口。实现官方 rmcp stdio ServerHandler → Streamable HTTP client 后，同命令 **GREEN**。
6. 官方 MCP HTTP fixture 覆盖 Schema/list、echo、额度错误 isError、协议 invalid_params、重新读取轮换令牌、撤销令牌、HTTP 500/404 在模拟已产生副作用后返回错误时各只发送一次、stdout 可被 SDK 消费、关闭 stdio 后 3 秒内正常退出。reqwest 禁重定向与自动重试，rmcp 禁 session-expired 重放，使用 call_tool_once；每次操作新建 MCP 会话，底层 HTTP pool 复用。连接/调用有超时，SDK/HTTP 原始错误不外泄。
7. 核对 PLAN §8.3 与账号模块，verify.tools 必须是目录数组。`cargo test --manifest-path cli/Cargo.toml --test accounts verify_accepts_tool_catalog_array_from_product_api` **RED**：原数量类型拒绝合法数组，返回 invalid account response；改成保留目录的 Vec<Value>，同 suite **GREEN**。账号模块实际返回 `['echo','time_now']`，fixture 已对齐。
8. 自审发现 TOML 多行字符串含伪标记可能被删。`cargo test --manifest-path cli/Cargo.toml --test clients markers_inside_user_strings` **RED**：apply 错误接受字符串中的伪管理块；增加实际 TOML 结构检查后 **GREEN**。同时覆盖已管理 JSON 条目被用户修改时拒绝 apply/remove，以及后续客户端冲突导致所有目标保持原样。

## 最终 Rust 验证

- `cargo fmt --manifest-path cli/Cargo.toml --check`：通过。
- `cargo test --manifest-path cli/Cargo.toml --locked`：通过，accounts 5、bridge 1、clients 7、doctor 6；无 skip。
- `cargo clippy --manifest-path cli/Cargo.toml --locked --all-targets -- -D warnings`：通过。
- `git diff --check -- cli`：通过。

本阶段只在 Linux 验证；未操作真实 Codex/Claude/Cursor 实例，macOS/Windows 与真实客户端启动应由对应平台验收补充。0600 是 Unix 权限验证；Windows 沿用用户目录权限，未实测 ACL。全仓库 `make check` 由主任务在所有并行模块集成后执行，不能将本记录的 Rust 通过描述为整仓库通过。文件检查拒绝既有符号链接；不声称抵御有权限并发替换目录的本机攻击者。

## 跨语言集成反馈修复

主任务的真实 Rust→Go journey发现Go网关已带`[loadout]`的错误被bridge重复加前缀。新增官方rmcp fixture已带前缀的工具isError与协议ErrorData断言，`cargo test --manifest-path cli/Cargo.toml --locked --test bridge` **RED**：实际`[loadout] [loadout] insufficient_balance`不等于预期单前缀；两个错误路径均仅对未带前缀的消息添加，同命令 **GREEN**。原无前缀错误/非重放/凭证测试保留。

## 设备码登录与最终回归

- `cargo test --manifest-path cli/Cargo.toml --locked --test device` **RED**：3 个真实 HTTP fixture 测试被 clap 拒绝，`--device` 尚未支持。增加 authorize / token 轮询和共享 verify 保存后，同命令 **GREEN**（3 项，约 8 秒真实轮询）。
- 覆盖服务器 interval=0 的 1 秒最小间隔、pending、slow_down 增加 5 秒、成功领取后 verify、过期/已领取保留旧凭证、非法核对 URL 拒绝、`--device` 与 `--token` 冲突；实际请求集合不含 approve，输出不含设备秘密或令牌。不打开浏览器；用户手动核对 URL 与代码并批准。
- 复核明确选择客户端的写入结果：`cargo test --manifest-path cli/Cargo.toml --locked --test clients explicit_client_apply_does_not_fail` **RED**：Claude 已成功写入，却因未选择的损坏 Codex 文件而返回错误。仅查询本次选择客户端的结果后，同命令 **GREEN**；未降低所选文件的预检契约。
- 最终 `cargo test --manifest-path cli/Cargo.toml --locked` **通过**：accounts 5、bridge 1、clients 8、device 3、doctor 6，共 23 项，无 skip。随后 `cargo clippy --manifest-path cli/Cargo.toml --locked --all-targets -- -D warnings` 与 `cargo fmt --manifest-path cli/Cargo.toml --all -- --check` 均通过。

## 交叉审查：Codex 用户修改与中途 I/O 失败

Web 代理在临时 HOME 实际执行 CLI 发现两项配置问题。新增真实文件测试后，`cargo test --manifest-path cli/Cargo.toml --locked --test clients` **RED**：`modified_codex_managed_block_is_never_overwritten_or_removed` 发现用户将 bridge 参数改为 user-edited 后仍被覆盖；`partially_written_clients_remain_owned_and_removable_after_later_io_failure` 使用 Cursor 备份路径为目录触发中途写入错误，已写 Claude 因缺少归属记录无法 remove。其他 8 项通过。

Codex 现与 JSON 一样要求精确归属，保留完整管理块快照以保护块内自定义修改。写入前先在 0600 管理记录中原子保存旧/新精确版本；全部文件写成功后收敛为最终版本。中途失败时两个版本均可识别，允许安全 remove 或 retry，不声称跨文件事务或回滚所有文件。相同 clients 测试 **GREEN**，10 项通过。格式化后继续全 CLI 与消费共享库的桌面回归。

交叉审查修复后的最终验证：`cargo test --manifest-path cli/Cargo.toml --locked` 25 项通过，fmt 与 clippy `--all-targets -- -D warnings` 通过。桌面 `make lint-desktop test-desktop build-desktop` 同步通过。
