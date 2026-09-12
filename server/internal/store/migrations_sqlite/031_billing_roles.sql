CREATE TABLE billing_roles (
 name TEXT PRIMARY KEY,
 multiplier_bp INTEGER NOT NULL CHECK (multiplier_bp BETWEEN 0 AND 1000000),
 description TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
INSERT INTO billing_roles(name, multiplier_bp, description) VALUES
 ('default', 10000, 'Standard rate'),
 ('member', 8000, 'Member rate'),
 ('vip', 5000, 'VIP rate');
ALTER TABLE users ADD COLUMN billing_role TEXT NOT NULL DEFAULT 'default' REFERENCES billing_roles(name);
ALTER TABLE usage_logs ADD COLUMN billing_role TEXT NOT NULL DEFAULT 'default' REFERENCES billing_roles(name);
ALTER TABLE usage_logs ADD COLUMN multiplier_bp INTEGER NOT NULL DEFAULT 10000;
ALTER TABLE tools ADD COLUMN allowed_roles TEXT NOT NULL DEFAULT '[]';
