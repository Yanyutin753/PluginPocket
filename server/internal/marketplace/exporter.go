// Package marketplace 的导出器：把 PluginPocket 市场条目渲染为 Codex 官方插件市场目录树。
// 产物可直接推到 git 仓库，由 `codex plugin marketplace add owner/repo` 接入。
package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"
)

// ExportInput 是导出输入：市场条目 + 预解析的技能文件（github 源由调用方解析，失败整条跳过）。
type ExportInput struct {
	ExecutableFiles       map[string]bool
	MemberExecutableFiles map[string]map[string]bool
	Slug                  string
	Name                  string
	Description           string
	Kind                  string // mcp / skill / bundle
	Transport             string
	Endpoint              string
	Version               string // 正式版本（semver）；空则回退内容指纹
	Spec                  json.RawMessage
	Files                 map[string]string            // skill 自身 / bundle 内成员的技能文件
	MemberFiles           map[string]map[string]string // bundle：成员 slug → 文件集
}

// ExportCodexMarketplace 渲染 `.agents/plugins/marketplace.json` 与 `plugins/<slug>/` 树。
// 无法渲染的条目（github 技能未解析、mcp 无端点、bundle 成员缺失）跳过并返回原因清单。
func ExportCodexMarketplace(name, displayName string, entries []ExportInput) (map[string][]byte, []string, error) {
	if name == "" || displayName == "" || !slugPattern.MatchString(name) {
		return nil, nil, fmt.Errorf("invalid marketplace identity")
	}
	bySlug := map[string]ExportInput{}
	for _, entry := range entries {
		bySlug[entry.Slug] = entry
	}
	type renderedPlugin struct {
		category string
		files    map[string][]byte
	}
	rendered := map[string]*renderedPlugin{}
	var order []string
	var skipped []string
	renderSkillFiles := func(pluginSlug, skillSlug string, files map[string]string, out map[string][]byte) bool {
		if len(files) == 0 {
			return false
		}
		for fileName, content := range files {
			out[fmt.Sprintf("plugins/%s/skills/%s/%s", pluginSlug, skillSlug, fileName)] = []byte(content)
		}
		return true
	}
	for _, entry := range entries {
		if !slugPattern.MatchString(entry.Slug) {
			skipped = append(skipped, entry.Slug+":invalid-slug")
			continue
		}
		files := map[string][]byte{}
		category := ""
		// Codex 只加载 plugin.json 的 mcpServers 声明；根目录 mcp.json 不是 Codex 约定。
		mcpServers := map[string]any{}
		switch entry.Kind {
		case "skill":
			if !renderSkillFiles(entry.Slug, entry.Slug, entry.Files, files) {
				skipped = append(skipped, entry.Slug+":skill-files-unresolved")
				continue
			}
			category = "Skills"
		case "mcp":
			server := mcpServerEntry(entry.Transport, entry.Endpoint)
			if server == nil {
				skipped = append(skipped, entry.Slug+":no-http-endpoint")
				continue
			}
			mcpServers[entry.Slug] = server
			category = "Tools"
		case "bundle":
			var spec struct {
				Includes []string `json:"includes"`
			}
			if err := json.Unmarshal(entry.Spec, &spec); err != nil || len(spec.Includes) == 0 {
				skipped = append(skipped, entry.Slug+":invalid-bundle")
				continue
			}
			ok := true
			for _, member := range spec.Includes {
				source, found := bySlug[member]
				if !found {
					skipped = append(skipped, fmt.Sprintf("%s:member-%s-missing", entry.Slug, member))
					ok = false
					break
				}
				switch source.Kind {
				case "skill":
					if !renderSkillFiles(entry.Slug, member, entry.MemberFiles[member], files) {
						skipped = append(skipped, fmt.Sprintf("%s:member-%s-files-unresolved", entry.Slug, member))
						ok = false
					}
				case "mcp":
					server := mcpServerEntry(source.Transport, source.Endpoint)
					if server != nil {
						mcpServers[member] = server
					} else {
						skipped = append(skipped, fmt.Sprintf("%s:member-%s-no-endpoint", entry.Slug, member))
						ok = false
					}
				default:
					skipped = append(skipped, fmt.Sprintf("%s:member-%s-unsupported", entry.Slug, member))
					ok = false
				}
			}
			if !ok {
				continue
			}
			category = "Bundles"
		default:
			skipped = append(skipped, entry.Slug+":unknown-kind")
			continue
		}
		// 正式版本优先（运营掌控，内容变更时由服务端 bump）；为空时回退内容指纹。
		version := entry.Version
		if version == "" {
			version = contentVersion(files)
		}
		metadata := map[string]any{
			"name":        entry.Slug,
			"version":     version,
			"description": entry.Description,
			"author":      "PluginPocket marketplace",
			"keywords":    []string{"pluginpocket", entry.Kind},
		}
		if len(mcpServers) > 0 {
			metadata["mcpServers"] = mcpServers
		}
		pluginJSON, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return nil, nil, err
		}
		files[fmt.Sprintf("plugins/%s/.codex-plugin/plugin.json", entry.Slug)] = append(pluginJSON, '\n')
		rendered[entry.Slug] = &renderedPlugin{category: category, files: files}
		order = append(order, entry.Slug)
	}
	sort.Strings(order)
	manifestPlugins := make([]map[string]any, 0, len(order))
	tree := map[string][]byte{}
	for _, slug := range order {
		plugin := rendered[slug]
		manifestPlugins = append(manifestPlugins, map[string]any{
			"name":     slug,
			"source":   map[string]string{"source": "local", "path": "./plugins/" + slug},
			"policy":   map[string]string{"installation": "AVAILABLE"},
			"category": plugin.category,
		})
		for path, content := range plugin.files {
			tree[path] = content
		}
	}
	manifest, err := json.MarshalIndent(map[string]any{
		"name":      name,
		"interface": map[string]string{"displayName": displayName},
		"plugins":   manifestPlugins,
	}, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	tree[".agents/plugins/marketplace.json"] = append(manifest, '\n')
	if len(manifestPlugins) == 0 {
		skipped = append(skipped, "marketplace-empty")
	}
	return tree, skipped, nil
}

