#![cfg(feature = "native")]
use rmcp::{ServiceExt, model::CallToolRequestParams};
use std::{process::Stdio, time::Duration};
use tokio::process::Command;
#[tokio::test]
async fn installed_desktop_binary_runs_bridge_without_gui_and_exits_on_stdin_close() {
    tokio::time::timeout(Duration::from_secs(8), async {
        let dir = tempfile::tempdir().unwrap();
        let executable = std::env::var_os("PLUGINPOCKET_DESKTOP_TEST_BIN")
            .unwrap_or_else(|| env!("CARGO_BIN_EXE_pluginpocket-desktop").into());
        let mut child = Command::new(executable)
            .arg("bridge")
            .env("HOME", dir.path())
            .env("USERPROFILE", dir.path())
            .env_remove("PLUGINPOCKET_CONFIG")
            .env_remove("DISPLAY")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .kill_on_drop(true)
            .spawn()
            .unwrap();
        let service = ().serve((child.stdout.take().unwrap(), child.stdin.take().unwrap())).await;
        assert!(
            service.is_ok(),
            "desktop executable must serve bridge without initializing a GUI"
        );
        let mut service = service.unwrap();
        let result = service
            .call_tool(CallToolRequestParams::new("echo"))
            .await
            .unwrap();
        assert_eq!(result.is_error, Some(true));
        assert!(
            result.content[0]
                .as_text()
                .unwrap()
                .text
                .contains("not logged in")
        );
        service.close().await.unwrap();
        assert!(child.wait().await.unwrap().success());
    })
    .await
    .expect("native bridge watchdog");
}
