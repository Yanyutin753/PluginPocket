package gateway

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/cache"
	"github.com/Yanyutin753/PluginPocket/server/internal/httpapi"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/dop251/goja"
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
	// TokenPerMinute/UserPerDay 覆盖默认限流（60/分钟/令牌、10000/日/用户），
	// 0 表示保持默认；大流量部署按容量调整。
	TokenPerMinute int64
	UserPerDay     int64
}
type Gateway struct {
	store           *store.Store
	options         Options
	mu              sync.Mutex
	scripts         sync.Map // settlement 脚本编译缓存：源码 → *goja.Program
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
	Settlement                   json.RawMessage
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
	if !ok || !strings.HasPrefix(raw, "ppt_") || g.store == nil {
		httpapi.Fail(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	principal, err := g.store.AuthToken(r.Context(), raw)
	if err != nil {
		if errors.Is(err, store.ErrUnauthorized) {
			httpapi.Fail(w, http.StatusUnauthorized, "unauthorized")
		} else {
			httpapi.Fail(w, http.StatusServiceUnavailable, "gateway_unavailable")
		}
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		httpapi.Fail(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	bindings, err := g.tools(ctx)
	cancel()
	if err != nil {
		httpapi.Fail(w, http.StatusServiceUnavailable, "gateway_unavailable")
		return
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "pluginpocket", Version: "0.2.0"}, nil)
	for _, binding := range bindings {
		server.AddTool(binding.definition, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return g.call(ctx, principal, binding, req.Params.Arguments), nil
		})
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true, MaxRequestBodyBytes: 1 << 20})
	http.NewCrossOriginProtection().Handler(handler).ServeHTTP(w, r)
}
func toolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "[pluginpocket] " + message}}}
}
func toolText(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}
func toolJSON(value any) *mcp.CallToolResult {
	encoded, err := json.Marshal(value)
	if err != nil {
		return toolError("响应编码失败")
	}
	return toolText(string(encoded))
}

// PreviewSettlement evaluates one sample through the billing decision without I/O.
func (g *Gateway) PreviewSettlement(settlement json.RawMessage, text string, isError bool) bool {
	return g.settleResult(settlement, &mcp.CallToolResult{IsError: isError, Content: []mcp.Content{&mcp.TextContent{Text: text}}})
}

// settleResult 是扣费结算中间件：true 才扣费，false 触发 finish 的退款路径。
// 空/非法 settlement 保持默认语义（上游 isError 即失败）。
func (g *Gateway) settleResult(settlement json.RawMessage, result *mcp.CallToolResult) bool {
	if result.IsError || len(settlement) == 0 {
		return !result.IsError
	}
	text := ""
	for _, block := range result.Content {
		if t, ok := block.(*mcp.TextContent); ok {
			text += t.Text
		}
	}
	var policy struct {
		Script  string `json:"script"`
		Content *struct {
			Path    string          `json:"path"`
			Equals  json.RawMessage `json:"equals"`
			Pattern string          `json:"pattern"`
		} `json:"content"`
	}
	if json.Unmarshal(settlement, &policy) != nil {
		return true
	}
	if policy.Script != "" {
		return g.runSettlementScript(policy.Script, text, result.IsError)
	}
	if policy.Content == nil {
		return true
	}
	if policy.Content.Pattern != "" {
		matched, err := regexp.MatchString(policy.Content.Pattern, text)
		return err == nil && matched
	}
	if policy.Content.Path == "" || len(policy.Content.Equals) == 0 {
		return true
	}
	var document any
	if json.Unmarshal([]byte(text), &document) != nil {
		return false
	}
	current := document
	for _, segment := range strings.Split(policy.Content.Path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = object[segment]
		if !ok {
			return false
		}
	}
	var expected any
	if json.Unmarshal(policy.Content.Equals, &expected) != nil {
		return false
	}
	values := []any{expected}
	if list, ok := expected.([]any); ok {
		values = list
	}
	for _, candidate := range values {
		if number, ok := current.(float64); ok {
			if want, ok := candidate.(float64); ok && number == want {
				return true
			}
			continue
		}
		if fmt.Sprintf("%v", current) == fmt.Sprintf("%v", candidate) {
			return true
		}
	}
	return false
}

// runSettlementScript 在 goja 沙箱里执行管理员脚本：无任何宿主绑定（无 I/O），
// 200ms 中断上限；编译结果按源码缓存。脚本异常、超时或非真值都判失败（退款侧）。
func (g *Gateway) runSettlementScript(source, text string, isError bool) bool {
	program, ok := g.scripts.Load(source)
	if !ok {
		compiled, err := goja.Compile("settlement", settlementScriptWrapper(source), true)
		if err != nil {
			return false
		}
		program = compiled
		g.scripts.Store(source, compiled)
	}
	vm := goja.New()
	if err := vm.Set("result", map[string]any{"isError": isError, "text": text}); err != nil {
		return false
	}
	timer := time.AfterFunc(200*time.Millisecond, func() { vm.Interrupt("settlement script timeout") })
	defer timer.Stop()
	value, err := vm.RunProgram(program.(*goja.Program))
	return err == nil && value != nil && value.ToBoolean()
}

