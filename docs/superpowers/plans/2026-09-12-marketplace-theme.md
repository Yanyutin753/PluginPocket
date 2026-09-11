# 插件市场昼夜主题适配

范围：保留目录布局、插画、内容与交互，公共目录和工作空间目录共同适配现有 light/dark/system 偏好。

最终方案：浅色保留薄荷底；深色使用深薄荷底、浅色标题和次级正文；原图改为真正透明背景，两种主题均保留原始亮度。此前降低不透明插图亮度的方案已撤销。不新增主题状态或依赖。

- [x] RED：组件加载真实目录样式，验证两个入口的深色表面亮度与正文对比度；浅色仍保持明亮。
- [x] GREEN：补齐深色变量及插画亮度样式。
- [x] 回归：目录键盘分页、筛选、失败重试、偏好测试；类型检查、构建通过，make check 已尝试但受环境阻断。
- [x] 记录审查与未验证范围。遵守本次 AGENTS.md，不使用浏览器自动化；桌面与手机视觉需人工验收。

## 第一轮执行证据（亮度滤镜方案已被后续透明修复替代）

命令在 WSL Ubuntu `/home/yangyang/loadout` 执行，PATH 显式包含 `/home/yangyang/.nvm/versions/node/v26.8.2/bin`、`/home/yangyang/.cargo/bin`、`/home/yangyang/.local/bin` 和系统命令目录。

- RED：`pnpm --dir web exec vitest run src/PluginsPage.test.tsx -t "adapts catalog"`，两个入口均失败：深色展台相对亮度仍为 0.8502636282796994，要求小于 0.1。之前 PATH/测试 CSS 导入问题属于准备失败，不计有效 RED。
- GREEN：`pnpm --dir web exec vitest run src/PluginsPage.test.tsx src/Localization.test.tsx`，33 项通过；包含同一主题测试、键盘分页、筛选与失败重试。主题测试通过 DOM CSS 计算结果验证表面亮度及标题/正文至少 4.5:1，不断言源码字符串。
- REFACTOR：无需逻辑抽象；仅使用已有 CSS 主题选择器。`pnpm exec biome check web/src/PluginsPage.test.tsx web/src/features/PluginsPage.css` 通过，`git diff --check` 通过。
- `pnpm --dir web exec vitest run src/Preferences.test.tsx`，4 项通过，含显式偏好持久化与系统变化响应。
- `pnpm --dir web build`，TypeScript 与生产构建通过。
- `make check`：发行预检 4 项通过后，在 database-check 退出，未设置 `LOADOUT_TEST_DATABASE_URL`；全量门禁未通过，不声称完整产品验收。
- Impeccable detector 已执行，只有原有字号/圆角 advisory；此次未改布局与字号。
- 独立审查未发现功能阻断；要求补齐执行证据，已落实。审查者独立测试因其 shell 未配置 pnpm PATH 未运行，上述测试由主执行者实际完成。

第一轮未验证范围（历史）：当时未完成透明抠图及真实浏览器视觉检查。透明素材已在下述修复完成；真实页面桌面/手机浏览器视觉仍未验收。未提交、推送或发布。

## 透明素材修复（用户已授权本地 Python）
使用原始1000×750插图做连通去底及边缘去底色，保留主体；移除 brightness 滤镜。先验证旧素材无 alpha 和滤镜断言失败，再生成透明 WebP、检查深浅背景合成与回归。

### 透明修复结果

- RED：`pnpm --dir web exec vitest run src/PluginsPage.test.tsx -t "adapts catalog"` 两个入口失败，computed filter 为 brightness(0.72)；Python/Pillow 检查原图失败：RGB、无 alpha。
- GREEN：原始1000×750素材经背景连通区域去底，另去除提手内部底色；边缘按背景色反解，主体不重绘。保存为lossless RGBA WebP，215644 bytes。移除整个brightness规则。Pillow验证alpha极值0/255、角落及提手洞透明、两眼细节不透明，全部通过。
- 初次边缘估计使细黑轮廓变浅；修正为只处理背景侧边缘、完整保留前景像素。双主题合成已查看500×375、180×135以及眼睛细节。此为素材视觉验收，不冒充真实页面桌面/手机浏览器验收。
- `pnpm --dir web exec vitest run src/PluginsPage.test.tsx src/Preferences.test.tsx`：19项通过，包括筛选、键盘分页、错误恢复和跟随系统；`pnpm --dir web build`、`pnpm exec biome check web/src/PluginsPage.test.tsx web/src/features/PluginsPage.css`、`git diff --check`通过。
- 尝试添加WebP头检查时误假定扩展VP8X；无损文件为合法VP8L。未保留手写格式解析，以Pillow真实解码和alpha像素检查验收素材。
- 本轮重新运行`make check`：发行预检4项通过，database-check因未设置LOADOUT_TEST_DATABASE_URL停止，完整门禁仍未通过。
- 本地复核产物：`.loadout/art-qa/cutout.py`、`marketplace-original.webp`、`marketplace-themes.png`，均为忽略的工作文件；应用只依赖仓库内最终WebP。上文“不透明素材/72%亮度”为上一轮历史，由本节替代。

独立审查已查看双主题合成：背景融合、主体和眼睛高光保留，未发现阻断性问题；DESIGN.md与最终无滤镜透明素材一致。
