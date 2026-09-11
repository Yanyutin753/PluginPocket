-- SQLite cannot alter a CHECK constraint. Rebuild both sides of the reference
-- without disabling foreign keys; preserve all existing bytes and references.
CREATE TABLE file_objects_v2 (
 sha256 TEXT PRIMARY KEY CHECK (length(sha256)=64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
 size INTEGER NOT NULL CHECK (size BETWEEN 0 AND 16777216),
 content BLOB,
 endpoint TEXT NOT NULL DEFAULT '',
 region TEXT NOT NULL DEFAULT '',
 bucket TEXT NOT NULL DEFAULT '',
 object_key TEXT NOT NULL DEFAULT '',
 CHECK ((content IS NOT NULL AND length(content)=size AND bucket='' AND object_key='') OR
        (content IS NULL AND bucket<>'' AND object_key<>'' AND region<>''))
);
INSERT INTO file_objects_v2 SELECT * FROM file_objects;
CREATE TABLE marketplace_git_files_v2 (
 path TEXT PRIMARY KEY,
 content BLOB NOT NULL,
 file_sha256 TEXT REFERENCES file_objects_v2(sha256),
 file_size INTEGER,
 CHECK ((file_sha256 IS NULL AND file_size IS NULL) OR
 (file_sha256 IS NOT NULL AND file_size IS NOT NULL AND file_size BETWEEN 0 AND 16777216 AND length(content)=0))
);
INSERT INTO marketplace_git_files_v2 SELECT * FROM marketplace_git_files;
DROP TABLE marketplace_git_files;
DROP TABLE file_objects;
ALTER TABLE file_objects_v2 RENAME TO file_objects;
ALTER TABLE marketplace_git_files_v2 RENAME TO marketplace_git_files;
