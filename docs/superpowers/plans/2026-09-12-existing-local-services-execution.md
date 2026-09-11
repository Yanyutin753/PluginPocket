# 接入现有本地 PostgreSQL / Redis

用户明确选择接入现有实例。此次仅调整 Git 忽略的本地 `.env` 和数据库资源，没有修改应用实现或其他工作区改动。

## 原因与配置

- 原 `make up` 日志显示 PostgreSQL `127.0.0.1:44035` 连接拒绝；Redis `127.0.0.1:52363` 也未监听。原 `/tmp/loadout-pg18`、`/tmp/loadout-pgdata`、`/tmp/loadout-redis810` 均不存在。
- 只读核验现有 PostgreSQL `127.0.0.1:5432`（17.6）和 Redis `127.0.0.1:6379`（8.2.1）连接成功。现有 PG 中没有 Loadout 数据库或角色。
- 在现有 PG 中创建独立 `loadout_dev` / `loadout_test` 数据库及各自同名登录角色，不授予超级用户权限。应用复用原本地开发数据库密码，测试角色使用随机密码；没有修改其他业务库或角色。
- 更新 `.env` 四个业务/测试连接 URL，保持管理员、加密密钥和 Redis namespace 配置。原配置保存在 `.loadout/env.before-existing-services-*`，备份与 `.env` 权限均为 0600。此记录不包含凭据。
- 这是现有实例上的新 Loadout 库，未恢复已消失临时库中的历史数据。版本不同于仓库推荐的 PostgreSQL 18.6 / Redis 8.10.1，以下验证不代表完整版本兼容验收。

## 验证

此次是环境接入，未新增或修改产品行为，不适用实现代码的 RED / GREEN / REFACTOR 顺序。

- Python psycopg 使用两个新连接分别查询 `current_database(), current_user`，确认业务与测试库/角色隔离。
- `make up`：退出 0，Go 完成迁移并与 Vite 一起后台运行。
- `curl -fsS --max-time 5 http://127.0.0.1:5173/readyz`：200，`{"redis":"ready","status":"ready"}`。
- `make status`：running，Web 5173、API 8787。
- Node HTTP 验证经过 Vite：匿名账号查询 401、使用 `.env` 管理员登录 200、账号查询 200 且 role=admin、验证会话注销 204；没有输出凭据或 Cookie。
- `node --env-file=.env` 加载环境后启动 `make check`：退出 2，Biome 在已有前端改动中报告 5 个错误，停于 Makefile lint；未修改这些文件。日志 `.loadout/existing-services-check.log`。后续完整测试未执行，不能宣称完整 harness 通过。
- `git diff --check`：退出 0。

开发服务保持运行。后续使用 `make up` / `make down` / `make restart`；数据库与 Redis 由现有实例维护。
