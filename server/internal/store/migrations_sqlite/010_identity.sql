ALTER TABLE users ADD COLUMN email TEXT;
CREATE UNIQUE INDEX users_email ON users(email);
ALTER TABLE users ADD COLUMN email_verified_at TEXT;
CREATE TABLE email_verifications (
 token_hash TEXT PRIMARY KEY,
 user_id INTEGER UNIQUE NOT NULL REFERENCES users(id),
 email TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE oauth_states (
 state_hash TEXT PRIMARY KEY,
 browser_hash TEXT NOT NULL,
 verifier TEXT NOT NULL,
 expires_at TEXT NOT NULL
);
CREATE INDEX oauth_states_expiry ON oauth_states(expires_at);
CREATE TABLE oauth_identities (
 provider TEXT NOT NULL,
 subject TEXT NOT NULL,
 user_id INTEGER NOT NULL REFERENCES users(id),
 PRIMARY KEY(provider,subject),
 UNIQUE(provider,user_id)
);