// settlementScriptWrapper 把管理员脚本包成接收 result 的函数体：
// ES 禁止顶层 return，包装后支持 return/多语句/循环。
func settlementScriptWrapper(source string) string {
	return "(function (result) {\n" + source + "\n})(result)"
}

// ValidateSettlementScript 供保存侧校验：大小上限 + 可编译（严格模式）。
func ValidateSettlementScript(source string) error {
	if len(source) == 0 || len(source) > 8192 {
		return errors.New("settlement script must be 1-8192 bytes")
	}
	_, err := goja.Compile("settlement", settlementScriptWrapper(source), true)
	return err
}

// markUnsettled 在保持上游内容可见的同时标记失败并说明已退款。
func markUnsettled(result *mcp.CallToolResult) {
	if result.IsError {
		return
	}
	result.IsError = true
	for _, block := range result.Content {
		if text, ok := block.(*mcp.TextContent); ok && !strings.HasPrefix(text.Text, "[pluginpocket]") {
			text.Text = "[pluginpocket] 结算检查未通过，本次不扣费：" + text.Text
		}
	}
}

func (g *Gateway) accountUsage(ctx context.Context, p store.Principal, args json.RawMessage) *mcp.CallToolResult {
	var input struct {
		Limit *int `json:"limit"`
	}
	if json.Unmarshal(args, &input) != nil {
		return toolError("limit 必须为整数")
	}
	limit := 20
	if input.Limit != nil {
		if *input.Limit < 1 || *input.Limit > 50 {
			return toolError("limit 必须在 1 到 50 之间")
		}
		limit = *input.Limit
	}
	now := time.Now().UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	// 排除 pending：汇总与最近记录只描述已结算（或被拒）的调用，不含正在执行的本次调用。
	summary := struct {
		TodayCalls int64 `json:"today_calls"`
		MonthCost  int64 `json:"month_cost"`
	}{}
	if err := g.store.Pool.QueryRow(ctx, "SELECT count(*) FILTER(WHERE created_at >= $2),COALESCE(sum(cost),0) FROM usage_logs WHERE user_id=$1 AND status!='pending' AND created_at >= $3", p.UserID, day, month).Scan(&summary.TodayCalls, &summary.MonthCost); err != nil {
		return toolError("暂时无法读取用量")
	}
	rows, err := g.store.Pool.Query(ctx, "SELECT tool,cost,status,created_at FROM usage_logs WHERE user_id=$1 AND status!='pending' ORDER BY id DESC LIMIT $2", p.UserID, limit)
	if err != nil {
		return toolError("暂时无法读取用量")
	}
	defer rows.Close()
	recent := []map[string]any{}
	for rows.Next() {
		var tool, status string
		var cost int64
		var at time.Time
		if rows.Scan(&tool, &cost, &status, &at) != nil {
			return toolError("暂时无法读取用量")
		}
		recent = append(recent, map[string]any{"tool": tool, "cost": cost, "status": status, "at": at.UTC().Format(time.RFC3339)})
	}
	if rows.Err() != nil {
		return toolError("暂时无法读取用量")
	}
	return toolJSON(map[string]any{"summary": summary, "recent": recent})
}

