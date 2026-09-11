package marketplace

import (
	"encoding/json"
	"strings"
	"testing"
)

func bundleSpec(slugs ...string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"includes": slugs})
	return raw
}

var exportFixture = []ExportInput{
	{Slug: "deepwiki", Name: "DeepWiki", Description: "Ask about repos", Kind: "mcp", Transport: "http", Endpoint: "https://mcp.deepwiki.com/mcp"},
	{Slug: "commit-style", Name: "提交信息规范", Description: "规范提交", Kind: "skill",
		Files: map[string]string{"SKILL.md": "---\nname: commit-style\n---\n正文"}},
	{Slug: "pro-skill", Name: "Pro", Kind: "skill", Spec: []byte(`{"source":"github"}`)}, // github 未解析 → 跳过
	{Slug: "github-server", Name: "GitHub official", Kind: "mcp", Transport: "unknown"},  // 无端点 → 跳过
	{Slug: "expert-pack", Name: "专家装备组", Description: "复刻专家", Kind: "bundle",
		Spec:        bundleSpec("commit-style", "deepwiki"),
		MemberFiles: map[string]map[string]string{"commit-style": {"SKILL.md": "---\nname: commit-style\n---\n正文"}}},
}

func render(t *testing.T) (map[string][]byte, []string) {
	t.Helper()
	files, skipped, err := ExportCodexMarketplace("loadout", "Loadout 插件市场", exportFixture)
	if err != nil {
		t.Fatal(err)
	}
	return files, skipped
}

func decode(t *testing.T, raw []byte, into any) {
	t.Helper()
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("invalid JSON %s: %v", raw, err)
	}
}

func TestExportMarketplaceManifest(t *testing.T) {
	files, skipped := render(t)
	if len(skipped) != 2 || !strings.Contains(strings.Join(skipped, ","), "pro-skill") || !strings.Contains(strings.Join(skipped, ","), "github-server") {
		t.Fatalf("skipped=%v want pro-skill and github-server", skipped)
	}
	var manifest struct {
		Name      string `json:"name"`
		Interface struct {
			DisplayName string `json:"displayName"`
		} `json:"interface"`
		Plugins []struct {
			Name   string `json:"name"`
			Source struct {
				Source string `json:"source"`
				Path   string `json:"path"`
			} `json:"source"`
			Category string `json:"category"`
		} `json:"plugins"`
	}
	decode(t, files[".agents/plugins/marketplace.json"], &manifest)
	if manifest.Name != "loadout" || manifest.Interface.DisplayName != "Loadout 插件市场" {
		t.Fatalf("manifest identity wrong: %+v", manifest)
	}
	names := map[string]string{}
	for _, plugin := range manifest.Plugins {
		if plugin.Source.Source != "local" || !strings.HasPrefix(plugin.Source.Path, "./plugins/") {
			t.Fatalf("plugin source wrong: %+v", plugin)
		}
		names[plugin.Name] = plugin.Category
	}
	if len(names) != 3 { // deepwiki、commit-style、expert-pack
		t.Fatalf("plugins=%v", names)
	}
}

