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
        "# user comment\nmodel = \"custom\"\n[mcp_servers.other]\ncommand = \"custom\" # keep me\n",
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
fn bridge_apply_preserves_other_settings_comments_and_is_idempotent_with_backup_and_remove() {
    let dir = tempfile::tempdir().unwrap();
    seed(&dir);
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    let original = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert_eq!(
        local
            .apply(&[], false)
            .expect("detected clients must configure")
            .len(),
        3
    );
    for path in [".codex/config.toml", ".claude.json", ".cursor/mcp.json"] {
        let file = dir.path().join(path);
        let applied = fs::read_to_string(&file).unwrap();
        assert!(!applied.contains("ppt_secret"));
        assert_eq!(
            fs::read_to_string(format!("{}.pluginpocket.bak", file.display())).unwrap(),
            if path.ends_with("toml") {
                original.clone()
            } else if path == ".claude.json" {
                r#"{"theme":"dark","mcpServers":{"other":{"command":"custom"}}}"#.into()
            } else {
                r#"{"otherSetting":true,"mcpServers":{"other":{"command":"custom"}}}"#.into()
            }
        );
        local.apply(&ALL, false).unwrap();
        assert_eq!(fs::read_to_string(file).unwrap(), applied);
    }
    let toml = fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap();
    assert!(toml.starts_with(&original));
    let doc = toml.parse::<toml_edit::DocumentMut>().unwrap();
    assert_eq!(
        doc["mcp_servers"]["pluginpocket"]["args"][0].as_str(),
        Some("bridge")
    );
    assert_eq!(
        doc["mcp_servers"]["pluginpocket"]["env"]["PLUGINPOCKET_CONFIG"].as_str(),
        local.config_path.to_str()
    );
    for path in [".claude.json", ".cursor/mcp.json"] {
        let doc: serde_json::Value =
            serde_json::from_slice(&fs::read(dir.path().join(path)).unwrap()).unwrap();
        assert_eq!(doc["mcpServers"]["other"]["command"], "custom");
        assert_eq!(
            doc["mcpServers"]["pluginpocket"]["command"],
            local.executable.to_str().unwrap()
        );
    }
    local.remove(&ALL).unwrap();
    local.remove(&ALL).unwrap();
    assert_eq!(
        fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap(),
        original
    );
    let doc: serde_json::Value =
        serde_json::from_slice(&fs::read(dir.path().join(".claude.json")).unwrap()).unwrap();
    assert_eq!(doc["theme"], "dark");
    assert!(doc["mcpServers"].get("pluginpocket").is_none());
}
#[test]
fn refuses_manual_entries_malformed_documents_and_links_without_overwriting() {
    for (kind, path, body) in [
        (
            Codex,
            ".codex/config.toml",
            "[mcp_servers.pluginpocket]\ncommand='mine'\n",
        ),
        (
            Codex,
            ".codex/config.toml",
            "mcp_servers={pluginpocket={command='mine'}}",
        ),
        (
            Codex,
            ".codex/config.toml",
            "# --- pluginpocket begin ---\nbroken",
        ),
        (
            Claude,
            ".claude.json",
            r#"{"mcpServers":{"pluginpocket":{"command":"mine"}}}"#,
        ),
        (Cursor, ".cursor/mcp.json", r#"{"mcpServers":[]}"#),
        (Claude, ".claude.json", "not json"),
    ] {
        let dir = tempfile::tempdir().unwrap();
        let local = local(&dir);
        credentials(&local, "https://example.com", "ppt_secret");
        fs::create_dir_all(dir.path().join(path).parent().unwrap()).unwrap();
        fs::write(dir.path().join(path), body).unwrap();
        assert!(local.apply(&[kind], false).is_err(), "accepted {body}");
        assert!(local.remove(&[kind]).is_err(), "removed {body}");
        assert_eq!(fs::read_to_string(dir.path().join(path)).unwrap(), body);
    }
}
#[test]
fn direct_mode_is_explicit_and_generated_strings_survive_parsing() {
    let dir = tempfile::tempdir().unwrap();
    let exe = dir.path().join("目录 C:\\bin\\load\"out.exe");
    let local = pluginpocket::LocalClient::new(
        dir.path().into(),
        dir.path().join("令牌/config.json"),
        exe.clone(),
    )
    .unwrap();
    credentials(&local, "https://example.com", "ppt_secret");
    local.apply(&ALL, false).unwrap();
    let doc = fs::read_to_string(dir.path().join(".codex/config.toml"))
        .unwrap()
        .parse::<toml_edit::DocumentMut>()
        .unwrap();
    assert_eq!(
        doc["mcp_servers"]["pluginpocket"]["command"].as_str(),
        exe.to_str()
    );
    local.apply(&ALL, true).unwrap();
    let doc: serde_json::Value =
        serde_json::from_slice(&fs::read(dir.path().join(".claude.json")).unwrap()).unwrap();
    assert_eq!(
        doc["mcpServers"]["pluginpocket"]["url"],
        "https://example.com/mcp"
    );
    assert_eq!(
        doc["mcpServers"]["pluginpocket"]["headers"]["Authorization"],
        "Bearer ppt_secret"
    );
    let doc = fs::read_to_string(dir.path().join(".codex/config.toml"))
        .unwrap()
        .parse::<toml_edit::DocumentMut>()
        .unwrap();
    assert_eq!(
        doc["mcp_servers"]["pluginpocket"]["bearer_token_env_var"].as_str(),
        Some("PLUGINPOCKET_TOKEN")
    );
    assert!(doc["mcp_servers"]["pluginpocket"].get("command").is_none());
}
#[test]
fn unknown_clients_fail_without_writes_and_no_detection_has_no_fake_success() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    assert!(local.apply(&[], false).is_err());
    let output = cli(&dir, &["apply", "--clients", "unknown"], "");
    assert!(!output.status.success());
    assert!(!dir.path().join(".codex").exists());
}
#[cfg(unix)]
#[test]
fn client_and_backup_symlinks_are_rejected() {
    use std::os::unix::fs::symlink;
    for backup in [false, true] {
        let dir = tempfile::tempdir().unwrap();
        let local = local(&dir);
        credentials(&local, "https://example.com", "ppt_secret");
        let target = dir.path().join("target");
        fs::write(&target, "unchanged").unwrap();
        let path = dir.path().join(if backup {
            ".claude.json.pluginpocket.bak"
        } else {
            ".claude.json"
        });
        symlink(&target, path).unwrap();
        if backup {
            fs::write(dir.path().join(".claude.json"), "{}").unwrap();
        }
        assert!(local.apply(&[Claude], false).is_err());
        assert_eq!(fs::read_to_string(target).unwrap(), "unchanged");
    }
}

