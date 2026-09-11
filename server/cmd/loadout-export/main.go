// loadout-export 将 Loadout 市场导出为 Codex 官方插件市场目录树。
// 产物推送 git 仓库后，用户执行 `codex plugin marketplace add owner/repo` 即接入。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/marketplace"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	database := flag.String("database", os.Getenv("LOADOUT_DATABASE_URL"), "PostgreSQL URL（只读）")
	out := flag.String("out", "", "输出目录（必需，建议指向干净的 git 工作区）")
	name := flag.String("name", "loadout", "市场标识（kebab-case）")
	display := flag.String("display", "Loadout 插件市场", "市场显示名")
	flag.Parse()
	if *database == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "loadout-export: --database and --out are required")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	deadline, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(deadline, *database)
	if err != nil {
		fail(err)
	}
	defer pool.Close()
	entries, unresolved, err := marketplace.LoadExportInputs(deadline, pool, marketplace.Options{
		BaseURL: os.Getenv("LOADOUT_GITHUB_API"),
		Token:   os.Getenv("LOADOUT_GITHUB_TOKEN"),
	})
	if err != nil {
		fail(err)
	}
	tree, skipped, err := marketplace.ExportCodexMarketplace(*name, *display, entries)
	if err != nil {
		fail(err)
	}
	written, plugins := 0, 0
	for path, content := range tree {
		if filepath.Base(path) == "plugin.json" {
			plugins++
		}
		target := filepath.Join(*out, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			fail(err)
		}
		written++
	}
	fmt.Printf("Exported %d marketplace files to %s (%d plugins)\n", written, *out, plugins)
	for _, item := range append(append([]string{}, skipped...), unresolved...) {
		fmt.Printf("skipped: %s\n", item)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "loadout-export:", err)
	os.Exit(1)
}
