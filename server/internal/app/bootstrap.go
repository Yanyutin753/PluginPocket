package app

import (
	"context"
	"errors"

	"github.com/Yanyutin753/PluginPocket/server/internal/auth"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
)

// BootstrapAdmin creates the deployment's first operator without changing existing accounts.
func BootstrapAdmin(ctx context.Context, s *store.Store, username, password string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("invalid bootstrap username")
	}
	hash, e := auth.HashPassword(password)
	if e != nil {
		return e
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(817392105)"); e != nil {
		return e
	}
	var role string
	e = tx.QueryRow(ctx, "SELECT role FROM users WHERE username=$1", username).Scan(&role)
	if e == nil {
		if role != "admin" {
			return errors.New("bootstrap username belongs to an existing user")
		}
		return nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	var id int64
	if e = tx.QueryRow(ctx, "INSERT INTO users(username,password_hash,role) VALUES($1,$2,'admin') RETURNING id", username, hash).Scan(&id); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO wallets(user_id) VALUES($1)", id); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