#[test]
fn markers_inside_user_strings_cannot_authorize_deleting_the_string() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    fs::create_dir(dir.path().join(".codex")).unwrap();
    let original = "description = '''\n# --- pluginpocket begin ---\n[mcp_servers.pluginpocket]\ncommand = \"text only\"\n# --- pluginpocket end ---\n'''\n";
    let path = dir.path().join(".codex/config.toml");
    fs::write(&path, original).unwrap();
    assert!(
        local.apply(&[Codex], false).is_err(),
        "string markers must not claim ownership"
    );
    assert_eq!(fs::read_to_string(path).unwrap(), original);
}

#[test]
fn modified_managed_entry_and_conflict_in_later_client_leave_all_files_unchanged() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    local.apply(&[Claude], false).unwrap();
    let path = dir.path().join(".claude.json");
    let mut changed: serde_json::Value = serde_json::from_slice(&fs::read(&path).unwrap()).unwrap();
    changed["mcpServers"]["pluginpocket"]["args"] = serde_json::json!(["user-edited"]);
    fs::write(&path, serde_json::to_vec(&changed).unwrap()).unwrap();
    assert!(local.apply(&[Codex, Claude], false).is_err());
    assert!(!dir.path().join(".codex/config.toml").exists());
    assert!(local.remove(&[Claude]).is_err());
    assert_eq!(
        serde_json::from_slice::<serde_json::Value>(&fs::read(path).unwrap()).unwrap(),
        changed
    );
}

