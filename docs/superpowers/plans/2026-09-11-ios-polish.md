# iOS 风格细节与加载状态

用户要求改善登录页、全部细节与骨架屏，检查 hover / 点击，并继续适配日夜主题、大屏、客户端。已有实现上进行视觉优化，保留工具箱角色及真实功能。无需重复批准。

1. 登录/注册使用居中精简表单、柔和底面、较大触控区域、密码可见性按钮；测试先 RED 再 GREEN。
2. 查询加载使用匹配列表/指标/表单的可访问骨架，不阻碍背景刷新；reduced-motion 关闭动画，aria-status 保留。
3. 全局交互统一 hover、active、focus-visible、disabled，客户端同步。
4. 浏览器逐类检查浅深主题与三视口，补团队详情/用量与404遗漏；组件测试及 make check 验证，记录真实范围。


## 追加验收范围与实现

用户澄清“loading page”实际指落地页：新增公共 /，概览迁至 /overview，登录后仍去工作空间；保留真实查询骨架。新增品牌专用艺术字、页头对齐、共享选择控件、图标整体点击、侧栏收起、右上账号菜单与左下账号设置、页脚。图片继续追加协作/连接/分析/安全与默认头像。

## RED / GREEN / REFACTOR

所有下列命令从仓库执行，先运行对应失败行为再实现。日志在 .loadout/，组件源和用例随代码保存。

| 行为 | 命令 | RED | GREEN |
|---|---|---|---|
| 密码显示/隐藏与键盘 | pnpm --dir web exec vitest run src/AuthInteraction.test.tsx | ios-auth-red.log：找不到显示密码按钮 | ios-auth-green.log |
| 显示密码关闭拼写/自动修正 | 同上 | ios-password-red.log：缺少 spellcheck=false | ios-password-green.log |
| 公共首页不请求私有账户 | pnpm --dir web exec vitest run src/PublicHome.test.tsx | landing-route-red.log：缺少落地页标题 | landing-route-green.log |
| 右上账号菜单、Escape与失败重试 | pnpm --dir web exec vitest run src/Product.test.tsx | account-header-red.log：缺少账号菜单入口 | account-header-green.log：16通过 |
| 图标也能打开选择菜单 | pnpm --dir web exec vitest run src/Preferences.test.tsx | select-icon-red.log：点击外置图标无listbox | select-icon-green.log：与Select共8通过 |
| 侧栏展开收起、链接仍可访问 | pnpm --dir web exec vitest run src/Product.test.tsx | sidebar-collapse-red.log：缺少收起按钮 | sidebar-collapse-green.log：17通过 |
| 左下账号设置入口 | pnpm --dir web exec vitest run src/Product.test.tsx | sidebar-user-red.log：缺少账号设置链接 | sidebar-user-green.log：17通过 |

导航对齐采用真实浏览器计算样式验证：修复前字号12/12/13/14px、登录中心54.25而其余50.25；修复后全部14px/20px且中心50.25。选中项鼠标操作后的背景/描边截图验证，键盘行为用真实Radix组件测试。jsdom补齐Element指针捕获接口（包含SVG），没有模拟Radix行为。重构统一全站与客户端Select、删除旧sidebar account-menu CSS。

## 检查过程

- Product/Operations/PublicHome 聚焦回归：41通过，landing-regression.log。
- review_workshop最终聚焦审查：未发现已证实P1/P2；复跑Product17、Select4、Preferences4通过。
- ios-final-check.log：第一轮完整 make check 已运行成功（此后用户继续追加侧栏/账号/图标等变更，不能代表最终状态）。
- ios-final-check-2.log：第二轮前端/桌面构建通过，Go限流测试 TestRateLimitIsSharedAndAtomic 出现 allowed68/want60；无后端代码修改。独立 go test -race -count=3 -run TestRateLimitIsSharedAndAtomic ./internal/gateway 通过（ios-rate-recheck.log）。最终完整检查需在追加资产及UI完成后再次运行并记录。


## 同主角、逐页独立场景（最终追加）

客户端同理：品牌图标与默认头像共享，内容插画按页面分配，保留薄荷绿工具箱主角。新增21张900×600透明WebP，分别覆盖账号7场景、管理7场景、工具与客户端7场景；完整prompt、原图与抠图记录见 docs/third-party/Workshop-account-scenes.md、Workshop-admin-scenes.md、Workshop-utility-scenes.md。客户端使用workshop-desktop，不再借用落地页的workshop-tools。落地页6张、概览hero、404empty各自保留；共享查询加载组件使用loading。全局空状态不再重复插入同一图片。

最终改动仅为装饰图片分配与既有Heading复用，未新增业务行为；相关组件回归：pnpm --dir web test（103通过，scene-web-test.log）；pnpm --dir desktop/ui test（7通过，scene-client-test.log）。此前标点归一化的RED见ui-copy-red.log，GREEN见ui-copy-green.log与ui-copy-desktop-green.log，插值内容保持原样。

真实浏览器最终检查：20个生产入口×浅深主题×1440/390，共80张内容加载完成后截图，无横向溢出、无破图；团队详情/用量8张独立只读fixture截图。客户端390、1440浅深主题和2560深色大屏均截图核对。预览环境无Tauri bridge，显示真实不可用/重试状态；没有登录或改写真实客户端配置。图集与DOM记录位于Codex visualizations/2026/09/10/01a08bfd-0b27-7442-828c-49b57a546d35/ios（scene-*、client-scene-*）。临时qa-team/qa-loading入口检查后删除。

ios-final-check-3.log曾因演示数据E2E缺少管理员令牌失败；并行后端工作更新后，最终 make check 已完整运行且退出0，日志 .loadout/ios-scenes-final-check.log，包含Web/客户端测试、构建、Rust/Go检查、Tauri deb打包、真实产品旅程与集成测试。此前失败不作为最终成功证据。git diff --check通过。未提交、推送或发布。

静态设计扫描完成：104条记录，其中97条字号/模式等建议，7条字体登记偏差为已使用的Bricolage与SFMono，已补齐DESIGN.md typography。未以扫描替代截图验收。共享Select额外以非忽略路径stdin执行Biome检查/格式化，随后复跑组件测试。
