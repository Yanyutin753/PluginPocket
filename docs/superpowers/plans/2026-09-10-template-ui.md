# Loadout template UI rebuild

用户最终指令：现有气质和探索稿全部弃用，由实施者从网上挑选成熟模板并落地；全部 UI 与图片重做。当前工作区实施，不提交、推送、部署或写真实客户端配置。

## Direction and source

采用 https://github.com/satnaing/shadcn-admin 的 MIT 开源后台设计：白/冷灰与深海军蓝主题、紧凑分组侧栏、清晰页标题、独立指标面板、克制边框、正常宽度的表单动作。已查看上游 public/images/shadcn-admin.png 与 src/styles/theme.css。复用本项目 React Router / Query / Zod / shadcn，不迁移上游路由或模拟业务。上游许可证随适配 token 保留。

## Implementation

1. 用户可观察导航分组与键盘跳转、概览命令复制成功及失败恢复先写测试并确认 RED。
2. 最小实现分组导航、概览接入指引与复制反馈，确认同组 GREEN。
3. 在绿灯下统一 Web 所有页面共享框架、表格、列表、表单、错误/空状态；重做登录/注册图片与布局。所有原业务/API/语言/主题继续有效。
4. 桌面端沿用同一视觉 token 与表单层级，保留原生交互。
5. 聚焦及相关组件回归、完整 make check；按仓库契约不使用浏览器自动化，桌面/移动人工视觉未执行则明确记录。

## Evidence

待填本次真实 RED / GREEN / REFACTOR 和 harness 输出。既有截图只作旧界面参考，不证明新实现。
