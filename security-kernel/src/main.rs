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
    append_hash_event, evaluate_authorization, verify_challenge_signature,
    verify_service_signature, AuthorizationRequest, HashEventInput, IdentityChallenge,
    ServiceSignatureInput, VerifyIdentityRequest,
};
use serde_json::json;
use std::{env, net::SocketAddr, sync::Arc};
use tower_http::trace::TraceLayer;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};
use uuid::Uuid;

#[derive(Clone)]
struct AppState {
    shared_secret: Arc<String>,
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
    let state = AppState {
        shared_secret: Arc::new(env::var("SECURITY_KERNEL_SHARED_SECRET").unwrap_or_default()),
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
    if req.uri().path() == "/healthz" || state.shared_secret.is_empty() {
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
        body: bytes.to_vec(),
    };
    if let Err(err) = verify_service_signature(&state.shared_secret, &input, &signature) {
        return (
            StatusCode::UNAUTHORIZED,
            Json(json!({"error": err.to_string()})),
        )
            .into_response();
    }
    next.run(Request::from_parts(parts, Body::from(bytes)))
        .await
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
