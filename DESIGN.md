---
name: PluginPocket · AI 装备工坊
description: 把 AI 的超能力，装进口袋。
colors:
  background: "#f5f5f7"
  foreground: "#202124"
  card: "#ffffff"
  sidebar: "#ededf0"
  muted: "#e9e9ee"
  muted-foreground: "#62646c"
  border: "#d8d9df"
  input-border: "#868993"
  primary: "#f7b500"
  primary-foreground: "#202124"
  link: "#765000"
  focus: "#62646c"
  nav-active: "#ffe5a0"
  success: "#34674b"
  destructive: "#b13d32"
typography:
  brand:
    fontFamily: Bricolage Grotesque, Manrope, system-ui, sans-serif
    fontSize: 20px
    fontWeight: 750
  code:
    fontFamily: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace
    fontSize: 13px
  display:
    fontFamily: Manrope, PingFang SC, Microsoft YaHei, system-ui, sans-serif
    fontSize: clamp(2.35rem, 4.8vw, 4.5rem)
    fontWeight: 800
    lineHeight: 1.18
    letterSpacing: -0.035em
  body:
    fontFamily: Manrope, PingFang SC, Microsoft YaHei, system-ui, sans-serif
    fontSize: 14px
  note:
    fontFamily: Caveat, cursive
    fontSize: 25px
    lineHeight: 0.95
rounded:
  md: 12px
  lg: 16px
  auth: 32px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-foreground}"
    rounded: "{rounded.md}"
    height: 44px
  workshop-action:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-foreground}"
    height: 58px
---

# Design System: PluginPocket

所有侧面板统一使用标题栏全屏/还原按钮，不再限于技能。只读详情同样支持，按钮使用通用“全屏”文案；全屏还原不绕过提交锁定。

技能编辑标题栏使用 Maximize2/Minimize2 图标切换浏览器内容区全屏，退出后恢复原宽度；保持文件草稿和焦点，全屏下 Escape 只还原，避免直接关闭编辑。

技能编辑采用最大1280px宽面板：218px可折叠目录、紧凑文件标签与工具栏、带行号和语法高亮的编辑区。Markdown支持编辑/预览/分栏，图片直接预览，其他二进制显示信息与下载；预览使用当前草稿。700px以下目录缩至145px且可隐藏，分栏预览改为上下排列。工作区内提供全屏入口，底部保存操作可见；基本信息、GitHub同步和文件管理按需展开。装备组仍使用通用宽度。

工具管理本轮使用同排动作的紧凑列表，显示共享自定义图标、标识、连接类型、规则摘要和额度状态。编辑抽屉分为基本信息/连接/参数/计费四段，段标题17px、说明13px、字段间18px；图片可文件上传/粘贴SVG/HTTPS，预览和移除就近呈现。常用规则按类型显示相关字段，JSON和深入说明按需展开，底部保存栏保持可见。延续560px抽屉与手机全宽，不以单元测试代替视觉验收。

市场管理新增独立侧栏入口；使用现有 Heading、类型按钮、搜索和记录列表。技能/装备组编辑沿用 SidePanel，装备组原生复选框列出名称与类型，未解析端点的 MCP 禁选并说明原因；精选导入逐项与批量反馈真实成功/失败。窄屏动作换行、记录纵排，复用语义 token 与焦点样式。

公共插件目录插图轮更新：首屏为薄荷色双栏展台，左侧标题/搜索、右侧专属工具箱插图；手机插图缩至180×135并置顶，分类固定四列。浅色展台 token 为 `--catalog-showcase: #e6f0e9`、`--catalog-ink: #243c30`、`--catalog-secondary: #53685b`；深色对应为 `#23372e`、`#e4f2e9`、`#b4cbbd`，跟随系统复用全局已解析主题，搜索及列表跟随主题。插图在两种主题均保持原始亮度，不加整体滤镜。卡片显示名称前两个字符的装饰字标（19px）与类型图标；摘要两行，详情保留完整内容。素材 `workshop-marketplace.webp` 为1000×750 RGBA透明插图；基于原始ImageGen图经用户授权本地Python连通抠底与边缘去底色，保留角色、眼睛高光和图标，大小215644 bytes。此条替代此前目录居中无插图设计。

