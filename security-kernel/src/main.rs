use axum::{
    body::{to_bytes, Body},
    extract::Json,
    extract::State,
    http::{Request, StatusCode},
    middleware::{self, Next},
    response::IntoResponse,
    routing::{get, post},
    Router,
};
use security_kernel::{
    append_hash_event, evaluate_authorization, now_unix_seconds, verify_challenge_signature,
    verify_service_signature, AuthorizationRequest, HashEventInput, IdentityChallenge,
    ServiceSignatureInput, VerifyIdentityRequest,
};
use serde_json::json;
use std::{env, net::SocketAddr, sync::Arc, time::Duration};
use tower_http::trace::TraceLayer;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};
use uuid::Uuid;

mod nonce_store;

use nonce_store::PostgresNonceStore;

#[derive(Clone)]
struct AppState {
    shared_secret: Arc<String>,
    max_clock_skew_seconds: i64,
    nonce_store: PostgresNonceStore,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    let port = env::var("SECURITY_KERNEL_PORT")
        .ok()
        .and_then(|value| value.parse::<u16>().ok())
        .unwrap_or(8090);
    let shared_secret = env::var("SECURITY_KERNEL_SHARED_SECRET").unwrap_or_default();
    if shared_secret.trim().is_empty() {
        panic!("SECURITY_KERNEL_SHARED_SECRET must not be empty");
    }
    let max_clock_skew_seconds = env::var("SECURITY_KERNEL_MAX_CLOCK_SKEW_SECONDS")
        .ok()
        .and_then(|value| value.parse::<i64>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(60);
    let database_url = env::var("SECURITY_KERNEL_DATABASE_URL").unwrap_or_default();
    if database_url.trim().is_empty() {
        panic!("SECURITY_KERNEL_DATABASE_URL must not be empty");
    }
    let nonce_store = PostgresNonceStore::connect(database_url.trim())
        .await
        .expect("connect security nonce database");
    let state = AppState {
        shared_secret: Arc::new(shared_secret),
        max_clock_skew_seconds,
        nonce_store: nonce_store.clone(),
    };
    tokio::spawn(run_nonce_cleanup(nonce_store, max_clock_skew_seconds));
    let address = SocketAddr::from(([0, 0, 0, 0], port));
    let app = Router::new()
        .route("/healthz", get(healthz))
        .route("/v1/identity/challenge", post(identity_challenge))
        .route("/v1/identity/verify", post(identity_verify))
        .route("/v1/ownership/attest", post(ownership_attest))
        .route("/v1/authorize", post(authorize))
        .route("/v1/license/evaluate", post(authorize))
        .route("/v1/context/filter", post(authorize))
        .layer(middleware::from_fn_with_state(
            state.clone(),
            verify_service_request,
        ))
        .layer(TraceLayer::new_for_http())
        .with_state(state);

    let listener = tokio::net::TcpListener::bind(address)
        .await
        .expect("bind security kernel");
    tracing::info!("security kernel listening on {}", address);
    axum::serve(listener, app)
        .await
        .expect("run security kernel");
}

async fn healthz(State(state): State<AppState>) -> impl IntoResponse {
    match state.nonce_store.is_ready().await {
        Ok(true) => (StatusCode::OK, Json(json!({"status":"ok"}))).into_response(),
        Ok(false) => (
            StatusCode::SERVICE_UNAVAILABLE,
            Json(json!({"status":"starting","reason":"security_nonce_table_unavailable"})),
        )
            .into_response(),
        Err(error) => {
            tracing::error!(%error, "security nonce database health check failed");
            (
                StatusCode::SERVICE_UNAVAILABLE,
                Json(
                    json!({"status":"unavailable","reason":"security_nonce_database_unavailable"}),
                ),
            )
                .into_response()
        }
    }
}

async fn verify_service_request(
    State(state): State<AppState>,
    req: Request<Body>,
    next: Next,
) -> impl IntoResponse {
    if req.uri().path() == "/healthz" {
        return next.run(req).await;
    }
    let (parts, body) = req.into_parts();
    let timestamp = match parts
        .headers
        .get("X-Security-Timestamp")
        .and_then(|value| value.to_str().ok())
    {
        Some(value) => value.to_string(),
        None => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({"error":"missing security timestamp"})),
            )
                .into_response()
        }
    };
    let nonce = match parts
        .headers
        .get("X-Security-Nonce")
        .and_then(|value| value.to_str().ok())
    {
        Some(value) => match Uuid::parse_str(value) {
            Ok(value) => value,
            Err(_) => {
                return (
                    StatusCode::UNAUTHORIZED,
                    Json(json!({"error":"missing or invalid security nonce"})),
                )
                    .into_response()
            }
        },
        _ => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({"error":"missing or invalid security nonce"})),
            )
                .into_response()
        }
    };
    let signature = match parts
        .headers
        .get("X-Security-Signature")
        .and_then(|value| value.to_str().ok())
    {
        Some(value) => value.to_string(),
        None => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({"error":"missing security signature"})),
            )
                .into_response()
        }
    };
    let bytes = match to_bytes(body, 1024 * 1024).await {
        Ok(bytes) => bytes,
        Err(_) => {
            return (
                StatusCode::BAD_REQUEST,
                Json(json!({"error":"invalid request body"})),
            )
                .into_response()
        }
    };
    let input = ServiceSignatureInput {
        timestamp,
        nonce: nonce.to_string(),
        body: bytes.to_vec(),
    };
    if let Err(err) = verify_service_signature(&state.shared_secret, &input, &signature) {
        return (
            StatusCode::UNAUTHORIZED,
            Json(json!({"error": err.to_string()})),
        )
            .into_response();
    }
    let now = now_unix_seconds() as i64;
    let request_timestamp =
        match validate_request_timestamp(&input.timestamp, now, state.max_clock_skew_seconds) {
            Ok(value) => value,
            Err(error) => {
                return (StatusCode::UNAUTHORIZED, Json(json!({ "error": error }))).into_response()
            }
        };
    match state
        .nonce_store
        .claim(&nonce, request_timestamp, state.max_clock_skew_seconds)
        .await
    {
        Ok(true) => {}
        Ok(false) => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({"error":"security request replay detected"})),
            )
                .into_response()
        }
        Err(error) => {
            tracing::error!(%error, "security nonce claim failed");
            return (
                StatusCode::SERVICE_UNAVAILABLE,
                Json(json!({"error":"security replay store unavailable"})),
            )
                .into_response();
        }
    }
    next.run(Request::from_parts(parts, Body::from(bytes)))
        .await
}

