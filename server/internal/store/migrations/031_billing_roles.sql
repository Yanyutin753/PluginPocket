CREATE TABLE billing_roles (
 name text PRIMARY KEY,
 multiplier_bp bigint NOT NULL CHECK (multiplier_bp BETWEEN 0 AND 1000000),
 description text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO billing_roles(name, multiplier_bp, description) VALUES
 ('default', 10000, 'Standard rate'),
 ('member', 8000, 'Member rate'),
 ('vip', 5000, 'VIP rate');
ALTER TABLE users ADD COLUMN billing_role text NOT NULL DEFAULT 'default' REFERENCES billing_roles(name);
ALTER TABLE usage_logs ADD COLUMN billing_role text NOT NULL DEFAULT 'default' REFERENCES billing_roles(name);
ALTER TABLE usage_logs ADD COLUMN multiplier_bp bigint NOT NULL DEFAULT 10000;
ALTER TABLE tools ADD COLUMN allowed_roles jsonb NOT NULL DEFAULT '[]'::jsonb;
