package main

import (
	"bufio"
	"bytes"
	"context"
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
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProductJourneyWithRealCLI(t *testing.T) {
	cliBinary := os.Getenv("LOADOUT_CLI_BINARY")
	serverBinary := os.Getenv("LOADOUT_SERVER_BINARY")
	raw := os.Getenv("LOADOUT_TEST_DATABASE_URL")
	if cliBinary == "" || serverBinary == "" || raw == "" {
		t.Skip("product journey requires real database and built CLI/server binaries")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	schema := fmt.Sprintf("journey_%d", time.Now().UnixNano())
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := conn.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Errorf("fixture cleanup failed: %v", err)
	}
	origin := "http://" + addr
	command := exec.CommandContext(ctx, serverBinary)
	command.Env = append(cleanProductEnv(), "LOADOUT_REDIS_URL="+os.Getenv("LOADOUT_TEST_REDIS_URL"), "LOADOUT_REDIS_NAMESPACE="+schema, "LOADOUT_DATABASE_URL="+u.String(), "LOADOUT_ADDR="+addr, "LOADOUT_PUBLIC_URL="+origin, "LOADOUT_ADMIN_USERNAME=operator", "LOADOUT_ADMIN_PASSWORD=correct horse battery staple", "LOADOUT_INITIAL_CREDITS=1000", "LOADOUT_WEB_DIR="+filepath.Join(filepath.Dir(filepath.Dir(serverBinary)), "web", "dist"))
	var logs bytes.Buffer
	command.Stdout = &logs
	command.Stderr = &logs
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 2 * time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, e := client.Get(origin + "/readyz")
		if e == nil {
			if err := response.Body.Close(); err != nil {
				t.Errorf("fixture cleanup failed: %v", err)
			}
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("product server did not become ready")
		}
		time.Sleep(25 * time.Millisecond)
	}
	request := func(method, path string, body any, want int) map[string]any {
		t.Helper()
		data, _ := json.Marshal(body)
		r, e := http.NewRequestWithContext(ctx, method, origin+"/api/v1"+path, bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		response, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer func() {
			if err := response.Body.Close(); err != nil {
				t.Errorf("fixture cleanup failed: %v", err)
			}
		}()
		var value map[string]any
		if response.StatusCode != 204 {
			if e = json.NewDecoder(response.Body).Decode(&value); e != nil {
				t.Fatal(e)
			}
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: %d %#v", method, path, response.StatusCode, value)
		}
		return value
	}
	request("POST", "/auth/register", map[string]string{"username": "journeyuser", "password": "correct horse battery staple"}, 201)
	created := request("POST", "/account/tokens", map[string]string{"name": "journey"}, 201)
	token := created["token"].(string)
	tokenID := int64(created["item"].(map[string]any)["id"].(float64))
	home := t.TempDir()
	configPath := filepath.Join(home, ".loadout", "config.json")
	env := append(cleanProductEnv(), "HOME="+home, "USERPROFILE="+home, "LOADOUT_CONFIG="+configPath, "NO_PROXY=*")
	runCLI := func(args ...string) string {
		t.Helper()
		command := exec.CommandContext(ctx, cliBinary, args...)
		command.Env = env
		output, e := command.CombinedOutput()
		if e != nil {
			t.Fatalf("CLI %s: %v %s", args[0], e, output)
		}
		return string(output)
	}
	runCLI("login", "--server", origin, "--token", token)
	runCLI("apply", "--clients", "codex,claude,cursor")
	for _, name := range []string{filepath.Join(home, ".codex", "config.toml"), filepath.Join(home, ".claude.json"), filepath.Join(home, ".cursor", "mcp.json")} {
		data, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(data, []byte(token)) {
			t.Fatal("gateway secret leaked into client configuration")
		}
		if !bytes.Contains(data, []byte("bridge")) {
			t.Fatal("bridge was not configured")
		}
	}
	bridge := exec.CommandContext(ctx, cliBinary, "bridge")
	bridge.Env = env
	mc := mcp.NewClient(&mcp.Implementation{Name: "journey", Version: "1"}, nil)
	session, err := mc.Connect(ctx, &mcp.CommandTransport{Command: bridge}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) < 2 {
		t.Fatalf("bridge tool list %v", err)
	}
	for range 2 {
		result, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "journey"}})
		if e != nil || result.IsError {
			payload, _ := json.Marshal(result)
			t.Fatalf("bridge call %v %s", e, payload)
		}
	}
	account := request("GET", "/account/me", nil, 200)
	balance := account["user"].(map[string]any)["balance"].(float64)
	if balance != 998 {
		t.Fatalf("balance=%v", balance)
	}
	usage := request("GET", "/account/usage", nil, 200)
	if len(usage["items"].([]any)) != 2 {
		t.Fatal("billing rows do not match calls")
	}
	request("DELETE", fmt.Sprintf("/account/tokens/%d", tokenID), nil, 204)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "revoked"}})
	if err == nil && !result.IsError {
		t.Fatal("revoked bridge token still worked")
	}

	// The same real HTTP server also connects operator funding, redemption,
	// shared team credit and the device login UI approval boundary.
	request("POST", "/auth/logout", nil, 204)
	request("POST", "/auth/login", map[string]string{"username": "operator", "password": "correct horse battery staple"}, 200)
	redemption := request("POST", "/admin/redemption-codes", map[string]any{"credits": 25, "note": "journey"}, 201)
	request("POST", "/auth/logout", nil, 204)
	request("POST", "/auth/login", map[string]string{"username": "journeyuser", "password": "correct horse battery staple"}, 200)
	redeemed := request("POST", "/account/redeem", map[string]any{"code": redemption["code"]}, 200)
	if redeemed["balance"].(float64) != 1023 {
		t.Fatalf("redemption balance=%v", redeemed["balance"])
	}
	request("POST", "/account/redeem", map[string]any{"code": redemption["code"]}, 409)
	team := request("POST", "/account/teams", map[string]any{"name": "Journey team"}, 201)
	teamID := int64(team["item"].(map[string]any)["id"].(float64))
	fundingPath := fmt.Sprintf("/account/teams/%d/fund", teamID)
	for range 2 {
		request("POST", fundingPath, map[string]any{"credits": 10, "idempotency_key": "journey-fund"}, 200)
	}
	teamToken := request("POST", "/account/tokens", map[string]any{"name": "Team CLI", "team_id": teamID}, 201)
	runCLI("login", "--server", origin, "--token", teamToken["token"].(string))
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "team"}})
	if err != nil || result.IsError {
		t.Fatalf("team bridge call failed: %v", err)
	}
	team = request("GET", fmt.Sprintf("/account/teams/%d", teamID), nil, 200)
	if team["item"].(map[string]any)["balance"].(float64) != 9 {
		t.Fatal("team balance did not follow bridge credential rotation")
	}

	// Read the CLI's displayed code, then perform the explicit approval that a
	// signed-in user would submit. The CLI itself cannot approve the device.
	device := exec.CommandContext(ctx, cliBinary, "login", "--device", "--server", origin)
	device.Env = env
	stdout, e := device.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	var diagnostic bytes.Buffer
	device.Stderr = &diagnostic
	if e = device.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if device.ProcessState == nil {
			_ = device.Process.Kill()
			_ = device.Wait()
		}
	}()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("device CLI did not display an authorization code")
	}
	fields := strings.Fields(scanner.Text())
	code := fields[len(fields)-1]
	if len(code) != 10 {
		t.Fatal("invalid displayed device code")
	}
	request("POST", "/account/devices/approve", map[string]string{"user_code": code}, 204)
	for scanner.Scan() {
	}
	if e = device.Wait(); e != nil {
		t.Fatalf("device login: %v %s", e, diagnostic.String())
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]string{"message": "device"}})
	if err != nil || result.IsError {
		t.Fatalf("device bridge call failed: %v", err)
	}
	account = request("GET", "/account/me", nil, 200)
	if account["user"].(map[string]any)["balance"].(float64) != 1012 {
		t.Fatal("device authorization did not select personal wallet")
	}
	runCLI("apply", "--remove", "--clients", "codex,claude,cursor")
	runCLI("logout")
	if _, err = os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("logout did not remove local credentials")
	}
	if strings.Contains(runCLI("doctor", "--server", origin), token) {
		t.Fatal("doctor leaked credential")
	}
}
