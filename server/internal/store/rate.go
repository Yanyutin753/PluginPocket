package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// AllowRequest atomically consumes a shared, database-clock fixed-window budget.
// Scopes and subjects must be bounded identifiers (for example, a user ID), not
// arbitrary anonymous client input unless using the expiring public: minute scope.
// An older queued request never resets a window.
func (s *Store) AllowRequest(ctx context.Context, scope string, subject int64, windowSeconds int64, limit int64) (bool, error) {
	if scope == "" || windowSeconds <= 0 || limit <= 0 {
		return false, errors.New("invalid rate budget")
	}

	// Anonymous peers rotate continuously; reclaim old minute windows even when
	// this caller has exhausted its own budget. Row locks skip concurrent requests.
	if strings.HasPrefix(scope, "public:") {
		_, err := s.Pool.Exec(ctx, `DELETE FROM rate_limits WHERE (scope,subject) IN (
   SELECT scope,subject FROM rate_limits
   WHERE scope LIKE 'public:%' AND window_id < floor(extract(epoch FROM statement_timestamp())/60)::bigint
   ORDER BY window_id LIMIT 100 FOR UPDATE SKIP LOCKED)`)
		if err != nil {
			return false, err
		}
	}
	var used int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO rate_limits(scope,subject,window_id,used)
 VALUES($1,$2,floor(extract(epoch FROM statement_timestamp())/$3::bigint)::bigint,1)
 ON CONFLICT(scope,subject) DO UPDATE SET
 window_id=GREATEST(rate_limits.window_id,EXCLUDED.window_id),
 used=CASE WHEN EXCLUDED.window_id>rate_limits.window_id THEN 1 ELSE rate_limits.used+1 END
 WHERE EXCLUDED.window_id>rate_limits.window_id OR rate_limits.used<$4
 RETURNING used`, scope, subject, windowSeconds, limit).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
