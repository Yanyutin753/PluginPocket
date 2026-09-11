use crate::{
    ClientKind, ClientState, LocalClient, Result,
    clients::{owns, toml_entry, toml_splice},
    config,
};
use base64::{Engine as _, engine::general_purpose::STANDARD};
use serde::Deserialize;
use serde_json::{Map, Value, json};
use std::{collections::HashSet, path::PathBuf};

/// 市场条目（与服务端 GET /api/v1/marketplace 的公开字段一致）。
#[derive(Debug, Deserialize)]
pub struct MarketItem {
    pub slug: String,
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub repo_url: String,
    #[serde(default)]
    pub transport: String,
    #[serde(default)]
    pub endpoint: String,
    #[serde(default)]
    pub stars: i64,
    #[serde(default)]
    pub installed: bool,
    #[serde(default = "default_kind")]
    pub kind: String,
    #[serde(default)]
    pub spec: Option<MarketSpec>,
}
fn default_kind() -> String {
    "mcp".to_owned()
}
/// skill：inline 文件集或 GitHub (repo, path)；bundle：成员 slug 列表。
#[derive(Debug, Deserialize)]
pub struct MarketSpec {
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub files: Option<std::collections::BTreeMap<String, String>>,
    #[serde(default)]
    pub file_manifest: Option<std::collections::BTreeMap<String, FileMetadata>>,
    #[serde(default)]
    pub includes: Option<Vec<String>>,
}
#[derive(Debug, Deserialize)]
pub struct FileMetadata {
    pub sha256: String,
    pub size: u64,
    pub executable: bool,
}

#[derive(Deserialize)]
#[serde(untagged)]
pub(crate) enum SkillFile {
    Text(String),
    Binary {
        encoding: String,
        content: String,
        #[serde(flatten)]
        metadata: FileMetadata,
    },
}

struct DecodedFile {
    bytes: Vec<u8>,
    #[cfg_attr(not(unix), allow(dead_code))]
    executable: bool,
}

fn decode_skill_files(
    files: std::collections::BTreeMap<String, SkillFile>,
) -> Result<std::collections::BTreeMap<String, DecodedFile>> {
    const MAX_FILE: usize = 8 * 1024 * 1024;
    if files.len() > 32 || !files.contains_key("SKILL.md") {
        return Err("skill must include SKILL.md and stay within 32 files");
    }
    let folded: HashSet<String> = files.keys().map(|name| name.to_lowercase()).collect();
    if folded.len() != files.len() {
        return Err("skill contains conflicting file names");
    }
    for name in files.keys() {
        if name.is_empty()
            || name.len() > 128
            || name.contains("..")
            || name.contains(['\\', ':', '<', '>', '"', '|', '?', '*'])
            || name.chars().any(|c| c.is_control())
            || name.split('/').any(|p| {
                let lower = p.to_lowercase();
                let stem = lower
                    .split('.')
                    .next()
                    .unwrap_or_default()
                    .trim_end_matches(' ');
                p.is_empty()
                    || p.ends_with(['.', ' '])
                    || matches!(stem, "con" | "prn" | "aux" | "nul")
                    || ((stem.starts_with("com") || stem.starts_with("lpt"))
                        && stem.len() == 4
                        && matches!(stem.as_bytes()[3], b'1'..=b'9'))
            })
            || name
                .match_indices('/')
                .any(|(i, _)| folded.contains(&name[..i].to_lowercase()))
        {
            return Err("skill contains an unsafe or conflicting file name");
        }
    }
    let mut decoded = std::collections::BTreeMap::new();
    let mut total = 0;
    for (name, file) in files {
        let (bytes, executable) = match file {
            SkillFile::Text(content) => (content.into_bytes(), false),
            SkillFile::Binary {
                encoding,
                content,
                metadata,
            } => {
                if encoding != "base64"
                    || metadata.size > MAX_FILE as u64
                    || content.len() > MAX_FILE.div_ceil(3) * 4
                {
                    return Err("skill contains an unsupported encoding or oversized file");
                }
                let bytes = STANDARD
                    .decode(content)
                    .map_err(|_| "skill contains invalid base64")?;
                let hash: String = ring::digest::digest(&ring::digest::SHA256, &bytes)
                    .as_ref()
                    .iter()
                    .map(|b| format!("{b:02x}"))
                    .collect();
                if bytes.len() as u64 != metadata.size || hash != metadata.sha256 {
                    return Err("skill file size or SHA256 does not match");
                }
                (bytes, metadata.executable)
            }
        };
        total += bytes.len();
        if bytes.len() > MAX_FILE || total > 32 * 1024 * 1024 {
            return Err("skill files exceed size limits");
        }
        if name == "SKILL.md" && std::str::from_utf8(&bytes).is_err() {
            return Err("SKILL.md must be UTF-8");
        }
        decoded.insert(name, DecodedFile { bytes, executable });
    }
    Ok(decoded)
}
/// heal_polluted_block 修复"外部 TOML 段被写进托管块中间"的污染（实测 codex plugin add
/// 会如此插入 [plugins.*]/[marketplaces.*]）：把块内不属于本条目的段搬到 end 标记之后，
/// 内容零丢失；块完好时原样返回。
fn heal_polluted_block(text: &str, begin: &str, end: &str) -> String {
    let (Some(b), Some(e)) = (text.find(begin), text.find(end)) else {
        return text.to_owned();
    };
    let block_end = e + end.len();
    let block = &text[b..block_end];
    let mut ours: Vec<&str> = Vec::new();
    let mut foreign: Vec<&str> = Vec::new();
    let mut foreign_now = false;
    for line in block.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with('[') && !trimmed.starts_with("[mcp_servers.") {
            foreign_now = true;
        }
        if trimmed == begin || trimmed == end {
            ours.push(line);
            continue;
        }
        if foreign_now {
            foreign.push(line);
        } else if !trimmed.is_empty() {
            // ours 只保留标记与条目行：块内空行属排版，与 manifest 记录的规范块对齐。
            ours.push(line);
        }
    }
    if foreign.is_empty() {
        return text.to_owned();
    }
    let mut healed = String::new();
    healed.push_str(&text[..b]);
    for line in &ours {
        healed.push_str(line);
        healed.push('\n');
    }
    for line in &foreign {
        healed.push_str(line);
        healed.push('\n');
    }
    healed.push_str(text[block_end..].trim_start_matches('\n'));
    healed
}

