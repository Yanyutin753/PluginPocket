ALTER TABLE marketplace_git_files ADD COLUMN file_sha256 TEXT REFERENCES file_objects(sha256);
ALTER TABLE marketplace_git_files ADD COLUMN file_size BIGINT;
ALTER TABLE marketplace_git_files ADD CONSTRAINT marketplace_git_file_reference CHECK (
 (file_sha256 IS NULL AND file_size IS NULL) OR
 (file_sha256 IS NOT NULL AND file_size IS NOT NULL AND file_size BETWEEN 0 AND 16777216 AND octet_length(content)=0)
);
