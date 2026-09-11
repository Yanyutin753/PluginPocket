CREATE TABLE marketplace_git_state (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 tree_hash text NOT NULL DEFAULT '',
 built_at timestamptz
);
INSERT INTO marketplace_git_state(singleton) VALUES (true);
CREATE TABLE marketplace_git_files (path text PRIMARY KEY, content bytea NOT NULL);
CREATE FUNCTION invalidate_marketplace_git() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE marketplace_git_state SET built_at = NULL WHERE singleton;
 RETURN NULL;
END;
$$;
CREATE TRIGGER marketplace_git_changed
AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON marketplace_items
FOR EACH STATEMENT EXECUTE FUNCTION invalidate_marketplace_git();
