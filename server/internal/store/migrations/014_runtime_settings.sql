CREATE TABLE runtime_settings (
 id smallint PRIMARY KEY CHECK (id = 1),
 revision bigint NOT NULL CHECK (revision > 0),
 public_values jsonb NOT NULL,
 encrypted_secrets bytea,
 updated_by bigint NOT NULL REFERENCES users(id),
 updated_at timestamptz NOT NULL DEFAULT statement_timestamp()
);
