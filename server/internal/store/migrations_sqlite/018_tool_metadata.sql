CREATE TABLE tool_metadata_overrides (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tool_id INTEGER NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
 remote_name TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 input_schema TEXT,
 updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 UNIQUE(tool_id, remote_name)
);
