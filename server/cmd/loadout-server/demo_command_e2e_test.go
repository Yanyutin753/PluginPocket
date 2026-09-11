package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProductJourneyDemoCommand(t *testing.T) {
	f := newProductFixture(t)
	p := f.server()
	binary := filepath.Join(f.root, "build/loadout-demo")
	upstream := exec.CommandContext(f.ctx, binary, "-listen", "127.0.0.1:0")
	stdout, err := upstream.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := upstream.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = upstream.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- upstream.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("mock shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			_ = upstream.Process.Kill()
			<-done
			t.Error("mock MCP did not stop")
		}
	})
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
		} else {
			ready <- ""
		}
	}()
	var upstreamURL string
	select {
	case line := <-ready:
		upstreamURL = strings.TrimPrefix(line, "Demo MCP ready: ")
		if !strings.HasPrefix(upstreamURL, "http://127.0.0.1:") {
			t.Fatal("mock MCP did not report loopback readiness")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("mock startup timed out")
	}
	cmd := exec.CommandContext(f.ctx, binary, "-origin", p.origin, "-upstream", upstreamURL)
	cmd.Env = append(cleanProductEnv(), "LOADOUT_ADMIN_USERNAME=operator", "LOADOUT_ADMIN_PASSWORD=correct horse battery staple")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("demo command: %v %s", err, output)
	}
	if !strings.Contains(string(output), "Demo data ready") {
		t.Fatalf("demo command did not report initialized data: %s", output)
	}
	client := p.client()
	p.api(client, "POST", "/auth/login", map[string]string{"username": "demo_owner", "password": "Loadout-demo-only-2026!"}, 200)
	if len(p.api(client, "GET", "/account/teams", nil, 200)["items"].([]any)) != 1 {
		t.Fatal("demo command did not populate team page")
	}
}
