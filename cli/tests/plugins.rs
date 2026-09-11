mod common;
use common::*;
use pluginpocket::ClientKind::{Claude, Codex, Cursor};
use std::fs;
const ALL: [pluginpocket::ClientKind; 3] = [Codex, Claude, Cursor];
fn seed(dir: &tempfile::TempDir) {
    fs::create_dir_all(dir.path().join(".codex")).unwrap();
    fs::create_dir_all(dir.path().join(".cursor")).unwrap();
    fs::write(
        dir.path().join(".codex/config.toml"),
        "model = \"custom\"\n[mcp_servers.other]\ncommand = \"custom\"\n",
    )
    .unwrap();
    fs::write(
        dir.path().join(".claude.json"),
        r#"{"theme":"dark","mcpServers":{"other":{"command":"custom"}}}"#,
    )
    .unwrap();
    fs::write(
        dir.path().join(".cursor/mcp.json"),
        r#"{"otherSetting":true,"mcpServers":{"other":{"command":"custom"}}}"#,
    )
    .unwrap();
}
#[test]
fn plugin_install_writes_managed_entries_with_markers_backup_and_uninstall() {
    let dir = tempfile::tempdir().unwrap();
    seed(&dir);
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    local
        .install_plugin("deepwiki", Some("https://mcp.deepwiki.com/mcp"), &ALL)
        .unwrap();
    let toml = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(toml.contains("# --- pluginpocket:deepwiki begin ---"));
    assert!(toml.contains("# --- pluginpocket:deepwiki end ---"));
    let doc = toml.parse::<toml_edit::DocumentMut>().unwrap();
    assert_eq!(
        doc["mcp_servers"]["pluginpocket-deepwiki"]["url"].as_str(),
        Some("https://mcp.deepwiki.com/mcp")
    );
    assert_eq!(
        doc["mcp_servers"]["other"]["command"].as_str(),
        Some("custom")
    );
    for path in [".claude.json", ".cursor/mcp.json"] {
        let value: serde_json::Value =
            serde_json::from_str(&fs::read_to_string(dir.path().join(path)).unwrap()).unwrap();
        assert_eq!(
            value["mcpServers"]["pluginpocket-deepwiki"],
            serde_json::json!({"type":"http","url":"https://mcp.deepwiki.com/mcp"})
        );
        assert_eq!(value["mcpServers"]["other"]["command"], "custom");
    }
    let manifest: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(dir.path().join(".pluginpocket/managed-clients.json")).unwrap(),
    )
    .unwrap();
    assert!(manifest.get("codex:deepwiki").is_some());
    assert!(manifest.get("claude:deepwiki").is_some());
    // 幂等：重复安装产生完全相同的配置
    local
        .install_plugin("deepwiki", Some("https://mcp.deepwiki.com/mcp"), &ALL)
        .unwrap();
    assert_eq!(
        fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap(),
        toml
    );
    // 与 bridge 条目共存
    local.apply(&ALL, false).unwrap();
    let both = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(both.contains("# --- pluginpocket begin ---"));
    assert!(both.contains("# --- pluginpocket:deepwiki begin ---"));
    let doc = both.parse::<toml_edit::DocumentMut>().unwrap();
    assert!(doc["mcp_servers"]["pluginpocket"]["command"].is_str());
    assert!(doc["mcp_servers"]["pluginpocket-deepwiki"]["url"].is_str());
    local.uninstall_plugin("deepwiki", &ALL).unwrap();
    let after = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(!after.contains("pluginpocket:deepwiki"));
    let doc = after.parse::<toml_edit::DocumentMut>().unwrap();
    assert!(doc["mcp_servers"]["pluginpocket"].is_table());
    assert!(doc["mcp_servers"]["other"]["command"].is_str());
    for path in [".claude.json", ".cursor/mcp.json"] {
        let value: serde_json::Value =
            serde_json::from_str(&fs::read_to_string(dir.path().join(path)).unwrap()).unwrap();
        assert!(value["mcpServers"]["pluginpocket-deepwiki"].is_null());
        assert_eq!(value["mcpServers"]["other"]["command"], "custom");
    }
    let manifest: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(dir.path().join(".pluginpocket/managed-clients.json")).unwrap(),
    )
    .unwrap();
    assert!(manifest.get("codex:deepwiki").is_none());
}
#[test]
fn plugin_slug_is_validated() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    assert!(
        local
            .install_plugin("../evil", Some("https://example.com"), &[Codex])
            .is_err()
    );
    assert!(
        local
            .install_plugin("a b", Some("https://example.com"), &[Codex])
            .is_err()
    );
}
#[test]
fn market_command_lists_items_and_cli_installs_from_endpoint() {
    let dir = tempfile::tempdir().unwrap();
    fs::create_dir_all(dir.path().join(".codex")).unwrap();
    let body = r#"{"items":[{"slug":"deepwiki","name":"DeepWiki","description":"Ask about repos","source":"curated","repo_url":"https://github.com/AsyncFuncAI/deepwiki-mcp","homepage":"","transport":"http","endpoint":"https://mcp.deepwiki.com/mcp","stars":0,"installed":false}]}"#;
    let (server, request) = fixture("200 OK", body);
    let local = local(&dir);
    credentials(&local, &server, "ppt_secret");
    let items = local.market().unwrap();
    assert_eq!(items.len(), 1);
    assert_eq!(items[0].slug, "deepwiki");
    assert_eq!(items[0].endpoint, "https://mcp.deepwiki.com/mcp");
    let _ = request.join().unwrap();
    let (server2, request2) = fixture("200 OK", body);
    credentials(&local, &server2, "ppt_secret");
    let output = cli(&dir, &["install", "deepwiki"], "");
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let _ = request2.join().unwrap();
    let toml = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(toml.contains("https://mcp.deepwiki.com/mcp"));
}

