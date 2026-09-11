package marketplace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yanyutin753/PluginPocket/server/internal/filestore"
)

func TestGitHubSkillBinaryAndExecutable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/skills/git/trees/HEAD":
			_ = json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": "skill/SKILL.md", "mode": "100644", "type": "blob", "sha": "1111111111111111111111111111111111111111", "size": 3}, map[string]any{"path": "skill/bin/run", "mode": "100755", "type": "blob", "sha": "2222222222222222222222222222222222222222", "size": 3}}})
		case "/repos/acme/skills/git/blobs/1111111111111111111111111111111111111111":
			_ = json.NewEncoder(w).Encode(map[string]any{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("# X"))})
		case "/repos/acme/skills/git/blobs/2222222222222222222222222222222222222222":
			_ = json.NewEncoder(w).Encode(map[string]any{"encoding": "base64", "content": "AP+A"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	files, err := ResolveSkillFilesV2(t.Context(), Options{BaseURL: server.URL}, "acme/skills", "skill")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["bin/run"].Content, []byte{0, 255, 128}) || !files["bin/run"].Executable {
		t.Fatalf("binary/mode lost: %+v", files)
	}
}

func TestGitRegistrySingleConnection(t *testing.T) {
	s := marketplaceDB(t)
	cfg := s.Pool.Config()
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	fs, err := filestore.New(pool, filestore.Options{InlineMaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 300<<10)
	_, _ = rand.New(rand.NewSource(1)).Read(body)
	manifest, err := StoreSkillFiles(t.Context(), fs, map[string]SkillFile{"SKILL.md": {Content: []byte("# X")}, "asset.bin": {Content: body}})
	if err != nil {
		t.Fatal(err)
	}
	spec, _ := json.Marshal(map[string]any{"source": "inline", "file_manifest": manifest})
	if _, err = pool.Exec(t.Context(), `INSERT INTO marketplace_items(slug,name,kind,source,transport,spec) VALUES('one-connection','One','skill','curated','unknown',$1)`, spec); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if _, err = NewGitRegistry(pool, Options{Files: fs}).Files(ctx); err != nil {
		t.Fatalf("registry needs another database connection: %v", err)
	}
}

func TestSkillFileLimitsAndGitLinks(t *testing.T) {
	if ValidateSkillFiles(map[string]SkillFile{"SKILL.md": {Content: make([]byte, MaxSkillFileBytes+1)}}) == nil {
		t.Fatal("accepted file over 8 MiB")
	}
	files := map[string]SkillFile{"SKILL.md": {Content: []byte("# X")}}
	for i := 0; i < 32; i++ {
		files[strings.Repeat("x", i+1)] = SkillFile{Content: []byte("x")}
	}
	if ValidateSkillFiles(files) == nil {
		t.Fatal("accepted 33 files")
	}
	files = map[string]SkillFile{"SKILL.md": {Content: []byte("# X")}, "a": {Content: make([]byte, 8<<20)}, "b": {Content: make([]byte, 8<<20)}, "c": {Content: make([]byte, 8<<20)}, "d": {Content: make([]byte, 8<<20)}}
	if ValidateSkillFiles(files) == nil {
		t.Fatal("accepted skill over 32 MiB")
	}
	for _, mode := range []string{"120000", "160000", "100600"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"tree": []any{map[string]any{"path": "skill/SKILL.md", "type": "blob", "mode": mode, "sha": "1111111111111111111111111111111111111111", "size": 1}}})
		}))
		if _, err := ResolveSkillFilesV2(t.Context(), Options{BaseURL: server.URL}, "acme/skills", "skill"); err == nil {
			t.Errorf("accepted Git mode %s", mode)
		}
		server.Close()
	}
}