公共插件目录本轮参考 Apify Store：居中标题与 700px 搜索框、带真实数量的类型导航、桌面三列/平板两列/手机单列卡片。类型图标用薄荷/黄色/中性 token，展示名称、slug、三行摘要与版本/托管元信息；整卡进入详情。每页 12 项，桌面数字分页、手机前后页与当前页数，URL 保留筛选与页码。此条仅更新公共目录，不改变其他数据列表页尺寸规范。

公共目录字号：主标题 32–52px 流式，结果标题 20px，卡片标题 17px，搜索 15px（手机 14px），摘要 14px，范围提示 13px，类型/slug/版本 12px。

2026-09-11 全站精修：公共市场 `/plugins` 与详情由 React 渲染，免登录，复用首页 PublicHeader、偏好设置及控制台 Heading/列表/按钮/命令区，沿用中英文和浅/深/系统主题。目录带 URL 搜索/分类筛选、空态与重试；不使用独立服务端页面皮肤。导航柔和底面与12px圆角，指标独立底面，列表提高阅读间距，登录欢迎区轻薄荷色面，首页终端统一深色命令区。原专属页面插画与功能保持。

## Overview

**Creative North Star: "AI 装备工坊"**

用户最新明确要求以更精致的 iOS 风格为优先：中性灰白画布、柔和圆角与清晰层次，保留第一张设计稿（exec-80fb9574）的黄色动作和原创薄荷绿工具箱角色。登录与加载使用更细线条、柔和陶瓷质感的新插画；业务操作保持清晰。此方向替代上一轮奶油/橄榄中性色，不回退到旧金属雕塑或蓝灰开发者工具视觉。

