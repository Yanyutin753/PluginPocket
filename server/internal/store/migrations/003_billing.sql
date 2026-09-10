CREATE TABLE plans (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 80),
 credits bigint NOT NULL CHECK(credits BETWEEN 1 AND 1000000000000),
 price_cents bigint NOT NULL CHECK(price_cents BETWEEN 0 AND 1000000000000),
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
 enabled boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE redemption_codes (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 code_hash text UNIQUE NOT NULL,
 credits bigint NOT NULL CHECK(credits BETWEEN 1 AND 1000000000000),
 note text NOT NULL,
 created_by bigint NOT NULL REFERENCES users(id),
 redeemed_by bigint REFERENCES users(id),
 redeemed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE orders (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id),
 plan_id bigint NOT NULL REFERENCES plans(id),
 credits bigint NOT NULL,
 price_cents bigint NOT NULL,
 currency text NOT NULL,
 status text NOT NULL CHECK(status IN ('pending','paid','cancelled','failed')),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX orders_user_cursor ON orders(user_id,id DESC);
