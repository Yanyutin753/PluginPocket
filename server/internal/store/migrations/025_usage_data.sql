ALTER TABLE usage_logs ADD COLUMN input_data text;
ALTER TABLE usage_logs ADD COLUMN output_data text;
ALTER TABLE usage_logs ADD COLUMN input_truncated boolean NOT NULL DEFAULT false;
ALTER TABLE usage_logs ADD COLUMN output_truncated boolean NOT NULL DEFAULT false;