参考 [awesome-design-md / PostHog](https://github.com/VoltAgent/awesome-design-md/blob/main/design-md/posthog/DESIGN.md) 的工程手册与友好插画语言；不复制其商标或角色。运行时 token 的唯一来源是 `web/src/styles.css`，此文档和 sidecar 是当前实现的提取记录。

**Key Characteristics:**

- 中性灰白画布与薄荷绿原创角色构成品牌识别。
- 黄色突出主要动作，细分隔线组织真实业务数据。
- 桌面宽松、手机自然纵排；状态与恢复操作保持可读。

## Colors

浅色 token 见 frontmatter；深色通过同名 CSS 变量覆盖，保持相同语义。

| 角色 | 深色 |
|---|---|
| background | #161719 |
| foreground | #f4f4f7 |
| card | #242528 |
| sidebar | #1c1d20 |
| muted | #303136 |
| muted-foreground | #b2b4bd |
| border | #42444b |
| input-border | #858995 |
| primary | #ffd15c |
| primary-foreground | #202124 |
| link | #ffd15c |
| focus | #ffd15c |
| nav-active | #4b4326 |
| success | #a0d6ab |
| destructive | #ffb3a6 |

主要按钮用深色文字配黄色底；正文链接用独立 link 色，输入边框用 input-border。成功与错误各有语义色。终端固定深中性灰底和浅色文字；三色终端圆点只是装饰。

## Typography

Web 优先使用自托管 Manrope Latin 可变字体（200–800），中文使用 PingFang SC / Microsoft YaHei / system-ui。桌面客户端优先 system-ui / -apple-system / BlinkMacSystemFont / Segoe UI，Manrope 和中文系统字体作为后续回退；两端字体顺序不相同。正文 14px，主视觉标题使用 frontmatter 的响应式规格。代码用 ui-monospace / SFMono-Regular / Consolas，数值使用 tabular-nums。

Caveat 自托管可变字体（400–700）仅用于英文手写注释，`font-display: swap`；手机注释 22px。许可为 SIL Open Font License 1.1，原文位于 [Caveat-OFL.txt](docs/third-party/Caveat-OFL.txt)，来源为官方 google/fonts 的 Caveat；Manrope 许可位于 `web/public/fonts/OFL-Manrope.txt`。不用手写字体承载表单、关键命令或中文正文。

## Layout

桌面 900px 起为 228px 侧栏，1280px 起为 248px；侧栏独立滚动。主内容最大 1360px 居中，避免 2560px 大屏把信息拉散。900px 以下使用可展开导航。

概览在 1100px 起左右两栏，图文比例 1:1.05；更窄时纵排。接入步骤 640px 起两列，手机一列；账号数据桌面四列、手机两列。登录/注册使用最大 1040px 的统一 auth-card，两栏分别为欢迎场景和表单。900px 以下隐藏欢迎场景，只保留最大 480px 的表单卡；不在手机表单后追加大幅插画。表格和独立代码区可局部横向滚动，页面不横向溢出。

## Elevation & Depth

界面以中性色面、1px 边界和留白区分层次。业务正文容器保持克制；登录卡使用 0 16px 64px、前景色 7% 的柔和阴影。彩铅插画保留原画的铅笔阴影；所有工坊素材使用真正透明背景，不再通过 multiply 或矩形纸面模拟融入。

## Shapes

面板 16px 圆角，按钮与输入 12px 圆角，登录外卡 32px、内部欢迎面板 22px 圆角。按钮与输入至少 44px，概览主动作 58px。轮廓清晰，避免装饰性卡片套卡片。

## Components

- 原创工具箱品牌图标约 42px；装饰图像使用空 alt，不代替控件标签。
- 概览主入口创建网关令牌；接入说明为有序列表，命令完整可复制，失败提供恢复。
- 表单和列表复用 shadcn 基础组件；loading/error/empty/success 均有真实状态。
- 登录欢迎区使用 workshop-welcome：小白色圆台、手持星星的角色与两块 API/code 积木。
- 页面加载使用 workshop-loading 连接插头场景、可访问的加载标题与骨架占位；不是定时模拟进度，不掩盖真实错误。
- 命令区使用深色背景、三色圆点和小角色；手写注释只作局部点缀。
- 焦点为 2px focus 色轮廓，偏移 4px；状态色过渡约 160ms，reduced-motion 时关闭。

当前生产插画均位于仓库内，尺寸和字节实测：

| 文件 | 尺寸 | bytes | 通道 |
|---|---|---:|---|
| web/public/images/workshop-credits.webp | 900×600 | 47,882 | RGBA |
| web/public/images/workshop-empty.webp | 900×600 | 35,234 | RGBA |
| web/public/images/workshop-hero.webp | 1200×900 | 292,646 | RGBA |
| web/public/images/workshop-loading.webp | 800×600 | 44,136 | RGBA |
| web/public/images/workshop-mark.webp | 256×256 | 17,076 | RGBA |
| web/public/images/workshop-tools.webp | 900×600 | 55,212 | RGBA |
| web/public/images/workshop-welcome.webp | 1200×900 | 69,026 | RGBA |

Hero 按 alpha > 8 的完整画迹边界重构画布，保留铅笔阴影，约 3% 最小透明留白，固定 4:3；未裁掉物体。新增 welcome/loading 由内置 ImageGen 参照原创角色生成，统一纯底后经用户授权的本地连通抠图、边缘去底色生成 RGBA；两张均在浅/深底合成检查，未用假棋盘格。生成原件分别为 `exec-c0037a9b-bf86-40dc-b4d8-81be96703760.png` 和 `exec-4a18a3e2-5fa6-4236-a481-5076fcb21357.png`，保留在 Codex generated_images；应用不依赖该本地目录。旧素材不再用于工坊页面。

## Do's and Don'ts

- 使用语义颜色、可见键盘焦点和真实业务数据。
- 插画保持透明 alpha，保留眼睛、高光及彩铅阴影。
- 错误给出恢复动作，状态同时使用文字和图标。
- 不回退到旧蓝灰或金属雕塑视觉。
- 不把关键说明只放进插画或只用颜色表达。
- 不以 DOM 测试替代桌面和移动端视觉检查。


## 2026-09-11 细节更新

- 公共首页 `/` 为产品落地页；工作空间概览位于 `/overview`，登录默认进入概览。
- 品牌字标自托管 Bricolage Grotesque，750 字重、opsz 48、wdth 100、字距 -0.045em；只作用于 PluginPocket，正文不变。完整 OFL 与来源见 docs/third-party/BricolageGrotesque.md。
- 导航操作统一 14px/20px、44px 点击高度。关键操作和选择文字保持单行；小屏允许整组调整位置。
- 选择框使用共享 Radix Select；前置图标在触发按钮内，图标/文字/箭头间距 8px，两侧内边距 12px（手机偏好控件 8px）。鼠标选中只显示勾选，键盘使用中性焦点线，浅色 #62646c、深色 #b2b4bd。
- 桌面侧栏支持 228px 展开与 80px 图标模式（更大断点原展开宽度规则保留）；移动端用原导航按钮。右上角账号菜单支持 Escape 返回焦点，左下角另有账号设置入口。
- 控制台页脚以 1360px 容器与主内容对齐，品牌图标/字标与服务状态入口组成；手机隐藏非关键英文口号。


### 页面场景分配

同一薄荷绿工具箱主角，内容图不跨页面重复。品牌mark/默认avatar和查询loading为共享系统元素。账号、管理、工具与客户端新增21张独立透明场景，来源清单见 docs/third-party/Workshop-account-scenes.md、Workshop-admin-scenes.md、Workshop-utility-scenes.md。客户端专用workshop-desktop；各场景均900×600，按固有比例缩放，使用同一浅深色页面底面。早期非工具箱主角的team/connect/insights/security版本仅存档，不在页面引用。


### 数据密集页面

默认一页10条，允许20/50条；分页只呈现真实记录范围及当前页，不展示未知总页数。列表分页失败保留当前数据，成功切页从列表开头阅读。空结果隐藏无效分页操作。桌面表格的数字列右对齐、使用等宽数字，表头在局部滚动时保留；手机使用同一语义表格的字段卡片布局。状态同时有文字及语义颜色，不仅依赖颜色。概览真实指标放在接入教程前，沿用原主角及专属页面插画。


### 按需右侧面板

用量详情复用 SidePanel，先展示工具、时间和计量摘要，再纵向呈现传入/传出文本；长内容局部滚动，未记录与截断采用明确文字提示。

工具列表以名称、短说明、调用成本与文字状态组织，参数进入右侧详情；工具/套餐编辑、插件市场与安装、用户调账、令牌/兑换码创建、团队操作、邮箱及系统配置复用 SidePanel（已安装的 Radix Dialog）。桌面宽 560px、窄屏全宽、标题与关闭固定、正文独立滚动；Escape 与关闭返回触发入口，提交中和一次性凭据未隐藏时阻止误关。系统配置主页面只呈现真实状态摘要。

抽屉标题 20px/700；工具记录标题沿用 15px，短说明 13px，成本和状态元信息 12px。

## PluginPocket 品牌更名

中文名为插件口袋；新字标沿用完整 Bricolage 字体，Web 为20px以适应较长名称。展开的桌面侧栏宽264px，为48px角色图标、字标和折叠按钮留足空间；窄屏继续使用原有换行导航。插画与配色保持既有工坊体系。

原生桌面与托盘图标复用Web的web/public/icon-512.png薄荷绿工具箱角色，由Tauri工具派生各平台图标，保持品牌一致。
