CREATE TABLE tool_catalog_revision (
 singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
 revision INTEGER NOT NULL CHECK (revision > 0)
);
INSERT INTO tool_catalog_revision(singleton, revision) VALUES (1, 1);
-- SQLite 无语句级组合事件触发器、无 TRUNCATE：按事件拆三个。
CREATE TRIGGER tools_catalog_changed_insert AFTER INSERT ON tools BEGIN
 UPDATE tool_catalog_revision SET revision = revision + 1 WHERE singleton;
END;
CREATE TRIGGER tools_catalog_changed_update AFTER UPDATE ON tools BEGIN
 UPDATE tool_catalog_revision SET revision = revision + 1 WHERE singleton;
END;
CREATE TRIGGER tools_catalog_changed_delete AFTER DELETE ON tools BEGIN
 UPDATE tool_catalog_revision SET revision = revision + 1 WHERE singleton;
END;