fn validate_request_timestamp(
    timestamp: &str,
    now: i64,
    max_clock_skew_seconds: i64,
) -> Result<i64, String> {
    let parsed = timestamp
        .parse::<i64>()
        .map_err(|_| "invalid security timestamp".to_string())?;
    if now.abs_diff(parsed) > max_clock_skew_seconds as u64 {
        return Err("security timestamp outside allowed window".to_string());
    }
    Ok(parsed)
}

async fn run_nonce_cleanup(store: PostgresNonceStore, max_clock_skew_seconds: i64) {
    let interval_seconds = max_clock_skew_seconds.max(30) as u64;
    let mut interval = tokio::time::interval(Duration::from_secs(interval_seconds));
    loop {
        interval.tick().await;
        match store.delete_expired().await {
            Ok(deleted) if deleted > 0 => {
                tracing::debug!(deleted, "expired security nonces deleted");
            }
            Ok(_) => {}
            Err(error) => tracing::error!(%error, "expired security nonce cleanup failed"),
        }
    }
}

async fn identity_challenge(Json(input): Json<serde_json::Value>) -> impl IntoResponse {
    let subject_id = input
        .get("subject_id")
        .and_then(|value| value.as_str())
        .and_then(|value| Uuid::parse_str(value).ok())
        .unwrap_or_else(Uuid::new_v4);
    let purpose = input
        .get("purpose")
        .and_then(|value| value.as_str())
        .unwrap_or("register_user");
    Json(IdentityChallenge::new(subject_id, purpose, 300)).into_response()
}

async fn identity_verify(Json(input): Json<VerifyIdentityRequest>) -> impl IntoResponse {
    match verify_challenge_signature(
        &input.challenge,
        &input.algorithm,
        &input.public_key,
        &input.signature,
    ) {
        Ok(verified) => Json(json!({"verified": true, "identity": verified})).into_response(),
        Err(err) => (
            StatusCode::UNAUTHORIZED,
            Json(json!({"verified": false, "error": err.to_string()})),
        )
            .into_response(),
    }
}

async fn ownership_attest(Json(input): Json<VerifyIdentityRequest>) -> impl IntoResponse {
    identity_verify(Json(input)).await
}

async fn authorize(Json(input): Json<AuthorizationRequest>) -> impl IntoResponse {
    Json(evaluate_authorization(&input))
}

#[allow(dead_code)]
async fn append_audit_event(Json(input): Json<HashEventInput>) -> impl IntoResponse {
    Json(append_hash_event(input))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn timestamp_validation_enforces_window() {
        assert!(validate_request_timestamp("100", 120, 30).is_ok());
        assert!(validate_request_timestamp("80", 120, 30).is_err());
        assert!(validate_request_timestamp("invalid", 120, 30).is_err());
    }

    #[tokio::test]
    async fn postgres_nonce_is_claimed_once_across_store_instances() {
        let Ok(database_url) = env::var("SECURITY_KERNEL_TEST_DATABASE_URL") else {
            return;
        };
        let first = PostgresNonceStore::connect(&database_url).await.unwrap();
        let second = PostgresNonceStore::connect(&database_url).await.unwrap();
        first
            .client_for_test()
            .batch_execute(
                r#"
                CREATE SCHEMA IF NOT EXISTS platform;
                CREATE TABLE IF NOT EXISTS platform.security_request_nonces (
                    nonce UUID PRIMARY KEY,
                    request_timestamp TIMESTAMPTZ NOT NULL,
                    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                    expires_at TIMESTAMPTZ NOT NULL,
                    metadata JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object')
                );
                "#,
            )
            .await
            .unwrap();
        assert!(first.is_ready().await.unwrap());
        let nonce = Uuid::new_v4();
        let now = now_unix_seconds() as i64;

        assert!(first.claim(&nonce, now, 60).await.unwrap());
        assert!(!second.claim(&nonce, now, 60).await.unwrap());
        first.delete_for_test(&nonce).await.unwrap();
    }
}
