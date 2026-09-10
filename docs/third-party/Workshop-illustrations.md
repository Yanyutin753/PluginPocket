# 工坊多元插画生成记录

五张图片使用内置 ImageGen 分别生成，每次调用一个资产。风格参考为本项目原创 workshop-welcome，未引用第三方角色或标志。统一哑光陶瓷材质、薄荷绿/黄色/浅蓝/珊瑚色；每张主体不同。按用户明确授权，统一纯白背景经本地连通区域抠图、边缘去底色和轻度抗锯齿处理，保留原画主体；输出真实 RGBA WebP，无假棋盘格。

原始 PNG 保留在 Codex generated_images/01a08c04-7900-7440-98c9-cbf7c196ba29；应用只依赖仓库内 web/public/images。全部在浅色 #f5f5f7 与深色 #161719 背景逐张人工检查；眼睛、高光保持不透明，钥匙与锁孔等背景区域已清除。头像为默认展示图，不表示用户真实外貌。

| 文件 | 尺寸 | bytes | alpha=0 像素 | 半透明像素 |
|---|---|---:|---:|---:|
| workshop-team.webp | 900×600 | 67014 | 294704 | 10827 |
| workshop-connect.webp | 900×600 | 52184 | 359528 | 12661 |
| workshop-insights.webp | 900×600 | 43634 | 324185 | 8459 |
| workshop-security.webp | 900×600 | 49734 | 307752 | 9852 |
| workshop-avatar.webp | 256×256 | 12530 | 33063 | 2488 |

## team

原始生成文件：exec-769ec381-ab7c-41ab-a15b-80ca37b2f815.png

最终提示词：

```text
Generate ONE 3:2 landscape production spot illustration, 1350x900. Reference image STYLE ONLY: premium matte ceramic toy forms, fine delicate black contours, soft studio shading, mint/cream yellow/pale blue/coral accents. NEW SUBJECT: exactly THREE different friendly little ceramic companions (one mint rounded blob, one buttery cream rounded cuboid, one pale blue short cylinder), each with expressive eyes and tiny smile, collaborating to assemble a coral and yellow modular tool mechanism. One holds a small wrench, one fits a golden gear, third steadies the shared module. No toolbox, no platform. Clearly separated cute silhouettes, cooperative composition. Centered all visible with 10% margin. Pure uniform WHITE #ffffff backdrop, no texture/no checkerboard/no ground shadow. Every object has fine closed dark contour, white eyes remain enclosed. No text, no metrics, no UI, no logos.
```

## connect

原始生成文件：exec-81963f6b-b63b-463b-a92f-270585c521d3.png

最终提示词：

```text
Generate ONE 3:2 landscape premium matte ceramic toy product spot illustration 1350x900. Reference is MATERIAL/STYLE ONLY. Totally new subject, NO character faces and NO toolbox: a miniature abstract connection harbor, three rounded ceramic module docks mint green/pale blue/coral joined by short curved golden cables and chunky tiny plugs, central ivory rounded hub with three clean socket openings, one golden connector visibly hovering just before its socket. Elegant low isometric perspective, thoughtfully arranged asymmetrical compact horizontal composition, clear premium iOS illustration, delicate dark contours, gentle satin shading. All modules physically plausible and distinct. Uniform pure WHITE #ffffff background, NO floor or ground shadow, no texture, no checkerboard. Fine dark closed outlines around all objects and cables for cutout, 10% margins. NO words, logos, labels, numbers, characters or UI.
```

## insights

原始生成文件：exec-776a288f-265d-4e32-be62-e4aa8738d80b.png

最终提示词：

```text
Generate ONE 3:2 landscape premium matte ceramic toy spot illustration 1350x900. STYLE reference only: delicate dark contours, refined satin shading, mint/cream yellow/pale blue/coral palette. NEW SUBJECT no characters or toolbox: a compact sculptural abstract analytics still life, three folded ceramic graph columns in different heights with rounded paper-fold edges (mint, blue, coral), a small ivory accordion-fold chart sheet beneath, and a large golden-rim magnifying glass leaning diagonally across one corner. Glass is lightly transparent pale blue with clearly outlined rim; no values or letters or labels or arrows. Thoughtful asymmetric spatial composition, data exploration rather than success promise. Pure uniform WHITE #ffffff backdrop, no ground shadow, no texture, no checkerboard, all art fully visible with10% margins. Fine dark outlines enclose all objects including ivory sheet. NO numbers, typography, fake metrics, UI, logos.
```

## security

原始生成文件：exec-6073c24c-f3f7-4220-b6c6-2a1fbae3befa.png

最终提示词：

```text
Generate ONE 3:2 landscape premium matte ceramic toy security spot illustration1350x900. Reference STYLE ONLY. New subject: a cute mint green three-dimensional shield character with tiny friendly eyes and gentle smile centered slightly left, leaning slightly, accompanied by one large buttery yellow key diagonally in front and one small pale blue rounded padlock to right with a coral keyhole detail. Clearly security/access management metaphor, not toolbox, no other characters. Premium iOS product illustration, refined satin ceramic materials, thin delicate black contours, gentle highlights. Balanced compact horizontal composition, all visible with 12% margins. Uniform pure WHITE #ffffff backdrop, no ground shadows, no texture, no checkerboard. All object edges enclosed with dark contours, lock shackle/key holes genuinely open to background. NO text, numbers, logos, UI or checkmark badges.
```

## avatar

原始生成文件：exec-07600a31-f6c3-4660-be5b-f6f3d09ff3f9.png

最终提示词：

```text
ONE square 1024x1024 avatar illustration, premium matte ceramic toy style matching reference. A NEW friendly mint green companion head and upper chest, front facing, soft rounded head with two big simple expressive black oval eyes, tiny happy smile, wearing a small buttery yellow beanie. Friendly original AI workshop teammate, NO toolbox, NO tools, NO blocks. Strong simple silhouette, perfectly readable at32px, fine delicate dark outlines, restrained smooth soft shading and premium clay/ceramic surface. Head occupies central75% of square with safe margins. Uniform pure WHITE background, no shadow, no texture, no checkerboard, no circle border. No text, logos, UI. Keep whites of eyes enclosed by dark outlines.
```

