use loadout_desktop::{LocalCommand, execute};
use std::{
    fs,
    io::{Read, Write},
    net::TcpListener,
    thread,
    time::{Duration, Instant},
};
fn local(dir: &tempfile::TempDir) -> loadout::LocalClient {
    loadout::LocalClient::new(
        dir.path().into(),
        dir.path().join(".loadout/config.json"),
        std::env::current_exe().unwrap(),
    )
    .unwrap()
}
fn fixture() -> (String, thread::JoinHandle<String>) {
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
                let body = r#"{"username":"alice","balance":42,"tools":["echo"]}"#;
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
            token: "ldt_desktop_secret".into(),
        },
    )
    .expect("desktop login persists real credentials");
    assert_eq!(response["username"], "alice");
    assert!(!response.to_string().contains("ldt_"));
    assert!(
        task.join()
            .unwrap()
            .to_lowercase()
            .contains("authorization: bearer ldt_desktop_secret")
    );
    let response = execute(
        &local,
        LocalCommand::Apply {
            clients: vec![loadout::ClientKind::Claude],
        },
    )
    .unwrap();
    assert_eq!(response[0]["configured"], true);
    let config: serde_json::Value =
        serde_json::from_slice(&fs::read(dir.path().join(".claude.json")).unwrap()).unwrap();
    assert_eq!(
        config["mcpServers"]["loadout"]["command"],
        local.executable.to_str().unwrap()
    );
    assert_eq!(
        config["mcpServers"]["loadout"]["args"],
        serde_json::json!(["bridge"])
    );
    assert!(!config.to_string().contains("ldt_"));
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
            clients: vec![loadout::ClientKind::Claude],
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
            .contains_key("loadout")
    );
    execute(&local, LocalCommand::Logout {}).unwrap();
    assert!(!local.config_path.exists());
    assert!(execute(&local, LocalCommand::Status {}).is_err());
}
#[test]
fn invoke_rejects_arbitrary_paths_shell_unknown_clients_and_unknown_actions() {
    for input in [
        r#"{"action":"shell","command":"touch /tmp/no"}"#,
        r#"{"action":"login","server":"https://example.com","token":"ldt_x","config_path":"/tmp/no"}"#,
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
