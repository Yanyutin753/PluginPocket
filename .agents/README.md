# 项目技能

三套官方技能已安装到本仓库 `.agents/skills/`，源码随项目共享，新 clone 不需要修改个人全局配置。Codex 可从项目技能目录发现；其他工具使用根 AGENTS.md 的路径加载约定。安装后的技能在后续轮次可用，本次已直接读取并应用。

| 工具 | 上游 | 固定提交 |
|---|---|---|
| Superpowers | https://github.com/obra/superpowers | b36e0829c6d0140e93cfef2ca599b1b07d4a7797 |
| Ponytail | https://github.com/DietrichGebert/ponytail | 356918eba965ee1eac64bd3a7f0dd02108350de5 |
| Impeccable | https://github.com/pbakaus/impeccable | 67d018fe052853c104a96d441ce175dd5ec4c39d |

安装日期：2026-09-10。Superpowers 取 upstream `skills/`；Ponytail 取六个 `skills/ponytail*`；Impeccable 取官方 Codex `.agents/skills/impeccable/`。均保留官方正文及对应许可证。Ponytail 六个技能共用 `skills/ponytail/LICENSE`。

这是项目技能安装，未声称启用各产品的全局 plugin lifecycle hooks。项目强制使用方式在根 AGENTS.md。Impeccable 含官方 launcher；第一次调用会下载并校验其版本固定的 engine，二进制留在用户缓存，不进入 Git：

```sh
.agents/skills/impeccable/scripts/impeccable context
```

更新：从官方仓库选择明确提交，替换对应源码和 LICENSE，更新本表，检查 SKILL.md 的相对引用及 launcher 执行位，再进行任务级验证。不要用批量 latest 更新覆盖项目规则。
