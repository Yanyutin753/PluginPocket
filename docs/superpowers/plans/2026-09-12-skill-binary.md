# 技能二进制链路执行记录

## 合同与范围

按 `2026-09-12-file-storage.md` 实现。`files_v2` 是按路径新增/替换，旧 `files` 同样合并，未提交的附件保留；`delete_files` 显式删除（禁止删除 SKILL.md）。所有发布保存 `file_manifest`，仅 SKILL.md 同时镜像到 `files` 供文本编辑器重开。下载 `format=2` 使用 base64、SHA256、原始大小与 executable；旧格式遇到非UTF8、NUL或执行位返回400 invalid_request。

技能最多32文件，单文件8MiB，总计32MiB。路径拒绝逃逸、Windows设备名/尾点尾空格/特殊字符、大小写文件别名和文件目录冲突。SKILL.md 必须UTF8。GitHub使用递归Git tree取得模式和固定blob SHA，再读取blob原始字节，拒绝截断树、symlink/submodule/其他模式。Git导出还原0644/0755。

`marketplace.Options.Files` 与 `app.Options.Files` 接入通用文件存储。服务启动和离线导出复用 `Config.FileStorageOptions()`。Git仓大于256KiB的压缩对象通过通用文件存储引用持久化（029的file_sha256/file_size，原content为空bytea），不可变历史对象仍可读。Git zlib对象与原文件是不同编码，因此可能有两个对象；大对象不再复制回市场Git缓存bytea列。新manifest对象缺失时导出整体失败，不能静默删掉已发布技能。

## TDD证据

命令前统一 `cd /home/yangyang/loadout && set -a && source .env && cd server`；使用隔离测试schema，不写生产配置或真实对象存储。

- RED `go test ./internal/app -run TestSkillBinaryRoundTrip -count=1 -v`：发布files_v2得到400（未知输入），期待201。
- GREEN 同命令：非UTF8附件base64 AP+A往返、执行位、文本修改保留附件、旧格式拒绝、显式删除均通过。
- RED `go test ./internal/marketplace -run TestGitHubSkillBinaryAndExecutable -count=1`：旧Contents请求404，尚未使用Git tree/blob。
- GREEN 同命令：固定blob读取保留0/255/128字节及100755。
- RED `go test ./internal/marketplace -run TestSkillManifestExportAndGitClone -count=1`：manifest导出二进制为空。
- GREEN 同命令：读取filestore，经导出树及真实git clone保持字节和0755。
- RED `go test ./internal/marketplace -run TestGitRegistryStoresLargeObjectsByReference -count=1`：未找到引用行，大Git对象仍直接bytea。
- GREEN 同命令：持久化引用、跨registry重建后读取对象与原始Git对象一致。
- RED `go test ./internal/marketplace -run TestSkillFilesRejectWindowsAliases -count=1`：大小写/设备名/尾点尾空格被接受。
- GREEN 同命令：全部拒绝。
- RED `go test ./internal/marketplace -run TestSkillManifestMissingObjectFailsExport -count=1`：缺失快照对象被静默忽略。
- GREEN 同命令：缺失对象导致导出失败。
- RED `go test ./internal/app ./internal/marketplace -run "TestSkillPublishingWithSingleDatabaseConnection|TestGitRegistrySingleConnection" -count=1`：单连接池2秒deadline分别返回503和context deadline exceeded。
- GREEN 同命令：发布Load/Store及Git构建通过 `WithTx` 复用事务，Git Files先关闭rows再读对象，两项通过。
- RED `go test ./internal/app -run TestSkillRoutesExtendOnlyBoundedFileDeadlines -count=1`：write deadline为零。
- GREEN 同命令：仅技能上传扩展read60秒/write120秒、文件下载write120秒，未取消或扩大其他API全局超时。

REFACTOR：将旧文本GitHub解析函数收敛到同一V2解析器，删除旧256KiB/Contents路径；旧接口仅做无损文本适配。将技能发布互斥锁移到读取旧快照之前，避免并发附件更新丢失。现有GitHub fixture适配tree/blob协议，保留原有断言。聚焦 `go test ./internal/app -run "TestGitHubSkillSyncPublishesStableSnapshot|TestSkillBinaryRoundTrip|TestConcurrentSkillAndBundleEditing" -count=1` 通过。

追加回归覆盖：非法base64、非法UTF8 SKILL.md、目录穿越、缺少S3时上传失败不发布、数量/8MiB/32MiB限额、Git symlink/submodule/特殊权限拒绝。

## 验证结果

`go test -race ./internal/app ./internal/marketplace ./cmd/loadout-export ./cmd/loadout-server -count=1` 通过（app 209.632s、marketplace 4.047s、server命令2.971s；export命令编译且无独立测试）。此整包回归开始后追加的单连接/局部deadline修改又经 `go test -race ./internal/app ./internal/marketplace -run "TestSkillBinary|TestSkillPublishingWithSingleDatabaseConnection|TestSkillRoutesExtendOnlyBoundedFileDeadlines|TestGitRegistrySingleConnection|TestGitRegistryStoresLargeObjectsByReference|TestSkillManifest|TestGitHubSkillBinary|TestSkillFilesRejectWindowsAliases|TestSkillFileLimitsAndGitLinks" -count=1` 通过（app 6.294s、marketplace 2.711s）。

完整 `make check` 由主任务统一运行；本子任务未提交、推送、发布、操作真实客户端配置或真实云资源。GitHub协议用本地HTTP fixture验证，未声称真实第三方联调。协议依据：[GitHub Git trees](https://docs.github.com/en/rest/git/trees?apiVersion=2022-11-28) 的mode/sha/truncated语义和 [Git blobs](https://docs.github.com/en/rest/git/blobs?apiVersion=2022-11-28) 的二进制blob读取。巨大仓库返回truncated会明确失败，当前不递归分页获取整个仓库。
