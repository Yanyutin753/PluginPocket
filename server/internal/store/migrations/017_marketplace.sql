CREATE TABLE marketplace_items (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 slug text NOT NULL UNIQUE,
 name text NOT NULL,
 description text NOT NULL DEFAULT '',
 source text NOT NULL CHECK(source IN ('curated','github')),
 repo_url text NOT NULL DEFAULT '',
 homepage text NOT NULL DEFAULT '',
 transport text NOT NULL CHECK(transport IN ('http','stdio','unknown')),
 endpoint text NOT NULL DEFAULT '',
 package text NOT NULL DEFAULT '',
 stars int NOT NULL DEFAULT 0,
 synced_at timestamptz,
 installed_tool_id bigint REFERENCES tools(id) ON DELETE SET NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO marketplace_items(slug,name,description,source,repo_url,homepage,transport,endpoint,package,stars) VALUES
 ('deepwiki','DeepWiki','Ask questions about any public GitHub repository; served as a public streamable HTTP MCP endpoint.','curated','https://github.com/AsyncFuncAI/deepwiki-mcp','https://deepwiki.com','http','https://mcp.deepwiki.com/mcp','',0),
 ('context7','Context7','Up-to-date documentation lookups for libraries and frameworks; public endpoint works without an account.','curated','https://github.com/upstash/context7','https://context7.com','http','https://mcp.context7.com/mcp','',0),
 ('puppeteer','Puppeteer','Official reference MCP server that automates a browser; requires a controlled host or allowlisted command.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-puppeteer',0),
 ('filesystem','Filesystem','Official reference MCP server for restricted filesystem access; requires a controlled host or allowlisted command.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-filesystem',0),
 ('memory','Memory','Official reference MCP server for a persistent knowledge graph; requires a controlled host or allowlisted command.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-memory',0),
 ('everything','Everything','Official reference MCP server exercising every MCP feature; useful for validation and demos.','curated','https://github.com/modelcontextprotocol/servers','','stdio','','@modelcontextprotocol/server-everything',0);
