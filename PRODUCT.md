# Loadout

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

用户指定 React 前端、Go 后端、最新稳定技术栈、完整 TDD；允许 Rust 客户端，当前实现 Rust CLI 与 Tauri 桌面端。前端采用 Vite、TypeScript、Tailwind CSS、shadcn/ui、TanStack Query、Zod、Lucide。

## Users

使用 Codex、Claude Code 或 Cursor 的开发者，需要省去寻找 MCP server、申请多套 key、逐个修改客户端配置的步骤。同时服务小团队统一配置、共享额度与用量管理。

## Product Purpose

网页开通并创建网关令牌；本地 login / apply 配置 bridge；运营方提供预设 MCP 池，网关统一鉴权、计量与额度。这些操作均使用真实 API；控制台同时提供团队、兑换和管理员运营。

## Capabilities and Constraints

已实现账号、令牌、MCP 上游池、事务账本、bridge、兑换码/管理员充值、团队和桌面接入。外部支付仅保留未配置接口；GitHub/邮件需要运营配置。所有业务数据来自 PostgreSQL，不展示虚构余额或订单。只托管运营方预设池凭证，不接收用户私人第三方 OAuth。

## Brand Commitments

Loadout，口号 “Your AI, fully loaded.”，中文为主。用户先选择 AI 装备工坊角色方向，随后明确要求更精致的 iOS 风格、升级登录和加载页面并适配多端。最新视觉以中性灰白与柔和圆角为基础，保留黄色动作、原创薄荷绿工具箱角色；登录和加载增加细腻陶瓷质感插画。保留浅色、深色和跟随系统。页面直接呈现真实额度、调用状态和可执行的恢复操作。

## Evidence on Hand

README.md、docs/PLAN.md、docs/API.md。没有客户数、收入、性能基准、使用量或真实套餐价格证据，不能编造。

## Product Principles

- 接入过程尽量少步骤，错误提供恢复动作。
- 默认 bridge 不将密钥写入各客户端配置；direct 仅允许显式选择。
- 服务状态、额度与调用结果必须来自真实接口。
- 先完整验证最小流程，再扩展运营功能。

## Accessibility & Inclusion

中文布局兼容窄屏；状态不能只靠颜色区分；键盘可操作、焦点明显；遵循 reduced-motion。
