CREATE INDEX usage_user_time ON usage_logs(user_id,created_at DESC);
CREATE INDEX usage_wallet_time ON usage_logs(wallet_id,created_at DESC);
CREATE INDEX usage_created_time ON usage_logs(created_at DESC);

-- User/wallet activity is correlated with time; retain joint frequencies so a
-- dormant account does not inherit the selectivity of another account's burst.
CREATE STATISTICS usage_user_time_stats (mcv) ON user_id,created_at FROM usage_logs;
CREATE STATISTICS usage_wallet_time_stats (mcv) ON wallet_id,created_at FROM usage_logs;
