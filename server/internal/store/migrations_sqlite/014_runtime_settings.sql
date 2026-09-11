CREATE TABLE runtime_settings (
 id INTEGER PRIMARY KEY CHECK (id = 1),
 revision INTEGER NOT NULL CHECK (revision > 0),
 public_values TEXT NOT NULL,
 encrypted_secrets BLOB,
 updated_by INTEGER NOT NULL REFERENCES users(id),
 updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
