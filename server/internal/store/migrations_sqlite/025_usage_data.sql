ALTER TABLE usage_logs ADD COLUMN input_data TEXT;
ALTER TABLE usage_logs ADD COLUMN output_data TEXT;
ALTER TABLE usage_logs ADD COLUMN input_truncated INTEGER NOT NULL DEFAULT 0 CHECK(input_truncated IN (0,1));
ALTER TABLE usage_logs ADD COLUMN output_truncated INTEGER NOT NULL DEFAULT 0 CHECK(output_truncated IN (0,1));
