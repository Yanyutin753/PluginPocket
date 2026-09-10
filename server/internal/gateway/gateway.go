package gateway

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/cache"
	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type Options struct {
	Cache         *cache.Client
	AllowPrivate  bool
	EncryptionKey []byte
	StdioCommands map[string]string
	Timeout       time.Duration
}
type Gateway struct {
	store           *store.Store
	options         Options
	mu              sync.Mutex
	catalog         []toolBinding
	catalogUntil    time.Time
	catalogVersion  uint64
	catalogRevision int64
	sessions        map[string]*upstreamSession
	httpPools       map[string]*httpSessionPool
	httpProviders   map[int64]*httpProvider
	httpLeases      sync.WaitGroup
	flight          singleflight.Group
	ctx             context.Context
	cancel          context.CancelFunc
	cacheSource     string
	cacheWatchDone  chan struct{}
}
type toolRow struct {
	ID                           int64
	Key, Name, Description, Kind string
	Cost                         int64
	Schema, Config               json.RawMessage
}
type toolBinding struct {
	row        toolRow
	definition *mcp.Tool
	remoteName string
}

func New(s *store.Store, o Options) *Gateway {
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	g := &Gateway{store: s, options: o, sessions: make(map[string]*upstreamSession), httpPools: make(map[string]*httpSessionPool), ctx: ctx, cancel: cancel}
	if o.Cache != nil {
		g.cacheSource = rand.Text()
		g.cacheWatchDone = make(chan struct{})
		go func() {
			defer close(g.cacheWatchDone)
			o.Cache.Watch(ctx, func(source string) {
				if source != g.cacheSource {
					g.invalidateLocal()
				}
			})
		}()
	}
	return g
}
func (g *Gateway) Close() {
	g.cancel()
	g.mu.Lock()
	sessions := g.sessions
	g.sessions = make(map[string]*upstreamSession)
	pools := g.httpPools
	g.httpPools = make(map[string]*httpSessionPool)
	g.mu.Unlock()
	for _, pool := range pools {
		pool.close(true)
	}
	g.httpLeases.Wait()
	for _, session := range sessions {
		session.close()
	}
	if g.cacheWatchDone != nil {
		<-g.cacheWatchDone
	}
}

func (g *Gateway) Invalidate() {
	g.invalidateLocal()
	if g.options.Cache != nil {
		_ = g.options.Cache.PublishInvalidation(g.ctx, g.cacheSource)
	}
}

func (g *Gateway) invalidateLocal() {
	g.mu.Lock()
	g.catalogUntil = time.Time{}
	g.catalogVersion++
	g.mu.Unlock()
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || !strings.HasPrefix(raw, "ldt_") || g.store == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	principal, err := g.store.AuthToken(r.Context(), raw)
	if err != nil {
		if errors.Is(err, store.ErrUnauthorized) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		} else {
			http.Error(w, "gateway_unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	bindings, err := g.tools(ctx)
	cancel()
	if err != nil {
		http.Error(w, "gateway_unavailable", http.StatusServiceUnavailable)
		return
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "loadout", Version: "0.2.0"}, nil)
	for _, binding := range bindings {
		server.AddTool(binding.definition, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return g.call(ctx, principal, binding, req.Params.Arguments), nil
		})
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true, MaxRequestBodyBytes: 1 << 20})
	http.NewCrossOriginProtection().Handler(handler).ServeHTTP(w, r)
}
func toolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "[loadout] " + message}}}
}
func toolText(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}

func (g *Gateway) call(parent context.Context, p store.Principal, binding toolBinding, args json.RawMessage) *mcp.CallToolResult {
	ctx, cancel := context.WithTimeout(parent, g.options.Timeout)
	defer cancel()
	key := rand.Text()
	if !g.admit(ctx, p) {
		g.recordDenied(ctx, p, binding.definition.Name, key)
		return toolError("调用频率或每日额度已达上限")
	}
	call, err := g.store.ReserveTool(ctx, p.UserID, p.TokenID, p.WalletID, binding.row.ID, binding.definition.Name, key)
	if errors.Is(err, store.ErrNotFound) {
		g.recordDenied(ctx, p, binding.definition.Name, key)
		return toolError("工具当前不可用")
	}
	if errors.Is(err, store.ErrInsufficientBalance) {
		return toolError("额度不足，请充值后重试")
	}
	if err != nil {
		return toolError("暂时无法开始调用")
	}
	started := time.Now()
	result := g.execute(ctx, binding, args)
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if err = g.store.Finish(finishCtx, call.ID, !result.IsError, time.Since(started)); err != nil {
		return toolError("调用结算待恢复，请查看用量记录，勿重复执行有副作用的操作")
	}
	return result
}

func (g *Gateway) recordDenied(ctx context.Context, p store.Principal, name, key string) {
	_, _ = g.store.Pool.Exec(ctx, "INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key,finished_at) VALUES ($1,$2,$3,$4,0,'denied',$5,now())", p.UserID, p.TokenID, p.WalletID, name, key)
}
func (g *Gateway) execute(ctx context.Context, b toolBinding, args json.RawMessage) *mcp.CallToolResult {
	if b.row.Kind == "builtin" {
		switch b.row.Key {
		case "echo":
			var input struct {
				Message *string `json:"message"`
			}
			if json.Unmarshal(args, &input) != nil || input.Message == nil {
				return toolError("message 必须为字符串")
			}
			return toolText(*input.Message)
		case "time_now":
			return toolText(time.Now().UTC().Format(time.RFC3339Nano))
		default:
			return toolError("未知内置工具")
		}
	}
	session, err := g.upstream(ctx, b.row)
	if err != nil {
		return toolError("上游连接失败")
	}
	if session.ctx != nil {
		ctx = session.ctx
	}
	defer func() { session.release(err) }()
	result, err := session.session.CallTool(ctx, &mcp.CallToolParams{Name: b.remoteName, Arguments: args})
	if err != nil {
		g.dropSession(ctx, b.row, err)
		return toolError("上游调用失败或超时")
	}
	if result.IsError {
		return toolError("上游未能完成调用，本次不扣费")
	}
	return result
}

