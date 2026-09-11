package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteUpgradeOriginalFileLimitPreservesGitReferences(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "upgrade.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	entries, err := migrationsSQLite.ReadDir("migrations_sqlite")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	for _, entry := range entries {
		raw, err := migrationsSQLite.ReadFile("migrations_sqlite/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name() == "028_file_objects.sql" {
			raw = []byte(strings.ReplaceAll(string(raw), "16777216", "8388608"))
		}
		if _, err = db.Exec(string(raw)); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		if entry.Name() == "029_git_file_objects.sql" {
			if _, err = db.Exec("INSERT INTO file_objects(sha256,size,content) VALUES(?,1,x'ff')", hash); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("INSERT INTO marketplace_git_files(path,content,file_sha256,file_size) VALUES('objects/existing',x'',?,1)", hash); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = db.Exec("INSERT INTO file_objects(sha256,size,content) VALUES(?,8388609,zeroblob(8388609))", strings.Repeat("b", 64)); err != nil {
		t.Fatalf("old 8MiB limit not upgraded: %v", err)
	}
	var got string
	if err = db.QueryRow("SELECT file_sha256 FROM marketplace_git_files WHERE path='objects/existing'").Scan(&got); err != nil || got != hash {
		t.Fatalf("existing Git reference lost: %v", err)
	}
	if _, err = db.Exec("DELETE FROM file_objects WHERE sha256=?", hash); err == nil {
		t.Fatal("upgraded foreign key missing")
	}
}

// sqliteMigrate 在临时 SQLite 库上按文件名序执行 SQLite 迁移轨道。
// 这是 ADR 0002 阶段 2 的地基验证：轨道必须可从零建出与 PG 等价的库。
func sqliteMigrate(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "loadout.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	entries, err := migrationsSQLite.ReadDir("migrations_sqlite")
	if err != nil {
		t.Fatal(err)
	}
	pgEntries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(pgEntries) {
		t.Fatalf("sqlite migration track must mirror pg track, got %d files", len(entries))
	}
	for _, entry := range entries {
		raw, err := migrationsSQLite.ReadFile("migrations_sqlite/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(string(raw)); err != nil {
			t.Fatalf("migration %s: %v", entry.Name(), err)
		}
	}
	return database
}

func TestSQLiteMarketChangeInvalidatesSharedGit(t *testing.T) {
	database := sqliteMigrate(t)
	for _, change := range []string{
		"INSERT INTO marketplace_items(slug,name,source,transport) VALUES ('revision-test','Test','curated','http')",
		"UPDATE marketplace_items SET description='changed' WHERE slug='revision-test'",
		"DELETE FROM marketplace_items WHERE slug='revision-test'",
	} {
		if _, err := database.Exec("UPDATE marketplace_git_state SET built_at=CURRENT_TIMESTAMP WHERE singleton"); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(change); err != nil {
			t.Fatal(err)
		}
		var invalid bool
		if err := database.QueryRow("SELECT built_at IS NULL FROM marketplace_git_state WHERE singleton").Scan(&invalid); err != nil || !invalid {
			t.Fatalf("missing atomic invalidation: %v", err)
		}
	}
	if _, err := database.Exec("INSERT INTO marketplace_git_files(path,content) VALUES ('objects/ab/cd',?)", []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	var content []byte
	if err := database.QueryRow("SELECT content FROM marketplace_git_files WHERE path='objects/ab/cd'").Scan(&content); err != nil || len(content) != 3 {
		t.Fatalf("git object persistence: %v", err)
	}
}

func TestSQLiteFileObjectsPreserveBytesAndEnforceMetadata(t *testing.T) {
	database := sqliteMigrate(t)
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if _, err := database.Exec("INSERT INTO file_objects(sha256,size,content) VALUES(?,?,?)", hash, 3, []byte{0, 255, 128}); err != nil {
		t.Fatal(err)
	}
	var got []byte
	if err := database.QueryRow("SELECT content FROM file_objects WHERE sha256=?", hash).Scan(&got); err != nil || len(got) != 3 || got[1] != 255 {
		t.Fatalf("binary storage %x %v", got, err)
	}
	for _, query := range []string{
		"UPDATE file_objects SET size=4",
		"UPDATE file_objects SET content=NULL",
		"UPDATE file_objects SET sha256='invalid'",
		"UPDATE file_objects SET bucket='private'",
	} {
		if _, err := database.Exec(query); err == nil {
			t.Fatalf("invalid metadata accepted: %s", query)
		}
	}
	if _, err := database.Exec("UPDATE file_objects SET content=NULL,bucket='private',object_key='files/key',region='auto',endpoint='https://storage.example.com'"); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteGitFileObjectReferences(t *testing.T) {
	db := sqliteMigrate(t)
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if _, err := db.Exec("INSERT INTO file_objects(sha256,size,content) VALUES(?,?,?)", hash, 3, []byte{0, 255, 128}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO marketplace_git_files(path,content,file_sha256,file_size) VALUES('objects/test',?,?,3)", []byte{}, hash); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"UPDATE marketplace_git_files SET content=x'01'",
		"UPDATE marketplace_git_files SET file_size=NULL",
		"UPDATE marketplace_git_files SET file_size=16777217",
		"UPDATE marketplace_git_files SET file_sha256=NULL",
		"UPDATE marketplace_git_files SET file_sha256='missing'",
	} {
		if _, err := db.Exec(query); err == nil {
			t.Fatalf("invalid Git file reference accepted: %s", query)
		}
	}
	if _, err := db.Exec("INSERT INTO marketplace_git_files(path,content) VALUES('HEAD',x'00')"); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteMigrationTrackBuildsEquivalentSchema(t *testing.T) {
	database := sqliteMigrate(t)
	for _, table := range []string{"users", "wallets", "sessions", "tokens", "tools", "usage_logs", "ledger",
		"rate_limits", "plans", "redemption_codes", "orders", "teams", "team_members", "team_invites",
		"team_transfers", "device_authorizations", "email_verifications", "oauth_states", "oauth_identities",
		"tool_catalog_revision", "runtime_settings", "marketplace_items", "tool_metadata_overrides"} {
		var name string
		if err := database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil || name != table {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	// 种子：内置工具 5（echo/time_now + 账号三件套）、市场 5（3 http + 2 skill）。
	var tools, market int
	if err := database.QueryRow("SELECT count(*) FROM tools").Scan(&tools); err != nil || tools != 5 {
		t.Fatalf("seeded tools=%d err=%v", tools, err)
	}
	if err := database.QueryRow("SELECT count(*) FROM marketplace_items").Scan(&market); err != nil || market != 5 {
		t.Fatalf("seeded marketplace=%d err=%v", market, err)
	}
	var settlement string
	if err := database.QueryRow("SELECT settlement FROM tools WHERE key='echo'").Scan(&settlement); err != nil || settlement != "{}" {
		t.Fatalf("settlement column missing or wrong: %q err=%v", settlement, err)
	}
	// 技能种子的 spec 是合法 JSON 且含文件集。
	var spec string
	if err := database.QueryRow("SELECT spec FROM marketplace_items WHERE slug='commit-style'").Scan(&spec); err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Source string            `json:"source"`
		Files  map[string]string `json:"files"`
	}
	if err := json.Unmarshal([]byte(spec), &parsed); err != nil || parsed.Source != "inline" || parsed.Files["SKILL.md"] == "" {
		t.Fatalf("skill seed spec invalid: %s err=%v", spec, err)
	}
}

func TestSQLiteLedgerAppendOnlyAndCatalogRevision(t *testing.T) {
	database := sqliteMigrate(t)
	var user, wallet int64
	if err := database.QueryRow("INSERT INTO users(username,password_hash) VALUES ('alice','h') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("INSERT INTO wallets(user_id,balance) VALUES (?,9) RETURNING id", user).Scan(&wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO ledger(wallet_id,user_id,delta,kind) VALUES (?,?,1,'adjustment')", wallet, user); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("UPDATE ledger SET delta=99"); err == nil {
		t.Fatal("ledger update must be rejected")
	}
	if _, err := database.Exec("DELETE FROM ledger"); err == nil {
		t.Fatal("ledger delete must be rejected")
	}
	var before, after int64
	if err := database.QueryRow("SELECT revision FROM tool_catalog_revision WHERE singleton").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO tools(key,name,kind) VALUES ('probe','Probe','builtin')"); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("SELECT revision FROM tool_catalog_revision WHERE singleton").Scan(&after); err != nil || after != before+1 {
		t.Fatalf("catalog revision trigger: before=%d after=%d err=%v", before, after, err)
	}
	if _, err := database.Exec("DELETE FROM tools WHERE key='probe'"); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("SELECT revision FROM tool_catalog_revision WHERE singleton").Scan(&after); err != nil || after != before+2 {
		t.Fatalf("catalog revision delete trigger: after=%d err=%v", after, err)
	}
}

func TestSQLiteWalletOwnerTrigger(t *testing.T) {
	database := sqliteMigrate(t)
	var user int64
	if err := database.QueryRow("INSERT INTO users(username,password_hash) VALUES ('owner','h') RETURNING id").Scan(&user); err != nil {
		t.Fatal(err)
	}
	var team int64
	if err := database.QueryRow("INSERT INTO teams(name) VALUES ('t') RETURNING id").Scan(&team); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO wallets(user_id,team_id,balance) VALUES (NULL,NULL,0)"); err == nil {
		t.Fatal("wallet without owner must be rejected")
	}
	if _, err := database.Exec("INSERT INTO wallets(user_id,team_id,balance) VALUES (?,?,0)", user, team); err == nil {
		t.Fatal("wallet with two owners must be rejected")
	}
	if _, err := database.Exec(fmt.Sprintf("INSERT INTO wallets(user_id,balance) VALUES (%d,1)", user)); err != nil {
		t.Fatalf("single-owner wallet must pass: %v", err)
	}
}
