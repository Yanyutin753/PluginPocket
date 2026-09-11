-- SQLite：正则 CHECK 由应用层负责（双方言差异已记录），仅加列。
ALTER TABLE marketplace_items ADD COLUMN version TEXT NOT NULL DEFAULT '1.0.0';
ALTER TABLE marketplace_items ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';
