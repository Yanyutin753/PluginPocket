package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Real processes and a private schema; no test hooks in production handlers.
type productFixture struct {
	t                         *testing.T
	ctx                       context.Context
	conn                      *pgx.Conn
	databaseURL, binary, root string
	redisURL, redisNamespace  string
}
type productProcess struct {
	f            *productFixture
	origin, addr string
	publicOrigin string
	command      *exec.Cmd
	done         chan error
	logs         bytes.Buffer
}

func newProductFixture(t *testing.T) *productFixture {
	t.Helper()
	raw, binary := os.Getenv("LOADOUT_TEST_DATABASE_URL"), os.Getenv("LOADOUT_SERVER_BINARY")
	if raw == "" || binary == "" {
		t.Skip("product E2E requires isolated PostgreSQL and built server")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("e2e_%d", time.Now().UnixNano())
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("remove isolated E2E schema: %v", err)
		}
		_ = conn.Close(cleanup)
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	if _, err = conn.Exec(ctx, "SET search_path TO "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	return &productFixture{t: t, ctx: ctx, conn: conn, databaseURL: u.String(), redisURL: os.Getenv("LOADOUT_TEST_REDIS_URL"), redisNamespace: schema, binary: binary, root: filepath.Dir(filepath.Dir(binary))}
}
func (f *productFixture) server() *productProcess {
	return f.serverWithOrigin("")
}

func (f *productFixture) serverWithOrigin(publicOrigin string) *productProcess {
	f.t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		f.t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	p := &productProcess{f: f, origin: "http://" + addr, addr: addr, publicOrigin: publicOrigin}
	f.t.Cleanup(func() { p.stop(false) })
	p.start()
	return p
}
func cleanProductEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "LOADOUT_") {
			env = append(env, entry)
		}
	}
	return env
}
func (p *productProcess) start() {
	p.f.t.Helper()
	p.logs.Reset()
	cmd := exec.CommandContext(p.f.ctx, p.f.binary)
	publicOrigin := p.publicOrigin
	if publicOrigin == "" {
		publicOrigin = p.origin
	}
	cmd.Env = append(cleanProductEnv(), "LOADOUT_REDIS_URL="+p.f.redisURL, "LOADOUT_REDIS_NAMESPACE="+p.f.redisNamespace, "LOADOUT_DATABASE_URL="+p.f.databaseURL, "LOADOUT_ADDR="+p.addr, "LOADOUT_PUBLIC_URL="+publicOrigin, "LOADOUT_WEB_DIR="+filepath.Join(p.f.root, "web/dist"), "LOADOUT_ADMIN_USERNAME=operator", "LOADOUT_ADMIN_PASSWORD=correct horse battery staple", "LOADOUT_INITIAL_CREDITS=1000", "LOADOUT_ALLOW_PRIVATE_UPSTREAMS=true", "LOADOUT_ENCRYPTION_KEY="+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	cmd.Stdout = &p.logs
	cmd.Stderr = &p.logs
	if err := cmd.Start(); err != nil {
		p.f.t.Fatal(err)
	}
	p.command = cmd
	p.done = make(chan error, 1)
	go func() { p.done <- cmd.Wait() }()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-p.done:
			p.command = nil
			p.f.t.Fatalf("product process exited before readiness: %v", err)
		default:
		}
		response, err := client.Get(p.origin + "/readyz")
		if err == nil {
			if err := response.Body.Close(); err != nil {
				p.f.t.Errorf("fixture cleanup failed: %v", err)
			}
			if response.StatusCode == 200 {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	p.stop(false)
	p.f.t.Fatal("product process readiness timed out")
}
func (p *productProcess) stop(graceful bool) {
	if p.command == nil {
		return
	}
	if graceful {
		_ = p.command.Process.Signal(os.Interrupt)
	} else {
		_ = p.command.Process.Kill()
	}
	select {
	case err := <-p.done:
		if graceful && err != nil {
			p.f.t.Errorf("graceful server exit: %v", err)
		}
	case <-time.After(7 * time.Second):
		_ = p.command.Process.Kill()
		<-p.done
		p.f.t.Error("server did not stop within deadline")
	}
	p.command = nil
}
func (p *productProcess) client() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 5 * time.Second}
}
func (p *productProcess) api(client *http.Client, method, path string, body any, want int) map[string]any {
	p.f.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		p.f.t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(p.f.ctx, method, p.origin+"/api/v1"+path, bytes.NewReader(raw))
	if err != nil {
		p.f.t.Fatal(err)
	}
	req.Header.Set("Origin", p.origin)
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		p.f.t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			p.f.t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	if response.StatusCode != want {
		p.f.t.Fatalf("%s %s status=%d want=%d", method, path, response.StatusCode, want)
	}
	result := map[string]any{}
	if want != 204 {
		if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
			p.f.t.Fatal(err)
		}
	}
	return result
}
func (p *productProcess) register(name string) (*http.Client, int64) {
	client := p.client()
	result := p.api(client, "POST", "/auth/register", map[string]string{"username": name, "password": "correct horse battery staple"}, 201)
	return client, int64(result["user"].(map[string]any)["id"].(float64))
}
func (p *productProcess) admin() *http.Client {
	client := p.client()
	p.api(client, "POST", "/auth/login", map[string]string{"username": "operator", "password": "correct horse battery staple"}, 200)
	return client
}
func (p *productProcess) token(client *http.Client) string {
	return p.api(client, "POST", "/account/tokens", map[string]string{"name": "E2E"}, 201)["token"].(string)
}
func (p *productProcess) mcp(token string) *mcp.ClientSession {
	p.f.t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "1"}, nil)
	s, err := c.Connect(p.f.ctx, &mcp.StreamableClientTransport{Endpoint: p.origin + "/mcp", HTTPClient: &http.Client{Transport: bearerHTTP{token}, Timeout: 40 * time.Second}, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		p.f.t.Fatal(err)
	}
	p.f.t.Cleanup(func() { _ = s.Close() })
	return s
}

