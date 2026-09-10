CREATE TABLE teams (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 80),
 seat_limit integer NOT NULL DEFAULT 5 CHECK(seat_limit BETWEEN 1 AND 1000),
 created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE wallets ADD COLUMN team_id bigint UNIQUE REFERENCES teams(id);
ALTER TABLE wallets ADD CONSTRAINT wallet_owner CHECK((user_id IS NOT NULL)::integer+(team_id IS NOT NULL)::integer=1);
CREATE TABLE team_members (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 team_id bigint NOT NULL REFERENCES teams(id),
 user_id bigint NOT NULL REFERENCES users(id),
 role text NOT NULL CHECK(role IN ('owner','member')),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(team_id,user_id)
);
CREATE INDEX member_user ON team_members(user_id,team_id);
CREATE TABLE team_invites (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 team_id bigint NOT NULL REFERENCES teams(id),
 code_hash text NOT NULL UNIQUE,
 created_by bigint NOT NULL REFERENCES users(id),
 accepted_by bigint REFERENCES users(id),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE team_transfers (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 team_id bigint NOT NULL REFERENCES teams(id),
 user_id bigint NOT NULL REFERENCES users(id),
 credits bigint NOT NULL CHECK(credits BETWEEN 1 AND 1000000000000),
 idempotency_key text NOT NULL,
 balance_after bigint NOT NULL,
 UNIQUE(team_id,user_id,idempotency_key)
);
