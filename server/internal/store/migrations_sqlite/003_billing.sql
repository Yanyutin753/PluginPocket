CREATE TABLE plans (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
 credits INTEGER NOT NULL CHECK (credits BETWEEN 1 AND 1000000000000),
 price_cents INTEGER NOT NULL CHECK (price_cents BETWEEN 0 AND 1000000000000),
 currency TEXT NOT NULL CHECK (length(currency) = 3),
 enabled INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE redemption_codes (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 code_hash TEXT UNIQUE NOT NULL,
 credits INTEGER NOT NULL CHECK (credits BETWEEN 1 AND 1000000000000),
 note TEXT NOT NULL,
 created_by INTEGER NOT NULL REFERENCES users(id),
 redeemed_by INTEGER REFERENCES users(id),
 redeemed_at TEXT,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE orders (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id INTEGER NOT NULL REFERENCES users(id),
 plan_id INTEGER NOT NULL REFERENCES plans(id),
 credits INTEGER NOT NULL,
 price_cents INTEGER NOT NULL,
 currency TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('pending','paid','cancelled','failed')),
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX orders_user_cursor ON orders(user_id,id DESC);
