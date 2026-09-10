---
name: Loadout · AI 装备工坊
description: 给你的 AI，装上超能力。
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

# Design System: Loadout

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
- 品牌字标自托管 Bricolage Grotesque，750 字重、opsz 48、wdth 100、字距 -0.045em；只作用于 Loadout，正文不变。完整 OFL 与来源见 docs/third-party/BricolageGrotesque.md。
- 导航操作统一 14px/20px、44px 点击高度。关键操作和选择文字保持单行；小屏允许整组调整位置。
- 选择框使用共享 Radix Select；前置图标在触发按钮内，图标/文字/箭头间距 8px，两侧内边距 12px（手机偏好控件 8px）。鼠标选中只显示勾选，键盘使用中性焦点线，浅色 #62646c、深色 #b2b4bd。
- 桌面侧栏支持 228px 展开与 80px 图标模式（更大断点原展开宽度规则保留）；移动端用原导航按钮。右上角账号菜单支持 Escape 返回焦点，左下角另有账号设置入口。
- 控制台页脚以 1360px 容器与主内容对齐，品牌图标/字标与服务状态入口组成；手机隐藏非关键英文口号。
