package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Yanyutin753/PluginPocket/server/internal/gateway"
	"github.com/Yanyutin753/PluginPocket/server/internal/marketplace"
	"github.com/jackc/pgx/v5"
)

type marketplaceItem struct {
	ID          int64           `json:"id"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Source      string          `json:"source"`
	RepoURL     string          `json:"repo_url"`
	Homepage    string          `json:"homepage"`
	Transport   string          `json:"transport"`
	Endpoint    string          `json:"endpoint"`
	Package     string          `json:"package"`
	Stars       int             `json:"stars"`
	SyncedAt    *time.Time      `json:"synced_at"`
	Kind        string          `json:"kind"`
	Version     string          `json:"version"`
	Spec        json.RawMessage `json:"spec,omitempty"`
	Installed   bool            `json:"installed"`
	InstalledID *int64          `json:"-"`
}

func (a *application) listMarketplace(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT id,slug,name,description,source,repo_url,homepage,transport,endpoint,package,stars,synced_at,installed_tool_id,kind,version,spec FROM marketplace_items ORDER BY (source='curated') DESC, stars DESC, id")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	items := []marketplaceItem{}
	for rows.Next() {
		var item marketplaceItem
		if e = rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Description, &item.Source, &item.RepoURL, &item.Homepage, &item.Transport, &item.Endpoint, &item.Package, &item.Stars, &item.SyncedAt, &item.InstalledID, &item.Kind, &item.Version, &item.Spec); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		item.Installed = item.InstalledID != nil
		items = append(items, item)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"items": items})
}

func (a *application) loadMarketplaceItem(w http.ResponseWriter, r *http.Request, slug string) (marketplaceItem, bool) {
	item, e := readMarketplaceItem(r.Context(), a.s.Pool, slug)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return item, false
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return item, false
	}
	return item, true
}

func readMarketplaceItem(ctx context.Context, database interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, slug string) (marketplaceItem, error) {
	var item marketplaceItem
	e := database.QueryRow(ctx, "SELECT id,slug,name,description,source,repo_url,homepage,transport,endpoint,package,stars,synced_at,installed_tool_id,kind,version,spec FROM marketplace_items WHERE slug=$1", slug).
		Scan(&item.ID, &item.Slug, &item.Name, &item.Description, &item.Source, &item.RepoURL, &item.Homepage, &item.Transport, &item.Endpoint, &item.Package, &item.Stars, &item.SyncedAt, &item.InstalledID, &item.Kind, &item.Version, &item.Spec)
	item.Installed = item.InstalledID != nil
	return item, e
}

// publicMarketplace 面向已登录用户与 CLI 令牌：市场是 HTTP MCP 目录，
// 不返回密封配置等内部字段；stdio 不进入市场。
func (a *application) publicMarketplace(w http.ResponseWriter, r *http.Request) {
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		if _, e := a.s.AuthToken(r.Context(), strings.TrimPrefix(header, "Bearer ")); e != nil {
			fail(w, 401, "unauthorized")
			return
		}
	} else if _, ok := a.currentUser(w, r, false); !ok {
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT slug,name,description,source,repo_url,homepage,transport,endpoint,stars,installed_tool_id,kind,spec FROM marketplace_items WHERE transport!='stdio' ORDER BY (source='curated') DESC, stars DESC, id")
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	type publicItem struct {
		Slug        string          `json:"slug"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Source      string          `json:"source"`
		RepoURL     string          `json:"repo_url"`
		Homepage    string          `json:"homepage"`
		Transport   string          `json:"transport"`
		Endpoint    string          `json:"endpoint"`
		Stars       int             `json:"stars"`
		Kind        string          `json:"kind"`
		Spec        json.RawMessage `json:"spec,omitempty"`
		Installed   bool            `json:"installed"`
	}
	items := []publicItem{}
	for rows.Next() {
		var item publicItem
		var installedID *int64
		if e = rows.Scan(&item.Slug, &item.Name, &item.Description, &item.Source, &item.RepoURL, &item.Homepage, &item.Transport, &item.Endpoint, &item.Stars, &installedID, &item.Kind, &item.Spec); e != nil {
			fail(w, 500, "internal_error")
			return
		}
		item.Installed = installedID != nil
		items = append(items, item)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	respond(w, 200, map[string]any{"items": items})
}

