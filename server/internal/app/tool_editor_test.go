package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestToolEditorSharedIconAndPolicy(t *testing.T) {
	f := marketplaceApp(t, nil)
	b := New(replicaStore(t, f.store), Options{Origin: "http://example.com"})
	var id int64
	if err := f.store.Pool.QueryRow(t.Context(), "SELECT id FROM tools WHERE key='echo'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/v1/admin/tools/%d", id)
	body := map[string]any{"key": "echo", "name": "Echo", "description": "", "kind": "builtin", "enabled": true, "units_per_call": 1, "input_schema": map[string]any{"type": "object"}, "icon": "https://example.com/icon.png", "settlement": map[string]any{"content": map[string]any{"pattern": "ok"}}}
	save := func(h http.Handler) {
		t.Helper()
		raw, _ := json.Marshal(body)
		w := request(h, "PATCH", path, string(raw), f.admin)
		if w.Code != 200 {
			t.Fatalf("save %d: %s", w.Code, w.Body)
		}
	}
	check := func(h http.Handler, icon string, policy bool) {
		t.Helper()
		w := request(h, "GET", "/api/v1/admin/tools", "", f.admin)
		var out struct{ Items []map[string]any }
		if json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal(w.Body)
		}
		for _, item := range out.Items {
			if item["key"] == "echo" {
				if item["icon"] != icon {
					t.Fatalf("icon = %v want %s", item["icon"], icon)
				}
				raw, _ := json.Marshal(item["settlement"])
				if strings.Contains(string(raw), "pattern") != policy {
					t.Fatalf("policy = %s", raw)
				}
				return
			}
		}
		t.Fatal("missing echo")
	}
	save(f.handler)
	check(b, "https://example.com/icon.png", true)
	delete(body, "icon")
	delete(body, "settlement")
	save(b)
	check(f.handler, "https://example.com/icon.png", true)
	body["icon"] = ""
	body["settlement"] = nil
	save(b)
	check(f.handler, "", false)
	body["settlement"] = map[string]any{"content": map[string]any{"pattern": "ok"}}
	save(f.handler)
	body["settlement"] = map[string]any{}
	save(b)
	check(f.handler, "", false)
}

