# CLI 技能生命周期修复

范围：技能更新与卸载；不改变市场 API、凭证或真实客户端配置。

根因：安装以新版文件清单比对旧版管理记录，附件增删会误判未托管；卸载仅枚举根目录，与包含斜杠的相对路径清单永远无法匹配。

方案：递归检查当前目录与已保存清单，拒绝外来文件、目录、符号链接；更新允许清单变化并删除已移除的旧附件及空目录，卸载使用相同检查。

## 验收与 TDD 证据

全部命令在 WSL 仓库根执行，Rust 使用 `/home/yangyang/.cargo/bin`。

- RED：`cargo test --manifest-path cli/Cargo.toml --locked --test plugins skill_lifecycle -- --nocapture`，退出 1。两项行为测试均为有效断言失败：新版增删附件的更新返回 `existing skill directory is unmanaged; resolve it manually`；没有外来文件的嵌套目录卸载返回 `skill directory contains files PluginPocket did not install; remove them first`。
- GREEN：最小实现后同一命令退出 0，2 项通过。
- REFACTOR：递归清单读取与核对作为两个安装/卸载共用的私有函数，无新依赖。补充外来嵌套文件、空目录、Unix 目录符号链接保护及更新后再次卸载验收。执行 `cargo fmt --manifest-path cli/Cargo.toml`，随后 `cargo test --manifest-path cli/Cargo.toml --locked --test plugins skill_lifecycle`，退出 0，2 项通过。
- 相关回归：`cargo test --manifest-path cli/Cargo.toml --locked`，退出 0，8 个集成测试文件共 43 项通过；包括二进制与执行位、大附件、所有目的路径预检及原有客户端配置保护。
- 静态检查：`cargo clippy --manifest-path cli/Cargo.toml --locked --all-targets -- -D warnings` 与 `git diff --check` 均退出 0。
- 全仓 `make check` 由主任务统一执行，结果记录在主任务执行记录；以上不代表全仓验收。Windows/macOS 未在本轮独立运行，符号链接保护测试为 Unix 专属。

验收映射：`skill_lifecycle_uninstalls_nested_files_but_preserves_foreign_content` 覆盖递归卸载、外来文件/空目录拒绝及不跟随链接；`skill_lifecycle_update_accepts_changed_file_manifest_and_removes_obsolete_files` 覆盖新版新增/删除附件、删除空的旧目录、拒绝带外来文件的更新与更新后卸载。