func (a *application) createSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		fail(w, 500, "internal_error")
		return
	}
	if err := controller.SetWriteDeadline(time.Now().Add(120 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		fail(w, 500, "internal_error")
		return
	}
	var in struct {
		Slug        string                             `json:"slug"`
		Name        string                             `json:"name"`
		Description string                             `json:"description"`
		Source      string                             `json:"source"`
		Version     string                             `json:"version"`
		Repo        string                             `json:"repo"`
		Path        string                             `json:"path"`
		Files       map[string]string                  `json:"files"`
		FilesV2     map[string]marketplace.EncodedFile `json:"files_v2"`
		DeleteFiles []string                           `json:"delete_files"`
	}
	if !decodeLimit(w, r, &in, 50<<20) {
		return
	}
	slug := in.Slug
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(in.Name, " ", "-"))
	}
	if !usernamePattern.MatchString(slug) || len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 || len(in.Description) > 2000 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	fileOptions := a.options.Marketplace
	fileOptions.Files = a.options.Files.WithTx(tx)
	// Serialize authoring before row locks: skill updates also write bundles
	// and the shared Git invalidation row. One order prevents cross-edit deadlocks.
	if _, e = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended(current_schema() || ':marketplace-publish', 0))"); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	spec := map[string]any{"source": in.Source}
	files := map[string]marketplace.SkillFile{}
	switch in.Source {
	case "inline":
		previous, err := readMarketplaceItem(r.Context(), tx, slug)
		if err == nil && previous.Kind == "skill" {
			files, err = marketplace.LoadSkillFiles(r.Context(), fileOptions, previous.Spec)
			if err != nil {
				fail(w, 503, "upstream_unavailable")
				return
			}
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			fail(w, 500, "internal_error")
			return
		}
		for name, content := range in.Files {
			files[name] = marketplace.SkillFile{Content: []byte(content)}
		}
		for name, encoded := range in.FilesV2 {
			if encoded.Encoding != "base64" {
				fail(w, 400, "invalid_request")
				return
			}
			content, err := base64.StdEncoding.Strict().DecodeString(encoded.Content)
			if err != nil {
				fail(w, 400, "invalid_request")
				return
			}
			files[name] = marketplace.SkillFile{Content: content, Executable: encoded.Executable}
		}
		for _, name := range in.DeleteFiles {
			if name == "SKILL.md" || !marketplace.SafeSkillPath(name) {
				fail(w, 400, "invalid_request")
				return
			}
			delete(files, name)
		}
	case "github":
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		var e error
		files, e = marketplace.ResolveSkillFilesV2(ctx, a.options.Marketplace, in.Repo, in.Path)
		if e != nil {
			fail(w, 502, "upstream_unavailable")
			return
		}
		spec["repo"] = in.Repo
		spec["path"] = in.Path
	default:
		fail(w, 400, "invalid_request")
		return
	}
	if marketplace.ValidateSkillFiles(files) != nil {
		fail(w, 400, "invalid_request")
		return
	}
	manifest, err := marketplace.StoreSkillFiles(r.Context(), fileOptions.Files, files)
	if err != nil {
		fail(w, 503, "upstream_unavailable")
		return
	}
	spec["file_manifest"] = manifest
	// Keep the editable document available to older admin consoles.
	spec["files"] = map[string]string{"SKILL.md": string(files["SKILL.md"].Content)}
	encoded, e := json.Marshal(spec)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	// 正式发版：内容指纹决定版本策略（不变沿用 / 显式发版 / 自动 patch+1）。
	nextHash := contentFingerprint("skill", in.Name, in.Description, string(encoded))
	var previousVersion, previousHash string
	var existed bool
	if e = tx.QueryRow(r.Context(), "SELECT version,content_hash FROM marketplace_items WHERE slug=$1 AND kind='skill' FOR UPDATE", slug).Scan(&previousVersion, &previousHash); e == nil {
		existed = true
	} else if !errors.Is(e, pgx.ErrNoRows) {
		fail(w, 500, "internal_error")
		return
	}
	version, err := formalVersion(previousVersion, previousHash, in.Version, nextHash)
	if err != nil {
		fail(w, 400, "invalid_request")
		return
	}
	var id int64
	e = tx.QueryRow(r.Context(), `INSERT INTO marketplace_items(slug,name,description,source,transport,kind,spec,version,content_hash)
	 VALUES($1,$2,$3,'curated','unknown','skill',$4,$5,$6)
	 ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,spec=EXCLUDED.spec,version=EXCLUDED.version,content_hash=EXCLUDED.content_hash,updated_at=now()
	 WHERE marketplace_items.kind='skill' RETURNING id`, slug, in.Name, in.Description, encoded, version, nextHash).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 409, "key_taken")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if existed && nextHash != previousHash {
		// A bundle embeds its skill files, so member updates are releases too.
		if _, e = tx.Exec(r.Context(), `UPDATE marketplace_items SET version=split_part(version,'.',1)||'.'||split_part(version,'.',2)||'.'||(split_part(version,'.',3)::bigint+1)::text,updated_at=now()
		 WHERE kind='bundle' AND spec->'includes' ? $1`, slug); e != nil {
			fail(w, 500, "internal_error")
			return
		}
	}
	item, e := readMarketplaceItem(r.Context(), tx, slug)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	status := 201
	if existed {
		status = 200
	}
	respond(w, status, map[string]any{"item": item})
}