func TestGitRegistryStoresLargeObjectsByReference(t *testing.T) {
	s := marketplaceDB(t)
	fs, err := filestore.New(s.Pool, filestore.Options{InlineMaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 300<<10)
	_, _ = rand.New(rand.NewSource(1)).Read(body)
	manifest, err := StoreSkillFiles(t.Context(), fs, map[string]SkillFile{"SKILL.md": {Content: []byte("# X")}, "asset.bin": {Content: body}})
	if err != nil {
		t.Fatal(err)
	}
	spec, _ := json.Marshal(map[string]any{"source": "inline", "file_manifest": manifest})
	if _, err = s.Pool.Exec(t.Context(), `INSERT INTO marketplace_items(slug,name,kind,source,transport,spec) VALUES('large-export','Large','skill','curated','unknown',$1)`, spec); err != nil {
		t.Fatal(err)
	}
	registry := NewGitRegistry(s.Pool, Options{Files: fs})
	repo, err := registry.Files(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var target string
	if err = s.Pool.QueryRow(t.Context(), `SELECT path FROM marketplace_git_files WHERE file_sha256 IS NOT NULL AND octet_length(content)=0 LIMIT 1`).Scan(&target); err != nil {
		t.Fatalf("large Git object was copied into registry bytea: %v", err)
	}
	got, err := NewGitRegistry(s.Pool, Options{Files: fs}).File(t.Context(), target)
	if err != nil || !bytes.Equal(got, repo[target]) {
		t.Fatalf("referenced object reload: %v", err)
	}
}

func TestSkillFilesRejectWindowsAliases(t *testing.T) {
	for _, name := range []string{"skill.md", "bin./run", "bin /run", "assets/CON.png", "aux", "COM1.txt", "LPT9", "q?.txt", "a|b"} {
		t.Run(name, func(t *testing.T) {
			files := map[string]SkillFile{"SKILL.md": {Content: []byte("# X")}, name: {Content: []byte("x")}}
			if ValidateSkillFiles(files) == nil {
				t.Fatalf("accepted cross-platform alias %q", name)
			}
		})
	}
	files := map[string]SkillFile{"SKILL.md": {Content: []byte("# X")}, "Bin": {Content: []byte("x")}, "bin/run": {Content: []byte("x")}}
	if ValidateSkillFiles(files) == nil {
		t.Fatal("accepted case-insensitive directory collision")
	}
}

func TestSkillManifestMissingObjectFailsExport(t *testing.T) {
	s := marketplaceDB(t)
	fs, err := filestore.New(s.Pool, filestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(t.Context(), `INSERT INTO marketplace_items(slug,name,kind,source,transport,spec) VALUES('missing-object','Missing','skill','curated','unknown','{"source":"inline","file_manifest":{"SKILL.md":{"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":3,"executable":false}}}')`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = LoadExportInputs(t.Context(), s.Pool, Options{Files: fs}); err == nil {
		t.Fatal("missing snapshot object was silently omitted")
	}
}

func TestSkillManifestExportAndGitClone(t *testing.T) {
	s := marketplaceDB(t)
	fs, err := filestore.New(s.Pool, filestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]SkillFile{"SKILL.md": {Content: []byte("# Binary")}, "bin/run": {Content: []byte{0, 255, 128}, Executable: true}}
	manifest, err := StoreSkillFiles(t.Context(), fs, files)
	if err != nil {
		t.Fatal(err)
	}
	spec, _ := json.Marshal(map[string]any{"source": "inline", "file_manifest": manifest})
	if _, err = s.Pool.Exec(t.Context(), `INSERT INTO marketplace_items(slug,name,kind,source,transport,spec) VALUES('binary-export','Binary','skill','curated','unknown',$1)`, spec); err != nil {
		t.Fatal(err)
	}
	options := Options{Files: fs}
	entries, unresolved, err := LoadExportInputs(t.Context(), s.Pool, options)
	if err != nil || len(unresolved) > 0 {
		t.Fatalf("load: %v %v", err, unresolved)
	}
	tree, _, err := ExportCodexMarketplace("pluginpocket", "PluginPocket", entries)
	if err != nil {
		t.Fatal(err)
	}
	target := "plugins/binary-export/skills/binary-export/bin/run"
	if !bytes.Equal(tree[target], files["bin/run"].Content) {
		t.Fatalf("export binary lost: %v", tree[target])
	}
	registry := NewGitRegistry(s.Pool, options)
	repo, err := registry.Files(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	server := serveRepo(t, func() map[string][]byte { return repo })
	dir := filepath.Join(t.TempDir(), "clone")
	gitClone(t, server.URL, dir)
	if got := readCloned(t, dir, target); got != string(files["bin/run"].Content) {
		t.Fatalf("clone changed bytes: %q", got)
	}
	info, err := os.Stat(filepath.Join(dir, target))
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("executable lost: %v %v", info, err)
	}
}
