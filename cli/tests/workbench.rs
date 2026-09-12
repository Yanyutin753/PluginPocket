use pluginpocket::{ClientKind, LocalClient};
mod common;

#[test]
fn failed_cli_equipment_and_device_login_operations_leave_safe_logs() {
    let dir = tempfile::tempdir().unwrap();
    let local = LocalClient::new(
        dir.path().into(),
        dir.path().join(".pluginpocket/config.json"),
        std::env::current_exe().unwrap(),
    )
    .unwrap();
    local
        .install_plugin(
            "example",
            Some("https://example.com/mcp"),
            &[ClientKind::Claude],
        )
        .unwrap();
    assert!(local.update(&[]).is_err());
    assert!(local.uninstall("example", &[ClientKind::Claude]).is_err());
    assert!(
        local
            .login_device("https://private.example/?secret=hidden", |_| Ok(()))
            .is_err()
    );
    let logs = local.logs().unwrap();
    let actions: Vec<_> = logs.iter().map(|log| log.action.as_str()).collect();
    assert_eq!(actions, ["update", "uninstall", "login"]);
    assert!(logs.iter().all(|log| log.level == "error"));
    let encoded = serde_json::to_string(&logs).unwrap();
    assert!(!encoded.contains("private.example"));
    assert!(!encoded.contains("hidden"));
}

#[test]
fn operation_logs_explain_auth_network_conflicts_and_integrity_without_exposing_inputs() {
    let dir = tempfile::tempdir().unwrap();
    let local = common::local(&dir);
    assert!(local.apply(&[ClientKind::Claude], false).is_err());
    assert!(local.login("http://127.0.0.1:1", "ppt_private").is_err());
    local
        .install_plugin(
            "example",
            Some("https://example.com/mcp"),
            &[ClientKind::Claude],
        )
        .unwrap();
    std::fs::write(
        dir.path().join(".claude.json"),
        r#"{"mcpServers":{"pluginpocket-example":{"url":"https://private.example"}}}"#,
    )
    .unwrap();
    assert!(
        local
            .uninstall_installed(
                "example",
                pluginpocket::InstalledKind::Mcp,
                &[ClientKind::Claude]
            )
            .is_err()
    );
    let (server, task) = common::fixture(
        "200 OK",
        r##"{"items":[{"slug":"broken","name":"Broken","kind":"skill","spec":{"source":"inline","files":{"notes.md":"private"}}}]}"##,
    );
    common::credentials(&local, &server, "ppt_private");
    assert!(local.install("broken", None, &[ClientKind::Codex]).is_err());
    task.join().unwrap();
    local.logout().unwrap();
    let logs = local.logs().unwrap();
    for (entry, category, recovery) in [
        (&logs[0], "配置", "登录"),
        (&logs[1], "登录", "网络"),
        (&logs[2], "卸载", "冲突"),
        (&logs[3], "安装", "完整性"),
        (&logs[4], "退出登录", "完成"),
    ] {
        assert!(entry.message.contains(category), "{}", entry.message);
        assert!(entry.message.contains(recovery), "{}", entry.message);
    }
    let serialized = serde_json::to_string(&logs).unwrap();
    for secret in ["ppt_private", "private.example", "127.0.0.1", "notes.md"] {
        assert!(!serialized.contains(secret));
    }
}

#[test]
fn log_write_failures_are_visible_without_reclassifying_successful_operations() {
    let dir = tempfile::tempdir().unwrap();
    let local = common::local(&dir);
    std::fs::create_dir_all(dir.path().join(".pluginpocket/operations.lock")).unwrap();
    assert!(local.logout().is_ok());
    assert!(
        local.logs().is_err(),
        "failed log persistence must not appear as an empty successful log read"
    );
    std::fs::remove_dir(dir.path().join(".pluginpocket/operations.lock")).unwrap();
    local.logout().unwrap();
    assert_eq!(local.logs().unwrap().len(), 1);
    std::fs::write(dir.path().join(".pluginpocket/operations.json"), "broken").unwrap();
    assert!(local.logout().is_ok());
    assert!(local.logs().is_err());
}
