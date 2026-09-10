mod common;
use common::*;
use std::fs;

#[test]
fn login_verifies_token_atomically_persists_private_credentials_and_status_refreshes() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    let (server, request) = fixture(
        "200 OK",
        r#"{"username":"alice","balance":42,"tools":["echo","time_now"]}"#,
    );
    let account = local
        .login(&server, "ldt_private")
        .expect("valid login must succeed");
    assert_eq!(account.balance, 42);
    let request = request.join().unwrap();
    assert!(request.starts_with("GET /api/v1/account/verify "));
    assert!(
        request
            .to_lowercase()
            .contains("authorization: bearer ldt_private")
    );
    let stored: serde_json::Value =
        serde_json::from_slice(&fs::read(&local.config_path).unwrap()).unwrap();
    assert_eq!(stored["token"], "ldt_private");
    assert_eq!(stored["serverUrl"], format!("{server}/"));
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        assert_eq!(
            fs::metadata(&local.config_path)
                .unwrap()
                .permissions()
                .mode()
                & 0o777,
            0o600
        );
    }
    let (server, request) = fixture(
        "200 OK",
        r#"{"username":"alice","balance":11,"tools":["echo","time_now","other"]}"#,
    );
    credentials(&local, &server, "ldt_new");
    assert_eq!(local.status().unwrap().account.balance, 11);
    request.join().unwrap();
    local.logout().unwrap();
    assert!(!local.config_path.exists());
    local.logout().unwrap();
    assert!(local.status().is_err());
}
#[test]
fn failed_login_preserves_existing_credentials_and_never_returns_raw_response() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    credentials(&local, "https://example.com", "ldt_old");
    let before = fs::read(&local.config_path).unwrap();
    for (status, body) in [
        ("401 Unauthorized", "ldt_secret"),
        ("200 OK", "ldt_secret"),
        ("302 Found", "ldt_secret"),
        (
            "200 OK",
            r#"{"username":"bad\u001b[2J","balance":1,"tools":["echo","time_now"]}"#,
        ),
    ] {
        let (server, request) = fixture(status, body);
        let error = local.login(&server, "ldt_secret").unwrap_err();
        assert!(!error.contains("ldt_secret"));
        assert_eq!(fs::read(&local.config_path).unwrap(), before);
        request.join().unwrap();
    }
    for server in [
        "file:///tmp/x",
        "https://user:secret@example.com",
        "https://example.com/?token=secret",
        "https://example.com/prefix",
    ] {
        assert!(local.login(server, "ldt_secret").is_err());
    }
    assert!(local.login("https://example.com", "invalid").is_err());
}
#[test]
fn cli_reads_secret_from_stdin_and_relogin_leaves_client_config_unchanged() {
    let dir = tempfile::tempdir().unwrap();
    fs::write(
        dir.path().join(".claude.json"),
        r#"{"mcpServers":{"other":{"command":"x"}}}"#,
    )
    .unwrap();
    let before = fs::read(dir.path().join(".claude.json")).unwrap();
    let (server, request) = fixture(
        "200 OK",
        r#"{"username":"alice","balance":42,"tools":["echo","time_now"]}"#,
    );
    let output = cli(&dir, &["login", "--server", &server], "ldt_stdin_secret\n");
    assert!(output.status.success(), "{output:?}");
    assert!(!String::from_utf8_lossy(&output.stdout).contains("ldt_"));
    assert!(!String::from_utf8_lossy(&output.stderr).contains("ldt_stdin_secret"));
    request.join().unwrap();
    assert_eq!(fs::read(dir.path().join(".claude.json")).unwrap(), before);
    assert!(cli(&dir, &["logout"], "").status.success());
    assert!(cli(&dir, &["version"], "").status.success());
}
#[cfg(unix)]
#[test]
fn credentials_reject_symlink_target_or_parent_without_touching_target() {
    use std::os::unix::fs::symlink;
    for parent_link in [false, true] {
        let dir = tempfile::tempdir().unwrap();
        let local = local(&dir);
        let external = dir.path().join("external");
        fs::create_dir(&external).unwrap();
        fs::write(external.join("config.json"), "untouched").unwrap();
        if parent_link {
            symlink(&external, dir.path().join(".loadout")).unwrap()
        } else {
            fs::create_dir(dir.path().join(".loadout")).unwrap();
            symlink(external.join("config.json"), &local.config_path).unwrap();
        }
        let (server, request) = fixture(
            "200 OK",
            r#"{"username":"alice","balance":42,"tools":["echo","time_now"]}"#,
        );
        assert!(local.login(&server, "ldt_secret").is_err());
        assert!(local.logout().is_err());
        assert_eq!(
            fs::read_to_string(external.join("config.json")).unwrap(),
            "untouched"
        );
        request.join().unwrap();
    }
}

#[test]
fn verify_accepts_tool_catalog_array_from_product_api() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    let (server, request) = fixture(
        "200 OK",
        r#"{"username":"alice","balance":42,"tools":["echo","time_now"]}"#,
    );
    let account = local
        .login(&server, "ldt_private")
        .expect("verify returns the actual tool catalog array");
    assert_eq!(serde_json::to_value(account).unwrap()["tools"][0], "echo");
    request.join().unwrap();
}
