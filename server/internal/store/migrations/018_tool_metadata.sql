CREATE TABLE tool_metadata_overrides (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tool_id bigint NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
 remote_name text NOT NULL,
 description text NOT NULL DEFAULT '',
 input_schema jsonb,
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tool_id, remote_name)
);