fn market_body(items: &serde_json::Value) -> String {
    serde_json::json!({"items": items}).to_string()
}
#[test]
fn skill_install_writes_managed_directories_and_uninstall_refuses_foreign_files() {
    let dir = tempfile::tempdir().unwrap();
    let items = serde_json::json!([{
        "slug":"commit-style","name":"提交信息规范","description":"","source":"curated",
        "repo_url":"","homepage":"","transport":"unknown","endpoint":"","stars":0,"installed":false,
        "kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"---\nname: commit-style\n---\n规范正文","refs.md":"参考"}}
    }]);
    let (server, request) = fixture("200 OK", &market_body(&items));
    let local = local(&dir);
    credentials(&local, &server, "ppt_secret");
    local.install("commit-style", None, &ALL).unwrap();
    let _ = request.join().unwrap();
    for base in [".codex/skills", ".claude/skills"] {
        let root = dir.path().join(base).join("commit-style");
        assert_eq!(
            std::fs::read_to_string(root.join("SKILL.md")).unwrap(),
            "---\nname: commit-style\n---\n规范正文"
        );
        assert!(root.join("refs.md").is_file());
    }
    assert!(!dir.path().join(".cursor/skills").exists());
    let manifest: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(dir.path().join(".pluginpocket/managed-clients.json")).unwrap(),
    )
    .unwrap();
    assert!(manifest.get("skill:codex:commit-style").is_some());
    // 卸载需要再次读取市场以识别条目类型：换一个新 fixture。
    let (server3, request3) = fixture("200 OK", &market_body(&items));
    credentials(&local, &server3, "ppt_secret");
    // 外来文件：卸载必须拒绝整目录删除
    fs::write(
        dir.path().join(".codex/skills/commit-style/user-note.md"),
        "user content",
    )
    .unwrap();
    assert!(local.uninstall("commit-style", &ALL).is_err());
    assert!(
        dir.path()
            .join(".codex/skills/commit-style/user-note.md")
            .is_file()
    );
    fs::remove_file(dir.path().join(".codex/skills/commit-style/user-note.md")).unwrap();
    let _ = request3.join().unwrap();
    let (server4, request4) = fixture("200 OK", &market_body(&items));
    credentials(&local, &server4, "ppt_secret");
    local.uninstall("commit-style", &ALL).unwrap();
    let _ = request4.join().unwrap();
    assert!(!dir.path().join(".codex/skills/commit-style").exists());
    assert!(!dir.path().join(".claude/skills/commit-style").exists());
}
#[test]
fn bundle_install_replicates_expert_setup_in_one_command() {
    let dir = tempfile::tempdir().unwrap();
    fs::create_dir_all(dir.path().join(".codex")).unwrap();
    let items = serde_json::json!([
        {"slug":"expert-pack","name":"专家装备组","description":"","source":"curated",
         "repo_url":"","homepage":"","transport":"unknown","endpoint":"","stars":0,"installed":false,
         "kind":"bundle","spec":{"includes":["deepwiki","commit-style"]}},
        {"slug":"deepwiki","name":"DeepWiki","description":"","source":"curated",
         "repo_url":"","homepage":"","transport":"http","endpoint":"https://mcp.deepwiki.com/mcp","stars":0,"installed":false,"kind":"mcp"},
        {"slug":"commit-style","name":"提交信息规范","description":"","source":"curated",
         "repo_url":"","homepage":"","transport":"unknown","endpoint":"","stars":0,"installed":false,
         "kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"---\nname: commit-style\n---\n正文"}}}
    ]);
    let (server, request) = fixture("200 OK", &market_body(&items));
    let local = local(&dir);
    credentials(&local, &server, "ppt_secret");
    local.install("expert-pack", None, &[Codex]).unwrap();
    let _ = request.join().unwrap();
    let toml = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(toml.contains("https://mcp.deepwiki.com/mcp"));
    assert!(
        dir.path()
            .join(".codex/skills/commit-style/SKILL.md")
            .is_file()
    );
    let (server2, request2) = fixture("200 OK", &market_body(&items));
    credentials(&local, &server2, "ppt_secret");
    local.uninstall("expert-pack", &[Codex]).unwrap();
    let _ = request2.join().unwrap();
    let toml = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(!toml.contains("deepwiki"));
    assert!(!dir.path().join(".codex/skills/commit-style").exists());
}

#[test]
fn gateway_member_installs_bridge_not_direct_entry() {
    let dir = tempfile::tempdir().unwrap();
    fs::create_dir_all(dir.path().join(".codex")).unwrap();
    let items = serde_json::json!([
        {"slug":"video-pack","name":"视频创作包","description":"","source":"curated",
         "repo_url":"","homepage":"","transport":"unknown","endpoint":"","stars":0,"installed":false,
         "kind":"bundle","spec":{"includes":["seedance","storyboard-skill"]}},
        {"slug":"seedance","name":"Seedance","description":"服务端独享","source":"curated",
         "repo_url":"","homepage":"","transport":"gateway","endpoint":"","stars":0,"installed":false,"kind":"mcp"},
        {"slug":"storyboard-skill","name":"分镜技能","description":"","source":"curated",
         "repo_url":"","homepage":"","transport":"unknown","endpoint":"","stars":0,"installed":false,
         "kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"---\nname: storyboard\n---\n分镜正文"}}}
    ]);
    let (server, request) = fixture("200 OK", &market_body(&items));
    let local = local(&dir);
    credentials(&local, &server, "ppt_secret");
    local.install("video-pack", None, &[Codex]).unwrap();
    let _ = request.join().unwrap();
    let toml = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(
        toml.contains("# --- pluginpocket begin ---"),
        "gateway 成员必须挂 bridge:\n{toml}"
    );
    assert!(toml.contains("args = [\"bridge\"]"));
    assert!(
        !toml.contains("pluginpocket-seedance"),
        "网关供给不落直连条目:\n{toml}"
    );
    assert!(
        dir.path()
            .join(".codex/skills/storyboard-skill/SKILL.md")
            .is_file()
    );
    // 卸载装备组：技能移除；bridge 保留（可能还有其他网关工具在用，由 remove 单独管理）
    let (server2, request2) = fixture("200 OK", &market_body(&items));
    credentials(&local, &server2, "ppt_secret");
    local.uninstall("video-pack", &[Codex]).unwrap();
    let _ = request2.join().unwrap();
    assert!(!dir.path().join(".codex/skills/storyboard-skill").exists());
    assert!(
        fs::read_to_string(dir.path().join(".codex/config.toml"))
            .unwrap()
            .contains("# --- pluginpocket begin ---")
    );
}

#[test]
fn update_refreshes_all_managed_plugins_to_latest() {
    let dir = tempfile::tempdir().unwrap();
    fs::create_dir_all(dir.path().join(".codex")).unwrap();
    let v1 = serde_json::json!([{"slug":"commit-style","name":"提交规范","description":"","source":"curated",
        "repo_url":"","homepage":"","transport":"unknown","endpoint":"","stars":0,"installed":false,
        "kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"---\nname: commit-style\n---\n版本一"}}}]);
    let (server, request) = fixture("200 OK", &market_body(&v1));
    let local = local(&dir);
    credentials(&local, &server, "ppt_secret");
    local.install("commit-style", None, &[Codex]).unwrap();
    let _ = request.join().unwrap();
    let file = dir.path().join(".codex/skills/commit-style/SKILL.md");
    assert!(fs::read_to_string(&file).unwrap().contains("版本一"));
    // 服务端内容迭代为版本二 → pluginpocket update 拉新覆盖。
    let v2 = serde_json::json!([{"slug":"commit-style","name":"提交规范","description":"","source":"curated",
        "repo_url":"","homepage":"","transport":"unknown","endpoint":"","stars":0,"installed":false,
        "kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"---\nname: commit-style\n---\n版本二"}}}]);
    let (server2, request2) = fixture("200 OK", &market_body(&v2));
    credentials(&local, &server2, "ppt_secret");
    let updated = local.update(&[Codex]).unwrap();
    let _ = request2.join().unwrap();
    assert_eq!(updated.len(), 1);
    assert!(fs::read_to_string(&file).unwrap().contains("版本二"));
}

#[test]
fn update_heals_blocks_polluted_by_codex_plugin_sections() {
    let dir = tempfile::tempdir().unwrap();
    fs::create_dir_all(dir.path().join(".codex")).unwrap();
    let v1 = serde_json::json!([{"slug":"deepwiki","name":"DeepWiki","description":"","source":"curated",
        "repo_url":"","homepage":"","transport":"http","endpoint":"https://mcp.deepwiki.com/mcp","stars":0,"installed":false,"kind":"mcp"}]);
    let (server, request) = fixture("200 OK", &market_body(&v1));
    let local = local(&dir);
    credentials(&local, &server, "ppt_secret");
    local.install("deepwiki", None, &[Codex]).unwrap();
    let _ = request.join().unwrap();
    // 模拟 codex plugin add 把外部段插进托管块中间（真机实测到的行为）。
    let config = dir.path().join(".codex/config.toml");
    let mut polluted = fs::read_to_string(&config).unwrap();
    polluted = polluted.replace(
        "# --- pluginpocket:deepwiki end ---",
        "[plugins.\"video-pack@pluginpocket\"]\nenabled = true\n\n[marketplaces.pluginpocket]\nsource_type = \"git\"\n# --- pluginpocket:deepwiki end ---",
    );
    fs::write(&config, polluted).unwrap();
    // update 必须自愈：净化污染、外部段保留在块外、内容刷新。
    let (server2, request2) = fixture("200 OK", &market_body(&v1));
    credentials(&local, &server2, "ppt_secret");
    local.update(&[Codex]).unwrap();
    let _ = request2.join().unwrap();
    let after = fs::read_to_string(&config).unwrap();
    let begin = after.find("# --- pluginpocket:deepwiki begin ---").unwrap();
    let end = after.find("# --- pluginpocket:deepwiki end ---").unwrap();
    assert!(
        !after[begin..end].contains("[plugins."),
        "块内不得残留外部段:\n{}",
        &after[begin..end]
    );
    assert!(
        after.contains("[plugins.\"video-pack@pluginpocket\"]"),
        "外部段必须保留"
    );
    assert!(after.contains("https://mcp.deepwiki.com/mcp"));
    let doc = after.parse::<toml_edit::DocumentMut>().unwrap();
    assert!(doc["mcp_servers"]["pluginpocket-deepwiki"]["url"].is_str());
    assert!(doc["plugins"]["video-pack@pluginpocket"]["enabled"].as_bool() == Some(true));
}
