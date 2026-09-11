//go:build load

// 端到端压测：真实服务器子进程 + 隔离 schema + 真实 HTTP。
// 运行：make load-test（需 PLUGINPOCKET_TEST_DATABASE_URL 与已构建二进制，限流放宽由 env 注入）。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/gateway"
	"github.com/Yanyutin753/PluginPocket/server/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type loadResult struct {
	name     string
	duration time.Duration
	requests int64
	failures int64
	latency  []time.Duration
}

func (r *loadResult) report() {
	total := len(r.latency)
	qps := float64(r.requests) / r.duration.Seconds()
	sort.Slice(r.latency, func(i, j int) bool { return r.latency[i] < r.latency[j] })
	percentile := func(p float64) time.Duration {
		if total == 0 {
			return 0
		}
		index := int(float64(total-1) * p)
		return r.latency[index]
	}
	fmt.Printf("[%s] QPS=%.0f 请求=%d 失败=%d p50=%s p95=%s p99=%s max=%s\n",
		r.name, qps, r.requests, r.failures,
		percentile(0.50), percentile(0.95), percentile(0.99), percentile(1.0))
}

type account struct {
	id, wallet, token int64
	raw               string
}
type loadServer struct {
	origin   string
	accounts []account
	logs     bytes.Buffer
	conn     *pgx.Conn
}

// startLoadServer 起真实子进程 + 隔离 schema + 直接种子账号（放宽限流由 env 注入）。
func startLoadServer(t *testing.T, users int) *loadServer {
	t.Helper()
	raw, binary := os.Getenv("PLUGINPOCKET_TEST_DATABASE_URL"), os.Getenv("PLUGINPOCKET_SERVER_BINARY")
	if raw == "" || binary == "" {
		t.Skip("load test requires isolated PostgreSQL and built server binary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	t.Cleanup(cancel)
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("load_%d", time.Now().UnixNano())
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = conn.Close(context.Background())
	})
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	if _, err = conn.Exec(ctx, "SET search_path TO "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	opened, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	opened.Close()

	accounts := make([]account, users)
	for i := range accounts {
		accounts[i].raw = fmt.Sprintf("ppt_load_%d", i)
		sum := sha256.Sum256([]byte(accounts[i].raw))
		if err = conn.QueryRow(ctx, `WITH u AS (INSERT INTO users(username,password_hash) VALUES ($1,'h') RETURNING id),
		  w AS (INSERT INTO wallets(user_id,balance) SELECT id,1000000000000 FROM u RETURNING id,user_id)
		  INSERT INTO tokens(user_id,wallet_id,name,prefix,token_hash) SELECT user_id,id,'t','ppt_l',$2 FROM w RETURNING id`,
			fmt.Sprintf("load%d", i), hex.EncodeToString(sum[:])).Scan(&accounts[i].token); err != nil {
			t.Fatal(err)
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	origin := "http://" + addr
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PLUGINPOCKET_") {
			env = append(env, entry)
		}
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = 9
	}
	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = append(env,
		"PLUGINPOCKET_DATABASE_URL="+u.String(),
		"PLUGINPOCKET_ADDR="+addr,
		"PLUGINPOCKET_PUBLIC_URL="+origin,
		"PLUGINPOCKET_WEB_DIR="+filepath.Join(filepath.Dir(filepath.Dir(binary)), "web/dist"),
		"PLUGINPOCKET_ENCRYPTION_KEY="+base64.StdEncoding.EncodeToString(key),
		"PLUGINPOCKET_RATE_TOKEN_PER_MINUTE=100000000",
		"PLUGINPOCKET_RATE_USER_PER_DAY=100000000000",
		"PLUGINPOCKET_ALLOW_PRIVATE_UPSTREAMS=true",
	)
	var logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ready := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		response, err := ready.Get(origin + "/readyz")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		select {
		case err := <-done:
			t.Fatalf("server exited: %v\n%s", err, logs.String())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})

	return &loadServer{origin: origin, accounts: accounts, conn: conn}
}

