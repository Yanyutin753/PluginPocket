use loadout::LocalClient;
use loadout_desktop::{LocalCommand, execute};
use std::{process::ExitCode, sync::Arc};
use tauri::{
    Manager, WebviewWindowBuilder,
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
};

#[tauri::command]
async fn local_command(
    command: LocalCommand,
    local: tauri::State<'_, Arc<LocalClient>>,
) -> Result<serde_json::Value, &'static str> {
    let local = Arc::clone(local.inner());
    tauri::async_runtime::spawn_blocking(move || execute(&local, command))
        .await
        .map_err(|_| "local command could not complete")?
}
fn start() -> Result<(), Box<dyn std::error::Error>> {
    let local = Arc::new(LocalClient::from_env()?);
    tauri::Builder::default()
        .manage(local)
        .invoke_handler(tauri::generate_handler![local_command])
        .setup(|app| {
            let config = app
                .config()
                .app
                .windows
                .first()
                .ok_or("main window configuration is missing")?;
            WebviewWindowBuilder::from_config(app, config)?
                .on_navigation(|url| {
                    (url.scheme() == "tauri" && url.host_str() == Some("localhost"))
                        || (url.scheme() == "http" && url.host_str() == Some("tauri.localhost"))
                        || (cfg!(debug_assertions)
                            && url.scheme() == "http"
                            && url.host_str() == Some("127.0.0.1")
                            && url.port() == Some(1420))
                })
                .on_new_window(|_, _| tauri::webview::NewWindowResponse::Deny)
                .build()?;
            let show = MenuItem::with_id(app, "show", "显示 Loadout", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &quit])?;
            let icon = app
                .default_window_icon()
                .cloned()
                .ok_or("application icon is missing")?;
            TrayIconBuilder::new()
                .icon(icon)
                .tooltip("Loadout 本地接入")
                .menu(&menu)
                .show_menu_on_left_click(true)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "show" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                    "quit" => app.exit(0),
                    _ => {}
                })
                .build(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event
                && window.hide().is_ok()
            {
                api.prevent_close();
            }
        })
        .run(tauri::generate_context!())?;
    Ok(())
}
fn main() -> ExitCode {
    let args: Vec<_> = std::env::args_os().skip(1).collect();
    if args.len() == 1 && args[0] == "bridge" {
        let result = LocalClient::from_env().and_then(|local| {
            tokio::runtime::Runtime::new()
                .map_err(|_| "could not start bridge runtime")?
                .block_on(loadout::bridge::run(local.config_path))
        });
        return match result {
            Ok(()) => ExitCode::SUCCESS,
            Err(message) => {
                eprintln!("loadout: {message}");
                ExitCode::FAILURE
            }
        };
    }
    if !args.is_empty() {
        eprintln!("loadout: unsupported desktop arguments");
        return ExitCode::FAILURE;
    }
    if start().is_err() {
        eprintln!("loadout: desktop could not start; check the local desktop environment");
        return ExitCode::FAILURE;
    }
    ExitCode::SUCCESS
}
