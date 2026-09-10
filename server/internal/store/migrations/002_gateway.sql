CREATE TABLE rate_limits (
 scope text NOT NULL,
 subject bigint NOT NULL,
 window_id bigint NOT NULL,
 used bigint NOT NULL CHECK (used > 0),
 PRIMARY KEY(scope,subject)
);