func TestLoadGatewayQPS(t *testing.T) {
	server := startLoadServer(t, 200)
	origin, accounts := server.origin, server.accounts
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 256, MaxConnsPerHost: 256}}
	post := func(body string, token string) (int, string) {
		request, _ := http.NewRequest(http.MethodPost, origin+"/mcp", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err != nil {
			return 0, err.Error()
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return response.StatusCode, string(raw)
	}
	// 探测：stateless JSON 模式是否接受直接 tools/call。
	status, body := post(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hi"}}}`, accounts[0].raw)
	if status != 200 || !strings.Contains(body, `"result"`) {
		t.Fatalf("direct stateless tools/call unsupported: %d %s", status, body[:min(300, len(body))])
	}

	run := func(name string, workers int, duration time.Duration, modulus int, body func(i int) string) *loadResult {
		result := &loadResult{name: name, duration: duration, latency: make([]time.Duration, 0, 1<<18)}
		var halted int32
		var mu sync.Mutex
		var index atomic.Int64
		var wg sync.WaitGroup
		start := make(chan struct{})
		beginAt := time.Now()
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for atomic.LoadInt32(&halted) == 0 {
					i := int(index.Add(1)) % modulus
					begin := time.Now()
					status, response := post(body(i), accounts[i].raw)
					elapsed := time.Since(begin)
					mu.Lock()
					result.latency = append(result.latency, elapsed)
					result.requests++
					if status != 200 || strings.Contains(response, `"error"`) {
						result.failures++
					}
					mu.Unlock()
				}
			}()
		}
		close(start)
		timer := time.AfterFunc(duration, func() { atomic.StoreInt32(&halted, 1) })
		wg.Wait()
		timer.Stop()
		result.duration = time.Since(beginAt)
		return result
	}

	// 预热目录缓存。
	for i := 0; i < 8; i++ {
		post(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, accounts[i].raw)
	}
	load := run("tools/call echo（完整计量链路）", 32, 20*time.Second, len(accounts), func(i int) string {
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"m%d"}}}`, i)
	})
	load.report()
	listing := run("tools/list（目录缓存路径）", 16, 10*time.Second, len(accounts), func(i int) string {
		return `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	})
	listing.report()
	if load.failures > 0 || listing.failures > 0 {
		t.Fatalf("load failures: call=%d list=%d\n%s", load.failures, listing.failures, server.logs.String()[:min(2000, server.logs.Len())])
	}
	var logged int64
	if err := server.conn.QueryRow(context.Background(), "SELECT count(*) FROM usage_logs WHERE status='ok'").Scan(&logged); err != nil || logged < load.requests {
		t.Fatalf("metered calls=%d want>=%d err=%v", logged, load.requests, err)
	}
	fmt.Printf("计量一致：usage_logs ok=%d，请求数=%d\n", logged, load.requests)
}

// scrapeRuntime 从 /metrics 提取 goroutine 数与 RSS 字节数。
func scrapeRuntime(origin string) (goroutines int, rss int64, err error) {
	response, err := http.Get(origin + "/metrics")
	if err != nil {
		return 0, 0, err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "go_goroutines ") {
			fmt.Sscanf(line, "go_goroutines %d", &goroutines)
		}
		if strings.HasPrefix(line, "process_resident_memory_bytes ") {
			// prometheus 文本格式可能是科学计数法（9.6e+07）。
			if value, e := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "process_resident_memory_bytes ")), 64); e == nil {
				rss = int64(value)
			}
		}
	}
	return goroutines, rss, nil
}

