package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GitRegistry publishes one persistent history shared by every replica.
type GitRegistry struct {
	pool    *pgxpool.Pool
	options Options
}

func NewGitRegistry(pool *pgxpool.Pool, options Options) *GitRegistry {
	return &GitRegistry{pool: pool, options: options}
}

func gitTip(files map[string][]byte) *[20]byte {
	refs := strings.TrimSpace(string(files["info/refs"]))
	sha, _, _ := strings.Cut(refs, "\t")
	raw, err := hex.DecodeString(sha)
	if err != nil || len(raw) != 20 {
		return nil
	}
	var tip [20]byte
	copy(tip[:], raw)
	return &tip
}

func (g *GitRegistry) ensureBuilt(ctx context.Context) error {
	const freshness = "SELECT COALESCE(built_at > statement_timestamp() - interval '1 minute', false), tree_hash FROM marketplace_git_state WHERE singleton"
	var fresh bool
	var previousHash string
	if err := g.pool.QueryRow(ctx, freshness).Scan(&fresh, &previousHash); err != nil {
		return err
	}
	if fresh {
		return nil
	}
	tx, err := g.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx, freshness+" FOR UPDATE").Scan(&fresh, &previousHash); err != nil {
		return err
	}
	if fresh {
		return tx.Commit(ctx)
	}
	entries, _, err := LoadExportInputs(ctx, tx, g.options)
	if err != nil {
		return err
	}
	tree, _, err := ExportCodexMarketplace("loadout", "Loadout 插件市场", entries)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(tree)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])
	if hash != previousHash {
		var refs []byte
		err = tx.QueryRow(ctx, "SELECT content FROM marketplace_git_files WHERE path='info/refs'").Scan(&refs)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		built, err := BuildGitMarketplace(tree, gitTip(map[string][]byte{"info/refs": refs}))
		if err != nil {
			return err
		}
		batch := &pgx.Batch{}
		for path, content := range built {
			batch.Queue("INSERT INTO marketplace_git_files(path,content) VALUES($1,$2) ON CONFLICT(path) DO UPDATE SET content=EXCLUDED.content WHERE marketplace_git_files.content IS DISTINCT FROM EXCLUDED.content", path, content)
		}
		if err = tx.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, "UPDATE marketplace_git_state SET tree_hash=$1, built_at=statement_timestamp() WHERE singleton", hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// File reads only the requested file; immutable objects outlive updates.
func (g *GitRegistry) File(ctx context.Context, path string) ([]byte, error) {
	var content []byte
	if strings.HasPrefix(path, "objects/") {
		err := g.pool.QueryRow(ctx, "SELECT content FROM marketplace_git_files WHERE path=$1", path).Scan(&content)
		if err == nil {
			return content, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if err := g.ensureBuilt(ctx); err != nil {
		return nil, err
	}
	err := g.pool.QueryRow(ctx, "SELECT content FROM marketplace_git_files WHERE path=$1", path).Scan(&content)
	return content, err
}

// Files provides the complete persisted repository for clone checks.
func (g *GitRegistry) Files(ctx context.Context) (map[string][]byte, error) {
	if err := g.ensureBuilt(ctx); err != nil {
		return nil, err
	}
	rows, err := g.pool.Query(ctx, "SELECT path,content FROM marketplace_git_files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := map[string][]byte{}
	for rows.Next() {
		var path string
		var content []byte
		if err = rows.Scan(&path, &content); err != nil {
			return nil, err
		}
		files[path] = content
	}
	return files, rows.Err()
}
