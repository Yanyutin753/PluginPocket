ALTER TABLE users ADD COLUMN email text UNIQUE;
ALTER TABLE users ADD COLUMN email_verified_at timestamptz;
CREATE TABLE email_verifications (
 token_hash text PRIMARY KEY,
 user_id bigint UNIQUE NOT NULL REFERENCES users(id),
 email text NOT NULL,
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE oauth_states (
 state_hash text PRIMARY KEY,
 browser_hash text NOT NULL,
 verifier text NOT NULL,
 expires_at timestamptz NOT NULL
);
CREATE INDEX oauth_states_expiry ON oauth_states(expires_at);
CREATE TABLE oauth_identities (
 provider text NOT NULL,
 subject text NOT NULL,
 user_id bigint NOT NULL REFERENCES users(id),
 PRIMARY KEY(provider,subject),
 UNIQUE(provider,user_id)
);
