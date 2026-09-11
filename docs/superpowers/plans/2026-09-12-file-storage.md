# 通用文件存储与技能二进制实现计划

用户授权：将文件存储做成通用基础能力，支持多家S3兼容服务，并修复技能二进制链路。当前工作区实现、保留其他任务修改，不提交/发布，不操作真实云资源或客户端配置。

## 设计与接口

- `internal/filestore`：`Ref{SHA256 string, Size int64}`、`Store`；`New(pool, Options) (*Store,error)`，`Put(ctx, []byte) (Ref,error)`、`Get(ctx, Ref) ([]byte,error)`。小文件默认≤256KiB保存bytea，大文件走私有S3，未配置时明确失败；通用对象上限16MiB（为技能8MiB的Git编码预留空间）。对象以SHA256去重，下载重验大小/哈希，不将云凭据或签名URL写入市场清单。
- 双轨迁移 `028_file_objects.sql` 保存摘要、大小、数据库字节或S3位置（bucket/key以及识别目标的endpoint/region），失败不发布引用。029使大Git对象持久引用同一文件存储，避免大文件又复制回数据库。030为已经应用旧028的数据库同步提升对象上限至16MiB，双方言升级测试保留内容和Git引用。配置来自环境，SDK签名，默认HTTPS；MinIO本地HTTP显式允许；无公开原始对象路由，业务层完成权限检查后读取。
- 市场spec新增 `file_manifest: {path:{sha256,size,executable}}`；旧`files: {path:text}`继续读取。新导入用原始字节，文件最多32个、单文件8MiB、整体32MiB；SKILL.md必须有效UTF-8。路径不可逃逸、禁止链接；可执行权限仅普通文件的0644/0755语义。
- 管理员写入支持 `files_v2: {path:{encoding:"base64",content:string,executable:boolean}}`；已有编辑器只更新SKILL.md时保留附件引用。新版本通过存储引用发布；历史损坏二进制不能自动恢复，需重新导入。
- `GET /api/v1/marketplace/{slug}/files?format=2` 返回 `{files:{path:{encoding:"base64",content,sha256,size,executable}}}`。旧接口仅对可无损文本返回字符串，否则明确要求升级，禁止悄悄损坏文件。CLI请求format=2，同时兼容旧服务字符串，安装前校验全部文件，按字节原子写入并恢复可执行位。
- Git市场和离线导出读取同一存储，保持二进制与执行位；失效/缺失对象不输出损坏成功。公共Git仅发布市场已有的公开技能，存储本身不开放。
- S3兼容范围用同一个AWS Go v2适配器：AWS、R2、MinIO、OSS S3端点、DigitalOcean Spaces、Backblaze B2，endpoint/region/path-style可配。兼容配置与真实供应商联调分别陈述；未提供云账号不宣称真实联调通过。

## 实施与验收

1. 存储层：先RED二进制DB跨实例/去重、S3成功失败/内容校验/配置，再实现与双方言迁移。
2. 技能链路：先RED含0/非UTF8附件的导入、保存、下载、导出/权限位，再接入清单；保持旧文本兼容。
3. CLI：先RED二进制安装、哈希不符拒写、路径/尺寸/权限，再实现format=2与旧格式兼容。
4. Web：支持附件文件上传、清单展示/错误恢复，SKILL.md文本编辑保留；更新Zod和中英文，不在浏览器执行附件。
5. 同步API/OpenAPI/PLAN/ENVIRONMENT/.env.example/README/AGENTS；相关测试、review及完整make check，记录真实限制。

## 执行记录

官方依据：AWS Go v2端点/校验文档；R2 S3示例（region auto）；OSS 2026-04-21 AWS SDK兼容文档（使用s3.oss-{region}端点）；Spaces/B2 S3兼容文档。只实现PutObject/GetObject所需交集，不引入ACL、浏览器直传或分片上传。

### Web与审查阶段

- RED `pnpm --dir web exec vitest run src/SkillAttachments.test.tsx`：缺少添加附件/目录控件，2项行为失败（.loadout/file-ui-red.log）。实现原始Base64读入、可执行位、明确删除、错误恢复与Zod清单字段。
- 第二个RED：33文件选择测试观察到先读取再拒绝（.loadout/file-ui-limit-red.log）；将路径/文件数/总体积校验移到任何FileReader之前。
- 第三个RED：Windows保留名路径未被拒绝（.loadout/file-ui-path-red.log）；复用safeFilePath完善可移植名称约束。
- GREEN `pnpm --dir web exec vitest run src/SkillAttachments.test.tsx src/Marketplace.test.tsx src/Localization.test.tsx`：32项通过（.loadout/file-ui-green.log）。随后用户另一个“优化技能编辑体验”任务正在整合SkillFileEditor，已协调其统一负责FileAttachments/MarketplaceEditor，保留本任务文件协议和边界；最终build/check等待共享工作区稳定。
- 核心存储RED/GREEN/SDK依据见2026-09-12-file-storage-core.md；技能链路见2026-09-12-skill-binary.md；CLI见2026-09-12-file-cli.md。
- review发现CLI5秒响应超时不适合大文件、Windows大小写/目录别名可能覆写；均已代理TDD修复，CLI files请求单独60秒、45MiB读取上限保留。
- review发现事务持连接时filestore再次借同池连接可死锁；核心新增WithTx事务复用，技能发布/Git构建使用同一事务连接，Git列表先释放rows再读取对象，以单连接池回归。
- 云对象生命周期明确：更新/删除技能只改清单，历史Git仍需对象；不做盲目自动删除或回收。云目标迁移另行复制校验，不改配置冒充数据迁移。
### 最终整合与验证

- 只读复审确认 WithTx 事务复用、关闭结果集后读取、030 双轨升级和有界文件请求超时，无剩余阻塞。复审未冒充新测试结果。
- 第一轮完整 `node --env-file=.env .loadout/tool-editor-check.mjs` 在 Go import 分组格式检查失败；针对两个二进制测试文件运行项目 golangci-lint fmt 修复。
- 第二轮在另一用户任务正在修改的 Marketplace.test.tsx 格式/非空断言检查停止；已协调等待该管理员界面任务冻结后重跑。日志 `.loadout/file-storage-check.log` 保存最终一次运行输出。
- 实际验证环境为本机 PostgreSQL 17.6、Redis 8.2.1；未提供云账号，未进行六家云服务真实联调；按用户约束不使用浏览器自动化，未声称完成桌面/移动视觉人工验收或 Windows 原生构建。- 第三轮因另一 GitHub 同步任务新增测试尚未格式化停止；只格式化 Marketplace.test.tsx 后，最终第四轮完整 `node --env-file=.env .loadout/tool-editor-check.mjs` **退出0**（内部 `make check`），日志 `.loadout/file-storage-check.log`。Go全包race（app181.090s、filestore6.040s、marketplace9.455s）、CLI、Web219项/31文件、桌面测试、Linux deb构建、真实服务生产/开发前端流程、进程恢复、dev/reload与HTTP/CLI集成均通过。
- 全量期间另一个用户任务新增所有 SidePanel 全屏。其冻结后追加 `pnpm exec biome check --error-on-warnings . && pnpm --dir web exec vitest run && pnpm --dir web build && git diff --check`，**退出0**，最新Web32测试文件通过，日志 `.loadout/file-final-web.log`；仅UI变化，因此不重复已通过且未变的Go与CLI检查。
- 所有协调任务已收到最终日志与退出码；未提交、推送、发布或配置真实云凭据。