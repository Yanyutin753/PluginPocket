# Loadout

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

用户指定 React 前端、Go 后端、最新稳定技术栈、完整 TDD；允许 Rust 客户端，当前选择原生 Rust CLI。前端采用 Vite、TypeScript、Tailwind CSS、shadcn/ui、TanStack Query、Zod、Lucide。

## Users

使用 Codex、Claude Code 或 Cursor 的开发者，需要省去寻找 MCP server、申请多套 key、逐个修改客户端配置的步骤。后续服务小团队统一配置与用量管理。

## Product Purpose

网页开通并创建网关令牌；本地 login / apply 配置 bridge；运营方提供预设 MCP 池，网关统一鉴权、计量与额度。以上为产品目标，当前基建仅实现服务状态检测。

## Capabilities and Constraints

M0.0：真实 Go 健康 API、React 接入状态、Rust doctor。账号、支付、上游池、额度及 bridge 尚未实现，不得展示虚构业务数据或冒充可用。只托管运营方预设池凭证，不接收用户私人第三方 OAuth。

## Brand Commitments

Loadout，口号 “Your AI, fully loaded.”，中文为主。用户明确偏好深色开发者工具风格。基建页面直接呈现真实连接状态与下一步说明。

## Evidence on Hand

README.md、docs/PLAN.md、docs/API.md。没有客户数、收入、性能基准、使用量或真实套餐价格证据，不能编造。

## Product Principles

- 接入过程尽量少步骤，错误提供恢复动作。
- 密钥不写入各客户端配置。
- 服务状态、额度与调用结果必须来自真实接口。
- 先完整验证最小流程，再扩展运营功能。

## Accessibility & Inclusion

中文布局兼容窄屏；状态不能只靠颜色区分；键盘可操作、焦点明显；遵循 reduced-motion。
