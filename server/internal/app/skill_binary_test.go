package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yanyutin753/loadout/server/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSkillBinaryRoundTripAndTextEditPreservesAttachment(t *testing.T) {
	f := marketplaceApp(t, nil)
	publish := func(body string, status int) {
		t.Helper()
		w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin)
		if w.Code != status {
			t.Fatalf("publish=%d %s", w.Code, w.Body)
		}
	}
	publish(`{"slug":"binary-test","name":"Binary","source":"inline","files":{"SKILL.md":"# Binary"},"files_v2":{"bin/run":{"encoding":"base64","content":"AP+A","executable":true}}}`, 201)
	check := func() {
		t.Helper()
		w := request(f.handler, "GET", "/api/v1/marketplace/binary-test/files?format=2", "", f.user)
		var got struct {
			Files map[string]struct {
				Content    string
				Encoding   string
				Executable bool
				Size       int64
				SHA256     string
			}
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil {
			t.Fatalf("download=%d %s", w.Code, w.Body)
		}
		file := got.Files["bin/run"]
		if file.Content != "AP+A" || file.Encoding != "base64" || !file.Executable || file.Size != 3 || len(file.SHA256) != 64 {
			t.Fatalf("binary changed: %+v", file)
		}
	}
	check()
	publish(`{"slug":"binary-test","name":"Binary","source":"inline","files":{"SKILL.md":"# Edited"}}`, 200)
	check()
	if w := request(f.handler, "GET", "/api/v1/marketplace/binary-test/files", "", f.user); w.Code != 400 {
		t.Fatalf("legacy binary response=%d %s", w.Code, w.Body)
	}
	publish(`{"slug":"binary-test","name":"Binary","source":"inline","delete_files":["SKILL.md"]}`, 400)
	publish(`{"slug":"binary-test","name":"Binary","source":"inline","delete_files":["bin/run"]}`, 200)
	if w := request(f.handler, "GET", "/api/v1/marketplace/binary-test/files", "", f.user); w.Code != 200 {
		t.Fatalf("legacy text=%d %s", w.Code, w.Body)
	}
}

type skillDeadlineRecorder struct {
	*httptest.ResponseRecorder
	read, write time.Time
}

func (w *skillDeadlineRecorder) SetReadDeadline(at time.Time) error  { w.read = at; return nil }
func (w *skillDeadlineRecorder) SetWriteDeadline(at time.Time) error { w.write = at; return nil }

func TestSkillRoutesExtendOnlyBoundedFileDeadlines(t *testing.T) {
	f := marketplaceApp(t, nil)
	for _, step := range []struct {
		method, path, body string
		read               bool
	}{
		{"POST", "/api/v1/admin/marketplace/skills", `{"slug":"deadlines","name":"Deadline","source":"inline","files":{"SKILL.md":"# X"}}`, true},
		{"GET", "/api/v1/marketplace/deadlines/files?format=2", "", false},
	} {
		r := httptest.NewRequest(step.method, "http://example.com"+step.path, strings.NewReader(step.body))
		r.Header.Set("Origin", "http://example.com")
		r.AddCookie(f.admin)
		w := &skillDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
		before := time.Now()
		f.handler.ServeHTTP(w, r)
		if w.Code >= 400 {
			t.Fatalf("file route failed %d %s", w.Code, w.Body)
		}
		if w.write.Before(before.Add(119*time.Second)) || w.write.After(time.Now().Add(121*time.Second)) {
			t.Fatalf("write deadline not bounded to 120 seconds: %v", w.write)
		}
		if step.read && (w.read.Before(before.Add(59*time.Second)) || w.read.After(time.Now().Add(61*time.Second))) {
			t.Fatalf("read deadline not bounded to 60 seconds: %v", w.read)
		}
		if !step.read && !w.read.IsZero() {
			t.Fatal("download should not change read deadline")
		}
	}
}

func TestSkillPublishingWithSingleDatabaseConnection(t *testing.T) {
	f := marketplaceApp(t, nil)
	cfg := f.store.Pool.Config()
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	handler := New(&store.Store{Pool: pool}, Options{Origin: "http://example.com"})
	bounded := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	for _, status := range []int{201, 200} {
		w := request(bounded, "POST", "/api/v1/admin/marketplace/skills", `{"slug":"single-connection","name":"One","source":"inline","files":{"SKILL.md":"# One"}}`, f.admin)
		if w.Code != status {
			t.Fatalf("publication needs a second connection: %d %s", w.Code, w.Body)
		}
	}
}

func TestSkillBinaryRejectsInvalidUploadsAndUnavailableStorage(t *testing.T) {
	f := marketplaceApp(t, nil)
	for _, file := range []string{
		`"SKILL.md":{"encoding":"base64","content":"/w=="}`,
		`"asset":{"encoding":"base64","content":"!!!"}`,
		`"../asset":{"encoding":"base64","content":"eA=="}`,
		`"skill.md":{"encoding":"base64","content":"eA=="}`,
	} {
		body := `{"slug":"invalid-upload","name":"Invalid","source":"inline","files":{"SKILL.md":"# X"},"files_v2":{` + file + `}}`
		if w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin); w.Code != 400 {
			t.Fatalf("invalid accepted %d %s", w.Code, w.Body)
		}
	}
	content := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 257<<10)))
	body := fmt.Sprintf(`{"slug":"invalid-upload","name":"Unavailable","source":"inline","files":{"SKILL.md":"# X"},"files_v2":{"asset":{"encoding":"base64","content":%q}}}`, content)
	if w := request(f.handler, "POST", "/api/v1/admin/marketplace/skills", body, f.admin); w.Code != 503 {
		t.Fatalf("unavailable storage %d %s", w.Code, w.Body)
	}
	var count int
	if err := f.store.Pool.QueryRow(t.Context(), `SELECT count(*) FROM marketplace_items WHERE slug='invalid-upload'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed upload published: %d %v", count, err)
	}
}
