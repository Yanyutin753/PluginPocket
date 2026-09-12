use pluginpocket_desktop::{LocalCommand, execute};
use std::{
    fs,
    io::{Read, Write},
    net::TcpListener,
    thread,
    time::{Duration, Instant},
};
fn local(dir: &tempfile::TempDir) -> pluginpocket::LocalClient {
    pluginpocket::LocalClient::new(
        dir.path().into(),
        dir.path().join(".pluginpocket/config.json"),
        std::env::current_exe().unwrap(),
    )
    .unwrap()
}
fn fixture() -> (String, thread::JoinHandle<String>) {
    response_fixture(r#"{"username":"alice","balance":42,"tools":["echo"]}"#.into())
}
fn response_fixture(body: String) -> (String, thread::JoinHandle<String>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let task = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(3);
        loop {
            if let Ok((mut stream, _)) = listener.accept() {
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .unwrap();
                let mut request = [0; 8192];
                let n = stream.read(&mut request).unwrap();
                write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()).unwrap();
                return String::from_utf8_lossy(&request[..n]).into_owned();
            }
            if Instant::now() > deadline {
                return String::new();
            }
            thread::sleep(Duration::from_millis(5));
        }
    });
    (url, task)
}
#[test]
fn limited_command_uses_real_local_library_for_login_configuration_status_and_logout() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    let (server, task) = fixture();
    let response = execute(
        &local,
        LocalCommand::Login {
            server,
            token: "ppt_desktop_secret".into(),
        },
    )
    .expect("desktop login persists real credentials");
    assert_eq!(response["username"], "alice");
    assert!(!response.to_string().contains("ppt_"));
    assert!(
        task.join()
            .unwrap()
            .to_lowercase()
            .contains("authorization: bearer ppt_desktop_secret")
    );
    let response = execute(
        &local,
        LocalCommand::Apply {
            clients: vec![pluginpocket::ClientKind::Claude],
        },
    )
    .unwrap();
    assert_eq!(response[0]["configured"], true);
    let config: serde_json::Value =
        serde_json::from_slice(&fs::read(dir.path().join(".claude.json")).unwrap()).unwrap();
    assert_eq!(
        config["mcpServers"]["pluginpocket"]["command"],
        local.executable.to_str().unwrap()
    );
    assert_eq!(
        config["mcpServers"]["pluginpocket"]["args"],
        serde_json::json!(["bridge"])
    );
    assert!(!config.to_string().contains("ppt_"));
    let offline = execute(&local, LocalCommand::Clients {}).unwrap();
    assert_eq!(offline[1]["configured"], true);
    let (server, task) = fixture();
    let mut credentials: serde_json::Value =
        serde_json::from_slice(&fs::read(&local.config_path).unwrap()).unwrap();
    credentials["serverUrl"] = server.into();
    fs::write(
        &local.config_path,
        serde_json::to_vec(&credentials).unwrap(),
    )
    .unwrap();
    let status = execute(&local, LocalCommand::Status {}).unwrap();
    assert_eq!(status["account"]["balance"], 42);
    task.join().unwrap();
    execute(
        &local,
        LocalCommand::Remove {
            clients: vec![pluginpocket::ClientKind::Claude],
        },
    )
    .unwrap();
    assert!(
        !serde_json::from_slice::<serde_json::Value>(
            &fs::read(dir.path().join(".claude.json")).unwrap()
        )
        .unwrap()["mcpServers"]
            .as_object()
            .unwrap()
            .contains_key("pluginpocket")
    );
    execute(&local, LocalCommand::Logout {}).unwrap();
    assert!(!local.config_path.exists());
    assert!(execute(&local, LocalCommand::Status {}).is_err());
}
#[test]
fn invoke_rejects_arbitrary_paths_shell_unknown_clients_and_unknown_actions() {
    for input in [
        r#"{"action":"shell","command":"touch /tmp/no"}"#,
        r#"{"action":"login","server":"https://example.com","token":"ppt_x","config_path":"/tmp/no"}"#,
        r#"{"action":"apply","clients":["unknown"]}"#,
        r#"{"action":"apply","clients":["codex"],"executable":"/tmp/no"}"#,
        r#"{"action":"logout","home":"/tmp/no"}"#,
    ] {
        assert!(
            serde_json::from_str::<LocalCommand>(input).is_err(),
            "accepted {input}"
        );
    }
}

