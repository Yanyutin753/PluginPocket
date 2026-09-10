CREATE TABLE tool_catalog_revision (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 revision bigint NOT NULL CHECK (revision > 0)
);
INSERT INTO tool_catalog_revision(singleton, revision) VALUES (true, 1);

CREATE FUNCTION advance_tool_catalog_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE tool_catalog_revision SET revision = revision + 1 WHERE singleton;
 RETURN NULL;
END;
$$;
CREATE TRIGGER tools_catalog_changed
AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON tools
FOR EACH STATEMENT EXECUTE FUNCTION advance_tool_catalog_revision();
