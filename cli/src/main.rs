use clap::{Parser, Subcommand, ValueEnum};
use loadout::{ClientKind, LocalClient, Result};
use std::{
    io::{self, IsTerminal, Write},
    process::ExitCode,
};

#[derive(Parser)]
#[command(
    name = "loadout",
    version,
    about = "Local companion for the Loadout MCP gateway"
)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}
#[derive(Clone, Copy, ValueEnum)]
enum Client {
    Codex,
    Claude,
    Cursor,
}
impl From<Client> for ClientKind {
    fn from(value: Client) -> Self {
        match value {
            Client::Codex => Self::Codex,
            Client::Claude => Self::Claude,
            Client::Cursor => Self::Cursor,
        }
    }
}
#[derive(Subcommand)]
enum Commands {
    /// Verify a token and save private local credentials. Omit --token to read securely.
    Login {
        #[arg(long, default_value = "http://127.0.0.1:8787")]
        server: String,
        #[arg(long, hide = true)]
        token: Option<String>,
        /// Authorize in your browser using a short-lived device code.
        #[arg(long, conflicts_with = "token")]
        device: bool,
    },
    /// Remove local credentials.
    Logout,
    /// Show account balance and configured clients.
    Status,
    /// Check server, credentials and supported client configuration.
    Doctor {
        #[arg(long)]
        server: Option<String>,
    },
    /// Configure detected clients. Direct mode writes tokens to Claude/Cursor configuration.
    Apply {
        #[arg(long, value_enum, value_delimiter = ',')]
        clients: Vec<Client>,
        #[arg(long)]
        direct: bool,
        #[arg(long, conflicts_with = "direct")]
        remove: bool,
    },
    /// Remove managed Loadout client entries.
    Remove {
        #[arg(long, value_enum, value_delimiter = ',')]
        clients: Vec<Client>,
    },
    /// Run the MCP stdio bridge.
    Bridge,
    /// Print the installed version.
    Version,
}
fn run(command: Commands) -> Result<()> {
    if matches!(command, Commands::Version) {
        println!("loadout {}", env!("CARGO_PKG_VERSION"));
        return Ok(());
    }
    let local = LocalClient::from_env()?;
    match command {
        Commands::Login {
            server,
            token,
            device,
        } => {
            let account = if device {
                local.login_device(&server, |prompt| {
                    println!(
                        "Open {} and verify this code: {}",
                        prompt.verification_uri, prompt.user_code
                    );
                    println!(
                        "Waiting for your explicit approval (expires in {} seconds).",
                        prompt.expires_in
                    );
                    io::stdout()
                        .flush()
                        .map_err(|_| "could not display device authorization")
                })?
            } else {
                let token = match token {
                    Some(token) => token,
                    None => {
                        if io::stdin().is_terminal() {
                            rpassword::prompt_password("Loadout token: ")
                                .map_err(|_| "could not read token")?
                        } else {
                            let mut line = String::new();
                            io::stdin()
                                .read_line(&mut line)
                                .map_err(|_| "could not read token")?;
                            line.trim_end_matches(['\r', '\n']).to_owned()
                        }
                    }
                };
                local.login(&server, &token)?
            };
            println!(
                "Logged in as {} ({} credits)",
                account.username, account.balance
            );
        }
        Commands::Logout => {
            local.logout()?;
            println!("Logged out");
        }
        Commands::Status => {
            let status = local.status()?;
            println!(
                "{}: {} credits",
                status.account.username, status.account.balance
            );
            print_clients(&status.clients);
        }
        Commands::Doctor { server } => {
            let doctor = local.doctor(server.as_deref())?;
            println!("Loadout {} is reachable", doctor.version);
            println!(
                "Credentials: {}",
                if doctor.authenticated {
                    "verified"
                } else {
                    "not logged in for this server"
                }
            );
            print_clients(&doctor.clients);
        }
        Commands::Apply {
            clients,
            direct,
            remove,
        } => {
            let clients: Vec<_> = clients.into_iter().map(Into::into).collect();
            let states = if remove {
                local.remove(&clients)?
            } else {
                local.apply(&clients, direct)?
            };
            print_clients(&states);
            if direct {
                eprintln!(
                    "Direct mode: tokens are stored in Claude/Cursor configuration. Codex requires LOADOUT_TOKEN in its environment."
                );
            }
        }
        Commands::Remove { clients } => {
            print_clients(&local.remove(&clients.into_iter().map(Into::into).collect::<Vec<_>>())?);
        }
        Commands::Bridge => {
            return tokio::runtime::Runtime::new()
                .map_err(|_| "could not start bridge runtime")?
                .block_on(loadout::bridge::run(local.config_path));
        }
        Commands::Version => {}
    }
    Ok(())
}
fn print_clients(clients: &[loadout::ClientState]) {
    for client in clients {
        println!(
            "{:?}: {}",
            client.client,
            if client.configured {
                "configured"
            } else if client.detected {
                "detected; not configured"
            } else {
                "not detected"
            }
        );
    }
}
fn main() -> ExitCode {
    match run(Cli::parse().command) {
        Ok(()) => ExitCode::SUCCESS,
        Err(message) => {
            eprintln!("loadout: {message}");
            ExitCode::FAILURE
        }
    }
}