func TestToolEditorIconValidation(t *testing.T) {
	f := marketplaceApp(t, nil)
	svg := func(s string) string {
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(s))
	}
	cases := []struct {
		icon   string
		status int
	}{
		{"https://example.com/icon.png", 201},
		{"https://example.com/" + strings.Repeat("a", 2047-len("https://example.com/")), 201},
		{"https://example.com/" + strings.Repeat("a", 2048-len("https://example.com/")), 201},
		{"https://example.com/" + strings.Repeat("a", 2049-len("https://example.com/")), 400},
		{svg(`<?xml version="1.0" encoding="UTF-8"?><svg/>`), 201},
		{svg(`<?xml version='1.0' encoding='utf-8' standalone='yes'?><svg/>`), 201},
		{svg(`<?xml version="1.0"?><?xml version="1.0"?><svg/>`), 400},
		{svg(`<svg><?xml version="1.0"?></svg>`), 400},
		{svg(`<!-- comment --><?xml version="1.0"?><svg/>`), 400},
		{svg(`<?xml nope?><svg/>`), 400},
		{svg(`<?xml-stylesheet href="https://evil.test/style.css"?><svg/>`), 400},
		{svg(`<svg><text font-family="Arial" font-size="12" font-weight="bold" text-anchor="middle" dominant-baseline="central" letter-spacing="1"><tspan dx="1" dy="2">A&amp;B</tspan></text></svg>`), 201},
		{svg(`<svg><text href="https://evil.test">A</text></svg>`), 400},
		{svg(`<svg><text onload="alert(1)">A</text></svg>`), 400},
		{svg(`<svg><desc>` + strings.Repeat("x", 65512) + `</desc></svg>`), 201},
		{svg(`<svg><defs><linearGradient id="paint"><stop offset="0" stop-color="red"/></linearGradient></defs><path fill="url(#paint)" d="M0 0h1"/></svg>`), 201},
		{"data:image/png;base64,%%%", 400},
		{"data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg onload="alert(1)"/>`)), 400},
		{"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg onload="alert(1)"/>`)), 400},
		{"data:image/webp;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg onload="alert(1)"/>`)), 400},
		{"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", 201},
		{svg(`<!DOCTYPE svg [<!ENTITY x "bad">]><svg>&x;</svg>`), 400},
		{svg(`<svg><animate attributeName="href" values="javascript:alert(1)"/></svg>`), 400},
		{svg(`<svg/><svg/>`), 400}, {svg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="currentColor" d="M0 0h24v24z"/></svg>`), 201},
		{"http://example.com/icon.png", 400}, {"https://secret@example.com/icon.png", 400}, {"javascript:alert(1)", 400}, {"data:text/html;base64,PHN2Zy8+", 400}, {svg(`<svg onload="alert(1)"/>`), 400}, {svg(`<svg><script>alert(1)</script></svg>`), 400}, {svg(`<svg><foreignObject/></svg>`), 400}, {svg(`<svg><image href="https://evil.test/a"/></svg>`), 400}, {svg(`<svg><path style="fill:url(https://evil.test/a)"/></svg>`), 400}, {svg(`<svg><path fill="url(https://evil.test/a)"/></svg>`), 400}, {svg(`<svg>`), 400}, {svg(`<html/>`), 400}, {svg(`<svg/><!--`), 400}, {svg(`<svg/>` + strings.Repeat(" ", 65536)), 400},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			body := map[string]any{"key": fmt.Sprintf("icon-%d", i), "name": "Icon", "kind": "http", "enabled": true, "units_per_call": 0, "input_schema": map[string]any{"type": "object"}, "config": map[string]any{"url": f.upstream}, "icon": tc.icon}
			raw, _ := json.Marshal(body)
			w := request(f.handler, "POST", "/api/v1/admin/tools", string(raw), f.admin)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d: %s", w.Code, tc.status, w.Body)
			}
		})
	}
}

func TestToolEditorBuiltins(t *testing.T) {
	f := marketplaceApp(t, nil)
	unknown := request(f.handler, "POST", "/api/v1/admin/tools", `{"key":"invented","name":"Invalid builtin","kind":"builtin","enabled":true,"units_per_call":0,"input_schema":{"type":"object"}}`, f.admin)
	if unknown.Code != 400 {
		t.Fatalf("arbitrary builtin: %d", unknown.Code)
	}
	for _, key := range []string{"tools_catalog", "account_usage", "account_balance"} {
		var id int64
		if err := f.store.Pool.QueryRow(t.Context(), "SELECT id FROM tools WHERE key=$1", key).Scan(&id); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf(`{"key":%q,"name":"Edited","kind":"builtin","enabled":true,"units_per_call":0,"input_schema":{"type":"object"}}`, key)
		w := request(f.handler, "PATCH", fmt.Sprintf("/api/v1/admin/tools/%d", id), body, f.admin)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", key, w.Code, w.Body)
		}
	}
}

func TestToolEditorSettlementPreview(t *testing.T) {
	f := marketplaceApp(t, nil)
	path := "/api/v1/admin/tools/settlement-preview"
	snapshot := func() string {
		t.Helper()
		var result string
		err := f.store.Pool.QueryRow(t.Context(), "SELECT (SELECT count(*) FROM usage_logs)::text || ':' || (SELECT count(*) FROM ledger)::text || ':' || (SELECT sum(balance) FROM wallets)::text").Scan(&result)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := snapshot()
	defer func() {
		if after := snapshot(); after != before {
			t.Fatalf("preview wrote billing state: %s -> %s", before, after)
		}
	}()
	for _, tc := range []struct {
		cookie *http.Cookie
		status int
	}{{nil, 401}, {f.user, 403}} {
		w := request(f.handler, "POST", path, `{"settlement":{},"text":"","is_error":false}`, tc.cookie)
		if w.Code != tc.status {
			t.Fatalf("auth: %d", w.Code)
		}
	}
	for _, tc := range []struct {
		policy, text    string
		isError, charge bool
	}{{`{}`, strings.Repeat("x", 65536), false, true}, {`{}`, "ok", false, true}, {`{}`, "ok", true, false}, {`{"content":{"path":"code","equals":[0,200]}}`, `{"code":200}`, false, true}, {`{"content":{"path":"code","equals":0}}`, `{"code":500}`, false, false}, {`{"content":{"pattern":"^ok$"}}`, "ok", false, true}, {`{"script":"return JSON.parse(result.text).ok;"}`, `{"ok":true}`, false, true}, {`{"script":"throw new Error('bad');"}`, "", false, false}, {`{"script":"while(true){}"}`, "", false, false}} {
		body, _ := json.Marshal(map[string]any{"settlement": json.RawMessage(tc.policy), "text": tc.text, "is_error": tc.isError})
		w := request(f.handler, "POST", path, string(body), f.admin)
		if w.Code != 200 {
			t.Fatalf("preview: %d %s", w.Code, w.Body)
		}
		var out struct{ Charge bool }
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Charge != tc.charge {
			t.Fatalf("%s: charge=%v", tc.policy, out.Charge)
		}
	}
	for _, body := range []string{`{"settlement":{"script":"return ("},"text":""}`, `{"settlement":{},"text":` + fmt.Sprintf("%q", strings.Repeat("x", 65537)) + `}`, `{"settlement":[],"text":""}`} {
		w := request(f.handler, "POST", path, body, f.admin)
		if w.Code != 400 {
			t.Fatalf("invalid: %d", w.Code)
		}
	}
}