type bearerHTTP struct{ token string }

func (b bearerHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	req := r.Clone(r.Context())
	req.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(req)
}
func (f *productFixture) balance(id int64) int64 {
	f.t.Helper()
	var balance int64
	if err := f.conn.QueryRow(f.ctx, "SELECT balance FROM wallets WHERE user_id=$1", id).Scan(&balance); err != nil {
		f.t.Fatal(err)
	}
	return balance
}
func TestProductJourneyWeb(t *testing.T) {
	if os.Getenv("LOADOUT_WEB_E2E") != "1" {
		t.Skip("Web E2E is launched explicitly by make test-e2e")
	}
	for _, mode := range []string{"production", "development"} {
		t.Run(mode, func(t *testing.T) {
			f := newProductFixture(t)
			var p *productProcess
			if mode == "development" {
				p = f.development()
			} else {
				p = f.server()
			}
			p.webJourney()
		})
	}
}

func (p *productProcess) webJourney() {
	f, t := p.f, p.f.t
	command := exec.CommandContext(f.ctx, "pnpm", "--dir", filepath.Join(f.root, "web"), "test:e2e")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = 2 * time.Second
	command.Env = append(cleanProductEnv(), "LOADOUT_E2E_ORIGIN="+p.origin, "LOADOUT_E2E_ADMIN_USERNAME=operator", "LOADOUT_E2E_ADMIN_PASSWORD=correct horse battery staple")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		t.Fatalf("real Web E2E: %v", err)
	}
	// Even success must have exercised durable business state.
	var users int
	if err := f.conn.QueryRow(f.ctx, "SELECT count(*) FROM users WHERE role='user'").Scan(&users); err != nil || users == 0 {
		t.Fatal("Web E2E created no real accounts")
	}
}

