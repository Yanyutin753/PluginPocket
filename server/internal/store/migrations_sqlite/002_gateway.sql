CREATE TABLE rate_limits (
 scope TEXT NOT NULL,
 subject INTEGER NOT NULL,
 window_id INTEGER NOT NULL,
 used INTEGER NOT NULL CHECK (used > 0),
 PRIMARY KEY(scope,subject)
);
