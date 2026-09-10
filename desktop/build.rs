fn main() {
    #[cfg(feature = "native")]
    tauri_build::try_build(
        tauri_build::Attributes::new()
            .app_manifest(tauri_build::AppManifest::new().commands(&["local_command"])),
    )
    .expect("failed to build desktop application metadata");
}
