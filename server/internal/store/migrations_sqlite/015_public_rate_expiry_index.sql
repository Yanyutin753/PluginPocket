CREATE INDEX rate_limits_public_expiry ON rate_limits(window_id)
 WHERE scope LIKE 'public:%';
