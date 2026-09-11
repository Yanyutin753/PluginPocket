-- 正式发版：运营方掌控的版本号（semver），内容指纹（content_hash）只用于
-- 判定"内容是否变化"：未显式发版的内容变更自动 patch +1，内容不变版本不动。
ALTER TABLE marketplace_items ADD COLUMN version text NOT NULL DEFAULT '1.0.0' CHECK (version ~ '^[0-9]+\.[0-9]+\.[0-9]+$');
ALTER TABLE marketplace_items ADD COLUMN content_hash text NOT NULL DEFAULT '';
