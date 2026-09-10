CREATE TABLE device_authorizations (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 device_hash text NOT NULL UNIQUE,
 user_code_hash text NOT NULL UNIQUE,
 approved_user_id bigint REFERENCES users(id),
 expires_at timestamptz NOT NULL,
 next_poll_at timestamptz NOT NULL DEFAULT now(),
 consumed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX device_expiry ON device_authorizations(expires_at);