// slugPattern 与 PluginPocket 市场 slug 规则一致；Codex 插件名要求 kebab-case，这里同源约束。
var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

// contentVersion 对插件文件树取确定性指纹（不含 plugin.json 自身），
// 输出 1.<hash8>：同内容同版本（幂等），内容变则版本变（触发客户端更新）。
func contentVersion(files map[string][]byte) string {
	sum := sha256.New()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		sum.Write([]byte(name))
		sum.Write([]byte{0})
		sum.Write(files[name])
		sum.Write([]byte{0})
	}
	return "1." + hex.EncodeToString(sum.Sum(nil))[:8]
}

// mcpServerEntry：公共 HTTP 端点 → 直连 HTTP MCP（Codex 配置类型值为 "http"）；网关供给 → 本地
// pluginpocket bridge（stdio，读登录凭证注入 Bearer，调用经计量）。两者都不满足则不可渲染。
func mcpServerEntry(transport, endpoint string) map[string]any {
	if transport == "http" && endpoint != "" {
		return map[string]any{"type": "http", "url": endpoint}
	}
	if transport == "gateway" {
		// bridge 默认即读 ~/.pluginpocket/config.json（由 HOME 解析）；
		// 不传 env——环境变量不做 ~ 展开，写死路径反而会破坏凭证查找。
		return map[string]any{
			"command": "pluginpocket",
			"args":    []string{"bridge"},
		}
	}
	return nil
}

// LoadExportInputs 从数据库装配导出输入：内联技能直接取文件，github 技能
// 由服务端解析（失败记入 unresolved，整条不导出——宁可少发不发坏的）。
func LoadExportInputs(ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, o Options) (entries []ExportInput, unresolved []string, err error) {
	rows, err := pool.Query(ctx, "SELECT slug,name,description,kind,transport,endpoint,version,spec FROM marketplace_items ORDER BY id")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var raw []ExportInput
	for rows.Next() {
		var entry ExportInput
		if err = rows.Scan(&entry.Slug, &entry.Name, &entry.Description, &entry.Kind, &entry.Transport, &entry.Endpoint, &entry.Version, &entry.Spec); err != nil {
			return nil, nil, err
		}
		raw = append(raw, entry)
	}
	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}
	resolveSkill := func(entry ExportInput) (map[string]string, map[string]bool) {
		files, loadErr := LoadSkillFiles(ctx, o, entry.Spec)
		if loadErr != nil {
			var spec struct {
				Manifest map[string]FileRef `json:"file_manifest"`
			}
			if json.Unmarshal(entry.Spec, &spec) == nil && len(spec.Manifest) > 0 {
				err = fmt.Errorf("skill %s snapshot unavailable: %w", entry.Slug, loadErr)
			}
			unresolved = append(unresolved, entry.Slug+":"+loadErr.Error())
			return nil, nil
		}
		content := map[string]string{}
		modes := map[string]bool{}
		for name, file := range files {
			content[name] = string(file.Content)
			if file.Executable {
				modes[name] = true
			}
		}
		return content, modes
	}

	for _, entry := range raw {
		switch entry.Kind {
		case "skill":
			entry.Files, entry.ExecutableFiles = resolveSkill(entry)
			if entry.Files == nil {
				continue
			}
		case "bundle":
			var spec struct {
				Includes []string `json:"includes"`
			}
			if json.Unmarshal(entry.Spec, &spec) != nil {
				continue
			}
			entry.MemberFiles = map[string]map[string]string{}
			entry.MemberExecutableFiles = map[string]map[string]bool{}
			complete := true
			for _, member := range spec.Includes {
				for _, candidate := range raw {
					if candidate.Slug == member && candidate.Kind == "skill" {
						files, modes := resolveSkill(candidate)
						if files == nil {
							complete = false
							break
						}
						entry.MemberFiles[member] = files
						entry.MemberExecutableFiles[member] = modes
					}
				}
			}
			if !complete {
				continue
			}
		}
		entries = append(entries, entry)
	}
	return entries, unresolved, err
}

// ExportExecutableFiles mirrors the skill paths emitted by ExportCodexMarketplace.
func ExportExecutableFiles(entries []ExportInput) map[string]bool {
	modes := map[string]bool{}
	for _, entry := range entries {
		for name, executable := range entry.ExecutableFiles {
			if executable {
				modes[fmt.Sprintf("plugins/%s/skills/%s/%s", entry.Slug, entry.Slug, name)] = true
			}
		}
		for member, files := range entry.MemberExecutableFiles {
			for name, executable := range files {
				if executable {
					modes[fmt.Sprintf("plugins/%s/skills/%s/%s", entry.Slug, member, name)] = true
				}
			}
		}
	}
	return modes
}
