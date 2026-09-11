# PluginPocket 全量更名执行计划

用户已确认 PluginPocket（插件口袋），并明确不需要旧版本兼容，可作为全新项目。英文口号 Your AI superpowers, in your pocket.；中文口号 把 AI 的超能力，装进口袋。

## 最终范围

全量更名：品牌、CLI/crate、Go module/import、桌面标识、服务名、环境变量 PLUGINPOCKET_*、新配置目录 .pluginpocket、客户端条目、镜像/发行产物、双方言数据库对象与迁移、测试和当前文档。令牌前缀改为 ppt_。不添加旧入口或配置迁移。现有真实数据库、私密 .env、客户端配置保持原状，新实例按新模板配置。保留旧本地运行目录的 gitignore，避免改名后误收集私密数据。

用户随后批准将原仓库改名。GitHub API 已确认 Yanyutin753/pluginpocket；本地 origin 与当前文档同步，发布下载 URL 使用 github.repository。线上域名未改。历史计划与 ADR 保留原文作为历史证据。

## 步骤

- [x] 先验收双语首页/登录的新品牌、桌面令牌交互与新 CLI 入口，确认 RED。
- [x] 全量更名源码、配置及路径；同步文档，保留用户既有修改。
- [x] 聚焦 GREEN 后运行相关回归、构建与 make check；审查未验证范围。

## RED

- CLI: cargo test --manifest-path cli/Cargo.toml --locked --test brand，失败于 pluginpocket executable must be installed。
- Web: pnpm --dir web exec vitest run src/features/LandingPage.test.tsx src/Preferences.test.tsx，3 失败/3 通过，新品牌导航、登录标题及英文口号尚不存在。
- Desktop: pnpm --dir desktop/ui exec vitest run src/App.test.tsx，4 失败/3 通过，新品牌令牌 label 尚不存在。
- 初次 pnpm PATH 不完整及 shell PATH 引用错误属于环境准备失败，不算 RED。有效运行通过固定 Node 26.8.2 路径完成。

## GREEN / 审查阶段记录

- pnpm install --frozen-lockfile 通过，未变更依赖版本；离线首次失败为供应链元数据未缓存。
- Web 全套：33 文件、230 测试通过；桌面 UI：7 测试通过；CLI 全套与桌面原生测试通过。
- Web 与 desktop/ui 的 TypeScript/Vite 生产构建通过。
- 初次 GREEN 的品牌测试把 version 输出误写为仅数字，实际约定为“命令名 版本”；修正期望为 pluginpocket + Cargo版本，保留完整品牌断言。两条旧口号统一后重复的翻译键已去重。
- make check 已通过四端 lint 后执行完整测试；Go race 不带数据库的独立运行通过，但不当作数据库验收证据。最终结果待下方记录。
- 独立 reviewer 两项问题已核对并修复：CI 上传旧二进制路径；Docker 忽略规则必须同时排除旧/新本地运行目录。旧目录只作为私密数据排除，不提供运行兼容。
- 新仓库名称经用户二次确认大小写为 Yanyutin753/PluginPocket；GitHub API 已完成更名和简介更新，origin 已更新，未推送代码、未触发发布、未改线上域名。
- 原生桌面盾牌/L图标改为同色系口袋/P几何SVG，并用 pnpm --dir desktop/ui tauri icon 派生 PNG/ICO/ICNS；人工查看128px图标。签名密钥未移动，文档使用明确的自填绝对路径。
- Web新字标20px，展开侧栏264px；设计规范同步。未使用浏览器自动化，尚未进行真实桌面/手机页面视觉验收。

## 图标最终调整

用户要求使用网页图标，已移除中途设计的P字标及desktop/icon.svg。最终从web/public/icon-512.png原图通过Tauri官方icon工具派生desktop/icons中的五个实际引用文件；源图不改动，128px结果已人工查看。README、设计与桌面说明以最终工具箱图标为准。

完整验证使用已有独立测试数据库/Redis地址，通过临时Node runner仅映射测试连接变量为PLUGINPOCKET_TEST_*；不写入或修改.env，不在记录输出凭据。应用运行不支持旧环境变量；开发者需按.env.example配置新实例。

## 最终验证

- 完整 make check 通过：真实 PostgreSQL/Redis、四端lint、Go race/CLI/Web测试、Linux deb打包、生产/开发Web HTTP旅程、真实MCP/计量/多实例/故障恢复、开发进程生命周期与HTTP集成。日志/tmp/pluginpocket-check.log，末尾4项HTTP集成通过，无make失败。
- 最终仓库大小写调整后另跑 make lint，通过（Go 0 issues、TypeScript、Biome、rustfmt/clippy）。
- 最终网页同款图标另跑 make build-desktop，通过；生成desktop/target/release/bundle/deb/PluginPocket_0.1.0_amd64.deb。
- PLUGINPOCKET_DESKTOP_TEST_BIN 指向最终release二进制的cargo test --manifest-path desktop/Cargo.toml --locked --test native_bridge，通过。
- 读取最终deb归档逐字节比较32/128/256px图标与工作区最终图标，三者一致。README中英文各29个本地引用存在，git diff --check通过。
- 未执行浏览器自动化；真实桌面/移动浏览器视觉、Windows/macOS原生GUI与线上部署未验收。未提交、未push；远程仅改名与简介，本地目录仍为当前会话工作区路径。
