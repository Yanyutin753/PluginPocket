mod common;
use base64::{Engine as _, engine::general_purpose::STANDARD};
use common::*;
use serde_json::{Value, json};
use std::{
    fs,
    io::{Read, Write},
    net::TcpListener,
    thread,
    time::Duration,
};

fn file(bytes: &[u8], executable: bool) -> Value {
    let hash: String = ring::digest::digest(&ring::digest::SHA256, bytes)
        .as_ref()
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect();
    json!({"encoding":"base64","content":STANDARD.encode(bytes),"sha256":hash,"size":bytes.len(),"executable":executable})
}

fn install(
    files: Value,
    bundle: bool,
) -> (tempfile::TempDir, pluginpocket::Result<()>, Vec<String>) {
    install_with_setup(files, bundle, |_| {})
}
fn install_with_setup(
    files: Value,
    bundle: bool,
    setup: impl FnOnce(&std::path::Path),
) -> (tempfile::TempDir, pluginpocket::Result<()>, Vec<String>) {
    install_with_delay(files, bundle, setup, Duration::ZERO)
}
fn install_with_delay(
    files: Value,
    bundle: bool,
    setup: impl FnOnce(&std::path::Path),
    delay: Duration,
) -> (tempfile::TempDir, pluginpocket::Result<()>, Vec<String>) {
    let dir = tempfile::tempdir().unwrap();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let server = format!("http://{}", listener.local_addr().unwrap());
    let mut items = vec![
        json!({"slug":"binary","name":"Binary","kind":"skill","spec":{"source":"inline","files":{"SKILL.md":"stale"},"file_manifest":{"SKILL.md":{"sha256":"unused","size":0,"executable":false}}}}),
    ];
    if bundle {
        items.push(
            json!({"slug":"kit","name":"Kit","kind":"bundle","spec":{"includes":["binary"]}}),
        );
    }
    let task = thread::spawn(move || {
        let mut requests = Vec::new();
        for body in [json!({"items":items}), json!({"files":files})] {
            let start = std::time::Instant::now();
            let mut stream = loop {
                match listener.accept() {
                    Ok((s, _)) => break s,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        if start.elapsed() > Duration::from_secs(2) {
                            return requests;
                        }
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(e) => panic!("{e}"),
                }
            };
            // Windows 上 accept 出的连接继承监听 socket 的非阻塞模式，必须切回阻塞；
            // 大请求体（~11MB base64 附件）必须完整读取后再写响应，否则客户端仍在
            // 发送时收到响应与关闭会被 RST，导致读响应失败（macOS CI 实测）。
            stream.set_nonblocking(false).unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(10)))
                .unwrap();
            let mut bytes = Vec::new();
            let mut buf = [0; 8192];
            loop {
                let n = stream.read(&mut buf).unwrap();
                assert!(n > 0);
                bytes.extend_from_slice(&buf[..n]);
                if let Some(i) = bytes.windows(4).position(|w| w == b"\r\n\r\n") {
                    let headers = String::from_utf8_lossy(&bytes[..i]).to_ascii_lowercase();
                    let length = headers
                        .lines()
                        .find_map(|l| {
                            l.strip_prefix("content-length: ")
                                .and_then(|v| v.parse::<usize>().ok())
                        })
                        .unwrap_or(0);
                    if bytes.len() >= i + 4 + length {
                        break;
                    }
                }
            }
            requests.push(String::from_utf8_lossy(&bytes).into_owned());
            if requests.len() == 2 {
                thread::sleep(delay);
            }
            let body = body.to_string();
            let _ = write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                body.len(),
                body
            );
        }
        requests
    });
    let local = local(&dir);
    credentials(&local, &server, "ppt_secret");
    setup(dir.path());
    let result = local.install(
        if bundle { "kit" } else { "binary" },
        None,
        &[pluginpocket::ClientKind::Codex],
    );
    (dir, result, task.join().unwrap())
}

#[test]
fn slow_file_download_is_allowed_beyond_metadata_timeout() {
    let (dir, result, _) = install_with_delay(
        json!({"SKILL.md":file(b"slow download", false)}),
        false,
        |_| {},
        Duration::from_secs(6),
    );
    result.unwrap();
    assert_eq!(
        fs::read(dir.path().join(".codex/skills/binary/SKILL.md")).unwrap(),
        b"slow download"
    );
}

#[test]
fn nonportable_file_names_and_case_collisions_are_rejected_before_writes() {
    let valid = file(b"content", false);
    for names in [
        vec!["A", "a"],
        vec!["A", "a/b"],
        vec!["a/B", "A/b"],
        vec!["trailing."],
        vec!["trailing "],
        vec!["folder./file"],
        vec!["CON"],
        vec!["prn.txt"],
        vec!["Aux.dat"],
        vec!["nul"],
        vec!["com1"],
        vec!["COM9.exe"],
        vec!["lpt1.txt"],
        vec!["LPT9"],
        vec!["a<b"],
        vec!["a>b"],
        vec!["a\"b"],
        vec!["a|b"],
        vec!["a?b"],
        vec!["a*b"],
    ] {
        let mut files = serde_json::Map::new();
        files.insert("SKILL.md".to_owned(), valid.clone());
        for name in &names {
            files.insert((*name).to_owned(), valid.clone());
        }
        let (dir, result, _) = install(Value::Object(files), false);
        assert!(result.is_err(), "accepted nonportable names {names:?}");
        assert!(!dir.path().join(".codex/skills/binary").exists());
    }
}