#[test]
fn explicit_client_apply_does_not_fail_after_writing_due_to_an_unselected_bad_config() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    fs::create_dir(dir.path().join(".codex")).unwrap();
    fs::write(dir.path().join(".codex/config.toml"), "invalid TOML").unwrap();
    let result = local
        .apply(&[Claude], false)
        .expect("only the explicitly selected client is relevant");
    assert_eq!(result.len(), 1);
    assert!(result[0].configured);
    assert_eq!(
        fs::read_to_string(dir.path().join(".codex/config.toml")).unwrap(),
        "invalid TOML"
    );
}

#[test]
fn modified_codex_managed_block_is_never_overwritten_or_removed() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    local.apply(&[Codex], false).unwrap();
    let path = dir.path().join(".codex/config.toml");
    let changed = fs::read_to_string(&path)
        .unwrap()
        .replace("\"bridge\"", "\"user-edited\"");
    fs::write(&path, &changed).unwrap();
    assert!(
        local.apply(&[Codex], false).is_err(),
        "must preserve user edits"
    );
    assert!(local.remove(&[Codex]).is_err(), "must preserve user edits");
    assert_eq!(fs::read_to_string(path).unwrap(), changed);
}

#[test]
fn codex_marketplace_sections_inside_managed_block_are_healed_not_rejected() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    local.apply(&[Codex], false).unwrap();
    let path = dir.path().join(".codex/config.toml");
    // 复现实测污染：`codex plugin marketplace add` / `codex plugin add` 把
    // [marketplaces.*]/[plugins.*] 段追加进托管块内部（块位于文件末尾时必然发生）。
    let polluted = fs::read_to_string(&path).unwrap().replace(
        "# --- pluginpocket end ---\n",
        "[marketplaces.pluginpocket]\nsource_type = \"git\"\nsource = \"http://127.0.0.1:8787/marketplace.git\"\n\n[plugins.\"demo-mcp@pluginpocket\"]\nenabled = true\n# --- pluginpocket end ---\n",
    );
    fs::write(&path, &polluted).unwrap();

    let states = local.apply(&[Codex], false).unwrap();
    assert!(states[0].configured, "polluted block must stay owned");
    let healed = fs::read_to_string(&path).unwrap();
    let block_start = healed.find("# --- pluginpocket begin ---").unwrap();
    let block_end = healed.find("# --- pluginpocket end ---").unwrap();
    let block = &healed[block_start..block_end];
    assert!(
        !block.contains("[marketplaces."),
        "foreign tables must move out of the managed block"
    );
    assert!(!block.contains("[plugins."));
    assert!(
        healed.contains("[marketplaces.pluginpocket]"),
        "foreign content must be preserved"
    );
    assert!(healed.contains("[plugins.\"demo-mcp@pluginpocket\"]"));
    let doc = healed.parse::<toml_edit::DocumentMut>().unwrap();
    assert_eq!(
        doc["mcp_servers"]["pluginpocket"]["args"][0].as_str(),
        Some("bridge")
    );
    assert_eq!(
        doc["plugins"]["demo-mcp@pluginpocket"]["enabled"].as_bool(),
        Some(true)
    );

    local.apply(&[Codex], false).unwrap();
    assert_eq!(fs::read_to_string(&path).unwrap(), healed, "idempotent");

    local.remove(&[Codex]).unwrap();
    let removed = fs::read_to_string(&path).unwrap();
    assert!(!removed.contains("pluginpocket begin"));
    assert!(
        removed.contains("[marketplaces.pluginpocket]"),
        "remove must keep foreign tables"
    );
}

#[test]
fn partially_written_clients_remain_owned_and_removable_after_later_io_failure() {
    let dir = tempfile::tempdir().unwrap();
    seed(&dir);
    let local = local(&dir);
    credentials(&local, "https://example.com", "ppt_secret");
    fs::create_dir(dir.path().join(".cursor/mcp.json.pluginpocket.bak")).unwrap();
    assert!(local.apply(&[Claude, Cursor], false).is_err());
    local
        .remove(&[Claude])
        .expect("an interrupted apply must leave successful entries recoverable");
    let doc: serde_json::Value =
        serde_json::from_slice(&fs::read(dir.path().join(".claude.json")).unwrap()).unwrap();
    assert!(doc["mcpServers"].get("pluginpocket").is_none());
    assert_eq!(doc["mcpServers"]["other"]["command"], "custom");
}
