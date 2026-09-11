CREATE TABLE session_access (
 access_hash TEXT PRIMARY KEY,
 session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 expires_at TEXT NOT NULL
);
CREATE INDEX session_access_session ON session_access(session_id);
