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
use std::{collections::HashMap, env, net::SocketAddr, sync::Arc};
use tokio::sync::Mutex;
use tower_http::trace::TraceLayer;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};
use uuid::Uuid;

#[derive(Clone)]
struct AppState {
    shared_secret: Arc<String>,
    max_clock_skew_seconds: i64,
    seen_nonces: Arc<Mutex<HashMap<String, i64>>>,
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
    let state = AppState {
        shared_secret: Arc::new(shared_secret),
        max_clock_skew_seconds,
        seen_nonces: Arc::new(Mutex::new(HashMap::new())),
    };
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

async fn healthz() -> impl IntoResponse {
    Json(json!({"status":"ok"}))
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
        Some(value) if Uuid::parse_str(value).is_ok() => value.to_string(),
        _ => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({"error":"missing or invalid security nonce"})),
            )
                .into_response()
        }
    };
    let now = now_unix_seconds() as i64;
    if let Err(error) = validate_request_timestamp(&timestamp, now, state.max_clock_skew_seconds) {
        return (StatusCode::UNAUTHORIZED, Json(json!({"error": error}))).into_response();
    }
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
        nonce: nonce.clone(),
        body: bytes.to_vec(),
    };
    if let Err(err) = verify_service_signature(&state.shared_secret, &input, &signature) {
        return (
            StatusCode::UNAUTHORIZED,
            Json(json!({"error": err.to_string()})),
        )
            .into_response();
    }
    if !claim_nonce(&state, &nonce, now).await {
        return (
            StatusCode::UNAUTHORIZED,
            Json(json!({"error":"security request replay detected"})),
        )
            .into_response();
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

async fn claim_nonce(state: &AppState, nonce: &str, now: i64) -> bool {
    let mut seen = state.seen_nonces.lock().await;
    seen.retain(|_, seen_at| now.saturating_sub(*seen_at) <= state.max_clock_skew_seconds);
    if seen.contains_key(nonce) {
        return false;
    }
    seen.insert(nonce.to_string(), now);
    true
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
    async fn nonce_can_only_be_claimed_once_within_window() {
        let state = AppState {
            shared_secret: Arc::new("test-secret".to_string()),
            max_clock_skew_seconds: 60,
            seen_nonces: Arc::new(Mutex::new(HashMap::new())),
        };
        let nonce = Uuid::new_v4().to_string();

        assert!(claim_nonce(&state, &nonce, 100).await);
        assert!(!claim_nonce(&state, &nonce, 101).await);
        assert!(claim_nonce(&state, &nonce, 200).await);
    }
}