func (g *Gateway) call(parent context.Context, p store.Principal, binding toolBinding, args json.RawMessage) *mcp.CallToolResult {
	ctx, cancel := context.WithTimeout(parent, g.options.Timeout)
	defer cancel()
	key := rand.Text()
	if !g.admit(ctx, p) {
		result := toolError("调用频率或每日额度已达上限")
		g.recordDenied(ctx, p, binding.definition.Name, key, callData(args, result))
		return result
	}
	call, err := g.store.ReserveTool(ctx, p.UserID, p.TokenID, p.WalletID, binding.row.ID, binding.definition.Name, key)
	if errors.Is(err, store.ErrNotFound) {
		result := toolError("工具当前不可用")
		g.recordDenied(ctx, p, binding.definition.Name, key, callData(args, result))
		return result
	}
	if errors.Is(err, store.ErrInsufficientBalance) {
		result := toolError("额度不足，请充值后重试")
		_ = g.store.FinishWithData(ctx, call.ID, false, 0, callData(args, result))
		return result
	}
	if err != nil {
		return toolError("暂时无法开始调用")
	}
	started := time.Now()
	result := g.execute(ctx, p, binding, args)
	// 结算中间件：默认仅协议层 isError 判失败；配置了 content 规则时叠加业务体检查。
	settled := g.settleResult(binding.row.Settlement, result)
	if !settled {
		markUnsettled(result)
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if err = g.store.FinishWithData(finishCtx, call.ID, settled, time.Since(started), callData(args, result)); err != nil {
		return toolError("调用结算待恢复，请查看用量记录，勿重复执行有副作用的操作")
	}
	return result
}

func (g *Gateway) recordDenied(ctx context.Context, p store.Principal, name, key string, data *store.CallData) {
	_, _ = g.store.Pool.Exec(ctx, "INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,status,request_key,finished_at,input_data,output_data,input_truncated,output_truncated) VALUES ($1,$2,$3,$4,0,'denied',$5,now(),$6,$7,$8,$9)", p.UserID, p.TokenID, p.WalletID, name, key, data.InputData, data.OutputData, data.InputTruncated, data.OutputTruncated)
}
func (g *Gateway) execute(ctx context.Context, p store.Principal, b toolBinding, args json.RawMessage) *mcp.CallToolResult {
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
		case "account_balance":
			var balance int64
			if g.store.Pool.QueryRow(ctx, "SELECT balance FROM wallets WHERE id=$1", p.WalletID).Scan(&balance) != nil {
				return toolError("暂时无法读取余额")
			}
			return toolJSON(map[string]any{"username": p.Username, "balance": balance})
		case "account_usage":
			return g.accountUsage(ctx, p, args)
		case "tools_catalog":
			rows, err := g.store.Pool.Query(ctx, "SELECT key,description,cost FROM tools WHERE enabled ORDER BY id")
			if err != nil {
				return toolError("暂时无法读取工具目录")
			}
			defer rows.Close()
			entries := []map[string]any{}
			for rows.Next() {
				var key, description string
				var cost int64
				if rows.Scan(&key, &description, &cost) != nil {
					return toolError("暂时无法读取工具目录")
				}
				entries = append(entries, map[string]any{"key": key, "description": description, "cost_per_call": cost})
			}
			if rows.Err() != nil {
				return toolError("暂时无法读取工具目录")
			}
			return toolJSON(entries)
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
				rows, err := g.store.Pool.Query(ctx, "SELECT id,key,name,description,kind,cost,input_schema,config,settlement FROM tools WHERE enabled AND id>$1 ORDER BY id LIMIT 128", after)
				if err != nil {
					return nil, err
				}
				var configured []toolRow
				for rows.Next() {
					var row toolRow
					if err = rows.Scan(&row.ID, &row.Key, &row.Name, &row.Description, &row.Kind, &row.Cost, &row.Schema, &row.Config, &row.Settlement); err != nil {
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
						g.applyOverrides(discovery, row.ID, definitions)
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

// UpstreamTools 发现某个 http/stdio 上游当前暴露的原始工具定义（含 Redis 缓存，
// 不应用 tool_metadata_overrides；覆盖态由 app 层另行合并展示）。
func (g *Gateway) UpstreamTools(ctx context.Context, toolID int64) ([]*mcp.Tool, error) {
	if err := g.ctx.Err(); err != nil {
		return nil, err
	}
	var row toolRow
	err := g.store.Pool.QueryRow(ctx, "SELECT id,key,name,description,kind,cost,input_schema,config,settlement FROM tools WHERE id=$1 AND enabled AND kind IN ('http','stdio')", toolID).Scan(&row.ID, &row.Key, &row.Name, &row.Description, &row.Kind, &row.Cost, &row.Schema, &row.Config, &row.Settlement)
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	revision := g.catalogRevision
	g.mu.Unlock()
	return g.remoteTools(ctx, row, revision)
}

// applyOverrides 按 (tool_id, remote_name) 用后台维护的描述/参数 schema 覆盖上游定义。
// 无效 schema 的覆盖整体忽略；空描述表示仅覆盖 schema。失败时保持上游定义，目录可用性优先。
func (g *Gateway) applyOverrides(ctx context.Context, toolID int64, definitions []*mcp.Tool) {
	rows, err := g.store.Pool.Query(ctx, "SELECT remote_name,description,input_schema FROM tool_metadata_overrides WHERE tool_id=$1", toolID)
	if err != nil {
		return
	}
	defer rows.Close()
	type override struct {
		description string
		schema      []byte
	}
	byName := map[string]override{}
	for rows.Next() {
		var name, description string
		var schema []byte
		if rows.Scan(&name, &description, &schema) != nil {
			return
		}
		byName[name] = override{description, schema}
	}
	if rows.Err() != nil {
		return
	}
	for _, tool := range definitions {
		o, ok := byName[tool.Name]
		if !ok {
			continue
		}
		if len(o.schema) > 0 {
			if ValidateToolSchema(o.schema) != nil {
				continue
			}
			var parsed any
			if json.Unmarshal(o.schema, &parsed) != nil {
				continue
			}
			tool.InputSchema = parsed
		}
		if o.description != "" {
			tool.Description = o.description
		}
	}
}

func (g *Gateway) admit(ctx context.Context, p store.Principal) bool {
	tokenPerMinute := g.options.TokenPerMinute
	if tokenPerMinute <= 0 {
		tokenPerMinute = 60
	}
	userPerDay := g.options.UserPerDay
	if userPerDay <= 0 {
		userPerDay = 10000
	}
	for _, limit := range []struct {
		scope   string
		id      int64
		seconds int64
		max     int64
	}{{"token", p.TokenID, 60, tokenPerMinute}, {"user", p.UserID, 86400, userPerDay}} {
		allowed, err := g.store.AllowRequest(ctx, limit.scope, limit.id, limit.seconds, limit.max)
		if err != nil || !allowed {
			return false
		}
	}
	return true
}