func (g *Gateway) tools(ctx context.Context) ([]toolBinding, error) {
	if err := g.ctx.Err(); err != nil {
		return nil, err
	}
	if err := g.observeCatalogRevision(ctx); err != nil {
		return nil, err
	}
	for {
		if err := g.ctx.Err(); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		if time.Now().Before(g.catalogUntil) {
			result := g.catalog
			g.mu.Unlock()
			return result, nil
		}
		g.mu.Unlock()
		completed := g.flight.DoChan("catalog", func() (any, error) {
			g.mu.Lock()
			version := g.catalogVersion
			revision := g.catalogRevision
			if time.Now().Before(g.catalogUntil) {
				result := g.catalog
				g.mu.Unlock()
				return result, nil
			}
			g.mu.Unlock()
			// Discovery is shared across requests; one caller leaving must not cache
			// a canceled, partial snapshot for everyone else.
			ctx, cancel := context.WithTimeout(g.ctx, 5*time.Second)
			defer cancel()

			var result []toolBinding
			active := make(map[string]bool)
			names := make(map[string]bool)
			var after int64
			for {
				rows, err := g.store.Pool.Query(ctx, "SELECT id,key,name,description,kind,cost,input_schema,config FROM tools WHERE enabled AND id>$1 ORDER BY id LIMIT 128", after)
				if err != nil {
					return nil, err
				}
				var configured []toolRow
				for rows.Next() {
					var row toolRow
					if err = rows.Scan(&row.ID, &row.Key, &row.Name, &row.Description, &row.Kind, &row.Cost, &row.Schema, &row.Config); err != nil {
						rows.Close()
						return nil, err
					}
					configured = append(configured, row)
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					return nil, err
				}
				if len(configured) == 0 {
					break
				}
				after = configured[len(configured)-1].ID
				groups := make([][]toolBinding, len(configured))
				var workers errgroup.Group
				workers.SetLimit(8)
				for i, row := range configured {
					if row.Kind == "builtin" {
						if ValidateToolSchema(row.Schema) == nil {
							groups[i] = []toolBinding{{row: row, definition: &mcp.Tool{Name: row.Key, Description: row.Description, InputSchema: row.Schema}}}
						}
						continue
					}
					active[sessionKey(row)] = true
					workers.Go(func() error {
						discovery, cancel := context.WithTimeout(ctx, 2*time.Second)
						defer cancel()
						definitions, err := g.remoteTools(discovery, row, revision)
						if err != nil {
							return nil
						}
						for _, tool := range definitions {
							definition := *tool
							// Escaping every provider underscore makes the first double
							// underscore an unambiguous separator, even for keys containing _u.
							definition.Name = strings.ReplaceAll(row.Key, "_", "_u") + "__" + tool.Name
							groups[i] = append(groups[i], toolBinding{row: row, definition: &definition, remoteName: tool.Name})
						}
						return nil
					})
				}
				_ = workers.Wait()
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				for _, group := range groups {
					for _, binding := range group {
						if names[binding.definition.Name] {
							return nil, errors.New("duplicate public tool name")
						}
						names[binding.definition.Name] = true
						result = append(result, binding)
					}
				}
			}

			g.mu.Lock()
			if err := g.ctx.Err(); err != nil {
				g.mu.Unlock()
				return nil, err
			}
			if version != g.catalogVersion {
				g.mu.Unlock()
				return nil, nil
			}
			var retired []*upstreamSession
			var retiredPools []*httpSessionPool
			for key, pool := range g.httpPools {
				if !active[key] {
					retiredPools = append(retiredPools, pool)
				}
			}
			for key, session := range g.sessions {
				if !active[key] {
					retired = append(retired, session)
					delete(g.sessions, key)
				}
			}
			g.catalog = result
			g.catalogUntil = time.Now().Add(5 * time.Second)
			g.mu.Unlock()
			for _, pool := range retiredPools {
				pool.close(false)
			}
			for _, session := range retired {
				session.close()
			}
			return result, nil
		})
		select {
		case <-g.ctx.Done():
			return nil, g.ctx.Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-completed:
			if result.Err != nil {
				return nil, result.Err
			}
			// A newer revision can invalidate the cache after this flight
			// publishes but before its retired sessions finish closing.
			// Recheck the current cache instead of returning that snapshot.
			continue
		}
	}
}

func (g *Gateway) admit(ctx context.Context, p store.Principal) bool {
	for _, limit := range []struct {
		scope   string
		id      int64
		seconds int64
		max     int64
	}{{"token", p.TokenID, 60, 60}, {"user", p.UserID, 86400, 10000}} {
		allowed, err := g.store.AllowRequest(ctx, limit.scope, limit.id, limit.seconds, limit.max)
		if err != nil || !allowed {
			return false
		}
	}
	return true
}
