# Web 真实 API E2E 执行记录

范围：执行 `2026-09-10-e2e-hardening.md` 任务 1。生产 App、Router、Query、Zod 与各业务页面由 Vitest + Testing Library 挂载；Node 原生 fetch 发真实 HTTP 到本轮构建的 Go 进程，后端使用隔离 PostgreSQL schema。未新增依赖、未修改生产业务代码、未使用浏览器自动化。

## 测试入口与边界

- `pnpm --dir web test` 仅执行 `src/**/*.test.{ts,tsx}`。
- `pnpm --dir web test:e2e` 使用 `vitest.e2e.config.ts`；缺少 `LOADOUT_E2E_ORIGIN` 立即失败，不隐式跳过。该 origin 必须是 Go fixture 启动的隔离测试服务。
- `TestProductJourneyWeb` 创建隔离 schema、启动真实 Go、传入测试管理员环境并执行前述脚本，退出清理进程和 schema。完整 harness 由根任务接入和执行。
- adapter 只补上 Node fetch 缺少的相对 URL、同源 Origin、Cookie 处理；Cookie 采用 Vitest jsdom 的 CookieJar，真实解析 HttpOnly、path 和 expiry，响应原样交由生产 Zod 消费。未 mock fetch 返回值、业务接口、Router 或 Query。
- 测试中绕过 Cookie 的原生请求仅用于独立验证 Bearer token、退出后旧 Cookie 失效和停用账号登录被拒绝。没有绕过 Go 鉴权。
- 使用者操作经 role/label 和 user-event；令牌创建包含真实 Tab → 选择钱包 → Tab → Enter。重新挂载 App 模拟页面刷新；并非浏览器视觉或 CORS/SameSite 实现验收。

## 验收映射

| 旅程 | DOM 操作和断言 | 真实后端断言 |
|---|---|---|
| 账号与令牌 | 注册、键盘创建令牌、隐藏明文、确认撤销、注销、错误密码恢复、重新登录恢复受保护路径、普通用户访问管理页被拒绝 | 注册身份、HttpOnly Cookie、Bearer 身份、列表不泄露明文、撤销 401、旧 Cookie 401、管理 API 403 |
| 运营与兑换 | 管理员登录、调账 137、停用/启用账号、调整 echo 成本为 3、停用/启用工具、创建/停用/启用套餐、生成并隐藏兑换码、换回普通用户、错误兑换恢复、兑换 211、重复兑换错误、邮件和支付未配置说明 | 停用账号登录 401、工具价格保存和公开目录过滤、套餐过滤、余额精确增加 348、211 额度流水仅一笔 |
| 团队与设备 | 建团队、转入 23、最后所有者退出错误、邀请第二用户、成员权限、团队令牌、退出团队、服务端会话注销后设备页重登恢复、无效设备码恢复、明确批准设备 | 个人余额减少 23、团队余额 23、团队令牌绑定成员及共享余额、退出后该令牌 401、设备令牌绑定成员及个人钱包、设备码重放 400 |

## RED / GREEN / REFACTOR 证据

本轮增加已有产品行为的集成覆盖，没有编造功能 RED，也没有修改生产逻辑。

1. 准备阶段：首次 `pnpm --dir web typecheck` 报 Testing Library `ByRoleOptions` 不支持 `exact`。去掉多余参数后通过；这是测试代码类型错误，不是产品 RED。
2. 首次真实集成运行：`LOADOUT_SERVER_BINARY=/home/yangyang/loadout/build/loadout-server LOADOUT_WEB_E2E=1 go -C server test ./cmd/loadout-server -run '^TestProductJourneyWeb$' -count=1 -v`，3 条旅程全部首次通过；Go 输出 `ok ... 6.259s`，本地日志 `/tmp/loadout-web-e2e-first.log`。记录为已有行为首次 GREEN。
3. 在绿灯下补强/整理：adapter 保留 Request 输入方法/请求体并组合 10 秒超时，键盘通过焦点断言，Bearer 验证真实用户名和钱包余额。相同旅程加 `-race` 通过，Go 输出 `ok ... 7.481s`；日志 `/tmp/loadout-web-e2e-refactor.log`。
4. 再补充账号停用/启用与无效设备码恢复。最初给出错误长度 `invalid-device`，后端正确返回 `invalid_request`，与测试预期无效码不一致。这是测试输入选择错误，不是生产 RED；改用长度正确的不存在代码 `0000000000` 后验证无效码恢复。
5. 最终聚焦命令：`LOADOUT_SERVER_BINARY=/home/yangyang/loadout/build/loadout-server LOADOUT_WEB_E2E=1 go -C server test -race ./cmd/loadout-server -run '^TestProductJourneyWeb$' -count=1 -v`，3 条旅程全部通过，Go 输出 `ok ... 8.193s`；日志 `/tmp/loadout-web-e2e-final.log`。
6. 相关回归：`pnpm --dir web test`，4 文件 55 测试通过；`pnpm --dir web build` 通过；最终 `pnpm --dir web typecheck`、`pnpm exec biome check web/src/e2e web/vitest.e2e.config.ts web/tsconfig.json web/package.json web/vite.config.ts` 通过。

完整 `make check` 结果由根任务最终记录，本子任务不把上述聚焦验证当作完整 harness。未进行桌面/移动端人工视觉、真实浏览器布局、外部邮件/OAuth/支付服务验收。

## 开发服务只读排障

本轮另检查正在运行的 `127.0.0.1:5173` Vite 服务：使用仓库已有 `es-module-lexer` 解析真实响应，从 main、Vite client、React refresh 递归检查 48 个静态/动态模块，均返回成功及 JavaScript/CSS 类型。代理 meta/plans/health 返回 200，匿名 account/me 与 tools 返回预期 401。

最初正则探测误读依赖文档中的示例 import，产生 MyComponent/types 等 404；已用 ESM 解析消除该探测噪声，不能把这些 404 报为产品缺陷。未修改真实业务数据、真实环境文件或重启开发进程。该检查不执行浏览器页面，不能证明前端运行时和视觉无误。
