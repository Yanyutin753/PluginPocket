mod common;
use axum::{
    Router,
    body::Body,
    extract::{Request, State},
    http::StatusCode,
    middleware::{self, Next},
    response::Response,
};
use rmcp::{
    ErrorData, RoleServer, ServerHandler, ServiceExt,
    model::*,
    service::RequestContext,
    transport::streamable_http_server::{
        StreamableHttpServerConfig, StreamableHttpService, session::local::LocalSessionManager,
    },
};
use std::{
    process::Stdio,
    sync::{
        Arc, Mutex,
        atomic::{AtomicUsize, Ordering},
    },
    time::Duration,
};
use tokio::{io::AsyncReadExt, process::Command};

#[derive(Clone)]
struct Gateway;
impl ServerHandler for Gateway {
    fn get_info(&self) -> ServerInfo {
        let mut info = ServerInfo::default();
        info.capabilities = ServerCapabilities::builder().enable_tools().build();
        info
    }
    async fn list_tools(
        &self,
        _: Option<PaginatedRequestParams>,
        _: RequestContext<RoleServer>,
    ) -> Result<ListToolsResult, ErrorData> {
        Ok(ListToolsResult {tools:vec![serde_json::from_value(serde_json::json!({"name":"echo","description":"Fixture echo","inputSchema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}})).unwrap()],..Default::default()})
    }
    async fn call_tool(
        &self,
        request: CallToolRequestParams,
        _: RequestContext<RoleServer>,
    ) -> Result<CallToolResponse, ErrorData> {
        if request.name == "prefixed_quota" {
            return Ok(CallToolResult::error(vec![ContentBlock::text(
                "[loadout] insufficient_balance",
            )])
            .into());
        }
        if request.name == "prefixed_protocol_error" {
            return Err(ErrorData::invalid_params(
                "[loadout] invalid_arguments",
                None,
            ));
        }
        if request.name == "quota" {
            return Ok(
                CallToolResult::error(vec![ContentBlock::text("insufficient_balance")]).into(),
            );
        }
        if request.name == "protocol_error" {
            return Err(ErrorData::invalid_params("invalid_arguments", None));
        }
        Ok(CallToolResult::success(vec![ContentBlock::text(
            request
                .arguments
                .unwrap_or_default()
                .get("text")
                .and_then(|v| v.as_str())
                .unwrap_or("")
                .to_owned(),
        )])
        .into())
    }
}
#[derive(Clone, Default)]
struct Trace {
    tokens: Arc<Mutex<Vec<String>>>,
    effects: Arc<AtomicUsize>,
}
async fn auth(State(trace): State<Trace>, request: Request, next: Next) -> Response {
    let token = request
        .headers()
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("")
        .to_owned();
    trace.tokens.lock().unwrap().push(token.clone());
    if token == "Bearer ldt_revoked" {
        return Response::builder()
            .status(StatusCode::UNAUTHORIZED)
            .body(Body::from("ldt_do_not_expose"))
            .unwrap();
    }
    if request.method() == "POST" {
        let (parts, body) = request.into_parts();
        let bytes = axum::body::to_bytes(body, 1024 * 1024).await.unwrap();
        let json: serde_json::Value = serde_json::from_slice(&bytes).unwrap();
        if json["method"] == "tools/call"
            && (json["params"]["name"] == "lost_response" || json["params"]["name"] == "expired")
        {
            trace.effects.fetch_add(1, Ordering::SeqCst);
            return Response::builder()
                .status(if json["params"]["name"] == "expired" {
                    404
                } else {
                    500
                })
                .body(Body::from("ldt_do_not_expose"))
                .unwrap();
        }
        return next
            .run(Request::from_parts(parts, Body::from(bytes)))
            .await;
    }
    next.run(request).await
}
async fn gateway() -> (String, Trace, tokio::task::JoinHandle<()>) {
    let trace = Trace::default();
    let service = StreamableHttpService::new(
        || Ok(Gateway),
        Arc::new(LocalSessionManager::default()),
        StreamableHttpServerConfig::default()
            .with_legacy_session_mode(false)
            .with_json_response(true),
    );
    let app = Router::new()
        .nest_service("/mcp", service)
        .layer(middleware::from_fn_with_state(trace.clone(), auth));
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let server = format!("http://{}", listener.local_addr().unwrap());
    let task = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    (server, trace, task)
}
#[tokio::test]
async fn stdio_bridge_forwards_real_sdk_list_call_errors_and_refreshes_credentials_without_replaying()
 {
    tokio::time::timeout(Duration::from_secs(25), async {
        let dir = tempfile::tempdir().unwrap();
        let local = common::local(&dir);
        let (server, trace, task) = gateway().await;
        common::credentials(&local, &server, "ldt_initial");
        let mut child = Command::new(env!("CARGO_BIN_EXE_loadout"))
            .arg("bridge")
            .env("HOME", dir.path())
            .env("USERPROFILE", dir.path())
            .env("LOADOUT_CONFIG", &local.config_path)
            .env("NO_PROXY", "*")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .kill_on_drop(true)
            .spawn()
            .unwrap();
        let service = ().serve((child.stdout.take().unwrap(), child.stdin.take().unwrap())).await;
        assert!(
            service.is_ok(),
            "bridge must establish official SDK stdio: {:?}",
            service.err()
        );
        let mut service = service.unwrap();
        let tools = service.list_tools(None).await.unwrap();
        assert_eq!(tools.tools[0].name, "echo");
        assert_eq!(
            tools.tools[0].input_schema.get("required").unwrap(),
            &serde_json::json!(["text"])
        );
        let result = service
            .call_tool(
                CallToolRequestParams::new("echo").with_arguments(
                    serde_json::json!({"text":"hello"})
                        .as_object()
                        .unwrap()
                        .clone(),
                ),
            )
            .await
            .unwrap();
        assert_eq!(result.content[0].as_text().unwrap().text, "hello");
        let quota = service
            .call_tool(CallToolRequestParams::new("quota"))
            .await
            .unwrap();
        assert_eq!(quota.is_error, Some(true));
        assert!(
            quota.content[0]
                .as_text()
                .unwrap()
                .text
                .contains("insufficient_balance")
        );
        let error = service
            .call_tool(CallToolRequestParams::new("protocol_error"))
            .await
            .unwrap_err();
        assert!(error.to_string().contains("invalid_arguments"));
        let prefixed = service
            .call_tool(CallToolRequestParams::new("prefixed_quota"))
            .await
            .unwrap();
        assert_eq!(
            prefixed.content[0].as_text().unwrap().text,
            "[loadout] insufficient_balance"
        );
        let prefixed = service
            .call_tool(CallToolRequestParams::new("prefixed_protocol_error"))
            .await
            .unwrap_err();
        match prefixed {
            rmcp::ServiceError::McpError(error) => {
                assert_eq!(error.message, "[loadout] invalid_arguments")
            }
            error => panic!("unexpected error: {error}"),
        }
        common::credentials(&local, &server, "ldt_rotated");
        service
            .call_tool(CallToolRequestParams::new("echo"))
            .await
            .unwrap();
        assert_eq!(
            trace.tokens.lock().unwrap().last().unwrap(),
            "Bearer ldt_rotated"
        );
        common::credentials(&local, &server, "ldt_revoked");
        let revoked = service
            .call_tool(CallToolRequestParams::new("echo"))
            .await
            .unwrap();
        assert_eq!(revoked.is_error, Some(true));
        assert!(
            !serde_json::to_string(&revoked)
                .unwrap()
                .contains("ldt_do_not_expose")
        );
        common::credentials(&local, &server, "ldt_rotated");
        for name in ["lost_response", "expired"] {
            let result = service
                .call_tool(CallToolRequestParams::new(name))
                .await
                .unwrap();
            assert_eq!(result.is_error, Some(true));
            assert!(
                !serde_json::to_string(&result)
                    .unwrap()
                    .contains("ldt_do_not_expose")
            );
        }
        assert_eq!(
            trace.effects.load(Ordering::SeqCst),
            2,
            "each failed tool must be sent exactly once"
        );
        service.close().await.unwrap();
        let status = tokio::time::timeout(Duration::from_secs(3), child.wait())
            .await
            .expect("bridge must exit when stdio closes")
            .unwrap();
        assert!(status.success());
        let mut stderr = String::new();
        child
            .stderr
            .take()
            .unwrap()
            .read_to_string(&mut stderr)
            .await
            .unwrap();
        assert!(!stderr.contains("ldt_"));
        task.abort();
    })
    .await
    .expect("bridge test watchdog");
}
