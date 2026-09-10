# Loadout

> **Your AI, fully loaded.** — 登录即武装的 MCP 订阅网关

[![Status](https://img.shields.io/badge/status-planning-orange)](docs/PLAN.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 这是什么

用 Codex / Claude Code / Cursor 的开发者，想用 MCP 工具得自己找 server、申请一堆 API key、手改各客户端配置文件 —— 换台机器全部重来。

**Loadout 把这些压缩成两步**：

```bash
# 1. 网页注册，复制一个令牌
# 2. 本地两条命令：
loadout login --server https://api.example.com
loadout apply        # 自动检测 Codex / Claude Code / Cursor 并写入配置
```

重启客户端，整套预设好的 MCP 工具直接可用。所有调用经过统一网关：**鉴权、计量、扣额度，后台看得见每一次调用**。

## 架构一览

```
Codex / Claude Code / Cursor
        │ stdio
   loadout bridge（本地转发，自动注入令牌）
        │ streamable HTTP + Bearer ldt_xxx
   Loadout 服务端 ── 账号 / 令牌 / 网关 / 计量扣费 / 管理后台
        │
   预设 MCP 上游池（builtin / http / stdio）
```

三个核心设计：

1. **bridge 为默认** —— 客户端配置里不落任何密钥，令牌轮换不触碰客户端配置；
2. **网关无状态** —— 每请求独立实例，可水平扩展；
3. **只托管自有预设池凭证** —— 不碰用户私人第三方账号的 OAuth token。

## 项目状态

规划阶段，完整的产品与技术方案见 **[docs/PLAN.md](docs/PLAN.md)**（含数据模型、API 设计、计费规则、里程碑与验收标准）。

| 里程碑 | 内容 | 状态 |
|---|---|---|
| M0 | 端到端骨架：网关 + 控制台 + CLI，冒烟全绿 | 🚧 进行中 |
| M1 | 可运营：上游管理、限流防滥用、Docker 部署 | ⏳ |
| M2 | 商业闭环：支付、套餐、兑换码 | ⏳ |
| M3 | 团队版：席位、共享额度、审计 | ⏳ |

## 参与开发

```bash
git clone https://github.com/Yanyutin753/loadout.git
cd loadout
# M0 骨架落地后此处补充 quickstart
```

方案与实现不一致时，先改 [docs/PLAN.md](docs/PLAN.md) 再改代码。

## License

[MIT](LICENSE) © Loadout contributors
