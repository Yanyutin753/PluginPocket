-- 031 was applied by some development servers before allowed_roles was added.
-- Preserve any restrictions already saved by servers with the complete 031.
ALTER TABLE tools ADD COLUMN IF NOT EXISTS allowed_roles jsonb NOT NULL DEFAULT '[]'::jsonb;