func (a *application) createBundle(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var in struct {
		Slug        string   `json:"slug"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Includes    []string `json:"includes"`
	}
	if !decode(w, r, &in) {
		return
	}
	slug := in.Slug
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(in.Name, " ", "-"))
	}
	if !usernamePattern.MatchString(slug) || len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 || len(in.Description) > 2000 || len(in.Includes) < 1 || len(in.Includes) > 32 {
		fail(w, 400, "invalid_request")
		return
	}
	seen := map[string]bool{}
	for _, include := range in.Includes {
		if seen[include] || !usernamePattern.MatchString(include) {
			fail(w, 400, "invalid_request")
			return
		}
		seen[include] = true
		var installable bool
		e := a.s.Pool.QueryRow(r.Context(), "SELECT kind='skill' OR (kind='mcp' AND (transport='gateway' OR (transport='http' AND endpoint<>''))) FROM marketplace_items WHERE slug=$1", include).Scan(&installable)
		if e != nil || !installable {
			fail(w, 400, "invalid_request")
			return
		}
	}
	spec, e := json.Marshal(map[string]any{"includes": in.Includes})
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, e = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended(current_schema() || ':marketplace-publish', 0))"); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var previousVersion, previousHash, previousKind string
	e = tx.QueryRow(r.Context(), "SELECT version,content_hash,kind FROM marketplace_items WHERE slug=$1 FOR UPDATE", slug).Scan(&previousVersion, &previousHash, &previousKind)
	existed := e == nil
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		fail(w, 500, "internal_error")
		return
	}
	if existed && previousKind != "bundle" {
		fail(w, 409, "key_taken")
		return
	}
	nextHash := contentFingerprint("bundle", in.Name, in.Description, string(spec))
	version, e := formalVersion(previousVersion, previousHash, "", nextHash)
	if e != nil {
		fail(w, 400, "invalid_request")
		return
	}
	if _, e = tx.Exec(r.Context(), `INSERT INTO marketplace_items(slug,name,description,source,transport,kind,spec,version,content_hash) VALUES($1,$2,$3,'curated','unknown','bundle',$4,$5,$6)
	 ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,spec=EXCLUDED.spec,version=EXCLUDED.version,content_hash=EXCLUDED.content_hash,updated_at=now() WHERE marketplace_items.kind='bundle'`, slug, in.Name, in.Description, spec, version, nextHash); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	item, e := readMarketplaceItem(r.Context(), tx, slug)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	status := 201
	if existed {
		status = 200
	}
	respond(w, status, map[string]any{"item": item})
}

// marketplaceFiles 返回技能文件集（内联直接返回，GitHub 由服务端解析），CLI 唯一文件来源。
func (a *application) marketplaceFiles(w http.ResponseWriter, r *http.Request) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(120 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		fail(w, 500, "internal_error")
		return
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		if _, e := a.s.AuthToken(r.Context(), strings.TrimPrefix(header, "Bearer ")); e != nil {
			fail(w, 401, "unauthorized")
			return
		}
	} else if _, ok := a.currentUser(w, r, false); !ok {
		return
	}
	item, ok := a.loadMarketplaceItem(w, r, r.PathValue("slug"))
	if !ok {
		return
	}
	if item.Kind != "skill" || len(item.Spec) == 0 {
		fail(w, 400, "invalid_request")
		return
	}
	files, err := marketplace.LoadSkillFiles(r.Context(), a.options.Marketplace, item.Spec)
	if err != nil {
		fail(w, 502, "upstream_unavailable")
		return
	}
	if r.URL.Query().Get("format") == "2" {
		respond(w, 200, map[string]any{"files": marketplace.EncodeSkillFiles(files)})
		return
	}
	legacy, err := marketplace.LegacySkillFiles(files)
	if err != nil {
		fail(w, 400, "invalid_request")
		return
	}
	respond(w, 200, map[string]any{"files": legacy})
}

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// contentFingerprint 对影响导出内容的字段取规范指纹。
func contentFingerprint(parts ...string) string {
	sum := sha256.New()
	for _, part := range parts {
		sum.Write([]byte(part))
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))[:16]
}

// formalVersion 计算条目本次提交的正式版本：内容不变沿用旧版本；
// 内容变化时优先显式版本，否则自动 patch +1（运营的"静默修复"路径）。
func formalVersion(previousVersion, previousHash, explicit, nextHash string) (string, error) {
	if explicit != "" && !versionPattern.MatchString(explicit) {
		return "", errors.New("invalid version")
	}
	if nextHash == previousHash {
		if previousVersion == "" {
			return "1.0.0", nil
		}
		return previousVersion, nil
	}
	if explicit != "" {
		return explicit, nil
	}
	if previousVersion == "" {
		return "1.0.0", nil
	}
	parts := strings.Split(previousVersion, ".")
	if len(parts) != 3 {
		return "1.0.1", nil
	}
	patch, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "1.0.1", nil
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1), nil
}

// publishPoolTool 把池内（密封凭证）工具发布为市场组件：无公共端点，
// transport=gateway，经 bridge 计量使用——"服务端独享"插件的 MCP 半边。

func (a *application) publishPoolTool(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var in struct {
		ToolID      int64  `json:"tool_id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decode(w, r, &in) {
		return
	}
	var key, name string
	e := a.s.Pool.QueryRow(r.Context(), "SELECT key,name FROM tools WHERE id=$1", in.ToolID).Scan(&key, &name)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	slug := in.Slug
	if slug == "" {
		slug = key
	}
	if in.Name != "" {
		name = in.Name
	}
	if !usernamePattern.MatchString(slug) || len(name) < 1 || len(name) > 80 || len(in.Description) > 2000 {
		fail(w, 400, "invalid_request")
		return
	}
	tag, e := a.s.Pool.Exec(r.Context(), "INSERT INTO marketplace_items(slug,name,description,source,transport,kind,installed_tool_id) VALUES($1,$2,$3,'curated','gateway','mcp',$4)", slug, name, in.Description, in.ToolID)
	if e != nil {
		if strings.Contains(e.Error(), "duplicate key") {
			fail(w, 409, "key_taken")
			return
		}
		fail(w, 500, "internal_error")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 500, "internal_error")
		return
	}
	item, ok := a.loadMarketplaceItem(w, r, slug)
	if !ok {
		return
	}
	respond(w, 201, map[string]any{"item": item})
}

