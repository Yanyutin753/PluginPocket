// Package demo prepares local demonstration data through the real public API.
package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Password = "PluginPocket-demo-only-2026!"

type response struct {
	User struct {
		ID      int64
		Balance int64
	}
	Item       struct{ ID int64 }
	Token      string
	Code       string
	UserCode   string `json:"user_code"`
	DeviceCode string `json:"device_code"`
}

type seeder struct {
	ctx    context.Context
	origin string
	err    error
}

func client() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Requests stop at the first failure. Errors never include credentials or response bodies.
func (s *seeder) api(c *http.Client, method, path string, body any, status int) response {
	var result response
	if s.err != nil {
		return result
	}
	raw, err := json.Marshal(body)
	if err != nil {
		s.err = errors.New("encode demo request")
		return result
	}
	req, err := http.NewRequestWithContext(s.ctx, method, s.origin+"/api/v1"+path, bytes.NewReader(raw))
	if err != nil {
		s.err = errors.New("invalid demo endpoint")
		return result
	}
	req.Header.Set("Origin", s.origin)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		s.err = fmt.Errorf("demo %s %s: request failed", method, path)
		return result
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != status {
		s.err = fmt.Errorf("demo %s %s: status %d, expected %d (use an empty demo database)", method, path, res.StatusCode, status)
		return result
	}
	if status != 204 && json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result) != nil {
		s.err = errors.New("invalid demo API response")
	}
	return result
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}
func (s *seeder) call(token, name string, args any, wantError bool) {
	if s.err != nil {
		return
	}
	c := mcp.NewClient(&mcp.Implementation{Name: "pluginpocket-demo", Version: "1"}, nil)
	session, err := c.Connect(s.ctx, &mcp.StreamableClientTransport{Endpoint: s.origin + "/mcp", HTTPClient: &http.Client{Transport: bearer{token}, Timeout: 10 * time.Second}, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		s.err = errors.New("demo MCP connection failed")
		return
	}
	defer func() { _ = session.Close() }()
	result, err := session.CallTool(s.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil || result.IsError != wantError {
		s.err = fmt.Errorf("demo MCP scenario %s failed", name)
	}
}

// Seed is intentionally one-shot: existing demo usernames fail before money is added.
func Seed(ctx context.Context, origin, upstream, username, password string) error {
	if err := localOrigin(origin); err != nil {
		return err
	}
	s := &seeder{ctx: ctx, origin: origin}
	admin := client()
	s.api(admin, "POST", "/auth/login", map[string]any{"username": username, "password": password}, 200)
	defer s.logout(admin)
	owner, member, empty, disabled := client(), client(), client(), client()
	accounts := []struct {
		name string
		c    *http.Client
	}{{"demo_owner", owner}, {"demo_member", member}, {"demo_empty", empty}, {"demo_disabled", disabled}}
	ids := make([]int64, len(accounts))
	for i, a := range accounts {
		result := s.api(a.c, "POST", "/auth/register", map[string]any{"username": a.name, "password": Password}, 201)
		ids[i] = result.User.ID
		defer s.logout(a.c)
	}
	s.api(admin, "POST", fmt.Sprintf("/admin/users/%d/balance", ids[0]), map[string]any{"delta": 5000, "note": "DEMO: operator adjustment", "idempotency_key": "demo-initial"}, 200)
	me := s.api(empty, "GET", "/account/me", nil, 200)
	if me.User.Balance > 0 {
		s.api(admin, "POST", fmt.Sprintf("/admin/users/%d/balance", ids[2]), map[string]any{"delta": -me.User.Balance, "note": "DEMO: exhausted balance", "idempotency_key": "demo-empty"}, 200)
	}
	for _, plan := range []struct {
		name           string
		credits, price int
		enabled        bool
	}{{"DEMO Starter", 1000, 1000, true}, {"DEMO Pro", 10000, 5000, true}, {"DEMO Archived", 500, 500, false}} {
		p := s.api(admin, "POST", "/admin/plans", map[string]any{"name": plan.name, "credits": plan.credits, "price_cents": plan.price, "currency": "CNY", "enabled": plan.enabled}, 201)
		if plan.enabled {
			s.api(owner, "POST", "/account/orders", map[string]any{"plan_id": p.Item.ID}, 503)
		}
	}
	code := s.api(admin, "POST", "/admin/redemption-codes", map[string]any{"credits": 500, "note": "DEMO: redeemed"}, 201)
	s.api(owner, "POST", "/account/redeem", map[string]any{"code": code.Code}, 200)
	s.api(admin, "POST", "/admin/redemption-codes", map[string]any{"credits": 2000, "note": "DEMO: unredeemed"}, 201)
	team := s.api(owner, "POST", "/account/teams", map[string]any{"name": "DEMO Workshop"}, 201)
	teamPath := fmt.Sprintf("/account/teams/%d", team.Item.ID)
	invite := s.api(owner, "POST", teamPath+"/invites", map[string]any{}, 201)
	s.api(member, "POST", "/account/team-invites/accept", map[string]any{"code": invite.Code}, 200)
	s.api(owner, "POST", teamPath+"/fund", map[string]any{"credits": 600, "idempotency_key": "demo-team"}, 200)
	for _, a := range accounts[:3] {
		token := s.api(a.c, "POST", "/account/tokens", map[string]any{"name": "DEMO personal"}, 201)
		for range 3 {
			s.call(token.Token, "echo", map[string]any{"message": "DEMO: hello from " + a.name}, a.name == "demo_empty")
		}
		if a.name == "demo_owner" && upstream != "" {
			s.api(admin, "POST", "/admin/tools", map[string]any{"key": "demo", "name": "DEMO MCP upstream", "description": "Local deterministic success and failure scenarios", "kind": "http", "enabled": true, "units_per_call": 7, "input_schema": map[string]any{"type": "object"}, "config": map[string]any{"url": upstream}}, 201)
			s.call(token.Token, "demo__success", map[string]any{}, false)
			s.call(token.Token, "demo__failure", map[string]any{}, true)
		}
	}
	teamToken := s.api(member, "POST", "/account/tokens", map[string]any{"name": "DEMO team", "team_id": team.Item.ID}, 201)
	s.call(teamToken.Token, "time_now", map[string]any{}, false)
	revoked := s.api(owner, "POST", "/account/tokens", map[string]any{"name": "DEMO retired laptop"}, 201)
	s.api(owner, "DELETE", fmt.Sprintf("/account/tokens/%d", revoked.Item.ID), nil, 204)
	device := s.api(owner, "POST", "/device/authorize", map[string]any{}, 200)
	s.api(owner, "POST", "/account/devices/approve", map[string]any{"user_code": device.UserCode}, 204)
	s.api(owner, "POST", "/device/token", map[string]any{"device_code": device.DeviceCode}, 200)
	s.api(admin, "PATCH", fmt.Sprintf("/admin/users/%d", ids[3]), map[string]any{"enabled": false}, 200)
	if s.err != nil {
		return s.err
	}
	return Expand(ctx, origin, username, password)
}

func (s *seeder) logout(c *http.Client) {
	// Only sessions actually created by this run need cleanup. Respect the
	// operation's cancellation and cap an individual logout at one second.
	u, err := url.Parse(s.origin)
	if err != nil || len(c.Jar.Cookies(u)) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()
	cleanup := &seeder{ctx: ctx, origin: s.origin}
	cleanup.api(c, "POST", "/auth/logout", map[string]any{}, 204)
}

func localOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return errors.New("demo requires a loopback HTTP origin")
	}
	return nil
}