fn command(
    local: &pluginpocket::LocalClient,
    input: serde_json::Value,
) -> pluginpocket::Result<serde_json::Value> {
    let command = serde_json::from_value(input).expect("workbench command is supported");
    execute(local, command)
}

#[test]
fn workbench_inventory_and_uninstall_are_offline_and_preserve_other_targets() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    local
        .install_plugin(
            "example",
            Some("https://example.com/mcp"),
            &[
                pluginpocket::ClientKind::Claude,
                pluginpocket::ClientKind::Cursor,
            ],
        )
        .unwrap();
    let inventory = command(&local, serde_json::json!({"action":"installed"})).unwrap();
    assert_eq!(
        inventory,
        serde_json::json!([{"slug":"example","kind":"mcp","clients":["claude","cursor"],"version":null}])
    );
    assert!(
        command(
            &local,
            serde_json::json!({"action":"uninstall_installed","slug":"example","kind":"mcp","clients":["codex"]})
        )
        .is_err()
    );
    command(
        &local,
        serde_json::json!({"action":"uninstall_installed","slug":"example","kind":"mcp","clients":["claude"]}),
    )
    .unwrap();
    assert_eq!(
        command(&local, serde_json::json!({"action":"installed"})).unwrap()[0]["clients"],
        serde_json::json!(["cursor"])
    );
    let path = dir.path().join(".cursor/mcp.json");
    let foreign = r#"{"mcpServers":{"pluginpocket-example":{"url":"https://foreign.example"}}}"#;
    fs::write(&path, foreign).unwrap();
    assert!(
        command(
            &local,
            serde_json::json!({"action":"uninstall_installed","slug":"example","kind":"mcp","clients":[]})
        )
        .is_err()
    );
    assert_eq!(fs::read_to_string(path).unwrap(), foreign);
}

#[test]
fn workbench_diagnostics_keep_local_checks_when_network_and_one_client_fail() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    fs::write(dir.path().join(".claude.json"), "broken json").unwrap();
    let response = command(
        &local,
        serde_json::json!({"action":"diagnostics","server":"http://127.0.0.1:1"}),
    )
    .unwrap();
    let checks = response["checks"].as_array().unwrap();
    assert_eq!(
        checks.iter().find(|c| c["id"] == "client-claude").unwrap()["label"],
        "Claude Code"
    );
    for (id, status) in [
        ("network", "error"),
        ("auth", "warning"),
        ("client-claude", "error"),
        ("client-codex", "warning"),
        ("bridge", "ok"),
    ] {
        assert_eq!(
            checks.iter().find(|c| c["id"] == id).unwrap()["status"],
            status
        );
    }
}

#[test]
fn workbench_logs_survive_reopening_and_never_include_inputs() {
    let dir = tempfile::tempdir().unwrap();
    let first = local(&dir);
    assert!(
        execute(
            &first,
            LocalCommand::Login {
                server: "https://secret.example/path?password=private".into(),
                token: "ppt_super_secret".into()
            }
        )
        .is_err()
    );
    execute(&first, LocalCommand::Logout {}).unwrap();
    let reopened = local(&dir);
    let logs = command(&reopened, serde_json::json!({"action":"logs"})).unwrap();
    let entries = logs.as_array().unwrap();
    assert_eq!(entries.len(), 2);
    assert_eq!(entries[0]["action"], "login");
    assert_eq!(entries[0]["level"], "error");
    assert!(entries[0]["timestamp"].as_u64().unwrap() > 1_000_000_000_000);
    for secret in ["secret.example", "password", "ppt_super_secret"] {
        assert!(!logs.to_string().contains(secret));
    }
    for _ in 0..205 {
        execute(&reopened, LocalCommand::Logout {}).unwrap();
    }
    assert_eq!(
        command(&local(&dir), serde_json::json!({"action":"logs"}))
            .unwrap()
            .as_array()
            .unwrap()
            .len(),
        200
    );
}

