CREATE TABLE marketplace_items (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 slug TEXT NOT NULL UNIQUE,
 name TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 source TEXT NOT NULL CHECK(source IN ('curated','github')),
 repo_url TEXT NOT NULL DEFAULT '',
 homepage TEXT NOT NULL DEFAULT '',
 transport TEXT NOT NULL CHECK(transport IN ('http','stdio','unknown','gateway')),
 endpoint TEXT NOT NULL DEFAULT '',
 package TEXT NOT NULL DEFAULT '',
 stars INTEGER NOT NULL DEFAULT 0,
 synced_at TEXT,
 installed_tool_id INTEGER REFERENCES tools(id) ON DELETE SET NULL,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
INSERT INTO marketplace_items(slug,name,description,source,repo_url,homepage,transport,endpoint,package,stars) VALUES
 ('deepwiki','DeepWiki','Ask questions about any public GitHub repository; served as a public streamable HTTP MCP endpoint.','curated','https://github.com/AsyncFuncAI/deepwiki-mcp','https://deepwiki.com','http','https://mcp.deepwiki.com/mcp','',0),
 ('context7','Context7','Up-to-date documentation lookups for libraries and frameworks; public endpoint works without an account.','curated','https://github.com/upstash/context7','https://context7.com','http','https://mcp.context7.com/mcp','',0),
 ('puppeteer','Puppeteer','Official reference MCP server that automates a browser; requires a controlled host or allowlisted command.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-puppeteer',0),
 ('filesystem','Filesystem','Official reference MCP server for restricted filesystem access; requires a controlled host or allowlisted command.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-filesystem',0),
 ('memory','Memory','Official reference MCP server for a persistent knowledge graph; requires a controlled host or allowlisted command.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-memory',0),
 ('everything','Everything','Official reference MCP server exercising every MCP feature; useful for validation and demos.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-everything',0);
