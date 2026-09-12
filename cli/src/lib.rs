pub mod bridge;
pub(crate) mod clients;
mod config;
mod device;
mod workbench;
pub use workbench::{
    DiagnosticCheck, Diagnostics, InstalledItem, InstalledKind, LogEntry, LogExport,
};
pub mod plugins;
pub use device::DevicePrompt;
pub use plugins::MarketItem;
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

pub type Result<T> = std::result::Result<T, &'static str>;
#[derive(Clone, Copy, Debug, Deserialize, Serialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "lowercase")]
pub enum ClientKind {
    Codex,
    Claude,
    Cursor,
}
#[derive(Debug, Serialize, Deserialize)]
pub struct Account {
    pub username: String,
    pub balance: i64,
    pub tools: Vec<serde_json::Value>,
}
#[derive(Debug, Serialize)]
pub struct ClientState {
    pub client: ClientKind,
    pub detected: bool,
    pub configured: bool,
}
#[derive(Debug, Serialize)]
pub struct Status {
    pub account: Account,
    pub clients: Vec<ClientState>,
}
#[derive(Debug, Serialize)]
pub struct Doctor {
    pub version: String,
    pub authenticated: bool,
    pub clients: Vec<ClientState>,
}
pub struct LocalClient {
    pub home: PathBuf,
    pub config_path: PathBuf,
    pub executable: PathBuf,
    log_write_failed: std::sync::atomic::AtomicBool,
}
impl LocalClient {
    pub fn new(home: PathBuf, config_path: PathBuf, executable: PathBuf) -> Result<Self> {
        if !home.is_absolute() || !config_path.is_absolute() || !executable.is_absolute() {
            return Err("local paths must be absolute");
        }
        let canonical_home = home
            .canonicalize()
            .map_err(|_| "could not resolve home directory")?;
        let config_path = config_path
            .strip_prefix(&home)
            .map(|p| canonical_home.join(p))
            .unwrap_or(config_path.clone());
        Ok(Self {
            home: canonical_home,
            config_path,
            executable,
            log_write_failed: std::sync::atomic::AtomicBool::new(false),
        })
    }
    pub fn from_env() -> Result<Self> {
        let home = std::env::var_os(if cfg!(windows) { "USERPROFILE" } else { "HOME" })
            .map(PathBuf::from)
            .ok_or("home directory is unavailable")?;
        let config_path = std::env::var_os("PLUGINPOCKET_CONFIG")
            .map(PathBuf::from)
            .unwrap_or_else(|| home.join(".pluginpocket/config.json"));
        Self::new(
            home,
            config_path,
            std::env::current_exe().map_err(|_| "could not resolve executable")?,
        )
    }
    pub fn login(&self, server: &str, token: &str) -> Result<Account> {
        let result = self.login_inner(server, token);
        self.record_operation("login", &result);
        result
    }
    fn login_inner(&self, server: &str, token: &str) -> Result<Account> {
        config::safe_path(&self.config_path)?;
        let account = config::verify(server, token)?;
        let credentials = config::Credentials {
            server: config::root_url(server)?.to_string(),
            token: token.to_owned(),
            username: account.username.clone(),
        };
        let bytes =
            serde_json::to_vec_pretty(&credentials).map_err(|_| "could not encode credentials")?;
        config::atomic_write(&self.config_path, &bytes)?;
        Ok(account)
    }
    pub fn logout(&self) -> Result<()> {
        let result = self.logout_inner();
        self.record_operation("logout", &result);
        result
    }
    fn logout_inner(&self) -> Result<()> {
        config::safe_path(&self.config_path)?;
        match std::fs::remove_file(&self.config_path) {
            Ok(()) => Ok(()),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(_) => Err("could not remove credentials"),
        }
    }
    pub fn status(&self) -> Result<Status> {
        let credentials = config::load(&self.config_path)?;
        Ok(Status {
            account: config::verify(&credentials.server, &credentials.token)?,
            clients: self.client_states()?,
        })
    }
    pub fn doctor(&self, server: Option<&str>) -> Result<Doctor> {
        let credentials = if config::read(&self.config_path)?.is_some() {
            Some(config::load(&self.config_path)?)
        } else {
            None
        };
        let server = server
            .or_else(|| credentials.as_ref().map(|c| c.server.as_str()))
            .unwrap_or("http://127.0.0.1:8787");
        let version = config::health(server)?;
        let authenticated = if let Some(credentials) = &credentials {
            if config::root_url(server)? == config::root_url(&credentials.server)? {
                config::verify(server, &credentials.token)?;
                true
            } else {
                false
            }
        } else {
            false
        };
        Ok(Doctor {
            version,
            authenticated,
            clients: self.client_states()?,
        })
    }
    pub fn apply(&self, clients: &[ClientKind], direct: bool) -> Result<Vec<ClientState>> {
        let result = self.write_clients(clients, direct, false);
        self.record_operation("apply", &result);
        result
    }
    pub fn remove(&self, clients: &[ClientKind]) -> Result<Vec<ClientState>> {
        let result = self.write_clients(clients, false, true);
        self.record_operation("remove", &result);
        result
    }
}
