use crate::{Account, LocalClient, Result, config};
use reqwest::Url;
use serde::Deserialize;
use std::time::{Duration, Instant};

/// The information a person must verify in their browser. Does not expose the device secret.
pub struct DevicePrompt {
    pub user_code: String,
    pub verification_uri: String,
    pub expires_in: u64,
}
#[derive(Deserialize)]
struct Authorization {
    device_code: String,
    user_code: String,
    verification_uri: String,
    expires_in: u64,
    interval: u64,
}
impl LocalClient {
    /// Request a device code and wait for explicit approval in the user's browser.
    pub fn login_device(
        &self,
        server: &str,
        present: impl FnOnce(&DevicePrompt) -> Result<()>,
    ) -> Result<Account> {
        let result = self.login_device_inner(server, present);
        self.record_operation("login", &result);
        result
    }
    fn login_device_inner(
        &self,
        server: &str,
        present: impl FnOnce(&DevicePrompt) -> Result<()>,
    ) -> Result<Account> {
        config::safe_path(&self.config_path)?;
        let mut url = config::root_url(server)?;
        url.set_path("/api/v1/device/authorize");
        let http = config::http()?;
        let response = http
            .post(url)
            .json(&serde_json::json!({}))
            .send()
            .map_err(|_| "could not start device authorization; check the server and retry")?;
        if !response.status().is_success() {
            return Err("device authorization unavailable; retry login later");
        }
        let authorization: Authorization = response
            .json()
            .map_err(|_| "server returned an invalid device authorization")?;
        let uri = Url::parse(&authorization.verification_uri)
            .map_err(|_| "server returned an invalid verification address")?;
        if !matches!(uri.scheme(), "http" | "https")
            || uri.host_str().is_none()
            || !uri.username().is_empty()
            || uri.password().is_some()
            || uri.query().is_some()
            || uri.fragment().is_some()
            || authorization.user_code.is_empty()
            || authorization.user_code.len() > 64
            || !authorization
                .user_code
                .chars()
                .all(|c| c.is_ascii_alphanumeric() || matches!(c, '-' | '_'))
            || authorization.device_code.is_empty()
            || authorization.device_code.len() > 4096
            || authorization.device_code.chars().any(char::is_control)
            || authorization.expires_in == 0
            || authorization.expires_in > 3600
        {
            return Err("server returned an invalid device authorization");
        }
        let deadline = Instant::now() + Duration::from_secs(authorization.expires_in);
        let mut interval = Duration::from_secs(authorization.interval.max(1));
        present(&DevicePrompt {
            user_code: authorization.user_code,
            verification_uri: uri.to_string(),
            expires_in: authorization.expires_in,
        })?;
        let mut url = config::root_url(server)?;
        url.set_path("/api/v1/device/token");
        loop {
            let remaining = deadline.saturating_duration_since(Instant::now());
            if remaining.is_zero() {
                return Err("device authorization expired; run login --device again");
            }
            std::thread::sleep(interval.min(remaining));
            if Instant::now() >= deadline {
                return Err("device authorization expired; run login --device again");
            }
            let response = http
                .post(url.clone())
                .json(&serde_json::json!({"device_code":authorization.device_code}))
                .send()
                .map_err(|_| "device authorization connection failed; run login --device again")?;
            if response.status().is_success() {
                #[derive(Deserialize)]
                struct Token {
                    token: String,
                }
                let result: Token = response
                    .json()
                    .map_err(|_| "server returned an invalid device token")?;
                return self.login_inner(server, &result.token);
            }
            let status = response.status().as_u16();
            #[derive(Deserialize)]
            struct Failure {
                error: String,
            }
            let failure: Failure = response
                .json()
                .map_err(|_| "device authorization failed; run login --device again")?;
            match (status, failure.error.as_str()) {
                (400, "authorization_pending") => {}
                (429, "slow_down") => interval = interval.saturating_add(Duration::from_secs(5)),
                (400, "expired_token") => {
                    return Err("device authorization expired; run login --device again");
                }
                (400, "invalid_grant") => {
                    return Err(
                        "device authorization was already used or is invalid; run login --device again",
                    );
                }
                _ => return Err("device authorization failed; run login --device again"),
            }
        }
    }
}
