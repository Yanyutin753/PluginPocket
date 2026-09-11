package marketplace

import (
	"context"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GitRegistry 把市场实时渲染为哑 HTTP git 裸仓文件集（懒构建 + TTL 缓存），
// 由服务端在 /marketplace.git 直接对外提供——"服务端即插件市场源"。
type GitRegistry struct {
	pool    *pgxpool.Pool
	options Options
	ttl     time.Duration

	mu      sync.Mutex
	files   map[string][]byte
	builtAt time.Time
	lastTip *[20]byte
}

func NewGitRegistry(pool *pgxpool.Pool, options Options) *GitRegistry {
	return &GitRegistry{pool: pool, options: options, ttl: time.Minute}
}

// gitTip 从裸仓文件集解析当前 main 指向（用于下一次构建的 parent 链）。
func gitTip(files map[string][]byte) *[20]byte {
	refs := strings.TrimSpace(string(files["info/refs"]))
	sha, _, _ := strings.Cut(refs, "\t")
	if len(sha) != 40 {
		return nil
	}
	raw, err := hex.DecodeString(sha)
	if err != nil || len(raw) != 20 {
		return nil
	}
	var tip [20]byte
	copy(tip[:], raw)
	return &tip
}

// Invalidate 在市场内容变更后调用；下次请求立即重建。
func (g *GitRegistry) Invalidate() {
	g.mu.Lock()
	g.builtAt = time.Time{}
	g.mu.Unlock()
}

// Files 返回 路径→内容 全集（超过 TTL 或失效后重建）。
func (g *GitRegistry) Files(ctx context.Context) (map[string][]byte, error) {
	g.mu.Lock()
	fresh := time.Since(g.builtAt) < g.ttl
	files := g.files
	parent := g.lastTip
	g.mu.Unlock()
	if fresh && files != nil {
		return files, nil
	}
	entries, _, err := LoadExportInputs(ctx, g.pool, g.options)
	if err != nil {
		return nil, err
	}
	tree, _, err := ExportCodexMarketplace("loadout", "Loadout 插件市场", entries)
	if err != nil {
		return nil, err
	}
	built, err := BuildGitMarketplace(tree, parent)
	if err != nil {
		return nil, err
	}
	// 累积合并：哑协议要求整条提交链的对象可达；旧对象按内容寻址保留，
	// 元数据（refs/HEAD 等）由新构建覆盖。删除插件留下的不可达对象无害。
	if files != nil {
		merged := make(map[string][]byte, len(files)+len(built))
		for path, content := range files {
			merged[path] = content
		}
		for path, content := range built {
			merged[path] = content
		}
		built = merged
	}
	tip := gitTip(built)
	g.mu.Lock()
	g.files = built
	g.builtAt = time.Now()
	if tip != nil {
		g.lastTip = tip
	}
	g.mu.Unlock()
	return built, nil
}
