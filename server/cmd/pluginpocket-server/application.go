package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/app"
	"github.com/Yanyutin753/PluginPocket/server/internal/cache"
	"github.com/Yanyutin753/PluginPocket/server/internal/config"
	"github.com/Yanyutin753/PluginPocket/server/internal/filestore"
	"github.com/Yanyutin753/PluginPocket/server/internal/gateway"
	"github.com/Yanyutin753/PluginPocket/server/internal/httpapi"
	"github.com/Yanyutin753/PluginPocket/server/internal/identity"
	"github.com/Yanyutin753/PluginPocket/server/internal/marketplace"
	"github.com/Yanyutin753/PluginPocket/server/internal/observability"
	"github.com/Yanyutin753/PluginPocket/server/internal/settings"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
)

func applicationHandler(ctx context.Context, cfg config.Config, logger *slog.Logger) (http.Handler, func(), error) {
	base := httpapi.New(cfg.WebDir, version)
	if cfg.DatabaseURL == "" {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "database_unconfigured"})
		})
		mux.Handle("/", base)
		return mux, func() {}, nil
	}
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	database, err := store.Open(startup, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return nil, nil, err
	}
	files, err := filestore.New(database.Pool, cfg.FileStorageOptions())
	if err != nil {
		database.Close()
		return nil, nil, err
	}
	if cfg.AdminUsername != "" {
		setup, stop := context.WithTimeout(ctx, 10*time.Second)
		err = app.BootstrapAdmin(setup, database, cfg.AdminUsername, cfg.AdminPassword)
		stop()
		if err != nil {
			database.Close()
			return nil, nil, err
		}
	}
	var sharedCache *cache.Client
	if cfg.RedisURL != "" {
		sharedCache, err = cache.Open(cfg.RedisURL, cfg.RedisNamespace)
		if err != nil {
			database.Close()
			return nil, nil, err
		}
	}
	g := gateway.New(database, gateway.Options{Cache: sharedCache, AllowPrivate: cfg.AllowPrivateUpstreams, EncryptionKey: cfg.EncryptionKey, StdioCommands: cfg.StdioCommands, TokenPerMinute: cfg.RateTokenPerMinute, UserPerDay: cfg.RateUserPerDay})
	runtime := &settings.Manager{Pool: database.Pool, Key: cfg.EncryptionKey, Origin: cfg.PublicURL, Defaults: settings.Values{
		InitialCredits: cfg.InitialCredits,
		GitHubEnabled:  cfg.GitHubClientID != "", GitHubClientID: cfg.GitHubClientID, GitHubClientSecret: cfg.GitHubClientSecret, GitHubOrg: cfg.GitHubOrg,
		SMTPEnabled: cfg.SMTPAddress != "", SMTPAddress: cfg.SMTPAddress, SMTPFrom: cfg.SMTPFrom, SMTPUsername: cfg.SMTPUsername, SMTPPassword: cfg.SMTPPassword,
	}}
	options := app.Options{Files: files, Runtime: runtime, InitialCredits: &cfg.InitialCredits, Origin: cfg.PublicURL, SecureCookies: strings.HasPrefix(cfg.PublicURL, "https://"), Gateway: g, EncryptionKey: cfg.EncryptionKey, Marketplace: marketplace.Options{Files: files, BaseURL: cfg.GitHubAPIURL, Token: cfg.GitHubToken}}
	identityOptions := identity.Options{Runtime: runtime, Origin: cfg.PublicURL, SecureCookies: options.SecureCookies, SMTPAllowLocalInsecure: cfg.SMTPAllowLocalInsecure}
	identityHandler := identity.New(database, identityOptions)
	mux := http.NewServeMux()
	metrics := observability.New(database.Pool)
	mux.Handle("GET /metrics", metrics.Handler())
	for _, path := range []string{"/api/v1/meta", "/api/v1/auth/github/", "/api/v1/auth/email/verify", "/api/v1/account/email", "/api/v1/account/email/"} {
		mux.Handle(path, identityHandler)
	}
	mux.Handle("/api/v1/health", base)
	appHandler := app.New(database, options)
	mux.Handle("/api/v1/", appHandler)
	// 服务端即插件市场源：哑 HTTP git 裸仓，codex plugin marketplace add <origin>/marketplace.git
	registry := marketplace.NewGitRegistry(database.Pool, options.Marketplace)
	mux.HandleFunc("GET /marketplace.git/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/marketplace.git/")
		content, err := registry.File(r.Context(), path)
		if errors.Is(err, pgx.ErrNoRows) {
			httpapi.Fail(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			httpapi.Fail(w, http.StatusServiceUnavailable, "marketplace_unavailable")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(content)
	})
	mux.Handle("/mcp", g)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ready, done := context.WithTimeout(r.Context(), time.Second)
		defer done()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if database.Pool.Ping(ready) != nil {
			w.WriteHeader(503)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
			return
		}
		redisState := "disabled"
		if sharedCache != nil {
			redisState = "ready"
			if sharedCache.Ping(ready) != nil {
				redisState = "degraded"
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready", "redis": redisState})
	})
	mux.Handle("/", base)
	recoveryCtx, stopRecovery := context.WithCancel(ctx)
	recovered := make(chan struct{})
	go func() {
		defer close(recovered)
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			recovery, cancel := context.WithTimeout(recoveryCtx, 10*time.Second)
			count, err := recoverExpiredCalls(recovery, database)
			cancel()
			if err != nil && recoveryCtx.Err() == nil {
				logger.Error("pending call recovery failed")
			}
			if count > 0 {
				logger.Info("recovered expired calls", "count", count)
			}
			select {
			case <-recoveryCtx.Done():
				return
			case <-timer.C:
			}
		}
	}()
	closeHandler := func() {
		stopRecovery()
		<-recovered
		g.Close()
		if sharedCache != nil {
			_ = sharedCache.Close()
		}
		database.Close()
	}
	return metrics.Wrap(mux), closeHandler, nil
}

func recoverExpiredCalls(ctx context.Context, database *store.Store) (int64, error) {
	var before time.Time
	if err := database.Pool.QueryRow(ctx, "SELECT statement_timestamp() - interval '5 minutes'").Scan(&before); err != nil {
		return 0, err
	}
	return database.RecoverPending(ctx, before)
}
