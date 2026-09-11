use std::{
    io::{Read, Write},
    net::TcpListener,
    process::{Command, Output, Stdio},
    thread,
};

fn cli(args: &[&str]) -> Output {
    let dir = tempfile::tempdir().unwrap();
    let mut child = Command::new(env!("CARGO_BIN_EXE_loadout"))
        .args(args)
        .env("HOME", dir.path())
        .env("USERPROFILE", dir.path())
        .env_remove("PLUGINPOCKET_CONFIG")
        .env("NO_PROXY", "*")
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    let deadline = std::time::Instant::now() + std::time::Duration::from_secs(10);
    while child.try_wait().unwrap().is_none() {
        if std::time::Instant::now() >= deadline {
            child.kill().unwrap();
            child.wait().unwrap();
            panic!("CLI exceeded the test watchdog deadline");
        }
        thread::sleep(std::time::Duration::from_millis(10));
    }
    child.wait_with_output().unwrap()
}

fn serve_once(status: &str, body: &str) -> (String, thread::JoinHandle<String>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let response = format!(
        "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    );
    let task = thread::spawn(move || {
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(5);
        loop {
            match listener.accept() {
                Ok((mut stream, _)) => {
                    // Windows 上 accept 出的连接继承监听 socket 的非阻塞模式，
                    // 必须显式切回阻塞，否则 read 立即返回 WouldBlock。
                    stream.set_nonblocking(false).unwrap();
                    stream
                        .set_read_timeout(Some(std::time::Duration::from_secs(2)))
                        .unwrap();
                    let mut request = [0; 4096];
                    let n = stream.read(&mut request).unwrap();
                    stream.write_all(response.as_bytes()).unwrap();
                    return String::from_utf8_lossy(&request[..n]).into_owned();
                }
                Err(err) if err.kind() == std::io::ErrorKind::WouldBlock => {
                    if std::time::Instant::now() >= deadline {
                        return String::new();
                    }
                    thread::sleep(std::time::Duration::from_millis(10));
                }
                Err(err) => panic!("accept: {err}"),
            }
        }
    });
    (url, task)
}

#[test]
fn doctor_checks_real_health_endpoint() {
    let (url, task) = serve_once(
        "200 OK",
        r#"{"status":"ok","service":"loadout","version":"test"}"#,
    );
    let output = cli(&["doctor", "--server", &url]);
    assert!(output.status.success(), "{:?}", output);
    assert!(String::from_utf8_lossy(&output.stdout).contains("PluginPocket test"));
    assert!(
        task.join()
            .unwrap()
            .starts_with("GET /api/v1/health HTTP/1.1")
    );
}

#[test]
fn doctor_rejects_error_and_unrelated_servers_without_exposing_response() {
    for (status, body) in [
        ("500 Internal Server Error", "secret-token-value"),
        ("200 OK", "secret-token-value"),
        (
            "200 OK",
            r#"{"status":"ok","service":"other","version":"test"}"#,
        ),
        (
            "200 OK",
            r#"{"status":"down","service":"loadout","version":"test"}"#,
        ),
        (
            "200 OK",
            r#"{"status":"ok","service":"loadout","version":""}"#,
        ),
        ("302 Found", "secret-token-value"),
        (
            "200 OK",
            r#"{"status":"ok","service":"loadout","version":"test\u001b[2J\nfake"}"#,
        ),
    ] {
        let (url, task) = serve_once(status, body);
        let output = cli(&["doctor", "--server", &url]);
        assert!(!output.status.success(), "accepted {status} {body}");
        assert!(output.stdout.is_empty());
        let error = String::from_utf8_lossy(&output.stderr);
        assert!(!error.is_empty());
        assert!(!error.contains("secret-token-value"));
        task.join().unwrap();
    }
}

#[test]
fn doctor_rejects_invalid_urls_and_reports_connection_failure() {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let offline = format!("http://{}", listener.local_addr().unwrap());
    drop(listener);
    for url in [
        "not-a-url",
        "file:///tmp/test",
        "http://user:secret@localhost",
        "https://example.com/?token=secret",
        "https://example.com/#token",
        "https://example.com/prefix",
        offline.as_str(),
    ] {
        let output = cli(&["doctor", "--server", url]);
        assert!(!output.status.success(), "accepted {url}");
        assert!(!String::from_utf8_lossy(&output.stderr).contains("secret"));
    }
}

#[test]
fn doctor_does_not_follow_redirects() {
    let target = TcpListener::bind("127.0.0.1:0").unwrap();
    target.set_nonblocking(true).unwrap();
    let origin = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}", origin.local_addr().unwrap());
    let location = format!("http://{}", target.local_addr().unwrap());
    let task = thread::spawn(move || {
        origin.set_nonblocking(true).unwrap();
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(5);
        while std::time::Instant::now() < deadline {
            if let Ok((mut stream, _)) = origin.accept() {
                stream.set_nonblocking(false).unwrap();
                stream
                    .set_read_timeout(Some(std::time::Duration::from_secs(2)))
                    .unwrap();
                let mut request = [0; 4096];
                assert!(stream.read(&mut request).unwrap() > 0);
                write!(stream, "HTTP/1.1 302 Found\r\nLocation: {location}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n").unwrap();
                return;
            }
            thread::sleep(std::time::Duration::from_millis(10));
        }
    });
    assert!(!cli(&["doctor", "--server", &url]).status.success());
    task.join().unwrap();
    assert!(matches!(target.accept(), Err(err) if err.kind() == std::io::ErrorKind::WouldBlock));
}

#[test]
fn doctor_times_out_when_server_never_responds() {
    // An open listening socket accepts the TCP handshake but never responds to HTTP.
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let started = std::time::Instant::now();
    let output = cli(&["doctor", "--server", &url]);
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("timed out"));
    assert!(started.elapsed() < std::time::Duration::from_secs(8));
}

#[test]
fn cli_exposes_only_implemented_commands() {
    let output = cli(&["--help"]);
    assert!(output.status.success());
    let help = String::from_utf8_lossy(&output.stdout);
    assert!(help.contains("doctor"));
    for command in [
        "login", "logout", "status", "apply", "remove", "bridge", "version",
    ] {
        assert!(help.contains(command));
    }
    let output = cli(&["--version"]);
    assert!(String::from_utf8_lossy(&output.stdout).contains("loadout 0.1.0"));
    assert!(!cli(&["login"]).status.success());
}
