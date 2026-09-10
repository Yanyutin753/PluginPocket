#![allow(dead_code)]
use std::{
    io::{Read, Write},
    net::TcpListener,
    process::{Command, Output, Stdio},
    thread,
    time::{Duration, Instant},
};
pub fn fixture(status: &str, body: &str) -> (String, thread::JoinHandle<String>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let response = format!(
        "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    );
    let handle = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(3);
        loop {
            match listener.accept() {
                Ok((mut stream, _)) => {
                    stream
                        .set_read_timeout(Some(Duration::from_secs(2)))
                        .unwrap();
                    let mut b = [0; 8192];
                    let n = stream.read(&mut b).unwrap();
                    stream.write_all(response.as_bytes()).unwrap();
                    return String::from_utf8_lossy(&b[..n]).into_owned();
                }
                Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                    if Instant::now() > deadline {
                        return String::new();
                    }
                    thread::sleep(Duration::from_millis(10));
                }
                Err(e) => panic!("{e}"),
            }
        }
    });
    (url, handle)
}
pub fn local(dir: &tempfile::TempDir) -> loadout::LocalClient {
    loadout::LocalClient::new(
        dir.path().to_path_buf(),
        dir.path().join(".loadout/config.json"),
        std::env::current_exe().unwrap(),
    )
    .unwrap()
}
pub fn credentials(local: &loadout::LocalClient, server: &str, token: &str) {
    std::fs::create_dir_all(local.config_path.parent().unwrap()).unwrap();
    std::fs::write(
        &local.config_path,
        serde_json::to_vec(
            &serde_json::json!({"serverUrl":server,"token":token,"username":"alice"}),
        )
        .unwrap(),
    )
    .unwrap();
}
pub fn cli(dir: &tempfile::TempDir, args: &[&str], input: &str) -> Output {
    let mut child = Command::new(env!("CARGO_BIN_EXE_loadout"))
        .args(args)
        .env("HOME", dir.path())
        .env("USERPROFILE", dir.path())
        .env_remove("LOADOUT_CONFIG")
        .env("NO_PROXY", "*")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    child
        .stdin
        .take()
        .unwrap()
        .write_all(input.as_bytes())
        .unwrap();
    let deadline = Instant::now() + Duration::from_secs(12);
    while child.try_wait().unwrap().is_none() {
        if Instant::now() > deadline {
            child.kill().unwrap();
            child.wait().unwrap();
            panic!("CLI watchdog")
        }
        thread::sleep(Duration::from_millis(10));
    }
    child.wait_with_output().unwrap()
}
