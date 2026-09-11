package httpapi

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the web error-code contract golden file")

func TestErrorCodesRegistry(t *testing.T) {
	codes := Codes()
	if len(codes) == 0 {
		t.Fatal("no error codes registered")
	}
	if !sort.StringsAreSorted(codes) {
		t.Fatalf("Codes() not sorted: %v", codes)
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
		statuses := statusesFor(code)
		if len(statuses) == 0 {
			t.Fatalf("code %q has no allowed status", code)
		}
		for _, status := range statuses {
			if status < 400 || status > 599 {
				t.Fatalf("code %q allows invalid status %d", code, status)
			}
		}
	}
	for _, code := range []string{"unauthorized", "invalid_request", "not_found", "internal_error"} {
		if !seen[code] {
			t.Fatalf("expected core code %q in registry", code)
		}
	}
}

// TestEmittedCodesRegistered scans every handler source for error-writing
// literals and fails on any code or (status, code) pair missing from the
// registry, so no unregistered code can reach the wire.
func TestEmittedCodesRegistered(t *testing.T) {
	literals := scanErrorLiterals(t)
	if len(literals) == 0 {
		t.Fatal("scan found no error literals; the scan itself is broken")
	}
	for _, l := range literals {
		if statusesFor(l.code) == nil {
			t.Errorf("%s: code %q is not registered", l.origin, l.code)
			continue
		}
		if !AllowedStatus(l.code, l.status) {
			t.Errorf("%s: status %d not allowed for code %q (allowed: %v)", l.origin, l.status, l.code, statusesFor(l.code))
		}
	}
}

// TestNoPlainTextHTTPErrors keeps every API error on the JSON envelope;
// http.Error/http.NotFound would bypass the registry contract.
func TestNoPlainTextHTTPErrors(t *testing.T) {
	for _, f := range handlerSources(t) {
		body := readSource(t, f)
		for _, banned := range []string{"http.Error(", "http.NotFound("} {
			for i, line := range strings.Split(body, "\n") {
				if strings.Contains(line, banned) {
					t.Errorf("%s:%d uses %s; use Fail(w, status, code) instead", f, i+1, strings.TrimSuffix(banned, "("))
				}
			}
		}
	}
}

func TestErrorCodesContract(t *testing.T) {
	entries := []map[string]any{}
	for _, code := range Codes() {
		entries = append(entries, map[string]any{"code": code, "status": statusesFor(code)})
	}
	got, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("..", "..", "..", "web", "src", "i18n", "error-codes.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("contract file missing (run go test ./internal/httpapi -run TestErrorCodesContract -update): %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("error-code contract out of date (run -update):\nwant:\n%s\ngot:\n%s", want, got)
	}
}

type errorLiteral struct {
	origin string
	status int
	code   string
}

var (
	failLiteral    = regexp.MustCompile(`\b(?:fail|failure|Fail)\(w, (\d{3}|http\.Status\w+), "([a-z_]+)"\)`)
	writeFailCheck = regexp.MustCompile(`\b(?:fail|failure|Fail)\(w, [^"]`)
	statusNames    = map[string]int{
		"http.StatusBadRequest":          400,
		"http.StatusUnauthorized":        401,
		"http.StatusForbidden":           403,
		"http.StatusNotFound":            404,
		"http.StatusMethodNotAllowed":    405,
		"http.StatusConflict":            409,
		"http.StatusGone":                410,
		"http.StatusTooManyRequests":     429,
		"http.StatusInternalServerError": 500,
		"http.StatusBadGateway":          502,
		"http.StatusServiceUnavailable":  503,
	}
)

func handlerSources(t *testing.T) []string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(dir)) // server/
	targets := []string{
		filepath.Join(root, "internal", "app"),
		filepath.Join(root, "internal", "identity"),
		filepath.Join(root, "internal", "httpapi"),
		filepath.Join(root, "internal", "gateway"),
		filepath.Join(root, "cmd", "loadout-server"),
	}
	var files []string
	for _, dir := range targets {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			files = append(files, filepath.Join(dir, name))
		}
	}
	if len(files) == 0 {
		t.Fatal("no handler sources found")
	}
	return files
}

func readSource(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func scanErrorLiterals(t *testing.T) []errorLiteral {
	t.Helper()
	var literals []errorLiteral
	for _, f := range handlerSources(t) {
		body := readSource(t, f)
		for _, m := range failLiteral.FindAllStringSubmatch(body, -1) {
			status, ok := statusNames[m[1]]
			if !ok {
				var parsed int
				if _, err := fmt.Sscanf(m[1], "%d", &parsed); err != nil {
					t.Fatalf("parse HTTP status %q: %v", m[1], err)
				}
				status = parsed
			}
			literals = append(literals, errorLiteral{origin: f, status: status, code: m[2]})
		}
		for i, line := range strings.Split(body, "\n") {
			if strings.Contains(line, "fail(w,") || strings.Contains(line, "failure(w,") || strings.Contains(line, "Fail(w,") {
				if strings.Contains(line, "Fail(w, status, code)") || strings.Contains(line, "httpapi.Fail(w, status, code)") {
					continue // the registry writer itself
				}
				if !failLiteral.MatchString(line) && writeFailCheck.MatchString(line) {
					t.Errorf("%s:%d non-literal error code escapes the registry scan: %s", f, i+1, strings.TrimSpace(line))
				}
			}
		}
	}
	return literals
}
