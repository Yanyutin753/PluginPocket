# 文件存储与 S3 兼容配置

`server/internal/filestore` 是通用内容存储：`Put(ctx, bytes)` 返回 SHA-256/大小引用，`Get(ctx, ref)` 校验完整性后返回原始字节。技能发布、下载、Git 仓库大对象和离线导出复用这层。库不提供公开对象ID下载接口；业务先检查权限，再读取。业务已有数据库事务时使用 `WithTx(tx)`，让文件元数据操作复用该事务和连接，避免连接池耗尽。发布到插件市场的技能和附件会通过公共Git源发布，请勿上传秘密。

默认不配置云服务时，≤256 KiB 文件放 PostgreSQL `bytea`；超过阈值必须配置私有S3存储，失败不会静默落回数据库。对象按SHA-256去重；通用层单对象上限16MiB，技能另限每文件8MiB、32文件/32MiB。Git压缩对象与原文件的编码不同，会形成独立内容对象。图标仍沿用64KiB小图方案；不会对技能附件做有损压缩。

## 配置

变量统一以 `PLUGINPOCKET_FILE_STORAGE_` 开头，服务端读取，重启生效；所有副本使用同一个数据库和同一云目标。

| 后缀 | 默认 / 说明 |
|---|---|
| `INLINE_MAX_BYTES` | `262144`；1–16777216。只影响新内容的存放选择，已有对象不迁移 |
| `ENDPOINT` | 空为AWS区域端点；其他服务填HTTPS origin，不含bucket、路径、查询或凭据 |
| `REGION` | S3签名区域；启用云存储时必填 |
| `BUCKET` | 预先创建的私有bucket；留空关闭云存储 |
| `ACCESS_KEY_ID` / `SECRET_ACCESS_KEY` | 云访问凭据，成对配置；只需目标前缀的PutObject/GetObject权限 |
| `SESSION_TOKEN` | 可选STS会话令牌；当前使用显式凭据，不自动读取IAM角色 |
| `PREFIX` | 对象键前缀，例如`pluginpocket/prod/files`，默认空 |
| `PATH_STYLE` | `false`；MinIO等需要path-style时设true |
| `ALLOW_HTTP` | `false`；只在受控本地MinIO测试使用true，不关闭TLS证书校验 |

使用官方AWS Go SDK v2，签名、服务协议、有限重试交给SDK。仅使用PutObject/GetObject交集，不设置公共ACL，不依赖供应商特有元数据；HTTP请求30秒上限。SDK自动校验采用when-required以兼容第三方，应用层仍始终校验SHA-256和大小。

## 常见服务

| 服务 | ENDPOINT示例 | REGION | PATH_STYLE |
|---|---|---|---|
| AWS S3 | 留空 | bucket所在区域，如`us-east-1` | false |
| Cloudflare R2 | `https://<account-id>.r2.cloudflarestorage.com` | `auto` | true |
| MinIO | `https://minio.example.com` | 实例区域，常见`us-east-1` | true |
| 阿里云 OSS S3接口 | `https://s3.oss-cn-hangzhou.aliyuncs.com` | `cn-hangzhou` | false |
| DigitalOcean Spaces | `https://nyc3.digitaloceanspaces.com` | `us-east-1`（按SDK接入说明） | false |
| Backblaze B2 | `https://s3.us-west-004.backblazeb2.com` | `us-west-004`（按实际bucket） | false |

OSS必须使用其S3兼容端点；原生OSS API签名不是本适配器的目标。OSS部分中国内地新bucket另受默认外网域名访问限制，应按供应商要求配置可用端点。表内区域和域名是示例，需替换为自己的真实配置。

官方参考：[AWS Go端点配置](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html)、[AWS校验策略](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/s3-checksums.html)、[R2 S3](https://developers.cloudflare.com/r2/get-started/s3/)、[MinIO SDK](https://docs.min.io/aistor/developers/sdk/go/)、[OSS AWS SDK兼容](https://www.alibabacloud.com/help/tc/oss/developer-reference/use-aws-sdks-to-access-oss)、[Spaces](https://docs.digitalocean.com/products/spaces/reference/s3-compatibility/)、[B2](https://www.backblaze.com/docs/en/cloud-storage-call-the-s3-compatible-api)。配置兼容不等于真实云联调：自动化覆盖本地HTTP S3协议替身、签名、二进制读写与故障，没有替用户创建云bucket或验证这些供应商账号。

## 生命周期与恢复

数据库记录摘要、大小与bucket/key/endpoint/region，不存云凭据。改prefix不影响旧对象读取；改bucket、endpoint或region会对旧云对象明确拒读，不能把切换配置当作数据迁移。迁移云目标时先复制对象并验证摘要，再设计元数据切换；本轮没有自动迁移工具。

删除技能附件只移除当前清单引用，历史Git提交仍可能需要旧内容，故不立即删除底层对象。上传成功但业务事务回滚可能留下可复用的无引用对象；当前不做自动垃圾回收，不能对该前缀配置盲目过期删除。备份必须同时覆盖PG与云对象，恢复时保持引用一致。

历史文本技能继续可读；历史被字符串编码损坏的二进制无法凭空恢复，需重新同步原仓库。旧CLI遇到二进制技能会收到明确错误，应升级CLI；新CLI兼容旧文本服务。