fn set_credentials(local: &pluginpocket::LocalClient, server: &str) {
    fs::create_dir_all(local.config_path.parent().unwrap()).unwrap();
    fs::write(
        &local.config_path,
        serde_json::to_vec(
            &serde_json::json!({"serverUrl":server,"token":"ppt_fixture","username":"alice"}),
        )
        .unwrap(),
    )
    .unwrap();
}

#[test]
fn workbench_update_uses_only_existing_targets_and_rejects_kind_changes() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    local
        .install_plugin(
            "example",
            Some("https://example.com/old"),
            &[pluginpocket::ClientKind::Claude],
        )
        .unwrap();
    fs::create_dir_all(dir.path().join(".codex")).unwrap();
    assert!(
        command(
            &local,
            serde_json::json!({"action":"update_installed","slug":"example","kind":"mcp","clients":["codex"]})
        )
        .is_err()
    );
    let (server, task) = response_fixture(serde_json::json!({"items":[{"slug":"example","name":"Example","kind":"mcp","transport":"http","endpoint":"https://example.com/new"}]}).to_string());
    set_credentials(&local, &server);
    command(
        &local,
        serde_json::json!({"action":"update_installed","slug":"example","kind":"mcp","clients":[]}),
    )
    .unwrap();
    task.join().unwrap();
    assert!(!dir.path().join(".codex/config.toml").exists());
    assert!(
        fs::read_to_string(dir.path().join(".claude.json"))
            .unwrap()
            .contains("https://example.com/new")
    );
    let before = fs::read(dir.path().join(".claude.json")).unwrap();
    let (server, task) = response_fixture(serde_json::json!({"items":[{"slug":"example","name":"Example","kind":"bundle","spec":{"includes":[]}}]}).to_string());
    set_credentials(&local, &server);
    assert!(
        command(
            &local,
            serde_json::json!({"action":"update_installed","slug":"example","kind":"mcp","clients":[]})
        )
        .is_err()
    );
    task.join().unwrap();
    assert_eq!(fs::read(dir.path().join(".claude.json")).unwrap(), before);
    let (server, task) = response_fixture(serde_json::json!({"items":[{"slug":"example","name":"Example","kind":"mcp","transport":"http","endpoint":"https://user:secret@example.com/mcp"}]}).to_string());
    set_credentials(&local, &server);
    assert!(
        command(
            &local,
            serde_json::json!({"action":"update_installed","slug":"example","kind":"mcp","clients":[]})
        )
        .is_err()
    );
    task.join().unwrap();
    assert_eq!(fs::read(dir.path().join(".claude.json")).unwrap(), before);
}

