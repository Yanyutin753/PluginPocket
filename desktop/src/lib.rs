use pluginpocket::LocalClient;
use serde::Deserialize;

#[derive(Deserialize)]
#[serde(tag = "action", rename_all = "snake_case", deny_unknown_fields)]
pub enum LocalCommand {
    Login {
        server: String,
        token: String,
    },
    Logout {},
    Status {},
    Clients {},
    Doctor {
        server: Option<String>,
    },
    Apply {
        clients: Vec<pluginpocket::ClientKind>,
    },
    Remove {
        clients: Vec<pluginpocket::ClientKind>,
    },
    Installed {},
    Logs {},
    Usage {},
    Ledger {},
    ExportLogs {
        entries: Vec<pluginpocket::LogEntry>,
    },
    Diagnostics {
        server: Option<String>,
    },
    UninstallInstalled {
        slug: String,
        kind: pluginpocket::InstalledKind,
        clients: Vec<pluginpocket::ClientKind>,
    },
    UpdateInstalled {
        slug: String,
        kind: pluginpocket::InstalledKind,
        clients: Vec<pluginpocket::ClientKind>,
    },
}

pub fn execute(
    local: &LocalClient,
    command: LocalCommand,
) -> pluginpocket::Result<serde_json::Value> {
    let value = match command {
        LocalCommand::Login { server, token } => {
            serde_json::to_value(local.login(&server, &token)?)
        }
        LocalCommand::Logout {} => {
            local.logout()?;
            Ok(serde_json::Value::Null)
        }
        LocalCommand::Status {} => serde_json::to_value(local.status()?),
        LocalCommand::Clients {} => serde_json::to_value(local.client_states()?),
        LocalCommand::Doctor { server } => serde_json::to_value(local.doctor(server.as_deref())?),
        LocalCommand::Apply { clients } => serde_json::to_value(local.apply(&clients, false)?),
        LocalCommand::Remove { clients } => serde_json::to_value(local.remove(&clients)?),
        LocalCommand::Installed {} => serde_json::to_value(local.installed()?),
        LocalCommand::Logs {} => serde_json::to_value(local.logs()?),
        LocalCommand::Usage {} => serde_json::to_value(local.usage()?),
        LocalCommand::Ledger {} => serde_json::to_value(local.ledger()?),
        LocalCommand::ExportLogs { entries } => serde_json::to_value(local.export_logs(&entries)?),
        LocalCommand::Diagnostics { server } => {
            serde_json::to_value(local.diagnostics(server.as_deref()))
        }
        LocalCommand::UninstallInstalled {
            slug,
            kind,
            clients,
        } => {
            local.uninstall_installed(&slug, kind, &clients)?;
            Ok(serde_json::Value::Null)
        }
        LocalCommand::UpdateInstalled {
            slug,
            kind,
            clients,
        } => {
            local.update_installed(&slug, kind, &clients)?;
            Ok(serde_json::Value::Null)
        }
    };
    value.map_err(|_| "could not encode local command result")
}
