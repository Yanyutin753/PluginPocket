use crate::{ClientKind, ClientState, LocalClient, Result, config, plugins::heal_polluted_block};
use serde_json::{Map, Value, json};
use std::{collections::HashSet, path::PathBuf};
use toml_edit::{DocumentMut, Item, Table, value};
const BEGIN: &str = "# --- pluginpocket begin ---";
const END: &str = "# --- pluginpocket end ---";
const ALL: [ClientKind; 3] = [ClientKind::Codex, ClientKind::Claude, ClientKind::Cursor];

impl ClientKind {
    pub(crate) fn name(self) -> &'static str {
        match self {
            Self::Codex => "codex",
            Self::Claude => "claude",
            Self::Cursor => "cursor",
        }
    }
}
impl LocalClient {
    pub(crate) fn client_path(&self, kind: ClientKind) -> PathBuf {
        self.home.join(match kind {
            ClientKind::Codex => ".codex/config.toml",
            ClientKind::Claude => ".claude.json",
            ClientKind::Cursor => ".cursor/mcp.json",
        })
    }
    pub(crate) fn manifest_path(&self) -> PathBuf {
        self.home.join(".pluginpocket/managed-clients.json")
    }
    pub(crate) fn manifest(&self) -> Result<Map<String, Value>> {
        match config::read(&self.manifest_path())? {
            Some(bytes) => {
                serde_json::from_slice(&bytes).map_err(|_| "invalid client management record")
            }
            None => Ok(Map::new()),
        }
    }
    pub fn client_states(&self) -> Result<Vec<ClientState>> {
        self.client_states_for(&ALL)
    }
    pub(crate) fn client_states_for(&self, selected: &[ClientKind]) -> Result<Vec<ClientState>> {
        let manifest = self.manifest()?;
        ALL.iter()
            .filter(|client| selected.contains(*client))
            .map(|&client| {
                let path = self.client_path(client);
                let bytes = config::read(&path)?;
                let detected = bytes.is_some()
                    || (client != ClientKind::Claude && path.parent().is_some_and(|p| p.is_dir()));
                let configured = match bytes {
                    None => false,
                    Some(bytes) => {
                        if client == ClientKind::Codex {
                            let text = std::str::from_utf8(&bytes)
                                .map_err(|_| "invalid client configuration encoding")?;
                            let text = heal_polluted_block(text, BEGIN, END);
                            let (_, managed) = toml_without_managed(&text)?;
                            managed.is_some_and(|block| {
                                owns(&manifest, client.name(), &Value::String(block))
                            })
                        } else {
                            let document: Value = serde_json::from_slice(&bytes)
                                .map_err(|_| "invalid client JSON configuration")?;
                            document
                                .get("mcpServers")
                                .and_then(|v| v.get("pluginpocket"))
                                .is_some_and(|entry| owns(&manifest, client.name(), entry))
                        }
                    }
                };
                Ok(ClientState {
                    client,
                    detected,
                    configured,
                })
            })
            .collect()
    }
    pub(crate) fn write_clients(
        &self,
        clients: &[ClientKind],
        direct: bool,
        remove: bool,
    ) -> Result<Vec<ClientState>> {
        let credentials = if remove {
            None
        } else {
            Some(config::load(&self.config_path)?)
        };
        let selected: Vec<ClientKind> = if clients.is_empty() {
            self.client_states()?
                .into_iter()
                .filter(|c| c.detected)
                .map(|c| c.client)
                .collect()
        } else {
            clients.to_vec()
        };
        if selected.is_empty() {
            return Err("no supported clients detected; specify --clients codex,claude,cursor");
        }
        let previous_manifest = self.manifest()?;
        let mut manifest = previous_manifest.clone();
        let mut updates = Vec::new();
        let mut seen = HashSet::new();
        for client in selected.iter().copied().filter(|c| seen.insert(c.name())) {
            let path = self.client_path(client);
            let old = config::read(&path)?;
            let backup = PathBuf::from(format!("{}.pluginpocket.bak", path.display()));
            config::safe_path(&backup)?;
            let entry = if let Some(credentials) = &credentials {
                self.entry(client, direct, credentials)?
            } else {
                Value::Null
            };
            let new = if client == ClientKind::Codex {
                let raw = std::str::from_utf8(old.as_deref().unwrap_or_default())
                    .map_err(|_| "invalid client configuration encoding")?;
                let text = heal_polluted_block(raw, BEGIN, END);
                let (remaining, existing) = toml_without_managed(&text)?;
                if existing
                    .is_some_and(|block| !owns(&manifest, client.name(), &Value::String(block)))
                {
                    return Err(
                        "existing pluginpocket configuration is unmanaged or modified; resolve it manually",
                    );
                }
                let mut output = remaining;
                if !remove {
                    if !output.is_empty() && !output.ends_with('\n') {
                        output.push('\n');
                    }
                    let block = format!("{BEGIN}\n{}{END}\n", toml_entry("pluginpocket", &entry)?);
                    output.push_str(&block);
                    manifest.insert(client.name().into(), Value::String(block));
                } else {
                    manifest.remove(client.name());
                }
                output.into_bytes()
            } else {
                let mut document: Value = match &old {
                    Some(bytes) => serde_json::from_slice(bytes)
                        .map_err(|_| "invalid client JSON configuration")?,
                    None => json!({}),
                };
                let object = document
                    .as_object_mut()
                    .ok_or("client JSON configuration must be an object")?;
                if object.get("mcpServers").is_some_and(|v| !v.is_object()) {
                    return Err("mcpServers must be an object");
                }
                let servers = object
                    .entry("mcpServers")
                    .or_insert_with(|| json!({}))
                    .as_object_mut()
                    .ok_or("mcpServers must be an object")?;
                if let Some(existing) = servers.get("pluginpocket")
                    && !owns(&manifest, client.name(), existing)
                {
                    return Err(
                        "existing pluginpocket configuration is unmanaged or modified; resolve it manually",
                    );
                }
                if remove {
                    servers.remove("pluginpocket");
                    manifest.remove(client.name());
                } else {
                    servers.insert("pluginpocket".into(), entry.clone());
                    manifest.insert(client.name().into(), entry);
                }
                let mut bytes = serde_json::to_vec_pretty(&document)
                    .map_err(|_| "could not encode client configuration")?;
                bytes.push(b'\n');
                bytes
            };
            if old.as_deref() == Some(new.as_slice()) || (remove && old.is_none()) {
                continue;
            }
            updates.push((path, backup, old, new));
        }
        // Validate all targets and management metadata before the first mutation.
        config::safe_path(&self.manifest_path())?;
        // Record both versions before changing any client. If a later write fails,
        // either the old or the new exact entry remains safe to retry or remove.
        if !updates.is_empty() {
            let mut journal = previous_manifest;
            for (name, entry) in &manifest {
                if journal.get(name) != Some(entry) {
                    let mut versions = match journal.remove(name) {
                        Some(Value::Array(versions)) => versions,
                        Some(previous) => vec![previous],
                        None => Vec::new(),
                    };
                    if !versions.contains(entry) {
                        versions.push(entry.clone());
                    }
                    journal.insert(name.clone(), Value::Array(versions));
                }
            }
            config::atomic_write(
                &self.manifest_path(),
                &serde_json::to_vec_pretty(&journal)
                    .map_err(|_| "could not encode client management record")?,
            )?;
        }
        for (path, backup, old, new) in updates {
            if let Some(old) = old {
                config::atomic_write(&backup, &old)?;
            }
            config::atomic_write(&path, &new)?;
        }
        config::atomic_write(
            &self.manifest_path(),
            &serde_json::to_vec_pretty(&manifest)
                .map_err(|_| "could not encode client management record")?,
        )?;
        self.client_states_for(&selected)
    }
    fn entry(
        &self,
        client: ClientKind,
        direct: bool,
        credentials: &config::Credentials,
    ) -> Result<Value> {
        if direct {
            let mut url = config::root_url(&credentials.server)?;
            url.set_path("/mcp");
            if client == ClientKind::Codex {
                return Ok(json!({"url":url.as_str(),"bearer_token_env_var":"PLUGINPOCKET_TOKEN"}));
            }
            Ok(
                json!({"type":"http","url":url.as_str(),"headers":{"Authorization":format!("Bearer {}",credentials.token)}}),
            )
        } else {
            let mut entry = json!({"command":self.executable.to_str().ok_or("executable path must be UTF-8")?,"args":["bridge"],"env":{"PLUGINPOCKET_CONFIG":self.config_path.to_str().ok_or("configuration path must be UTF-8")?}});
            if client == ClientKind::Claude {
                entry["type"] = json!("stdio");
            }
            Ok(entry)
        }
    }
}
pub(crate) fn owns(manifest: &Map<String, Value>, name: &str, entry: &Value) -> bool {
    manifest.get(name).is_some_and(|record| {
        record == entry
            || record
                .as_array()
                .is_some_and(|versions| versions.contains(entry))
    })
}
fn toml_without_managed(text: &str) -> Result<(String, Option<String>)> {
    toml_splice(text, BEGIN, END, "pluginpocket")
}
pub(crate) fn toml_splice(
    text: &str,
    begin: &str,
    end: &str,
    key: &str,
) -> Result<(String, Option<String>)> {
    let original = text
        .parse::<DocumentMut>()
        .map_err(|_| "invalid client TOML configuration")?;
    let begins: Vec<_> = text.match_indices(begin).collect();
    let ends: Vec<_> = text.match_indices(end).collect();
    let managed = !begins.is_empty();
    let mut managed_block = None;
    if managed
        && original
            .get("mcp_servers")
            .and_then(|i| i.get(key))
            .is_none()
    {
        return Err("PluginPocket markers do not enclose an actual client entry");
    }
    let remaining = if begins.is_empty() && ends.is_empty() {
        text.to_owned()
    } else {
        if begins.len() != 1 || ends.len() != 1 || begins[0].0 >= ends[0].0 {
            return Err("invalid PluginPocket configuration markers");
        }
        let start = begins[0].0;
        let mut end = ends[0].0 + end.len();
        if (start > 0 && !text[..start].ends_with('\n'))
            || !text[end..].starts_with(['\n', '\r']) && end != text.len()
        {
            return Err("invalid PluginPocket configuration markers");
        }
        if text[end..].starts_with("\r\n") {
            end += 2;
        } else if text[end..].starts_with('\n') {
            end += 1;
        }
        managed_block = Some(text[start..end].to_owned());
        format!("{}{}", &text[..start], &text[end..])
    };
    let document = remaining
        .parse::<DocumentMut>()
        .map_err(|_| "invalid TOML outside PluginPocket markers")?;
    if document
        .get("mcp_servers")
        .and_then(|i| i.get(key))
        .is_some()
    {
        return Err(
            "existing pluginpocket configuration has no managed markers; resolve it manually",
        );
    }
    Ok((remaining, managed_block))
}
pub(crate) fn toml_entry(key: &str, entry: &Value) -> Result<String> {
    let mut document = DocumentMut::new();
    let mut servers = Table::new();
    servers.set_implicit(true);
    let mut pluginpocket = Table::new();
    for (key, v) in entry.as_object().ok_or("invalid client entry")? {
        pluginpocket[key] = match v {
            Value::String(s) => value(s),
            Value::Array(values) => {
                let mut a = toml_edit::Array::new();
                for s in values {
                    a.push(s.as_str().ok_or("invalid client argument")?);
                }
                value(a)
            }
            Value::Object(values) => {
                let mut table = toml_edit::InlineTable::new();
                for (key, v) in values {
                    table.insert(key, v.as_str().ok_or("invalid client environment")?.into());
                }
                value(table)
            }
            _ => return Err("invalid client entry"),
        };
    }
    servers[key] = Item::Table(pluginpocket);
    document["mcp_servers"] = Item::Table(servers);
    Ok(document.to_string())
}
