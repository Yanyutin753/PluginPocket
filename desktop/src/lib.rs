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
    };
    value.map_err(|_| "could not encode local command result")
}