#[cfg(unix)]
#[test]
fn all_destination_paths_are_checked_before_writing() {
    let (dir, result, _) = install_with_setup(
        json!({"SKILL.md":file(b"new",false),"z":file(b"payload",false)}),
        false,
        |home| {
            let root = home.join(".codex/skills/binary");
            fs::create_dir_all(&root).unwrap();
            fs::write(root.join("SKILL.md"), "old").unwrap();
            fs::write(home.join("outside"), "secret").unwrap();
            std::os::unix::fs::symlink(home.join("outside"), root.join("z")).unwrap();
            fs::write(
                home.join(".pluginpocket/managed-clients.json"),
                json!({"skill:codex:binary":["SKILL.md","z"]}).to_string(),
            )
            .unwrap();
        },
    );
    assert!(result.is_err());
    assert_eq!(
        fs::read_to_string(dir.path().join(".codex/skills/binary/SKILL.md")).unwrap(),
        "old"
    );
    assert_eq!(
        fs::read_to_string(dir.path().join("outside")).unwrap(),
        "secret"
    );
}

#[test]
fn binary_skill_and_bundle_install_exact_bytes_and_modes() {
    for bundle in [false, true] {
        let (dir, result, requests) = install(
            json!({"SKILL.md":file(b"instructions",false),"assets/raw.bin":file(b"\0\xff\xfe",false),"scripts/run":file(b"#!/bin/sh\nexit 99\n",true)}),
            bundle,
        );
        result.unwrap();
        let root = dir.path().join(".codex/skills/binary");
        assert_eq!(fs::read(root.join("SKILL.md")).unwrap(), b"instructions");
        assert_eq!(
            fs::read(root.join("assets/raw.bin")).unwrap(),
            b"\0\xff\xfe"
        );
        assert!(requests[1].starts_with("GET /api/v1/marketplace/binary/files?format=2 "));
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                fs::metadata(root.join("assets/raw.bin"))
                    .unwrap()
                    .permissions()
                    .mode()
                    & 0o777,
                0o644
            );
            assert_eq!(
                fs::metadata(root.join("scripts/run"))
                    .unwrap()
                    .permissions()
                    .mode()
                    & 0o777,
                0o755
            );
        }
    }
}

#[test]
fn invalid_files_rejected_before_any_write() {
    let valid = file(b"abc", false);
    let mut bad_hash = valid.clone();
    bad_hash["sha256"] = json!("0".repeat(64));
    let mut bad_size = valid.clone();
    bad_size["size"] = json!(9);
    let mut bad_base64 = valid.clone();
    bad_base64["content"] = json!("***");
    for files in [
        json!({"SKILL.md":valid,"z":bad_hash}),
        json!({"SKILL.md":valid,"z":bad_size}),
        json!({"SKILL.md":valid,"z":bad_base64}),
        json!({"SKILL.md":file(b"\xff",false)}),
        json!({"SKILL.md":valid,"a":valid,"a/b":valid}),
        json!({"SKILL.md":valid,"C:evil":valid}),
        json!({"SKILL.md":valid,"a/./b":valid}),
    ] {
        let (dir, result, _) = install(files, false);
        assert!(result.is_err(), "invalid files accepted");
        assert!(
            !dir.path().join(".codex/skills/binary").exists(),
            "partial install"
        );
    }
}

#[test]
fn old_server_text_files_remain_compatible() {
    let (dir, result, _) = install(json!({"SKILL.md":"legacy", "refs.txt":"文本"}), false);
    result.unwrap();
    assert_eq!(
        fs::read_to_string(dir.path().join(".codex/skills/binary/SKILL.md")).unwrap(),
        "legacy"
    );
}

#[test]
fn binary_skill_accepts_large_attachments_and_enforces_limits() {
    let attachment = vec![0xff; 8 * 1024 * 1024];
    let encoded = file(&attachment, false);
    let (dir, result, _) = install(
        json!({"SKILL.md":file(b"",false),"large.bin":encoded}),
        false,
    );
    result.unwrap();
    assert_eq!(
        fs::read(dir.path().join(".codex/skills/binary/large.bin")).unwrap(),
        attachment
    );
    let mut too_large = encoded.clone();
    too_large["size"] = json!(8 * 1024 * 1024 + 1);
    let many: serde_json::Map<String, Value> = (0..32)
        .map(|i| (format!("f{i}"), file(b"", false)))
        .chain(std::iter::once(("SKILL.md".to_owned(), file(b"", false))))
        .collect();
    for files in [
        json!({"SKILL.md":file(b"",false),"large.bin":too_large}),
        json!({"SKILL.md":file(b"x",false),"a":encoded,"b":encoded,"c":encoded,"d":encoded}),
        Value::Object(many),
    ] {
        let (dir, result, _) = install(files, false);
        assert!(result.is_err());
        assert!(!dir.path().join(".codex/skills/binary").exists());
    }
}
