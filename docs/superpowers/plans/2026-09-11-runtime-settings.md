# 系统配置热更新实施计划

设计：`../specs/2026-09-11-runtime-settings.md`。用户在审查修复中追加系统配置需求，沿用当前工作区实现。

- [x] settings包/迁移：Values、Manager.Read/Write、版本冲突、秘密加密与校验；真实PG RED/GREEN，独立backend记录。
- [x] identity/启动接入：环境默认与数据库快照合并，meta/OAuth/SMTP下一请求生效；真实fixture测试，复用底层HTTP client。
- [x] app：GET/PATCH管理员同源接口，事务序列化与版本校验，写后最终权限重验；密码注册读取当前赠送额度；测试新用户余额、另一manager读取、权限和秘密边界。
- [x] Web：新增SystemSettingsPage和AdminOnly路由，表单复用Field/Input/Button，TanStack Query/Zod，双语；SystemSettings.test覆盖保存、秘密保留、错误、冲突、键盘和非管理员。
- [x] 同步API/OpenAPI/ENVIRONMENT/HARNESS，标准lint及完整make check。

实现步骤均先运行新行为测试确认RED，再写最少实现并同命令GREEN；结构性类型准备不算业务实现。不得将测试依赖失败当RED。

## Web 顺序证据

- RED：`pnpm --dir web exec vitest run src/SystemSettings.test.tsx`（3失败：/admin/settings未注册，页面不存在，管理员权限与表单缺失）。
- GREEN：新增页面、路由、Zod/Query与保存错误状态后同命令3通过。
- 本地化RED：`pnpm --dir web exec vitest run src/SystemSettings.test.tsx -t localizes` 缺少英文System settings；补新页面目录后4通过，涵盖载入失败重试和无加密密钥时禁用秘密输入。
- 密钥替换/清除按行为单独验证：未序列化secret字段时`-t replaces` RED（后端拒绝，不出现保存成功）；实现显式清除/非空替换、空白省略后单项GREEN。完整同文件曾因前例语言localStorage污染失败；修正fixture每例清空偏好后5项全部通过。
- `pnpm --dir web typecheck`通过；`uvx --from openapi-spec-validator==0.9.0 openapi-spec-validator docs/openapi.json`为OK。
- 前端保留原组件和主题；没有浏览器自动化，尚未桌面/手机人工视觉验收。

## 最终验证

完整 `make check` 退出0，日志 `/tmp/loadout-final-check.log`；Web103项、所有Go竞态、桌面/CLI/真实PG与Redis、多副本及生产/开发Web链路均通过。细项及追加边界修复见 `2026-09-11-review-fixes.md`。

用户实际页面最初404来自尚未重启的旧Go进程，执行本地make restart后使用现有管理员凭证实际GET200，字段形状正确且无秘密原文字段，验证会话随后退出204；不更改系统配置值。网页热更新配置与升级Go程序的重启要求已补入README/ENVIRONMENT。人工桌面/移动视觉及真实供应商联调仍未覆盖。
