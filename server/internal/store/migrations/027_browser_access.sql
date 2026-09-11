CREATE TABLE session_access (
 access_hash text PRIMARY KEY,
 session_id bigint NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL
);
CREATE INDEX session_access_session ON session_access(session_id);