// TestLoadGatewayMemorySafety：持续高压负载下采样 goroutine/RSS，
// 负载停止后断言 goroutine 回落、RSS 停止增长（无泄露稳态）。
func TestLoadGatewayMemorySafety(t *testing.T) {
	server := startLoadServer(t, 64)
	origin, accounts := server.origin, server.accounts
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 512, MaxConnsPerHost: 512}}
	post := func(token string) (int, string) {
		request, _ := http.NewRequest(http.MethodPost, origin+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"m"}}}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err != nil {
			return 0, err.Error()
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return response.StatusCode, string(raw)
	}
	// 预热（JIT 目录、连接池、堆增长到位）。
	for i := 0; i < 500; i++ {
		if status, _ := post(accounts[i%len(accounts)].raw); status != 200 {
			t.Fatalf("warmup status %d", status)
		}
	}
	time.Sleep(5 * time.Second)
	baselineGoroutines, baselineRSS, err := scrapeRuntime(origin)
	if err != nil {
		t.Fatal(err)
	}
	type sample struct {
		elapsed   time.Duration
		goroutine int
		rss       int64
	}
	var samples []sample
	var failures atomic.Int64
	var index atomic.Int64
	var halted int32
	var wg sync.WaitGroup
	loadStart := time.Now()
	for w := 0; w < 64; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.LoadInt32(&halted) == 0 {
				account := accounts[int(index.Add(1))%len(accounts)]
				if status, body := post(account.raw); status != 200 || strings.Contains(body, `"error"`) {
					failures.Add(1)
				}
			}
		}()
	}
	// 负载 60 秒：每 5 秒采样。
	for i := 0; i < 12; i++ {
		time.Sleep(5 * time.Second)
		goroutines, rss, err := scrapeRuntime(origin)
		if err != nil {
			t.Fatal(err)
		}
		samples = append(samples, sample{time.Since(loadStart), goroutines, rss})
		t.Logf("负载中 t=%s goroutines=%d rss=%dMB", time.Since(loadStart).Round(time.Second), goroutines, rss>>20)
	}
	atomic.StoreInt32(&halted, 1)
	wg.Wait()
	// 压测客户端主动关闭 keepalive 空闲连接，把连接 goroutine 与业务泄露分开归因；
	// 服务端 IdleTimeout=60s 兜底，不依赖客户端行为。
	client.CloseIdleConnections()
	time.Sleep(15 * time.Second)
	idleGoroutines, idleRSS, err := scrapeRuntime(origin)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("空闲后 goroutines=%d（基线 %d） rss=%dMB（基线 %dMB）", idleGoroutines, baselineGoroutines, idleRSS>>20, baselineRSS>>20)
	if failures.Load() > 0 {
		t.Fatalf("load failures: %d", failures.Load())
	}
	if idleGoroutines > baselineGoroutines+50 {
		t.Fatalf("goroutine leak: idle=%d baseline=%d", idleGoroutines, baselineGoroutines)
	}
	// Go 运行时高负载后堆目标变大、不立即归还 OS，空闲 RSS 高于基线属正常稳态；
	// 泄露的本质判据是负载内仍在单调增长（mid/last 断言）与 goroutine 回落。
	if idleRSS > baselineRSS*3 {
		t.Fatalf("RSS did not settle: idle=%d baseline=%d", idleRSS, baselineRSS)
	}
	// 负载后半段 RSS 不再单调上涨（对比中点与末段均值，增幅 <15%）。
	mid := samples[len(samples)/2]
	last := samples[len(samples)-1]
	if mid.rss > 0 && float64(last.rss-mid.rss)/float64(mid.rss) > 0.15 {
		t.Fatalf("RSS still climbing late in load: mid=%d last=%d", mid.rss, last.rss)
	}
	fmt.Printf("内存安全：goroutines 基线=%d 峰值末段=%d 空闲=%d；RSS 基线=%dMB 末段=%dMB 空闲=%dMB\n",
		baselineGoroutines, last.goroutine, idleGoroutines, baselineRSS>>20, last.rss>>20, idleRSS>>20)
}

