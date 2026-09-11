-- CREATE STATISTICS 为 PostgreSQL 专属优化；SQLite 查询计划器自行估计，仅保留索引。
CREATE INDEX usage_user_time ON usage_logs(user_id,created_at DESC);
CREATE INDEX usage_wallet_time ON usage_logs(wallet_id,created_at DESC);
CREATE INDEX usage_created_time ON usage_logs(created_at DESC);
