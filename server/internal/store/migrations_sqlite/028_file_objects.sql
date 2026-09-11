CREATE TABLE file_objects (
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
