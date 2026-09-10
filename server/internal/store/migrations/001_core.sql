CREATE TABLE users (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 username text NOT NULL UNIQUE CHECK (username ~ '^[a-zA-Z0-9_-]{3,32}$'),
 password_hash text NOT NULL,
 role text NOT NULL DEFAULT 'user' CHECK (role IN ('user','admin')),
 enabled boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE wallets (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id bigint UNIQUE REFERENCES users(id),
 balance bigint NOT NULL DEFAULT 0 CHECK(balance >= 0),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id),
 session_hash text NOT NULL UNIQUE,
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE TABLE tokens (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id),
 wallet_id bigint NOT NULL REFERENCES wallets(id),
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 80),
 prefix text NOT NULL,
 token_hash text NOT NULL UNIQUE,
 revoked_at timestamptz,
 last_used_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tokens_user_cursor ON tokens(user_id,id DESC);
CREATE TABLE tools (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 key text NOT NULL UNIQUE,
 name text NOT NULL,
 description text NOT NULL DEFAULT '',
 kind text NOT NULL CHECK(kind IN ('builtin','http','stdio')),
 enabled boolean NOT NULL DEFAULT true,
 cost bigint NOT NULL DEFAULT 1 CHECK(cost >= 0),
 input_schema jsonb NOT NULL DEFAULT '{"type":"object"}',
 config jsonb NOT NULL DEFAULT '{}',
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE usage_logs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id),
 token_id bigint NOT NULL REFERENCES tokens(id),
 wallet_id bigint NOT NULL REFERENCES wallets(id),
 tool text NOT NULL,
 cost bigint NOT NULL DEFAULT 0 CHECK(cost >= 0),
 requested_cost bigint NOT NULL DEFAULT 0 CHECK(requested_cost >= 0),
 status text NOT NULL CHECK(status IN ('pending','ok','error','denied','recovered')),
 duration_ms bigint NOT NULL DEFAULT 0 CHECK(duration_ms >= 0),
 request_key text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 UNIQUE(token_id,request_key)
);
CREATE INDEX usage_user_cursor ON usage_logs(user_id,id DESC);
CREATE INDEX usage_wallet_cursor ON usage_logs(wallet_id,id DESC);
CREATE INDEX usage_tool_time ON usage_logs(tool,created_at DESC);
CREATE INDEX usage_pending ON usage_logs(created_at) WHERE status='pending';
CREATE TABLE ledger (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 wallet_id bigint NOT NULL REFERENCES wallets(id),
 user_id bigint NOT NULL REFERENCES users(id),
 call_id bigint REFERENCES usage_logs(id),
 actor_id bigint REFERENCES users(id),
 balance_after bigint,
 delta bigint NOT NULL,
 kind text NOT NULL,
 note text NOT NULL DEFAULT '',
 idempotency_key text,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(wallet_id,idempotency_key),
 UNIQUE(call_id,kind)
);
CREATE INDEX ledger_wallet_cursor ON ledger(wallet_id,id DESC);
CREATE FUNCTION reject_ledger_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ledger is append-only'; END;
$$;
CREATE TRIGGER ledger_append_only BEFORE UPDATE OR DELETE ON ledger FOR EACH ROW EXECUTE FUNCTION reject_ledger_mutation();
INSERT INTO tools(key,name,description,kind,cost,input_schema) VALUES
 ('echo','Echo','Returns the supplied message.','builtin',1,'{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}'),
 ('time_now','Current time','Returns the current UTC time.','builtin',1,'{"type":"object","properties":{},"additionalProperties":false}');
