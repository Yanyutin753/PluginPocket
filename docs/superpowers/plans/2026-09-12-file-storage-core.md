# 通用存储核心执行记录

采用 ponytail 与 test-driven-development；范围仅 filestore、配置、双方言迁移、SDK 依赖。

## RED / GREEN

- RED：`cd server && go test ./internal/filestore`（从根 .env 加载测试数据库）：真实 PG 新建 schema 后，`TestInlineBinaryAcrossInstances`、`TestS3RoundtripAndFailure` 因桩实现返回 `file storage unavailable` 失败；`TestOptionsRejectUnsafeEndpoint` 因不安全端点被接受失败。
- GREEN：同命令，真实 PG 与 httptest S3 签名请求通过。覆盖跨实例二进制、摘要去重、大小边界、无云配置拒绝大文件、上传失败不留引用、下载哈希校验、配置目标变化拒读。
- 配置 RED：`go test ./internal/filestore ./internal/config`，filestore 通过；`TestFileStorageConfiguration` 因默认阈值为 0 而失败。
- GREEN / REFACTOR：`go test -race ./internal/filestore ./internal/config` 通过，最终 filestore 1.875s、config cached；真实 PG 验证 12 个并发实例内容去重及空文件往返。`go test ./internal/store -run TestSQLite` 通过（11.151s），覆盖双方言文件数一致与 SQLite 二进制、摘要/长度/后端位置约束。`gofmt` 与 `go mod tidy` 完成。
- 存储子任务不运行完整 `make check`；由主任务统一执行，真实 AWS/R2/MinIO/OSS/Spaces/B2 联调未运行。独立代码审查交主任务整合阶段。

## 实现边界

AWS 官方 SDK Go v2：通过 `go list -m -json ...@latest` 核验 2026-09-09 稳定版，精确固定 `service/s3 v1.113.0`、`credentials v1.20.4`，主 SDK `v1.47.0`。依据：https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/migrate-gosdk.html 。仅 PutObject/GetObject；checksum 设为 when-required 以兼容服务，内容仍强制自行重验 SHA256。请求限制 30 秒，最多两次尝试。

云对象 endpoint/region/bucket/key 保存数据库。当前配置必须匹配云目标后才读，凭据不入库。前缀变化不影响旧 key。无真实云资源操作；厂商真实联调未验证。数据库写入失败后的云孤儿对象未做自动 GC（只留内容寻址对象，后续运维需求再添加）。

## Git 缓存引用补充

- 通用对象上限调整为 16MiB，技能业务仍独立限制 8MiB，为 Git blob/zlib 编码膨胀留空间。028 双方言与配置阈值同步。
- 029 双方言给 Git 缓存添加 `file_sha256` 外键与 `file_size`，引用行要求 content 为空且大小有效；兼容旧 bytea 行。
- RED：`go test ./internal/filestore -run TestGenericStoreAllowsGitEncodingOverhead` 因 16MiB inline 配置被拒失败；`go test ./internal/store -run TestSQLiteGitFileObjectReferences` 因缺少新字段失败。
- GREEN：`go test -race ./internal/filestore ./internal/config` 通过（filestore 2.187s），8MiB+1024 原始对象通过真实 PG 往返。
- 为 app/export 统一配置映射新增 `Config.FileStorageOptions()`；`go test ./internal/config -run TestFileStorageConfiguration` 先因空映射失败，再实现字段映射。
- 最终 `go test -race ./internal/config` 通过（1.040s）；包含 029 的 `go test ./internal/store -run TestSQLite` 全轨通过（29.798s）。

## 审查修复：事务连接与已应用迁移

- `Store.WithTx(pgx.Tx)` 浅拷贝复用调用方事务，内部仅 QueryRow/Exec 两方法接口，原 Store 继续使用 pool；不隐式提交。上游发布锁持有期间不会再借连接。
- RED：`go test ./internal/filestore -run TestTransactionReusesSingleConnectionAndRollsBack`，单连接池因重复借连接超时（2.81s）；GREEN 同命令通过（0.404s）。验证事务内二进制读写、回滚后引用不可见，以及原 Store 不被修改。云上传不随 PG 回滚，可能留下未引用对象，仍遵循前述孤儿对象边界。
- 新增 030 双方言升级原 028 的 8MiB 约束。PG替换 CHECK；SQLite在启用外键下重建父子表，复制全部数据和 Git 引用再换名，不手动改真实库。
- RED：`go test ./internal/store -run 'TestSQLiteUpgradeOriginalFileLimit|TestPostgresUpgradeOriginalFileLimit'`，两轨都因原8MiB CHECK 拒绝 8388609 字节而失败；GREEN 同命令通过（2.337s）。SQLite还断言旧 Git 引用保留、删除被引用对象仍受外键保护。
- 回归：`go test -race ./internal/filestore ./internal/config` 通过（2.547s/1.026s）；`go test ./internal/store -run TestSQLite` 全轨通过（18.452s）。完整 make check 由主任务统一执行。
