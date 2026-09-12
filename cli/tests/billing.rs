mod common;
use common::*;

#[test]
fn usage_and_ledger_fetch_scoped_account_data_with_bearer_token() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    let (server, request) = fixture(
        "200 OK",
        r#"{"items":[{"id":7,"user_id":1,"token_id":2,"wallet_id":3,"tool":"echo","cost":3,"status":"ok","duration_ms":120,"created_at":"2026-09-12T10:00:00Z","billing_role":"standard","multiplier_bp":10000}],"next_cursor":"42"}"#,
    );
    credentials(&local, &server, "ppt_private");
    let page = local.usage().expect("usage fetch must succeed");
    assert_eq!(page.items.len(), 1);
    assert_eq!(page.items[0].tool, "echo");
    assert_eq!(page.items[0].cost, 3);
    assert_eq!(page.items[0].status, "ok");
    assert_eq!(page.items[0].duration_ms, 120);
    assert_eq!(page.items[0].multiplier_bp, 10000);
    assert_eq!(page.next_cursor, "42");
    let request = request.join().unwrap();
    assert!(request.starts_with("GET /api/v1/account/usage"));
    assert!(
        request
            .to_lowercase()
            .contains("authorization: bearer ppt_private")
    );

    let (server, request) = fixture(
        "200 OK",
        r#"{"items":[{"id":9,"delta":-3,"kind":"reservation","note":"echo","created_at":"2026-09-12T10:00:01Z","balance_after":97}],"next_cursor":""}"#,
    );
    credentials(&local, &server, "ppt_private");
    let page = local.ledger().expect("ledger fetch must succeed");
    assert_eq!(page.items.len(), 1);
    assert_eq!(page.items[0].delta, -3);
    assert_eq!(page.items[0].kind, "reservation");
    assert_eq!(page.items[0].note, "echo");
    assert_eq!(page.items[0].balance_after, 97);
    assert_eq!(page.next_cursor, "");
    let request = request.join().unwrap();
    assert!(request.starts_with("GET /api/v1/account/ledger"));
    assert!(
        request
            .to_lowercase()
            .contains("authorization: bearer ppt_private")
    );
}

#[test]
fn usage_and_ledger_reject_failed_or_malformed_responses() {
    let dir = tempfile::tempdir().unwrap();
    let local = local(&dir);
    let (server, _request) = fixture("401 Unauthorized", r#"{"error":"unauthorized"}"#);
    credentials(&local, &server, "ppt_private");
    assert_eq!(
        local.usage(),
        Err("usage request failed; check the server and token")
    );

    let (server, _request) = fixture("401 Unauthorized", r#"{"error":"unauthorized"}"#);
    credentials(&local, &server, "ppt_private");
    assert_eq!(
        local.ledger(),
        Err("ledger request failed; check the server and token")
    );

    let (server, _request) = fixture("200 OK", r#"{"items":"nope"}"#);
    credentials(&local, &server, "ppt_private");
    assert_eq!(
        local.usage(),
        Err("server returned an invalid usage response")
    );

    let (server, _request) = fixture("200 OK", r#"{"items":"nope"}"#);
    credentials(&local, &server, "ppt_private");
    assert_eq!(
        local.ledger(),
        Err("server returned an invalid ledger response")
    );
}
