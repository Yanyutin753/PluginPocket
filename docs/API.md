# M0.0 HTTP 契约

`GET /api/v1/health` 和 `GET /healthz` 返回 HTTP 200、`Content-Type: application/json`、`Cache-Control: no-store`：

```json
{"status":"ok","service":"loadout","version":"0.1.0"}
```

`status` 必须为 `ok`，`service` 必须为 `loadout`，`version` 必须为非空且不含控制字符字符串。网页及 CLI 同时校验 HTTP 状态、JSON 与字段；这只证明服务运行，不代表账号、上游或计费可用。HEAD 返回相同状态且无 body。其他 method 返回 405 JSON。

错误响应：`{"error":"not_found"}` 或 `{"error":"method_not_allowed"}`。未知 `/api`、`/api/*`、`/mcp`、`/mcp/*` 和不存在的静态资源返回 JSON 404，不能返回 SPA 首页。

`loadout doctor --server URL` 接受 HTTP(S) 服务根地址；拒绝用户名、密码、query、fragment 和非根路径，避免误把反向代理前缀或凭证当服务器地址。连接总超时 5 秒，不跟随重定向；不打印上游响应体和用户输入的 URL。输出成功时包含服务版本，失败时退出码非零。M0.0 尚无自动登录、凭证读取和配置写入。
