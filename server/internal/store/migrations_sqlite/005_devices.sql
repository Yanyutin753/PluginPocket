CREATE TABLE device_authorizations (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 device_hash TEXT NOT NULL UNIQUE,
 user_code_hash TEXT NOT NULL UNIQUE,
 approved_user_id INTEGER REFERENCES users(id),
 expires_at TEXT NOT NULL,
 next_poll_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
 consumed_at TEXT,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX device_expiry ON device_authorizations(expires_at);
