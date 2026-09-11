CREATE TABLE marketplace_git_state (
 singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
 tree_hash TEXT NOT NULL DEFAULT '',
 built_at TEXT
);
INSERT INTO marketplace_git_state(singleton) VALUES (1);
CREATE TABLE marketplace_git_files (path TEXT PRIMARY KEY, content BLOB NOT NULL);
CREATE TRIGGER marketplace_git_changed_insert AFTER INSERT ON marketplace_items BEGIN
 UPDATE marketplace_git_state SET built_at = NULL WHERE singleton;
END;
CREATE TRIGGER marketplace_git_changed_update AFTER UPDATE ON marketplace_items BEGIN
 UPDATE marketplace_git_state SET built_at = NULL WHERE singleton;
END;
CREATE TRIGGER marketplace_git_changed_delete AFTER DELETE ON marketplace_items BEGIN
 UPDATE marketplace_git_state SET built_at = NULL WHERE singleton;
END;
