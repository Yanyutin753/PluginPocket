-- SQLite 基线：013 的 idempotency 约束演进已并入（SQLite 不支持改约束）。
-- 正则类 CHECK（username/currency）由应用层负责，SQLite 侧保留长度与枚举约束。
-- 时间统一 ISO8601 UTC 文本；布尔为 0/1 整数；jsonb 为 TEXT。
CREATE TABLE users (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 username TEXT NOT NULL UNIQUE CHECK (length(username) BETWEEN 3 AND 32),
 password_hash TEXT NOT NULL,
 role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user','admin')),
 enabled INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE wallets (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id INTEGER UNIQUE REFERENCES users(id),
 balance INTEGER NOT NULL DEFAULT 0 CHECK (balance >= 0),
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE sessions (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id INTEGER NOT NULL REFERENCES users(id),
 session_hash TEXT NOT NULL UNIQUE,
 expires_at TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE TABLE tokens (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id INTEGER NOT NULL REFERENCES users(id),
 wallet_id INTEGER NOT NULL REFERENCES wallets(id),
 name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
 prefix TEXT NOT NULL,
 token_hash TEXT NOT NULL UNIQUE,
 revoked_at TEXT,
 last_used_at TEXT,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX tokens_user_cursor ON tokens(user_id,id DESC);
CREATE TABLE tools (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 key TEXT NOT NULL UNIQUE,
 name TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 kind TEXT NOT NULL CHECK (kind IN ('builtin','http','stdio')),
 enabled INTEGER NOT NULL DEFAULT 1,
 cost INTEGER NOT NULL DEFAULT 1 CHECK (cost >= 0),
 input_schema TEXT NOT NULL DEFAULT '{"type":"object"}',
 config TEXT NOT NULL DEFAULT '{}',
 updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE usage_logs (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id INTEGER NOT NULL REFERENCES users(id),
 token_id INTEGER NOT NULL REFERENCES tokens(id),
 wallet_id INTEGER NOT NULL REFERENCES wallets(id),
 tool TEXT NOT NULL,
 cost INTEGER NOT NULL DEFAULT 0 CHECK (cost >= 0),
 requested_cost INTEGER NOT NULL DEFAULT 0 CHECK (requested_cost >= 0),
 status TEXT NOT NULL CHECK (status IN ('pending','ok','error','denied','recovered')),
 duration_ms INTEGER NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
 request_key TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 finished_at TEXT,
 UNIQUE(token_id,request_key)
);
CREATE INDEX usage_user_cursor ON usage_logs(user_id,id DESC);
CREATE INDEX usage_wallet_cursor ON usage_logs(wallet_id,id DESC);
CREATE INDEX usage_tool_time ON usage_logs(tool,created_at DESC);
CREATE INDEX usage_pending ON usage_logs(created_at) WHERE status='pending';
CREATE TABLE ledger (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 wallet_id INTEGER NOT NULL REFERENCES wallets(id),
 user_id INTEGER NOT NULL REFERENCES users(id),
 call_id INTEGER REFERENCES usage_logs(id),
 actor_id INTEGER REFERENCES users(id),
 balance_after INTEGER,
 delta INTEGER NOT NULL,
 kind TEXT NOT NULL,
 note TEXT NOT NULL DEFAULT '',
 idempotency_key TEXT,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 UNIQUE(wallet_id,kind,idempotency_key),
 UNIQUE(call_id,kind)
);
CREATE INDEX ledger_wallet_cursor ON ledger(wallet_id,id DESC);
-- SQLite 触发器不支持组合事件：UPDATE 与 DELETE 各一个。
CREATE TRIGGER ledger_no_update BEFORE UPDATE ON ledger BEGIN SELECT RAISE(ABORT, 'ledger is append-only'); END;
CREATE TRIGGER ledger_no_delete BEFORE DELETE ON ledger BEGIN SELECT RAISE(ABORT, 'ledger is append-only'); END;
INSERT INTO tools(key,name,description,kind,cost,input_schema) VALUES
 ('echo','Echo','Returns the supplied message.','builtin',1,'{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}'),
 ('time_now','Current time','Returns the current UTC time.','builtin',1,'{"type":"object","properties":{},"additionalProperties":false}');