func TestExportMcpPluginTree(t *testing.T) {
	files, _ := render(t)
	var plugin struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	decode(t, files["plugins/deepwiki/.codex-plugin/plugin.json"], &plugin)
	if plugin.Name != "deepwiki" || plugin.Version == "" || plugin.Description != "Ask about repos" {
		t.Fatalf("plugin.json wrong: %+v", plugin)
	}
	var mcp struct {
		McpServers map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"mcpServers"`
	}
	decode(t, files["plugins/deepwiki/mcp.json"], &mcp)
	if entry, ok := mcp.McpServers["deepwiki"]; !ok || entry.Type != "streamable-http" || entry.URL != "https://mcp.deepwiki.com/mcp" {
		t.Fatalf("mcp.json wrong: %+v", mcp)
	}
}

func TestExportSkillAndBundlePluginTrees(t *testing.T) {
	files, _ := render(t)
	if got := string(files["plugins/commit-style/skills/commit-style/SKILL.md"]); !strings.Contains(got, "name: commit-style") {
		t.Fatalf("skill file wrong: %q", got)
	}
	// bundle：成员技能进 skills/，成员 MCP 进 mcp.json。
	if got := string(files["plugins/expert-pack/skills/commit-style/SKILL.md"]); !strings.Contains(got, "正文") {
		t.Fatalf("bundle skill wrong: %q", got)
	}
	var mcp struct {
		McpServers map[string]struct {
			URL string `json:"url"`
		} `json:"mcpServers"`
	}
	decode(t, files["plugins/expert-pack/mcp.json"], &mcp)
	if entry, ok := mcp.McpServers["deepwiki"]; !ok || entry.URL != "https://mcp.deepwiki.com/mcp" {
		t.Fatalf("bundle mcp wrong: %+v", mcp)
	}
	var plugin struct {
		Name string `json:"name"`
	}
	decode(t, files["plugins/expert-pack/.codex-plugin/plugin.json"], &plugin)
	if plugin.Name != "expert-pack" {
		t.Fatalf("bundle plugin.json wrong: %+v", plugin)
	}
}

func TestLoadExportInputsAssemblesFromDatabase(t *testing.T) {
	s := marketplaceDB(t)
	ctx := t.Context()
	// 种子里已有 2 个内联技能 + 3 个 http MCP；补一个 bundle。
	if _, err := s.Pool.Exec(ctx, `INSERT INTO marketplace_items(slug,name,description,source,transport,kind,spec)
	 VALUES ('starter','起步装备组','','curated','unknown','bundle','{"includes":["commit-style","deepwiki"]}')`); err != nil {
		t.Fatal(err)
	}
	entries, unresolved, err := LoadExportInputs(ctx, s.Pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]ExportInput{}
	for _, entry := range entries {
		bySlug[entry.Slug] = entry
	}
	if len(unresolved) != 0 {
		t.Fatalf("unresolved=%v", unresolved)
	}
	if entry := bySlug["commit-style"]; entry.Kind != "skill" || entry.Files["SKILL.md"] == "" {
		t.Fatalf("skill input wrong: %+v", entry)
	}
	if entry := bySlug["deepwiki"]; entry.Kind != "mcp" || entry.Endpoint == "" {
		t.Fatalf("mcp input wrong: %+v", entry)
	}
	if entry := bySlug["starter"]; entry.Kind != "bundle" || entry.MemberFiles["commit-style"]["SKILL.md"] == "" {
		t.Fatalf("bundle input wrong: %+v", entry)
	}
	files, skipped, err := ExportCodexMarketplace("loadout", "Loadout 插件市场", entries)
	if err != nil || len(skipped) != 0 {
		t.Fatalf("export skipped=%v err=%v", skipped, err)
	}
	if files[".agents/plugins/marketplace.json"] == nil || files["plugins/starter/skills/commit-style/SKILL.md"] == nil {
		t.Fatalf("export tree incomplete: %d files", len(files))
	}
}

func TestExportGatewayComponentUsesBridge(t *testing.T) {
	entries := []ExportInput{
		{Slug: "seedance", Name: "Seedance", Kind: "mcp", Transport: "gateway"},
		{Slug: "video-pack", Name: "视频创作包", Kind: "bundle",
			Spec:        bundleSpec("seedance", "commit-style"),
			MemberFiles: map[string]map[string]string{"commit-style": {"SKILL.md": "---\nname: commit-style\n---\nx"}}},
		{Slug: "commit-style", Name: "提交规范", Kind: "skill", Files: map[string]string{"SKILL.md": "s"}},
	}
	files, skipped, err := ExportCodexMarketplace("loadout", "L", entries)
	if err != nil || len(skipped) != 0 {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	var mcp struct {
		McpServers map[string]map[string]any `json:"mcpServers"`
	}
	decode(t, files["plugins/seedance/mcp.json"], &mcp)
	entry := mcp.McpServers["seedance"]
	if entry["command"] != "loadout" {
		t.Fatalf("gateway entry must be bridge stdio: %+v", entry)
	}
	args, _ := entry["args"].([]any)
	if len(args) != 1 || args[0] != "bridge" {
		t.Fatalf("bridge args wrong: %+v", entry)
	}
	if _, hasEnv := entry["env"]; hasEnv {
		t.Fatal("bridge entry must not pin env paths (~ does not expand)")
	}
	decode(t, files["plugins/video-pack/mcp.json"], &mcp)
	if entry := mcp.McpServers["seedance"]; entry["command"] != "loadout" {
		t.Fatalf("bundle gateway member wrong: %+v", entry)
	}
}
