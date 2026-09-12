use crate::{ClientKind, LocalClient, Result, config};
use serde::{Deserialize, Serialize};
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Debug, Serialize)]
pub struct InstalledItem {
    pub slug: String,
    pub kind: String,
    pub clients: Vec<ClientKind>,
    pub version: Option<String>,
}
#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum InstalledKind {
    Mcp,
    Skill,
}
impl InstalledKind {
    pub(crate) fn as_str(self) -> &'static str {
        match self {
            Self::Mcp => "mcp",
            Self::Skill => "skill",
        }
    }
}
#[derive(Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct LogEntry {
    pub timestamp: u64,
    pub level: String,
    pub action: String,
    pub message: String,
}
#[derive(Debug, Serialize)]
pub struct LogExport {
    pub path: String,
    pub count: usize,
}
#[derive(Debug, Serialize)]
pub struct DiagnosticCheck {
    pub id: String,
    pub label: String,
    pub status: String,
    pub detail: String,
}
#[derive(Debug, Serialize)]
pub struct Diagnostics {
    pub checks: Vec<DiagnosticCheck>,
}
impl LocalClient {
    pub fn export_logs(&self, entries: &[LogEntry]) -> Result<LogExport> {
        if entries.is_empty() || entries.len() > 200 {
            return Err("select between 1 and 200 local log entries");
        }
        let saved = self.logs()?;
        if entries.iter().any(|entry| !saved.contains(entry)) {
            return Err("selected logs changed; refresh the log list before exporting");
        }
        let name = format!(
            "operations-{}-{}.txt",
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos(),
            std::process::id()
        );
        let path = self.home.join(".pluginpocket/exports").join(name);
        let exported_path = path.to_str().ok_or("export path must be UTF-8")?.to_owned();
        let text = entries
            .iter()
            .map(|entry| {
                format!(
                    "{}\t{}\t{}\t{}\n",
                    entry.timestamp, entry.level, entry.action, entry.message
                )
            })
            .collect::<String>();
        config::atomic_write_new(&path, text.as_bytes())?;
        Ok(LogExport {
            path: exported_path,
            count: entries.len(),
        })
    }
    pub fn logs(&self) -> Result<Vec<LogEntry>> {
        if self
            .log_write_failed
            .load(std::sync::atomic::Ordering::Relaxed)
        {
            return Err(
                "operation log could not be saved; repair log permissions or content and retry the operation",
            );
        }
        self.read_logs()
    }
    fn read_logs(&self) -> Result<Vec<LogEntry>> {
        let Some(bytes) = config::read(&self.home.join(".pluginpocket/operations.json"))? else {
            return Ok(Vec::new());
        };
        let entries: Vec<LogEntry> =
            serde_json::from_slice(&bytes).map_err(|_| "invalid local operation log")?;
        if entries.len() > 200 {
            return Err("local operation log exceeds limit");
        }
        Ok(entries)
    }
    // Logging failure must not turn an already completed configuration write into a failed operation.
    pub(crate) fn record_operation<T>(&self, action: &'static str, result: &Result<T>) {
        let failed = self
            .append_operation(action, result.as_ref().err().copied())
            .is_err();
        self.log_write_failed
            .store(failed, std::sync::atomic::Ordering::Relaxed);
    }
    fn append_operation(&self, action: &'static str, error: Option<&'static str>) -> Result<()> {
        let label = match action {
            "login" => "登录",
            "logout" => "退出登录",
            "apply" => "客户端配置",
            "remove" => "移除客户端配置",
            "install" => "安装装备",
            "update" | "update_installed" => "更新装备",
            "uninstall" | "uninstall_installed" => "卸载装备",
            "bridge" => "Bridge 请求",
            _ => return Err("unsupported log action"),
        };
        let lock_path = self.home.join(".pluginpocket/operations.lock");
        config::safe_path(&lock_path)?;
        std::fs::create_dir_all(lock_path.parent().ok_or("invalid log path")?)
            .map_err(|_| "could not create log directory")?;
        let mut options = std::fs::OpenOptions::new();
        options.read(true).write(true).create(true).truncate(false);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            options.mode(0o600);
        }
        let lock = options
            .open(&lock_path)
            .map_err(|_| "could not open log lock")?;
        lock.lock().map_err(|_| "could not lock operation log")?;
        let mut entries = self.read_logs()?;
        entries.push(LogEntry {
            timestamp: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_millis() as u64,
            level: if error.is_none() { "info" } else { "error" }.into(),
            action: action.into(),
            message: if let Some(error) = error {
                format!("{label}失败：{}", recovery_message(error))
            } else {
                format!("{label}已完成。")
            },
        });
        if entries.len() > 200 {
            entries.drain(..entries.len() - 200);
        }
        config::atomic_write(
            &self.home.join(".pluginpocket/operations.json"),
            &serde_json::to_vec(&entries).map_err(|_| "could not encode operation log")?,
        )
    }
    pub fn record_bridge_failure(&self, error: &'static str) {
        self.record_operation::<()>("bridge", &Err(error));
    }

    pub fn diagnostics(&self, server: Option<&str>) -> Diagnostics {
        let mut checks = Vec::new();
        let mut add = |id: &str, label: &str, status: &str, detail: &str| {
            checks.push(DiagnosticCheck {
                id: id.into(),
                label: label.into(),
                status: status.into(),
                detail: detail.into(),
            })
        };
        let credentials = config::load(&self.config_path);
        let server = server.or_else(|| credentials.as_ref().ok().map(|c| c.server.as_str()));
        match server.map(config::health) {
            Some(Ok(_)) => add(
                "network",
                "服务连接",
                "ok",
                "健康接口可用；不代表 MCP 工具调用成功。",
            ),
            Some(Err(_)) => add(
                "network",
                "服务连接",
                "error",
                "无法验证服务健康状态，请检查服务地址与网络。",
            ),
            None => add(
                "network",
                "服务连接",
                "warning",
                "尚未提供服务地址，请先登录或填写地址。",
            ),
        }
        match credentials {
            Ok(ref c)
                if server.is_some_and(|server| {
                    config::root_url(server).ok() == config::root_url(&c.server).ok()
                }) =>
            {
                if config::verify(&c.server, &c.token).is_ok() {
                    add("auth", "账号凭证", "ok", "服务端已验证当前账号凭证。")
                } else {
                    add(
                        "auth",
                        "账号凭证",
                        "error",
                        "凭证验证未完成，请检查网络或重新登录。",
                    )
                }
            }
            Ok(_) => add(
                "auth",
                "账号凭证",
                "warning",
                "诊断地址与已登录服务不同，未发送凭证。",
            ),
            Err(_) if matches!(config::read(&self.config_path), Ok(None)) => {
                add("auth", "账号凭证", "warning", "尚未登录，请先登录。")
            }
            Err(_) => add("auth", "账号凭证", "error", "本地凭证不可用，请重新登录。"),
        }
        for client in [ClientKind::Codex, ClientKind::Claude, ClientKind::Cursor] {
            let (status, detail) = match self.client_states_for(&[client]) {
                Ok(states) if states.first().is_some_and(|s| s.configured) => {
                    ("ok", "托管 bridge 配置可读；尚未执行 MCP 工具调用。")
                }
                Ok(states) if states.first().is_some_and(|s| s.detected) => {
                    ("warning", "客户端配置可读，尚未配置托管 bridge。")
                }
                Ok(_) => ("warning", "未检测到客户端配置，可在概览中选择并配置。"),
                Err(_) => (
                    "error",
                    "配置或托管清单不可读，请修复格式与文件权限后重试。",
                ),
            };
            add(
                &format!("client-{}", client.name()),
                match client {
                    ClientKind::Codex => "Codex",
                    ClientKind::Claude => "Claude Code",
                    ClientKind::Cursor => "Cursor",
                },
                status,
                detail,
            );
        }
        let available = std::fs::metadata(&self.executable).is_ok_and(|meta| {
            if !meta.is_file() {
                return false;
            }
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                meta.permissions().mode() & 0o111 != 0
            }
            #[cfg(not(unix))]
            {
                true
            }
        });
        add(
            "bridge",
            "Bridge 可执行文件",
            if available { "ok" } else { "error" },
            if available {
                "本地可执行文件可用；尚未启动 bridge 或执行 MCP 工具调用。"
            } else {
                "可执行文件缺失或不可执行，请重新安装桌面应用。"
            },
        );
        Diagnostics { checks }
    }
}

