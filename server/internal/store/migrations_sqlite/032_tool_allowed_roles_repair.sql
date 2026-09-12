-- PostgreSQL-only repair for an early deployed 031; SQLite 031 already adds
-- allowed_roles. Keep existing restrictions and the migration tracks aligned.
SELECT 1;
