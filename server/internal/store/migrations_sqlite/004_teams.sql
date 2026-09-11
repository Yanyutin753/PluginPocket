CREATE TABLE teams (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
 seat_limit INTEGER NOT NULL DEFAULT 5 CHECK (seat_limit BETWEEN 1 AND 1000),
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
-- SQLite 不允许 ADD COLUMN 带 UNIQUE：用独立唯一索引（多 NULL 允许，与 PG 一致）。
ALTER TABLE wallets ADD COLUMN team_id INTEGER REFERENCES teams(id);
CREATE UNIQUE INDEX wallets_team ON wallets(team_id);
-- SQLite 布尔为整数，可直接相加。
CREATE TRIGGER wallet_owner_validate INSERT ON wallets BEGIN
 SELECT CASE WHEN (NEW.user_id IS NOT NULL) + (NEW.team_id IS NOT NULL) <> 1
  THEN RAISE(ABORT, 'wallet must have exactly one owner') END;
END;
CREATE TABLE team_members (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 team_id INTEGER NOT NULL REFERENCES teams(id),
 user_id INTEGER NOT NULL REFERENCES users(id),
 role TEXT NOT NULL CHECK (role IN ('owner','member')),
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 UNIQUE(team_id,user_id)
);
CREATE INDEX member_user ON team_members(user_id,team_id);
CREATE TABLE team_invites (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 team_id INTEGER NOT NULL REFERENCES teams(id),
 code_hash TEXT NOT NULL UNIQUE,
 created_by INTEGER NOT NULL REFERENCES users(id),
 accepted_by INTEGER REFERENCES users(id),
 expires_at TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE team_transfers (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 team_id INTEGER NOT NULL REFERENCES teams(id),
 user_id INTEGER NOT NULL REFERENCES users(id),
 credits INTEGER NOT NULL CHECK (credits BETWEEN 1 AND 1000000000000),
 idempotency_key TEXT NOT NULL,
 balance_after INTEGER NOT NULL,
 UNIQUE(team_id,user_id,idempotency_key)
);