fn file_names(files: &std::collections::BTreeMap<String, DecodedFile>) -> Vec<String> {
    files.keys().cloned().collect()
}

fn skill_directory_files(root: &std::path::Path, prefix: &str) -> Result<Vec<String>> {
    let mut files = Vec::new();
    for entry in std::fs::read_dir(root).map_err(|_| "could not inspect skill directory")? {
        let entry = entry.map_err(|_| "could not inspect skill directory")?;
        let name = format!("{prefix}{}", entry.file_name().to_string_lossy());
        let kind = entry
            .file_type()
            .map_err(|_| "could not inspect skill file")?;
        if kind.is_file() {
            files.push(name);
        } else if kind.is_dir() {
            let nested = skill_directory_files(&entry.path(), &format!("{name}/"))?;
            if nested.is_empty() {
                // Empty directories are not in a file manifest; preserve foreign additions.
                files.push(format!("{name}/"));
            }
            files.extend(nested);
        } else {
            return Err("skill directory contains a link or unsupported file");
        }
    }
    files.sort();
    Ok(files)
}

fn check_skill_directory(root: &std::path::Path, recorded: &[String]) -> Result<()> {
    let mut expected = recorded.to_vec();
    expected.sort();
    if skill_directory_files(root, "")? != expected {
        return Err(
            "skill directory contains files PluginPocket did not install; remove them first",
        );
    }
    Ok(())
}
pub fn validate_slug(slug: &str) -> Result<()> {
    if slug.is_empty()
        || slug.len() > 64
        || !slug
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
    {
        return Err("plugin slug may only contain letters, digits, hyphen and underscore");
    }
    Ok(())
}
/// 本地直连安装只接受 HTTP(S) 端点；路径允许（如 /mcp），但不得带凭证、查询或片段。
fn endpoint_url(raw: &str) -> Result<&str> {
    let url = config::parse_url(raw)?;
    if url.username().is_empty()
        && url.password().is_none()
        && url.query().is_none()
        && url.fragment().is_none()
    {
        Ok(raw)
    } else {
        Err("endpoint must be a plain HTTP(S) URL without credentials, query or fragment")
    }
}
impl LocalClient {
    pub fn market(&self) -> Result<Vec<MarketItem>> {
        let credentials = config::load(&self.config_path)?;
        config::market(&credentials.server, &credentials.token)
    }
    pub fn install_plugin(
        &self,
        slug: &str,
        url: Option<&str>,
        clients: &[ClientKind],
    ) -> Result<Vec<ClientState>> {
        validate_slug(slug)?;
        if let Some(raw) = url {
            let endpoint = endpoint_url(raw)?.to_owned();
            return self.write_plugin(slug, &endpoint, clients, false);
        }
        let items = self.market()?;
        self.install_mcp(&items, slug, None, clients)
    }
    fn install_mcp(
        &self,
        items: &[MarketItem],
        slug: &str,
        url: Option<&str>,
        clients: &[ClientKind],
    ) -> Result<Vec<ClientState>> {
        validate_slug(slug)?;
        let endpoint = match url {
            Some(raw) => endpoint_url(raw)?.to_owned(),
            None => items
                .iter()
                .find(|item| {
                    item.slug == slug && item.transport == "http" && !item.endpoint.is_empty()
                })
                .map(|item| item.endpoint.clone())
                .ok_or("plugin has no known HTTP endpoint; pass --url")?,
        };
        self.write_plugin(slug, &endpoint, clients, false)
    }
    pub fn uninstall_plugin(&self, slug: &str, clients: &[ClientKind]) -> Result<Vec<ClientState>> {
        validate_slug(slug)?;
        self.write_plugin(slug, "", clients, true)
    }
    /// 一键安装市场条目：mcp 写客户端配置，skill 写技能目录，bundle 递归成员。
    pub fn install(&self, slug: &str, url: Option<&str>, clients: &[ClientKind]) -> Result<()> {
        let items = self.market()?;
        self.install_from(&items, slug, url, clients)
    }
    fn install_from(
        &self,
        items: &[MarketItem],
        slug: &str,
        url: Option<&str>,
        clients: &[ClientKind],
    ) -> Result<()> {
        let item = items
            .iter()
            .find(|item| item.slug == slug)
            .ok_or("plugin not found in marketplace")?;
        match item.kind.as_str() {
            "mcp" => {
                if item.transport == "gateway" {
                    // 网关供给：凭证在服务端，装 bridge 走计量；不写任何直连条目。
                    self.write_clients(clients, false, false)?;
                } else if let Some(raw) = url {
                    let endpoint = endpoint_url(raw)?.to_owned();
                    self.write_plugin(slug, &endpoint, clients, false)?;
                } else {
                    self.install_mcp(items, slug, None, clients)?;
                }
            }
            "skill" => {
                self.install_skill_from(items, slug, clients)?;
            }
            "bundle" => {
                let includes = item
                    .spec
                    .as_ref()
                    .and_then(|spec| spec.includes.clone())
                    .ok_or("bundle has no member list")?;
                for member in includes {
                    self.install_from(items, &member, url, clients)?;
                }
            }
            _ => return Err("unsupported marketplace item"),
        }
        Ok(())
    }
    /// 一键刷新全部托管插件/技能到服务端最新内容（bridge 条目除外——它本来就实时）。
    pub fn update(&self, clients: &[ClientKind]) -> Result<Vec<String>> {
        let manifest = self.manifest()?;
        let mut slugs = std::collections::BTreeSet::new();
        for key in manifest.keys() {
            let parts: Vec<&str> = key.split(':').collect();
            let (slug, owner) = if parts.len() == 3 && parts[0] == "skill" {
                (parts[2], parts[1])
            } else if parts.len() == 2 {
                (parts[1], parts[0])
            } else {
                continue; // bridge 条目（键为客户端名），服务端目录天然实时。
            };
            let owned = clients.iter().any(|c| c.name() == owner);
            if clients.is_empty() || owned {
                slugs.insert(slug.to_owned());
            }
        }
        if slugs.is_empty() {
            return Ok(Vec::new());
        }
        let items = self.market()?;
        let mut updated = Vec::new();
        for slug in &slugs {
            self.install_from(&items, slug, None, clients)?;
            updated.push(slug.clone());
        }
        Ok(updated)
    }

