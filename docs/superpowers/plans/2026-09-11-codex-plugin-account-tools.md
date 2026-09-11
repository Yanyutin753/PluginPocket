# 2026-09-11 Codex 插件化账号工具（account_balance / account_usage / tools_catalog）

## 背景

用户反馈：客户端没有出现产品承诺的效果。调查结论（均在本机验证）：

- `loadout apply` / bridge 机制已实现且有测试覆盖，但本机从未执行：CLI 未构建安装（PATH 无 `loadout`）、`~/.loadout/config.json` 不存在、`~/.codex/config.toml` 无 `mcp_servers.loadout` 条目。
- 即使跑通，网关目录也只有 `echo`、`time_now` 两个内置工具（`migrations/001_core.sql`）。服务端自身能力（余额、用量、目录）未包装为 MCP 工具——即"把服务端端口包装成插件给 Codex 调用"确实缺失。

## 范围（用户未即时应答，按推荐项推进）

只读账号能力三件套，全部 `cost=0` 不扣额度；操作类（兑换码/团队/订单）与真实客户端配置写入留待后续确认。

## TDD 证据

- RED：`cd server && go test -count=1 -run TestAccountBuiltinTools ./internal/gateway/`
  失败：`catalog missing account_balance; have map[echo:true time_now:true]`——来自缺失行为。
- GREEN：新增 `migrations/016_account_tools.sql`（三行 builtin 种子）+ `gateway.execute` 增加 `account_balance` / `account_usage` / `tools_catalog` 分支（`execute` 增加 `store.Principal` 参数）。同一命令通过。
- 回归：`go test -race -count=1 ./...`（server 全包）通过。适配了 7 处对内置工具数量=2 的偶然断言（boundary/hardening/metadata_validation/catalog_revision/redis/distributed，逐一确认只改计数不改行为语义）。

## 端到端验证（真实运行实例）

1. `make restart` 加载迁移 016；
2. 真实 API 注册用户 + 创建令牌；
3. 隔离 `HOME=/tmp/loadout-e2e/home` 下：`loadout login --server http://127.0.0.1:8787 --token ldt_…` → `loadout apply --clients codex` → `.codex/config.toml` 出现托管块；
4. stdio 驱动 `loadout bridge`：
   - `tools/list` → `['account_balance','account_usage','echo','time_now','tools_catalog']`
   - `tools/call account_balance` → `{"balance":1000,"username":"codex_plugin_e2e2"}`

## 本机启用（用户真实环境，未执行，需本人运行）

```bash
make build-cli
cp cli/target/release/loadout ~/.cargo/bin/   # 或加入 PATH，路径需长期有效
loadout login --server http://127.0.0.1:8787          # 粘贴网页端令牌，或 --device 浏览器授权
loadout apply --clients codex                        # 写入 ~/.codex/config.toml 托管块（自动备份）
# 重启 Codex 后 mcp 工具即可用；移除：loadout remove --clients codex
```

## 未验证范围

- 真实 `~/.codex/config.toml` 的写入（按仓库契约未经用户确认不修改真实客户端配置）；
- `make check` 结果见下方记录（本文件落笔时在跑）。

## make check

命令：`set -a; source .env; set +a; make check`（2026-09-11 本机）
结果：退出码 0。10 个子目标全部进入并完成：database/redis/skills 检查、lint（biome + golangci-lint）、test（Go -race 全包 / Rust CLI / React）、test-desktop、test-e2e、test-process（4/4）、test-dev（6/6）、tests/integration.test.mjs。日志：`/tmp/loadout-check.log`。

## 残留

- 验证用户 `codex_plugin_e2e2`（id 63）与其令牌留在本机 demo 数据库；如需清理：网页端令牌页撤销，或 `DELETE /api/v1/account/tokens/235`。
- 隔离验证目录 `/tmp/loadout-e2e/`（含临时 HOME 与 cookies），可随时删除。