// TestLoadGatewaySlowUpstreams：慢上游（长耗时 MCP 调用）× 高 QPS 的组合风险。
// 验证：快调用不被慢调用拖垮；同上游并发超过 32 租约槽时排队而非死锁；
// 超过上游 30s 超时的调用干净失败并退款；结束后无 goroutine 泄露。
func TestLoadGatewaySlowUpstreams(t *testing.T) {
	server := startLoadServer(t, 96)
	origin, accounts := server.origin, server.accounts
	// 进程内慢上游：按参数休眠后返回。
	slow := mcp.NewServer(&mcp.Implementation{Name: "slowup", Version: "1"}, nil)
	slow.AddTool(&mcp.Tool{Name: "slow", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"sleep_ms": map[string]any{"type": "number"}}}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var arguments struct {
			SleepMS int64 `json:"sleep_ms"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &arguments)
		select {
		case <-time.After(time.Duration(arguments.SleepMS) * time.Millisecond):
			return toolResultText("done"), nil
		case <-ctx.Done():
			return toolResultText("canceled"), nil
		}
	})
	upstream := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return slow }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true}))
	t.Cleanup(upstream.Close)
	key := make([]byte, 32)
	for i := range key {
		key[i] = 9
	}
	sealed, err := gateway.SealConfig(key, []byte(fmt.Sprintf(`{"url":%q}`, upstream.URL)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.conn.Exec(context.Background(), "INSERT INTO tools(key,name,kind,config) VALUES ('slowup','Slow upstream','http',$1)", sealed); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 512, MaxConnsPerHost: 512}}
	post := func(body string, token string) (int, string) {
		request, _ := http.NewRequest(http.MethodPost, origin+"/mcp", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err != nil {
			return 0, err.Error()
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return response.StatusCode, string(raw)
	}
	slowBody := func(ms int) string {
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"slowup__slow","arguments":{"sleep_ms":%d}}}`, ms)
	}
	echoBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"m"}}}`
	// 等目录收录慢上游。
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		_, body := post(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, accounts[0].raw)
		if strings.Contains(body, "slowup__slow") {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 阶段 A：12 路慢调用（8s）+ 48 路快调用，30 秒。
	var fastOK, fastFail, slowOK, slowFail atomic.Int64
	var index atomic.Int64
	var halted int32
	var wg sync.WaitGroup
	fastStart := time.Now()
	for w := 0; w < 48; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.LoadInt32(&halted) == 0 {
				account := accounts[int(index.Add(1))%len(accounts)]
				if status, body := post(echoBody, account.raw); status == 200 && !strings.Contains(body, `"error"`) {
					fastOK.Add(1)
				} else {
					fastFail.Add(1)
				}
			}
		}()
	}
	for w := 0; w < 12; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.LoadInt32(&halted) == 0 {
				account := accounts[int(index.Add(1))%len(accounts)]
				begin := time.Now()
				status, body := post(slowBody(8000), account.raw)
				elapsed := time.Since(begin)
				if status == 200 && strings.Contains(body, "done") {
					slowOK.Add(1)
					t.Logf("慢调用完成 %s", elapsed.Round(100*time.Millisecond))
				} else {
					slowFail.Add(1)
					t.Logf("慢调用异常 status=%d %s", status, body[:min(120, len(body))])
				}
			}
		}()
	}
	time.Sleep(30 * time.Second)
	atomic.StoreInt32(&halted, 1)
	wg.Wait()
	fastDuration := time.Since(fastStart)
	fmt.Printf("[阶段A 混合] 快调用 QPS=%.0f ok=%d fail=%d；慢调用 ok=%d fail=%d\n",
		float64(fastOK.Load())/fastDuration.Seconds(), fastOK.Load(), fastFail.Load(), slowOK.Load(), slowFail.Load())
	if fastFail.Load() > 0 || slowFail.Load() > 0 {
		t.Fatalf("混合负载出现失败 fast=%d slow=%d", fastFail.Load(), slowFail.Load())
	}

	// 阶段 B：80 路并发慢调用（8s）一次性打出——超过每上游 32 租约槽，应排队完成而非失败。
	burstStart := time.Now()
	var burstWG sync.WaitGroup
	var burstOK, burstFail atomic.Int64
	for w := 0; w < 80; w++ {
		burstWG.Add(1)
		go func(n int) {
			defer burstWG.Done()
			account := accounts[n%len(accounts)]
			status, body := post(slowBody(8000), account.raw)
			if status == 200 && strings.Contains(body, "done") {
				burstOK.Add(1)
			} else {
				burstFail.Add(1)
			}
		}(w)
	}
	burstWG.Wait()
	fmt.Printf("[阶段B 超槽并发] 80路8s慢调用 全部耗时=%s ok=%d fail=%d\n", time.Since(burstStart).Round(time.Second), burstOK.Load(), burstFail.Load())
	if burstFail.Load() > 0 {
		t.Fatalf("超槽并发出现失败 %d", burstFail.Load())
	}

	// 阶段 C：单次超过上游 30s 超时 → 干净失败 + 退款（usage_logs error 且 cost=0）。
	status, body := post(slowBody(35000), accounts[0].raw)
	t.Logf("[阶段C 超时] status=%d isError=%t", status, strings.Contains(body, `"isError":true`))
	var cost int64
	var state string
	if err = server.conn.QueryRow(context.Background(), "SELECT status,cost FROM usage_logs WHERE tool='slowup__slow' ORDER BY id DESC LIMIT 1").Scan(&state, &cost); err != nil {
		t.Fatal(err)
	}
	if state != "error" || cost != 0 {
		t.Fatalf("超时调用必须退款: status=%s cost=%d", state, cost)
	}

	// 收尾：连接归还后 goroutine 回落（慢上游池会保留有界空闲会话，阈值放宽）。
	client.CloseIdleConnections()
	time.Sleep(15 * time.Second)
	goroutines, rss, err := scrapeRuntime(origin)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("结束后 goroutines=%d rss=%dMB", goroutines, rss>>20)
	if goroutines > 400 {
		t.Fatalf("goroutine 异常堆积: %d", goroutines)
	}
	var pending int64
	if err = server.conn.QueryRow(context.Background(), "SELECT count(*) FROM usage_logs WHERE status='pending' AND tool='slowup__slow'").Scan(&pending); err != nil || pending > 0 {
		t.Fatalf("慢上游存在未结算调用: %d err=%v", pending, err)
	}
	fmt.Printf("结算一致：慢上游无 pending 残留\n")
}

func toolResultText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
