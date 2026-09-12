use crate::{Account, Result};
use reqwest::{Url, blocking::Client, redirect::Policy};
use serde::{Deserialize, Serialize};
use std::{
    fs,
    io::{Read, Write},
    path::Path,
    time::Duration,
};

#[derive(Serialize, Deserialize)]
pub(crate) struct Credentials {
    #[serde(rename = "serverUrl")]
    pub server: String,
    pub token: String,
    pub username: String,
}

pub(crate) fn root_url(server: &str) -> Result<Url> {
    let url = Url::parse(server).map_err(|_| "server must be an HTTP(S) root URL")?;
    if !matches!(url.scheme(), "http" | "https")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
        || url.path() != "/"
    {
        return Err("server must be an HTTP(S) root URL without credentials, query or fragment");
    }
    Ok(url)
}
pub(crate) fn parse_url(raw: &str) -> Result<Url> {
    let url = Url::parse(raw).map_err(|_| "endpoint must be an HTTP(S) URL")?;
    if !matches!(url.scheme(), "http" | "https") || url.host_str().is_none() {
        return Err("endpoint must be an HTTP(S) URL");
    }
    Ok(url)
}
pub(crate) fn market(server: &str, token: &str) -> Result<Vec<crate::MarketItem>> {
    let mut url = root_url(server)?;
    url.set_path("/api/v1/marketplace");
    let response = http()?
        .get(url)
        .bearer_auth(token)
        .send()
        .map_err(|error| {
            if error.is_timeout() {
                "connection timed out; check the server and retry"
            } else {
                "could not connect; check the server and retry"
            }
        })?;
    if !response.status().is_success() {
        return Err("marketplace request failed; check the server and token");
    }
    #[derive(Deserialize)]
    struct Payload {
        items: Vec<crate::MarketItem>,
    }
    let payload: Payload = response
        .json()
        .map_err(|_| "server returned an invalid marketplace response")?;
    if payload.items.len() > 500
        || payload
            .items
            .iter()
            .any(|item| item.slug.is_empty() || item.slug.chars().any(char::is_control))
    {
        return Err("server returned an invalid marketplace response");
    }
    Ok(payload.items)
}
pub(crate) fn skill_files(
    server: &str,
    token: &str,
    slug: &str,
) -> Result<std::collections::BTreeMap<String, crate::plugins::SkillFile>> {
    let mut url = root_url(server)?;
    url.set_path(&format!("/api/v1/marketplace/{slug}/files"));
    url.set_query(Some("format=2"));
    let response = http()?
        .get(url)
        .timeout(Duration::from_secs(60))
        .bearer_auth(token)
        .send()
        .map_err(|_| "could not connect; check the server and retry")?;
    if !response.status().is_success() {
        return Err("skill files request failed; check the server and token");
    }
    #[derive(Deserialize)]
    struct Payload {
        files: std::collections::BTreeMap<String, crate::plugins::SkillFile>,
    }
    const MAX_RESPONSE: u64 = 45 * 1024 * 1024;
    if response
        .content_length()
        .is_some_and(|size| size > MAX_RESPONSE)
    {
        return Err("skill files response exceeds size limit");
    }
    let mut bytes = Vec::new();
    response
        .take(MAX_RESPONSE + 1)
        .read_to_end(&mut bytes)
        .map_err(|_| "could not read skill files response")?;
    if bytes.len() as u64 > MAX_RESPONSE {
        return Err("skill files response exceeds size limit");
    }
    let payload: Payload = serde_json::from_slice(&bytes)
        .map_err(|_| "server returned an invalid skill files response")?;
    Ok(payload.files)
}
pub(crate) fn usage(server: &str, token: &str) -> Result<crate::UsagePage> {
    let mut url = root_url(server)?;
    url.set_path("/api/v1/account/usage");
    url.set_query(Some("limit=100"));
    let response = http()?
        .get(url)
        .bearer_auth(token)
        .send()
        .map_err(connection_error)?;
    if !response.status().is_success() {
        return Err("usage request failed; check the server and token");
    }
    response
        .json()
        .map_err(|_| "server returned an invalid usage response")
}
pub(crate) fn ledger(server: &str, token: &str) -> Result<crate::LedgerPage> {
    let mut url = root_url(server)?;
    url.set_path("/api/v1/account/ledger");
    let response = http()?
        .get(url)
        .bearer_auth(token)
        .send()
        .map_err(connection_error)?;
    if !response.status().is_success() {
        return Err("ledger request failed; check the server and token");
    }
    response
        .json()
        .map_err(|_| "server returned an invalid ledger response")
}
pub(crate) fn validate_token(token: &str) -> Result<()> {
    if !token.starts_with("ppt_")
        || token.len() <= 4
        || token.len() > 4096
        || token.chars().any(|c| c.is_control() || c.is_whitespace())
    {
        return Err("invalid token; paste a PluginPocket token");
    }
    Ok(())
}
pub(crate) fn http() -> Result<Client> {
    Client::builder()
        .timeout(Duration::from_secs(5))
        .redirect(Policy::none())
        .retry(reqwest::retry::never())
        .build()
        .map_err(|_| "could not initialize HTTP client")
}
fn connection_error(error: reqwest::Error) -> &'static str {
    if error.is_timeout() {
        "connection timed out; check the server and retry"
    } else {
        "could not connect; check the server and retry"
    }
}
pub(crate) fn verify(server: &str, token: &str) -> Result<Account> {
    validate_token(token)?;
    let mut url = root_url(server)?;
    url.set_path("/api/v1/account/verify");
    let response = http()?
        .get(url)
        .bearer_auth(token)
        .send()
        .map_err(connection_error)?;
    if !response.status().is_success() {
        return Err("token verification failed; check the server and token");
    }
    let account: Account = response
        .json()
        .map_err(|_| "server returned an invalid account response")?;
    if account.username.trim().is_empty()
        || account.username.chars().any(char::is_control)
        || account.balance < 0
    {
        return Err("server returned an invalid account response");
    }
    Ok(account)
}
pub(crate) fn health(server: &str) -> Result<String> {
    #[derive(Deserialize)]
    struct Health {
        status: String,
        service: String,
        version: String,
    }
    let mut url = root_url(server)?;
    url.set_path("/api/v1/health");
    let response = http()?.get(url).send().map_err(connection_error)?;
    if !response.status().is_success() {
        return Err("server returned an unsuccessful HTTP status");
    }
    let health: Health = response.json().map_err(|error| {
        if error.is_timeout() {
            "connection timed out; check the server and retry"
        } else {
            "server returned an invalid health response"
        }
    })?;
    if health.status != "ok"
        || health.service != "pluginpocket"
        || health.version.trim().is_empty()
        || health.version.chars().any(char::is_control)
    {
        return Err("response is not a healthy PluginPocket service");
    }
    Ok(health.version)
}
// Reject symlinks at every component, including dangling links, before reading or writing.
pub(crate) fn safe_path(path: &Path) -> Result<()> {
    for part in path.ancestors() {
        match fs::symlink_metadata(part) {
            Ok(meta) if meta.file_type().is_symlink() => {
                return Err("refusing a symbolic link in configuration path");
            }
            Ok(_) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(_) => return Err("could not inspect configuration path"),
        }
    }
    Ok(())
}
pub(crate) fn read(path: &Path) -> Result<Option<Vec<u8>>> {
    safe_path(path)?;
    let file = match fs::File::open(path) {
        Ok(file) => file,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(_) => return Err("could not read configuration"),
    };
    if !file
        .metadata()
        .map_err(|_| "could not inspect configuration")?
        .is_file()
    {
        return Err("configuration must be a regular file");
    }
    let mut bytes = Vec::new();
    file.take(4 * 1024 * 1024 + 1)
        .read_to_end(&mut bytes)
        .map_err(|_| "could not read configuration")?;
    if bytes.len() > 4 * 1024 * 1024 {
        return Err("configuration exceeds size limit");
    }
    Ok(Some(bytes))
}
pub(crate) fn load(path: &Path) -> Result<Credentials> {
    let credentials: Credentials =
        serde_json::from_slice(&read(path)?.ok_or("not logged in; run pluginpocket login")?)
            .map_err(|_| "invalid local credentials; log in again")?;
    root_url(&credentials.server)?;
    validate_token(&credentials.token)?;
    Ok(credentials)
}
pub(crate) fn atomic_write(path: &Path, bytes: &[u8]) -> Result<()> {
    atomic_write_inner(path, bytes, true)
}
pub(crate) fn atomic_write_new(path: &Path, bytes: &[u8]) -> Result<()> {
    atomic_write_inner(path, bytes, false)
}
fn atomic_write_inner(path: &Path, bytes: &[u8], replace: bool) -> Result<()> {
    safe_path(path)?;
    let parent = path
        .parent()
        .ok_or("configuration path must have a parent")?;
    fs::create_dir_all(parent).map_err(|_| "could not create configuration directory")?;
    safe_path(path)?;
    let mut file = tempfile::NamedTempFile::new_in(parent)
        .map_err(|_| "could not create private temporary file")?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        file.as_file()
            .set_permissions(fs::Permissions::from_mode(0o600))
            .map_err(|_| "could not set private file permissions")?;
    }
    file.write_all(bytes)
        .map_err(|_| "could not write configuration")?;
    file.as_file()
        .sync_all()
        .map_err(|_| "could not sync configuration")?;
    safe_path(path)?;
    if replace {
        file.persist(path)
    } else {
        file.persist_noclobber(path)
    }
    .map_err(|_| "could not save local file atomically")?;
    Ok(())
}
