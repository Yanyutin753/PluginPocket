use crate::{Result, config};
use rmcp::{
    ErrorData, RoleClient, RoleServer, ServerHandler, ServiceError, ServiceExt,
    model::*,
    service::{RequestContext, RunningService},
    transport::{
        StreamableHttpClientTransport, streamable_http_client::StreamableHttpClientTransportConfig,
    },
};
use std::{path::PathBuf, time::Duration};

struct Bridge {
    config_path: PathBuf,
    http: reqwest::Client,
    local: Option<crate::LocalClient>,
}
impl Bridge {
    fn record_failure(&self, error: &'static str) {
        if let Some(local) = &self.local {
            local.record_bridge_failure(error);
        }
    }
    async fn connect(&self) -> Result<RunningService<RoleClient, ()>> {
        // A new MCP client per operation uses current credentials and discards stale sessions.
        // The reqwest pool still reuses HTTP connections; never replay tool calls after failure.
        let credentials = config::load(&self.config_path)?;
        let mut url = config::root_url(&credentials.server)?;
        url.set_path("/mcp");
        let options = StreamableHttpClientTransportConfig::with_uri(url.to_string())
            .auth_header(credentials.token)
            .reinit_on_expired_session(false);
        let transport = StreamableHttpClientTransport::with_client(self.http.clone(), options);
        tokio::time::timeout(Duration::from_secs(10), ().serve(transport))
            .await
            .map_err(|_| "gateway connection timed out")?
            .map_err(|_| "gateway connection failed; check credentials and server")
    }
}
fn protocol_error(error: ServiceError) -> ErrorData {
    match error {
        ServiceError::McpError(mut error) => {
            if !error.message.starts_with("[pluginpocket]") {
                error.message = format!("[pluginpocket] {}", error.message).into();
            }
            error
        }
        _ => ErrorData::internal_error(
            "[pluginpocket] gateway request failed; no tool call was retried",
            None,
        ),
    }
}
fn call_error(message: &'static str) -> CallToolResponse {
    CallToolResult::error(vec![ContentBlock::text(format!(
        "[pluginpocket] {message}"
    ))])
    .into()
}
impl ServerHandler for Bridge {
    fn get_info(&self) -> ServerInfo {
        let mut info = ServerInfo::default();
        info.capabilities = ServerCapabilities::builder().enable_tools().build();
        info.server_info.name = "pluginpocket".into();
        info.server_info.version = env!("CARGO_PKG_VERSION").into();
        info
    }
    async fn list_tools(
        &self,
        request: Option<PaginatedRequestParams>,
        _: RequestContext<RoleServer>,
    ) -> std::result::Result<ListToolsResult, ErrorData> {
        let mut upstream = self.connect().await.map_err(|message| {
            self.record_failure(message);
            ErrorData::internal_error(format!("[pluginpocket] {message}"), None)
        })?;
        let result =
            tokio::time::timeout(Duration::from_secs(30), upstream.list_tools(request)).await;
        let _ = upstream.close().await;
        if !matches!(&result, Ok(Ok(_))) {
            self.record_failure("gateway request failed; no tool call was retried");
        }
        result
            .map_err(|_| {
                ErrorData::internal_error("[pluginpocket] gateway request timed out", None)
            })?
            .map_err(protocol_error)
    }
    async fn call_tool(
        &self,
        request: CallToolRequestParams,
        _: RequestContext<RoleServer>,
    ) -> std::result::Result<CallToolResponse, ErrorData> {
        let mut upstream = match self.connect().await {
            Ok(upstream) => upstream,
            Err(message) => {
                self.record_failure(message);
                return Ok(call_error(message));
            }
        };
        let result =
            tokio::time::timeout(Duration::from_secs(30), upstream.call_tool_once(request)).await;
        let _ = upstream.close().await;
        if !matches!(&result, Ok(Ok(_))) {
            self.record_failure("gateway request failed; no tool call was retried");
        }
        match result {
            Ok(Ok(mut response)) => {
                if let CallToolResponse::Complete(ref mut result) = response
                    && result.is_error == Some(true)
                {
                    self.record_failure("gateway tool returned an error");
                    for content in &mut result.content {
                        if let ContentBlock::Text(text) = content
                            && !text.text.starts_with("[pluginpocket]")
                        {
                            text.text = format!("[pluginpocket] {}", text.text);
                        }
                    }
                }
                Ok(response)
            }
            Ok(Err(ServiceError::McpError(error))) => {
                Err(protocol_error(ServiceError::McpError(error)))
            }
            _ => Ok(call_error(
                "gateway request failed; no tool call was retried",
            )),
        }
    }
}
/// Serve MCP on stdio. Diagnostics must go to stderr; stdout belongs to the SDK.
pub async fn run(config_path: PathBuf) -> Result<()> {
    run_inner(config_path, None).await
}
/// Desktop bridge uses its already validated local paths for safe persistent failure logs.
pub async fn run_local(local: crate::LocalClient) -> Result<()> {
    run_inner(local.config_path.clone(), Some(local)).await
}
async fn run_inner(config_path: PathBuf, local: Option<crate::LocalClient>) -> Result<()> {
    let http = reqwest::Client::builder()
        .timeout(Duration::from_secs(30))
        .connect_timeout(Duration::from_secs(5))
        .redirect(reqwest::redirect::Policy::none())
        .retry(reqwest::retry::never())
        .build()
        .map_err(|_| "could not initialize HTTP client")?;
    let service = Bridge {
        config_path,
        http,
        local,
    }
    .serve(rmcp::transport::stdio())
    .await
    .map_err(|_| "MCP stdio initialization failed")?;
    service
        .waiting()
        .await
        .map_err(|_| "MCP stdio service failed")?;
    Ok(())
}