// Only fixed local categories reach disk; unknown errors never include their original text.
fn recovery_message(error: &str) -> &'static str {
    match error {
        "not logged in; run pluginpocket login" => "尚未登录，请先登录后重试。",
        "invalid token; paste a PluginPocket token"
        | "invalid local credentials; log in again"
        | "token verification failed; check the server and token" => {
            "账号凭证无效或验证失败，请检查令牌并重新登录。"
        }
        "connection timed out; check the server and retry"
        | "could not connect; check the server and retry"
        | "gateway connection timed out"
        | "gateway connection failed; check credentials and server"
        | "could not start device authorization; check the server and retry"
        | "device authorization connection failed; run login --device again" => {
            "服务连接失败，请检查网络、服务地址与登录状态后重试。"
        }
        "server must be an HTTP(S) root URL"
        | "server must be an HTTP(S) root URL without credentials, query or fragment"
        | "endpoint must be a plain HTTP(S) URL without credentials, query or fragment"
        | "endpoint must be an HTTP(S) URL" => {
            "服务地址格式不安全或无效，请使用不含凭证和查询参数的 HTTP(S) 地址。"
        }
        "existing pluginpocket configuration is unmanaged or modified; resolve it manually"
        | "existing pluginpocket configuration has no managed markers; resolve it manually"
        | "existing skill directory is unmanaged; resolve it manually"
        | "skill directory contains files PluginPocket did not install; remove them first" => {
            "检测到配置冲突或外来文件，已保留冲突内容；请备份并手动核对后重试。"
        }
        "skill must include SKILL.md and stay within 32 files"
        | "skill file size or SHA256 does not match"
        | "skill contains invalid base64"
        | "skill contains an unsupported encoding or oversized file"
        | "skill files exceed size limits"
        | "skill contains conflicting file names"
        | "skill contains an unsafe or conflicting file name"
        | "SKILL.md must be UTF-8" => {
            "技能文件完整性或安全校验失败，请联系发布者修复文件后重新安装。"
        }
        "refusing a symbolic link in configuration path"
        | "skill directory contains a link or unsupported file" => {
            "路径包含符号链接或不支持的文件，已停止操作；请检查本地路径。"
        }
        "invalid client JSON configuration"
        | "invalid client TOML configuration"
        | "invalid client configuration encoding"
        | "client JSON configuration must be an object"
        | "mcpServers must be an object"
        | "invalid client management record"
        | "invalid skill management record" => {
            "本地配置格式损坏，请备份并修复配置或托管清单后重试。"
        }
        "client is not an installed target"
        | "equipment is not installed"
        | "ambiguous installed equipment kind" => {
            "安装目标已变化，请刷新装备列表并选择已安装的客户端。"
        }
        "equipment is no longer in the marketplace"
        | "plugin not found in marketplace"
        | "installed equipment kind or transport changed; review the marketplace entry" => {
            "市场条目已删除或类型发生变化，请核对市场条目后重试。"
        }
        "gateway request failed; no tool call was retried" => {
            "网关调用失败，未自动重试；请运行连接诊断并检查服务状态。"
        }
        "gateway tool returned an error" => "上游工具返回错误，请检查调用输入及服务端用量详情。",
        "MCP stdio initialization failed"
        | "MCP stdio service failed"
        | "could not start bridge runtime" => {
            "本地 Bridge 启动或通信失败，请重新启动客户端并运行连接诊断。"
        }
        _ => "操作未完成，请运行连接诊断，检查本地文件权限与服务状态后重试。",
    }
}
