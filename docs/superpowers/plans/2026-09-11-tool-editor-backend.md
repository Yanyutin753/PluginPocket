# 工具编辑后端执行记录（2026-09-11）

范围：共享 tools.icon、PATCH 保留规则、内置账号工具编辑、同引擎结算试算；API 契约、PG/SQLite 026 迁移。实现计划见 `2026-09-11-tool-editor.md`。

## RED

先添加 `TestToolEditorSharedIconAndPolicy`、`TestToolEditorIconValidation`、`TestToolEditorBuiltins`、`TestToolEditorSettlementPreview`、`TestSQLiteToolIcon`，然后执行：

```sh
/home/yangyang/.nvm/versions/node/v26.8.2/bin/node --env-file=.env .loadout/pg-test.mjs test ./internal/app ./internal/store -run 'TestToolEditor|TestSQLiteToolIcon' -count=1
```

真实失败：图标字段被旧解码器拒绝（400）；账号内置工具保存400；试算未路由（405）；SQLite 缺少icon列。此前直接调用系统node不支持 `--env-file`，属于环境失败，不计RED。

## GREEN / REFACTOR

按原路径最小扩展：tools共享列、原事务内读取并保留省略字段；图标使用标准库URL/base64/XML静态允许名单；预览使用网关 `settleResult` 的小型导出包装，每次建立无I/O评估器，避免未保存脚本累积缓存。默认body限制16KiB不变，保存与预览单独512KiB。

同一聚焦命令通过（app 1.364s、store 1.527s）。补充64KiB边界、恶意SVG、局部渐变、试算不写用量/账本/余额验证后：

```sh
/home/yangyang/.nvm/versions/node/v26.8.2/bin/node --env-file=.env .loadout/pg-test.mjs test ./internal/app -run TestToolEditor -count=1
```

通过（1.354s）。补充断言初版在t.Cleanup内使用已取消的t.Context导致测试清理失败，改为测试返回前defer采样；无生产逻辑改变。

验收映射：跨副本保存/读回/省略保持/清空 → SharedIconAndPolicy；SVG脚本、事件、foreignObject、外链/CSS、实体、动画、非法XML/base64及大小边界 → IconValidation；三个账号内置编辑 → Builtins；鉴权、默认/业务码/正则/脚本、错误退款、超时、无账单变化 → SettlementPreview；镜像迁移默认与写读 → SQLiteToolIcon。

## 回归

```sh
/home/yangyang/.nvm/versions/node/v26.8.2/bin/node --env-file=.env .loadout/pg-test.mjs test -race ./internal/app ./internal/gateway ./internal/store -count=1
```

运行中，结果补录。OpenAPI 已使用现有 Biome 格式化。全量 `make check` 由主任务统一执行；未使用浏览器自动化。

### MIME 类型补强（独立 RED / GREEN）

`test ./internal/app -run TestToolEditorIconValidation -count=1`（同一Node环境包装）：新增SVG伪装为png/jpeg/webp的三个用例均以201失败（预期400）。加入各格式魔数检查后，`test ./internal/app -run TestToolEditor -count=1` 通过（1.144s）。避免只相信data URL标签；PNG真实样本仍可保存。

广泛race回归：app通过（125.494s），store通过（16.110s）；gateway旧有限频测试 `TestAdmitUsesConfiguredLimits` 在并行负载中失败（70次通过，预期60次）。读取测试确认它在真实时间内循环窗口限流；隔离执行 `test -race ./internal/gateway -run 'TestAdmitUsesConfiguredLimits|TestSettlement' -count=1` 通过（3.063s）。完整gateway独立重跑中，不修改无关限频实现或断言。

最终独立完整gateway race重跑通过（23.756s）。最终新增行为race验证 `test -race ./internal/app ./internal/store -run 'TestToolEditor|TestSQLiteToolIcon' -count=1` 通过（app 4.600s，store 3.898s）；`biome check docs/openapi.json` 和相关文件 `git diff --check` 均通过。全量make check仍由主任务执行。

### 审查跟进：常见SVG兼容与URL边界

先增测试：初始XML声明、单引号UTF-8声明、重复/非初始/stylesheet PI拒绝、静态text/tspan与安全字体属性、HTTPS URL边界。`test ./internal/app -run TestToolEditorIconValidation -count=1` 有效RED：合法声明/text返回400、超长URL返回201。

实现仅允许单个字节起点的XML1.0声明（可选UTF-8、standalone）；其它PI、DOCTYPE/实体继续禁止。允许text/tspan和font-family/font-size/font-weight/text-anchor/dominant-baseline/dx/dy/letter-spacing，不开放CSS/href/事件。HTTPS上限统一2048字节，并同步API契约。

GREEN期间一次被并发账号改动的临时编译错误阻塞（browserTTL参数数量），未修改相关代码；另修正测试内URL前缀长度手算错误，改为 `2048-len(prefix)` 避免边界歧义。最终 `test -race ./internal/app -run TestToolEditor -count=1` 通过（6.669s）。未重复全量server测试；主任务接续make check。