    /// 卸载市场条目；bundle 递归卸载成员。
    pub fn uninstall(&self, slug: &str, clients: &[ClientKind]) -> Result<()> {
        let items = self.market()?;
        self.uninstall_from(&items, slug, clients)
    }
    fn uninstall_from(
        &self,
        items: &[MarketItem],
        slug: &str,
        clients: &[ClientKind],
    ) -> Result<()> {
        let item = items
            .iter()
            .find(|item| item.slug == slug)
            .ok_or("plugin not found in marketplace")?;
        match item.kind.as_str() {
            "mcp" => {
                if item.transport != "gateway" {
                    self.write_plugin(slug, "", clients, true)?;
                }
                // 网关供给成员的 bridge 可能还有其他工具在用：卸载不动 bridge（pluginpocket remove 单独管理）。
            }
            "skill" => {
                self.uninstall_skill(slug, clients)?;
            }
            "bundle" => {
                let includes = item
                    .spec
                    .as_ref()
                    .and_then(|spec| spec.includes.clone())
                    .ok_or("bundle has no member list")?;
                for member in includes {
                    self.uninstall_from(items, &member, clients)?;
                }
            }
            _ => return Err("unsupported marketplace item"),
        }
        Ok(())
    }
    fn skill_root(&self, client: ClientKind, slug: &str) -> Result<std::path::PathBuf> {
        let base = match client {
            ClientKind::Codex => ".codex/skills",
            ClientKind::Claude => ".claude/skills",
            ClientKind::Cursor => {
                return Err("cursor has no skills directory; use --clients codex,claude");
            }
        };
        Ok(self.home.join(base).join(slug))
    }
    fn install_skill_from(
        &self,
        items: &[MarketItem],
        slug: &str,
        clients: &[ClientKind],
    ) -> Result<()> {
        validate_slug(slug)?;
        let item = items
            .iter()
            .find(|item| item.slug == slug)
            .ok_or("plugin not found in marketplace")?;
        let files = match item
            .spec
            .as_ref()
            .filter(|spec| spec.source == "inline" && spec.file_manifest.is_none())
        {
            Some(spec) => spec
                .files
                .clone()
                .ok_or("inline skill has no files")?
                .into_iter()
                .map(|(name, text)| (name, SkillFile::Text(text)))
                .collect(),
            None => {
                let credentials = config::load(&self.config_path)?;
                config::skill_files(&credentials.server, &credentials.token, slug)?
            }
        };
        let files = decode_skill_files(files)?;
        let selected: Vec<ClientKind> = if clients.is_empty() {
            self.client_states()?
                .into_iter()
                .filter(|c| c.detected)
                .map(|c| c.client)
                .collect()
        } else {
            clients.to_vec()
        };
        let mut manifest = self.manifest()?;
        config::safe_path(&self.manifest_path())?;
        let mut destinations = Vec::new();
        for client in selected
            .iter()
            .copied()
            .filter(|c| matches!(c, ClientKind::Codex | ClientKind::Claude))
        {
            let root = self.skill_root(client, slug)?;
            config::safe_path(&root)?;
            let recorded: Vec<String> = if root.exists() {
                let record = manifest
                    .get(&format!("skill:{}:{slug}", client.name()))
                    .ok_or("existing skill directory is unmanaged; resolve it manually")?;
                let recorded: Vec<String> = serde_json::from_value(record.clone())
                    .map_err(|_| "invalid skill management record")?;
                check_skill_directory(&root, &recorded)?;
                recorded
            } else {
                Vec::new()
            };
            for name in files.keys() {
                let path = root.join(name);
                config::safe_path(&path)?;
                if path.is_dir() {
                    return Err("skill file destination is a directory");
                }
            }
            destinations.push((client, root, recorded));
        }
        for (client, root, recorded) in destinations {
            for (name, content) in &files {
                let path = root.join(name);
                config::atomic_write(&path, &content.bytes)?;
                #[cfg(unix)]
                {
                    use std::os::unix::fs::PermissionsExt;
                    std::fs::set_permissions(
                        &path,
                        std::fs::Permissions::from_mode(if content.executable {
                            0o755
                        } else {
                            0o644
                        }),
                    )
                    .map_err(|_| "could not set skill file permissions")?;
                }
            }
            for name in recorded.iter().filter(|name| !files.contains_key(*name)) {
                let path = root.join(name);
                config::safe_path(&path)?;
                std::fs::remove_file(&path).map_err(|_| "could not remove obsolete skill file")?;
                let mut parent = path.parent();
                while let Some(dir) = parent.filter(|dir| *dir != root) {
                    match std::fs::remove_dir(dir) {
                        Ok(()) => parent = dir.parent(),
                        Err(error) if error.kind() == std::io::ErrorKind::DirectoryNotEmpty => {
                            break;
                        }
                        Err(_) => return Err("could not remove obsolete skill directory"),
                    }
                }
            }
            manifest.insert(
                format!("skill:{}:{slug}", client.name()),
                serde_json::json!(file_names(&files)),
            );
        }
        config::atomic_write(
            &self.manifest_path(),
            &serde_json::to_vec_pretty(&manifest)
                .map_err(|_| "could not encode client management record")?,
        )?;
        Ok(())
    }
    fn uninstall_skill(&self, slug: &str, clients: &[ClientKind]) -> Result<()> {
        validate_slug(slug)?;
        let selected: Vec<ClientKind> = if clients.is_empty() {
            self.client_states()?
                .into_iter()
                .filter(|c| c.detected)
                .map(|c| c.client)
                .collect()
        } else {
            clients.to_vec()
        };
        let mut manifest = self.manifest()?;
        for client in selected
            .iter()
            .copied()
            .filter(|c| matches!(c, ClientKind::Codex | ClientKind::Claude))
        {
            let manifest_key = format!("skill:{}:{slug}", client.name());
            let recorded = manifest
                .get(&manifest_key)
                .and_then(|value| value.as_array())
                .map(|names| {
                    names
                        .iter()
                        .filter_map(|name| name.as_str().map(str::to_owned))
                        .collect::<Vec<String>>()
                });
            let Some(recorded) = recorded else {
                continue;
            };
            let root = self.skill_root(client, slug)?;
            config::safe_path(&root)?;
            if root.exists() {
                check_skill_directory(&root, &recorded)?;
                std::fs::remove_dir_all(&root).map_err(|_| "could not remove skill directory")?;
            }
            manifest.remove(&manifest_key);
        }
        config::atomic_write(
            &self.manifest_path(),
            &serde_json::to_vec_pretty(&manifest)
                .map_err(|_| "could not encode client management record")?,
        )?;
        Ok(())
    }
    fn write_plugin(
        &self,
        slug: &str,
        endpoint: &str,
        clients: &[ClientKind],
        remove: bool,
    ) -> Result<Vec<ClientState>> {
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
        let begin = format!("# --- pluginpocket:{slug} begin ---");
        let end = format!("# --- pluginpocket:{slug} end ---");
        let key = format!("pluginpocket-{slug}");
        let previous_manifest = self.manifest()?;
        let mut manifest = previous_manifest.clone();
        let mut updates = Vec::new();
        let mut seen = HashSet::new();
        for client in selected.iter().copied().filter(|c| seen.insert(*c)) {
            let manifest_key = format!("{}:{}", client.name(), slug);
            let path = self.client_path(client);
            let old = config::read(&path)?;
            let backup = PathBuf::from(format!("{}.pluginpocket.bak", path.display()));
            config::safe_path(&backup)?;
            let new = if client == ClientKind::Codex {
                let entry = json!({"url": endpoint});
                let mut text = std::str::from_utf8(old.as_deref().unwrap_or_default())
                    .map_err(|_| "invalid client configuration encoding")?
                    .to_owned();
                text = heal_polluted_block(&text, &begin, &end);
                let old = Some(text.clone().into_bytes());
                let (remaining, existing) = toml_splice(&text, &begin, &end, &key)?;
                if existing
                    .is_some_and(|block| !owns(&manifest, &manifest_key, &Value::String(block)))
                {
                    let _ = old;
                    return Err(
                        "existing pluginpocket configuration is unmanaged or modified; resolve it manually",
                    );
                }
                let mut output = remaining;
                if !remove {
                    if !output.is_empty() && !output.ends_with('\n') {
                        output.push('\n');
                    }
                    let block = format!("{begin}\n{}{end}\n", toml_entry(&key, &entry)?);
                    output.push_str(&block);
                    manifest.insert(manifest_key.clone(), Value::String(block));
                } else {
                    manifest.remove(&manifest_key);
                }
                output.into_bytes()
            } else {
                let entry = json!({"type":"http","url":endpoint});
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
                if let Some(existing) = servers.get(&key)
                    && !owns(&manifest, &manifest_key, existing)
                {
                    return Err(
                        "existing pluginpocket configuration is unmanaged or modified; resolve it manually",
                    );
                }
                if remove {
                    servers.remove(&key);
                    manifest.remove(&manifest_key);
                } else {
                    servers.insert(key.clone(), entry.clone());
                    manifest.insert(manifest_key, entry);
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
        config::safe_path(&self.manifest_path())?;
        if !updates.is_empty() {
            let mut journal: Map<String, Value> = previous_manifest;
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
}
