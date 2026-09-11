CREATE TABLE file_objects (
 sha256 TEXT PRIMARY KEY CHECK (sha256 ~ '^[0-9a-f]{64}$'),
 size BIGINT NOT NULL CHECK (size BETWEEN 0 AND 16777216),
 content BYTEA,
 endpoint TEXT NOT NULL DEFAULT '',
 region TEXT NOT NULL DEFAULT '',
 bucket TEXT NOT NULL DEFAULT '',
 object_key TEXT NOT NULL DEFAULT '',
 CHECK ((content IS NOT NULL AND octet_length(content)=size AND bucket='' AND object_key='') OR
        (content IS NULL AND bucket<>'' AND object_key<>'' AND region<>''))
);
