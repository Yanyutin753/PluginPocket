-- 生态扩展：市场条目分三种 kind —— mcp（HTTP MCP 插件）/ skill（Agent Skill）/ bundle（装备组）。
ALTER TABLE marketplace_items ADD COLUMN kind text NOT NULL DEFAULT 'mcp' CHECK(kind IN ('mcp','skill','bundle'));
ALTER TABLE marketplace_items ADD COLUMN spec jsonb NOT NULL DEFAULT '{}'::jsonb;
-- skill spec：{"source":"inline","files":{"SKILL.md":"..."}} 或 {"source":"github","repo":"owner/repo","path":"skills/x"}
-- bundle spec：{"includes":["slug-a","slug-b"]}
INSERT INTO marketplace_items(slug,name,description,source,transport,kind,spec) VALUES
 ('commit-style','提交信息规范','按约定式提交写中文提交信息：类型、范围、动机与验证结果。','curated','unknown','skill','{"source":"inline","files":{"SKILL.md":"---\nname: commit-style\ndescription: 按约定式提交写中文提交信息（类型、范围、动机、验证）。\n---\n\n# 提交信息规范\n\n写提交信息时遵循约定式提交：\n\n- 首行 `类型(范围): 摘要`，类型使用 feat/fix/docs/refactor/test/chore；\n- 正文说明动机与实现要点，列出真实执行过的验证命令与结果；\n- 不夸大范围，未验证的内容不写“已通过”。\n"}}'::jsonb),
 ('verify-before-done','完成前验证','声称完成前先跑验证：聚焦测试、完整检查、真实命令与输出。','curated','unknown','skill','{"source":"inline","files":{"SKILL.md":"---\nname: verify-before-done\ndescription: 声称完成前先执行验证命令并核对输出。\n---\n\n# 完成前验证\n\n在宣称任务完成之前：\n\n1. 运行能覆盖改动的聚焦测试；\n2. 运行项目定义的完整检查（如 make check）；\n3. 报告真实命令、结果与未验证范围，不用推断代替证据。\n"}}'::jsonb);