// Use the user's actual make up/down entrypoints, with private ports and data.
func (f *productFixture) development() *productProcess {
	f.t.Helper()
	freeAddress := func() string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			f.t.Fatal(err)
		}
		addr := listener.Addr().String()
		_ = listener.Close()
		return addr
	}
	apiAddr, webAddr := freeAddress(), freeAddress()
	_, webPort, _ := net.SplitHostPort(webAddr)
	p := &productProcess{f: f, origin: "http://" + webAddr, addr: webAddr}
	env := append(cleanProductEnv(), "LOADOUT_REDIS_URL="+f.redisURL, "LOADOUT_REDIS_NAMESPACE="+f.redisNamespace, "LOADOUT_DATABASE_URL="+f.databaseURL,
		"LOADOUT_ADDR="+apiAddr, "LOADOUT_API_ORIGIN=http://"+apiAddr,
		"LOADOUT_DEV_PORT="+webPort, "LOADOUT_PUBLIC_URL="+p.origin,
		"LOADOUT_RUN_DIR="+f.t.TempDir(), "LOADOUT_ADMIN_USERNAME=operator",
		"LOADOUT_ADMIN_PASSWORD=correct horse battery staple", "LOADOUT_INITIAL_CREDITS=1000",
		"LOADOUT_ENCRYPTION_KEY="+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)),
		"LOADOUT_WEB_DIR="+filepath.Join(f.root, "web/dist"), "LOADOUT_ALLOW_PRIVATE_UPSTREAMS=false",
		"LOADOUT_STDIO_COMMANDS=", "LOADOUT_GITHUB_CLIENT_ID=", "LOADOUT_GITHUB_CLIENT_SECRET=", "LOADOUT_GITHUB_ORG=",
		"LOADOUT_SMTP_ADDRESS=", "LOADOUT_SMTP_FROM=", "LOADOUT_SMTP_USERNAME=", "LOADOUT_SMTP_PASSWORD=", "LOADOUT_SMTP_ALLOW_LOCAL_INSECURE=false")
	f.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "make", "down")
		command.Dir, command.Env = f.root, env
		if output, err := command.CombinedOutput(); err != nil {
			f.t.Errorf("isolated development shutdown: %v %s", err, output)
		}
	})
	command := exec.CommandContext(f.ctx, "make", "up")
	command.Dir, command.Env = f.root, env
	if output, err := command.CombinedOutput(); err != nil {
		f.t.Fatalf("isolated development startup: %v %s", err, output)
	}
	response, err := p.client().Get(p.origin + "/readyz")
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			f.t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	var ready map[string]string
	if err := json.NewDecoder(response.Body).Decode(&ready); err != nil || response.StatusCode != 200 || ready["status"] != "ready" {
		f.t.Fatalf("development proxy did not reach ready database: status=%d decode=%v", response.StatusCode, err)
	}
	return p
}

func TestProductJourneyDevelopmentMCP(t *testing.T) {
	f := newProductFixture(t)
	p := f.development()
	user, id := p.register("dev-proxy-user")
	token := p.token(user)
	session := p.mcp(token)
	result, err := session.CallTool(f.ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"message": "through Vite"}})
	if err != nil || result.IsError {
		t.Fatalf("MCP through development origin: result=%v err=%v", result, err)
	}
	if got := f.balance(id); got != 999 {
		t.Fatalf("development MCP balance=%d want=999", got)
	}
}

func (p *productProcess) bridge(token, executable string) *mcp.ClientSession {
	p.f.t.Helper()
	cli := os.Getenv("LOADOUT_CLI_BINARY")
	if cli == "" {
		p.f.t.Skip("bridge E2E requires built CLI")
	}
	home := p.f.t.TempDir()
	env := append(cleanProductEnv(), "HOME="+home, "USERPROFILE="+home, "LOADOUT_CONFIG="+filepath.Join(home, "config.json"), "NO_PROXY=*")
	login := exec.CommandContext(p.f.ctx, cli, "login", "--server", p.origin, "--token", token)
	login.Env = env
	if output, err := login.CombinedOutput(); err != nil {
		p.f.t.Fatalf("isolated CLI login: %v %s", err, output)
	}
	command := exec.CommandContext(p.f.ctx, executable, "bridge")
	command.Env = env
	mc := mcp.NewClient(&mcp.Implementation{Name: "e2e-stdio", Version: "1"}, nil)
	session, err := mc.Connect(p.f.ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		p.f.t.Fatal(err)
	}
	p.f.t.Cleanup(func() { _ = session.Close() })
	return session
}
