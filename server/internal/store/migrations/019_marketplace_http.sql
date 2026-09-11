-- 产品决策：插件市场只提供 HTTP MCP（服务端池的 stdio 仍由部署者经工具管理受控接入）。
DELETE FROM marketplace_items WHERE source='curated' AND transport='stdio';
INSERT INTO marketplace_items(slug,name,description,source,repo_url,homepage,transport,endpoint,package,stars) VALUES
 ('microsoft-learn','Microsoft Learn','Search official Microsoft documentation, samples and learning content; public endpoint without an account.','curated','https://github.com/MicrosoftDocs/learn','https://learn.microsoft.com','http','https://learn.microsoft.com/api/mcp/','',0);
