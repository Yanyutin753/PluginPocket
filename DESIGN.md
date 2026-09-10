---
name: Loadout
description: 深色开发者工具；真实状态、清晰边界、直接操作。
colors:
  background: '#0c1017'
  foreground: '#edf1f7'
  card: '#131a24'
  muted: '#1c2532'
  muted-foreground: '#a3b1c4'
  border: '#303e51'
  primary: '#8eb8ff'
  primary-foreground: '#10213c'
  success: '#80dcb3'
  destructive: '#ff9d9d'
typography:
  headline:
    fontFamily: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei",
      sans-serif
    fontSize: 1.875rem
    fontWeight: 600
    lineHeight: 2.25rem
    letterSpacing: -0.025em
  title:
    fontFamily: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei",
      sans-serif
    fontSize: 1.25rem
    fontWeight: 500
    lineHeight: 1.75rem
  body:
    fontFamily: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei",
      sans-serif
    fontSize: 1rem
    fontWeight: 400
    lineHeight: 1.75rem
  label:
    fontFamily: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei",
      sans-serif
    fontSize: 0.875rem
    fontWeight: 400
    lineHeight: 1.5rem
  code:
    fontFamily: ui-monospace, SFMono-Regular, Consolas, monospace
    fontSize: 0.875rem
    fontWeight: 400
    lineHeight: 1.5rem
rounded:
  md: calc(0.75rem - 4px)
  lg: 0.75rem
spacing:
  '2': 0.5rem
  '3': 0.75rem
  '4': 1rem
  '5': 1.25rem
  '6': 1.5rem
  '8': 2rem
  '10': 2.5rem
  '14': 3.5rem
components:
  button-primary:
    backgroundColor: '{colors.primary}'
    textColor: '{colors.primary-foreground}'
    rounded: '{rounded.md}'
    height: 2.25rem
    padding: 0.5rem 0.75rem
  button-primary-hover:
    backgroundColor: 'color-mix(in oklab, #8eb8ff 90%, transparent)'
  status-panel:
    backgroundColor: '{colors.card}'
    textColor: '{colors.foreground}'
    rounded: '{rounded.lg}'
    padding: 1.5rem
  command-block:
    backgroundColor: '{colors.card}'
    textColor: '{colors.foreground}'
    typography: '{typography.code}'
    rounded: '{rounded.lg}'
    padding: 1.25rem
  separator:
    backgroundColor: '{colors.border}'
    height: 1px
    width: 100%
---

# Design System: Loadout

## Overview

**Creative North Star: "深色开发者工具"**

采用用户确认的深色开发者工具风格：冷色深灰背景、克制的蓝色动作、清晰文字与边界。中文内容以系统无衬线字体为主，等宽字体用于代码与版本。当前基线从已实现的单页提取，后续页面沿用这些视觉关系。

**Key Characteristics:**

- 深色底面与轻微提亮的容器形成层级。
- 蓝色标识动作，状态以文字、图标和颜色共同表达。
- 单一阅读轴、适中密度、窄屏自然排列。

## Colors

主色为清冷浅蓝，中性色从近黑背景逐步提亮到正文。上方 frontmatter 的值与 `web/src/styles.css` 对应；颜色别名不另建一套色值。

- **Primary**：`primary` 用于主要动作、品牌图标、焦点与选择高亮；`primary-foreground` 确保蓝底按钮文字清晰。
- **Neutral**：`background` 是页面底面，`card` 是状态与命令容器，`muted` 是基础组件的弱强调底面；`foreground` 为正文，`muted-foreground` 为辅助文字，`border` 区分区域。
- **Status**：`success` 表达已连接；`destructive` 表达连接错误，二者均与不同图标、文字配合。

## Typography

主标题使用 headline，状态标题使用 title，介绍段落使用 body，辅助说明使用 label，终端命令使用 code。其余小标题沿用正文或较大正文层级；品牌和流程区域标题为 1.125rem。标题不使用等宽体，版本值使用平台等宽字体。介绍和说明最大行宽为 65ch。

## Layout

容器最大宽度为 64rem，居中；页面左右留白在窄屏为 1.25rem，从 40rem 断点起为 2rem。主内容垂直留白从 2.5rem 变为 3.5rem，区域间隔为 2.5rem。主间距以 4px 为基数，紧密关联的标题与说明使用较小间隔。

当前页面按标题、连接状态、流程说明、CLI 命令排列。状态容器内边距从 1.5rem 变为 2rem；宽屏状态与按钮并排，窄屏垂直排列。三步说明从一列变为三列。版本信息允许断行，代码块独立横向滚动，不扩张页面宽度。具体页面意图见 PRODUCT.md 和表面 brief。

## Elevation & Depth

当前页面没有投影。深度来自底面与容器的明度差、细边框和留白。主要按钮的键盘焦点使用半透明主色光环；这是交互反馈，不是容器阴影。

## Shapes

容器与代码块使用 lg 圆角，按钮使用 md 圆角；边框和分隔线为 1px。Lucide 线条图标遵循既有组件大小：品牌 20px、状态 24px、按钮与终端标识 16px。

## Components

- **主要按钮**：消费 shadcn Button 的 default 变体；上方 padding 对应当前含图标按钮，纯文字默认水平内边距为 1rem。字号 0.875rem、字重 500、行高 1.25rem。hover 降低底色不透明度；focus-visible 为 3px 主色半透明环；disabled 降至 50% 不透明度且禁止触发。保留原生按下行为，没有额外缩放动画。
- **状态容器**：使用 card、细边框、圆角，信息区以 Separator 分隔。polite live region 呈现检查中、已连接、连接错误；检查时禁用按钮并隐藏旧版本，错误提示确认服务启动后重试。
- **命令块**：与状态容器同色，等宽文字、独立滚动和键盘焦点；通用焦点轮廓为 2px 主色、外偏移 4px。
- **分隔线**：消费 shadcn Separator，使用 border 颜色，不添加装饰阴影。

已引入的 Button 还保留官方生成的其他变体 API；当前页面只消费 default，未使用变体不构成新增产品交互。页面尚无表单输入、导航或标签组件，不提前制定它们的视觉规则。交互过渡采用 Tailwind 默认 150ms 与 cubic-bezier(0.4, 0, 0.2, 1)；reduced-motion 时关闭动画和过渡。

## Do's and Don'ts

- Do 使用语义颜色 token，并保持中文、长版本号与代码可读。
- Do 为键盘焦点保留明显反馈；可横向滚动的命令区域必须可聚焦。
- Do 同时提供状态文字与图标，错误给出可执行的恢复指引。
- Don't 把成功、错误或检查中的区别只交给颜色。
- Don't 用不可操作的导航或虚构业务数据填充当前基建页面。
- Don't 添加入场动画；交互过渡遵循 reduced-motion。
