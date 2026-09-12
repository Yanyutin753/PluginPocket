use std::fs;

fn tauri_config() -> serde_json::Value {
    let path = format!("{}/tauri.conf.json", env!("CARGO_MANIFEST_DIR"));
    serde_json::from_str(&fs::read_to_string(path).expect("tauri.conf.json readable"))
        .expect("tauri.conf.json parses")
}

#[test]
fn updater_config_is_release_ready() {
    let config = tauri_config();
    let updater = config
        .pointer("/plugins/updater")
        .expect("plugins.updater configured")
        .as_object()
        .expect("plugins.updater is an object");

    let endpoints = updater
        .get("endpoints")
        .and_then(|value| value.as_array())
        .expect("updater endpoints listed");
    assert!(!endpoints.is_empty(), "updater needs at least one endpoint");
    assert!(endpoints.iter().all(|endpoint| {
        endpoint
            .as_str()
            .is_some_and(|url| url.starts_with("https://"))
    }));

    assert!(
        updater
            .get("pubkey")
            .and_then(|value| value.as_str())
            .is_some_and(|key| !key.trim().is_empty()),
        "updater needs the release minisign public key"
    );

    assert_eq!(
        config.pointer("/plugins/updater/windows/installMode"),
        Some(&serde_json::json!("passive")),
        "windows updates install passively without extra prompts"
    );
}

#[test]
fn tauri_config_version_matches_cargo_manifest() {
    let config = tauri_config();
    let cargo = fs::read_to_string(format!("{}/Cargo.toml", env!("CARGO_MANIFEST_DIR")))
        .expect("Cargo.toml readable");
    let declared = cargo
        .lines()
        .find_map(|line| line.strip_prefix("version = "))
        .expect("Cargo.toml declares its own version");
    assert_eq!(
        config.get("version").and_then(|value| value.as_str()),
        Some(declared.trim_matches('"')),
        "tauri.conf.json version must match the Cargo manifest"
    );
}
