mod common;
use std::{
    fs,
    io::{Read, Write},
    net::TcpListener,
    thread,
    time::{Duration, Instant},
};

fn fixture(
    outcomes: Vec<(u16, &'static str)>,
    expires: u64,
    unsafe_uri: bool,
) -> (String, thread::JoinHandle<Vec<(String, Instant)>>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let uri = if unsafe_uri {
        "https://user:ppt_uri_secret@example.com/devices".to_owned()
    } else {
        format!("{url}/devices")
    };
    let authorization=serde_json::json!({"device_code":"ldd_private_device","user_code":"ABCD-1234","verification_uri":uri,"expires_in":expires,"interval":0}).to_string();
    let mut responses = vec![(200, authorization)];
    responses.extend(outcomes.into_iter().map(|(s, b)| (s, b.to_owned())));
    let task = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(11);
        let mut requests = vec![];
        for (status, body) in responses {
            loop {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        stream
                            .set_read_timeout(Some(Duration::from_secs(2)))
                            .unwrap();
                        let mut bytes = Vec::new();
                        let mut buf = [0; 4096];
                        loop {
                            let n = stream.read(&mut buf).unwrap();
                            assert!(n > 0);
                            bytes.extend_from_slice(&buf[..n]);
                            if let Some(i) = bytes.windows(4).position(|b| b == b"\r\n\r\n") {
                                let headers = String::from_utf8_lossy(&bytes[..i]);
                                let length = headers
                                    .lines()
                                    .find_map(|l| {
                                        l.to_ascii_lowercase()
                                            .strip_prefix("content-length: ")
                                            .and_then(|n| n.parse::<usize>().ok())
                                    })
                                    .unwrap_or(0);
                                if bytes.len() >= i + 4 + length {
                                    break;
                                }
                            }
                        }
                        requests.push((String::from_utf8(bytes).unwrap(), Instant::now()));
                        write!(stream,"HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()).unwrap();
                        break;
                    }
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        if Instant::now() > deadline {
                            return requests;
                        }
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(e) => panic!("fixture: {e}"),
                }
            }
        }
        requests
    });
    (url, task)
}
#[test]
fn device_login_polls_pending_and_slow_down_then_verifies_and_stores_token_without_approving() {
    let dir = tempfile::tempdir().unwrap();
    let (server, task) = fixture(
        vec![
            (400, r#"{"error":"authorization_pending"}"#),
            (429, r#"{"error":"slow_down"}"#),
            (200, r#"{"token":"ppt_device_secret"}"#),
            (200, r#"{"username":"alice","balance":42,"tools":["echo"]}"#),
        ],
        30,
        false,
    );
    let output = common::cli(&dir, &["login", "--device", "--server", &server], "");
    assert!(output.status.success(), "{output:?}");
    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(stdout.contains("ABCD-1234"));
    assert!(stdout.contains(&format!("{server}/devices")));
    assert!(stdout.contains("alice"));
    for secret in ["ppt_device_secret", "ldd_private_device"] {
        assert!(!stdout.contains(secret));
        assert!(!String::from_utf8_lossy(&output.stderr).contains(secret));
    }
    let requests = task.join().unwrap();
    assert_eq!(requests.len(), 5);
    assert!(requests[0].0.starts_with("POST /api/v1/device/authorize "));
    for request in &requests[1..4] {
        assert!(request.0.starts_with("POST /api/v1/device/token "));
        assert!(request.0.contains("ldd_private_device"));
        assert!(!request.0.contains("approve"));
    }
    assert!(requests[4].0.starts_with("GET /api/v1/account/verify "));
    assert!(
        requests[4]
            .0
            .to_lowercase()
            .contains("authorization: bearer ppt_device_secret")
    );
    assert!(requests[1].1.duration_since(requests[0].1) >= Duration::from_millis(900));
    assert!(requests[3].1.duration_since(requests[2].1) >= Duration::from_millis(5900));
    let stored: serde_json::Value =
        serde_json::from_slice(&fs::read(dir.path().join(".pluginpocket/config.json")).unwrap())
            .unwrap();
    assert_eq!(stored["token"], "ppt_device_secret");
}
#[test]
fn expired_or_consumed_device_requests_preserve_old_credentials_and_report_safe_error() {
    for error in ["expired_token", "invalid_grant"] {
        let dir = tempfile::tempdir().unwrap();
        let local = common::local(&dir);
        common::credentials(&local, "https://example.com", "ppt_old");
        let before = fs::read(&local.config_path).unwrap();
        let (server, task) = fixture(
            vec![(
                400,
                if error == "expired_token" {
                    r#"{"error":"expired_token","secret":"ppt_never_echo"}"#
                } else {
                    r#"{"error":"invalid_grant","secret":"ppt_never_echo"}"#
                },
            )],
            30,
            false,
        );
        let output = common::cli(&dir, &["login", "--device", "--server", &server], "");
        assert!(!output.status.success());
        assert!(!String::from_utf8_lossy(&output.stderr).contains("ppt_never_echo"));
        assert_eq!(fs::read(&local.config_path).unwrap(), before);
        assert_eq!(task.join().unwrap().len(), 2);
    }
}
#[test]
fn device_login_rejects_unsafe_verification_uri_expiry_and_token_option_conflict() {
    for unsafe_uri in [true, false] {
        let dir = tempfile::tempdir().unwrap();
        let (server, task) = fixture(vec![], 1, unsafe_uri);
        let output = common::cli(&dir, &["login", "--device", "--server", &server], "");
        assert!(!output.status.success());
        assert!(!String::from_utf8_lossy(&output.stdout).contains("ppt_uri_secret"));
        assert!(!String::from_utf8_lossy(&output.stderr).contains("ppt_uri_secret"));
        assert!(!dir.path().join(".pluginpocket/config.json").exists());
        assert_eq!(task.join().unwrap().len(), 1);
    }
    let dir = tempfile::tempdir().unwrap();
    let output = common::cli(&dir, &["login", "--device", "--token", "ppt_secret"], "");
    assert!(!output.status.success());
    assert!(!String::from_utf8_lossy(&output.stderr).contains("ppt_secret"));
}