#[test]
fn workbench_skill_inventory_uninstall_and_bridge_failure_log_use_real_local_state() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    let (server, task) = response_fixture(serde_json::json!({"items":[{"slug":"guide","name":"Guide","kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"# Guide","docs/help.md":"Help"}}}]}).to_string());
    set_credentials(&local, &server);
    local
        .install("guide", None, &[pluginpocket::ClientKind::Codex])
        .unwrap();
    task.join().unwrap();
    fs::remove_file(&local.config_path).unwrap();
    assert_eq!(
        command(&local, serde_json::json!({"action":"installed"})).unwrap(),
        serde_json::json!([{"slug":"guide","kind":"skill","clients":["codex"],"version":null}])
    );
    let root = dir.path().join(".codex/skills/guide");
    fs::write(root.join("foreign.txt"), "keep").unwrap();
    assert!(
        command(
            &local,
            serde_json::json!({"action":"uninstall_installed","slug":"guide","kind":"skill","clients":[]})
        )
        .is_err()
    );
    assert!(root.join("foreign.txt").exists());
    fs::remove_file(root.join("foreign.txt")).unwrap();
    command(
        &local,
        serde_json::json!({"action":"uninstall_installed","slug":"guide","kind":"skill","clients":[]}),
    )
    .unwrap();
    assert!(!root.exists());
    let logs = command(&local, serde_json::json!({"action":"logs"})).unwrap();
    assert_eq!(logs[0]["action"], "install");
}

#[test]
fn workbench_operations_distinguish_mcp_and_skill_with_the_same_slug() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    local
        .install_plugin(
            "shared",
            Some("https://example.com/mcp"),
            &[pluginpocket::ClientKind::Claude],
        )
        .unwrap();
    let (server, task) = response_fixture(serde_json::json!({"items":[{"slug":"shared","name":"Guide","kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"# Guide"}}}]}).to_string());
    set_credentials(&local, &server);
    local
        .install("shared", None, &[pluginpocket::ClientKind::Codex])
        .unwrap();
    task.join().unwrap();
    command(&local, serde_json::json!({"action":"uninstall_installed","slug":"shared","kind":"mcp","clients":[]})).unwrap();
    assert_eq!(
        command(&local, serde_json::json!({"action":"installed"})).unwrap(),
        serde_json::json!([{"slug":"shared","kind":"skill","clients":["codex"],"version":null}])
    );
    assert!(dir.path().join(".codex/skills/shared/SKILL.md").exists());
}

#[test]
fn workbench_exports_only_selected_real_logs_to_a_new_private_file() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    execute(&local, LocalCommand::Logout {}).unwrap();
    assert!(
        execute(
            &local,
            LocalCommand::Apply {
                clients: vec![pluginpocket::ClientKind::Claude]
            }
        )
        .is_err()
    );
    let logs = command(&local, serde_json::json!({"action":"logs"})).unwrap();
    let selected = serde_json::json!([logs[1].clone()]);
    let first = command(
        &local,
        serde_json::json!({"action":"export_logs","entries":selected}),
    )
    .unwrap();
    assert_eq!(first["count"], 1);
    let path = std::path::Path::new(first["path"].as_str().unwrap());
    // home 来自已规范化的临时目录：macOS 会带 /private 前缀、Windows 为 \\?\ 与
    // 完整用户名形式，两侧都规范化后比较目录归属。
    let normalize = |path: &std::path::Path| -> std::path::PathBuf {
        std::fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf())
    };
    assert_eq!(
        normalize(path.parent().unwrap()),
        normalize(&dir.path().join(".pluginpocket/exports"))
    );
    let contents = fs::read_to_string(path).unwrap();
    assert!(contents.contains(logs[1]["message"].as_str().unwrap()));
    assert!(!contents.contains(logs[0]["message"].as_str().unwrap()));
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        assert_eq!(
            fs::metadata(path).unwrap().permissions().mode() & 0o777,
            0o600
        );
    }
    let second = command(
        &local,
        serde_json::json!({"action":"export_logs","entries":selected}),
    )
    .unwrap();
    assert_ne!(first["path"], second["path"]);
    let mut forged = logs[0].clone();
    forged["message"] = "ppt_forged_secret".into();
    assert!(
        command(
            &local,
            serde_json::json!({"action":"export_logs","entries":[forged]})
        )
        .is_err()
    );
    assert_eq!(fs::read_dir(path.parent().unwrap()).unwrap().count(), 2);
}

#[test]
#[cfg(unix)]
fn workbench_export_refuses_a_linked_export_directory() {
    let dir = tempfile::tempdir().unwrap();
    let outside = tempfile::tempdir().unwrap();
    let local = local(&dir);
    local.logout().unwrap();
    std::os::unix::fs::symlink(outside.path(), dir.path().join(".pluginpocket/exports")).unwrap();
    let logs = command(&local, serde_json::json!({"action":"logs"})).unwrap();
    assert!(
        command(
            &local,
            serde_json::json!({"action":"export_logs","entries":logs})
        )
        .is_err()
    );
    assert_eq!(fs::read_dir(outside.path()).unwrap().count(), 0);
}