func (a *application) syncMarketplace(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	count, e := marketplace.Sync(ctx, a.s.Pool, a.options.Marketplace)
	if e != nil {
		fail(w, 502, "upstream_unavailable")
		return
	}
	respond(w, 200, map[string]any{"synced": count})
}

func (a *application) installMarketplace(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var in struct {
		Slug      string          `json:"slug"`
		Key       string          `json:"key"`
		Name      string          `json:"name"`
		Units     *int64          `json:"units_per_call"`
		Transport string          `json:"transport"`
		Config    json.RawMessage `json:"config"`
	}
	if !decode(w, r, &in) {
		return
	}
	item, ok := a.loadMarketplaceItem(w, r, in.Slug)
	if !ok {
		return
	}
	if item.Kind != "mcp" {
		fail(w, 400, "invalid_request")
		return
	}
	if item.Installed {
		fail(w, 409, "already_installed")
		return
	}
	transport := in.Transport
	if transport == "" {
		transport = item.Transport
	}
	if transport == "unknown" {
		fail(w, 400, "transport_required")
		return
	}
	if transport != "http" {
		fail(w, 400, "transport_not_supported")
		return
	}
	config := in.Config
	if len(config) == 0 || string(config) == "null" {
		if transport == "http" && item.Endpoint != "" {
			encoded, _ := json.Marshal(map[string]string{"url": item.Endpoint})
			config = encoded
		} else {
			fail(w, 400, "config_required")
			return
		}
	}
	if a.options.Gateway == nil || a.options.Gateway.ValidateConfig(transport, config) != nil {
		fail(w, 400, "invalid_request")
		return
	}
	sealed, e := gateway.SealConfig(a.options.EncryptionKey, config)
	if e != nil {
		fail(w, 503, "upstream_unavailable")
		return
	}
	key := in.Key
	if key == "" {
		key = item.Slug
	}
	name := in.Name
	if name == "" {
		name = item.Name
	}
	units := int64(1)
	if in.Units != nil {
		units = *in.Units
	}
	if !usernamePattern.MatchString(key) || len(name) < 1 || len(name) > 80 || len(item.Description) > 2000 || units < 0 || units > 1000000000000 {
		fail(w, 400, "invalid_request")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id int64
	if e = tx.QueryRow(r.Context(), "INSERT INTO tools(key,name,description,kind,enabled,cost,input_schema,config) VALUES($1,$2,$3,$4,true,$5,'{\"type\":\"object\"}'::jsonb,$6) RETURNING id", key, name, item.Description, transport, units, sealed).Scan(&id); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	var linked int64
	if e = tx.QueryRow(r.Context(), "UPDATE marketplace_items SET installed_tool_id=$1, updated_at=now() WHERE id=$2 AND installed_tool_id IS NULL RETURNING id", id, item.ID).Scan(&linked); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok = currentUserQuery(w, r, true, tx); !ok {
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if a.options.Gateway != nil {
		a.options.Gateway.Invalidate()
	}
	installed, ok := a.loadMarketplaceItem(w, r, in.Slug)
	if !ok {
		return
	}
	respond(w, 201, map[string]any{"item": installed, "tool_id": id})
}

func (a *application) uninstallMarketplace(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	var in struct {
		Slug string `json:"slug"`
	}
	if !decode(w, r, &in) {
		return
	}
	item, ok := a.loadMarketplaceItem(w, r, in.Slug)
	if !ok {
		return
	}
	if item.InstalledID == nil {
		fail(w, 409, "not_installed")
		return
	}
	tx, e := a.s.Pool.Begin(r.Context())
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, e = tx.Exec(r.Context(), "UPDATE tools SET enabled=false, updated_at=now() WHERE id=$1", *item.InstalledID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, e = tx.Exec(r.Context(), "UPDATE marketplace_items SET installed_tool_id=NULL, updated_at=now() WHERE id=$1", item.ID); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if _, ok = currentUserQuery(w, r, true, tx); !ok {
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if a.options.Gateway != nil {
		a.options.Gateway.Invalidate()
	}
	updated, ok := a.loadMarketplaceItem(w, r, in.Slug)
	if !ok {
		return
	}
	respond(w, 200, map[string]any{"item": updated})
}

func (a *application) upstreamTools(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var kind string
	var enabled bool
	e := a.s.Pool.QueryRow(r.Context(), "SELECT kind,enabled FROM tools WHERE id=$1", id).Scan(&kind, &enabled)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if kind == "builtin" {
		fail(w, 400, "invalid_request")
		return
	}
	if !enabled {
		fail(w, 409, "tool_disabled")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	definitions, e := a.options.Gateway.UpstreamTools(ctx, id)
	if e != nil {
		fail(w, 502, "upstream_unavailable")
		return
	}
	rows, e := a.s.Pool.Query(r.Context(), "SELECT remote_name,description,input_schema FROM tool_metadata_overrides WHERE tool_id=$1 ORDER BY remote_name", id)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	defer rows.Close()
	type override struct {
		RemoteName  string          `json:"remote_name"`
		Description string          `json:"description"`
		Schema      json.RawMessage `json:"input_schema,omitempty"`
	}
	overrides := []override{}
	for rows.Next() {
		var item override
		if rows.Scan(&item.RemoteName, &item.Description, &item.Schema) != nil {
			fail(w, 500, "internal_error")
			return
		}
		overrides = append(overrides, item)
	}
	if rows.Err() != nil {
		fail(w, 500, "internal_error")
		return
	}
	type remote struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema any    `json:"input_schema"`
	}
	tools := make([]remote, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, remote{Name: definition.Name, Description: definition.Description, InputSchema: definition.InputSchema})
	}
	respond(w, 200, map[string]any{"tools": tools, "overrides": overrides})
}

func (a *application) saveMetadata(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var kind string
	e := a.s.Pool.QueryRow(r.Context(), "SELECT kind FROM tools WHERE id=$1", id).Scan(&kind)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if kind == "builtin" {
		fail(w, 400, "invalid_request")
		return
	}
	var in struct {
		RemoteName  string          `json:"remote_name"`
		Description string          `json:"description"`
		Schema      json.RawMessage `json:"input_schema"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Description = strings.TrimSpace(in.Description)
	hasSchema := len(in.Schema) > 0 && string(in.Schema) != "null"
	if len(in.RemoteName) < 1 || len(in.RemoteName) > 128 || len(in.Description) > 2000 {
		fail(w, 400, "invalid_request")
		return
	}
	if hasSchema && gateway.ValidateToolSchema(in.Schema) != nil {
		fail(w, 400, "invalid_request")
		return
	}
	if in.Description == "" && !hasSchema {
		fail(w, 400, "invalid_request")
		return
	}
	schema := in.Schema
	if !hasSchema {
		schema = nil
	}
	var overrideID int64
	e = a.s.Pool.QueryRow(r.Context(), `INSERT INTO tool_metadata_overrides(tool_id,remote_name,description,input_schema) VALUES($1,$2,$3,$4)
	 ON CONFLICT(tool_id,remote_name) DO UPDATE SET description=EXCLUDED.description,input_schema=EXCLUDED.input_schema,updated_at=now() RETURNING id`,
		id, in.RemoteName, in.Description, schema).Scan(&overrideID)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if a.options.Gateway != nil {
		a.options.Gateway.Invalidate()
	}
	respond(w, 200, map[string]any{"item": map[string]any{"tool_id": id, "remote_name": in.RemoteName, "description": in.Description, "input_schema": schema}})
}

func (a *application) deleteMetadata(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if len(name) < 1 || len(name) > 128 {
		fail(w, 400, "invalid_request")
		return
	}
	tag, e := a.s.Pool.Exec(r.Context(), "DELETE FROM tool_metadata_overrides WHERE tool_id=$1 AND remote_name=$2", id, name)
	if e != nil {
		fail(w, 500, "internal_error")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 404, "not_found")
		return
	}
	if a.options.Gateway != nil {
		a.options.Gateway.Invalidate()
	}
	w.WriteHeader(http.StatusNoContent)
}
