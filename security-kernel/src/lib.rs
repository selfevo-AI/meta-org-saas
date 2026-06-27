use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use hmac::{Hmac, Mac};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    error::Error,
    fmt,
    time::{SystemTime, UNIX_EPOCH},
};
use uuid::Uuid;

type HmacSha256 = Hmac<Sha256>;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SecurityError {
    message: String,
}

impl SecurityError {
    fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
        }
    }
}

impl fmt::Display for SecurityError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

impl Error for SecurityError {}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum DistributionMode {
    #[serde(rename = "saas")]
    SaaS,
    #[serde(rename = "saas_org_private")]
    SaaSOrgPrivate,
    #[serde(rename = "single_org_commercial")]
    SingleOrgCommercial,
    #[serde(rename = "private_deployment")]
    PrivateDeployment,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum LicenseMode {
    Community,
    Commercial,
    Enterprise,
    PrivateContract,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct Actor {
    pub actor_id: Uuid,
    pub actor_type: String,
    pub authority_tier: String,
    pub is_platform_admin: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct Resource {
    pub module_key: String,
    pub resource_type: String,
    pub action: String,
    pub scope_level: String,
    pub organization_id: Option<Uuid>,
    pub required_authority_tier: String,
    pub required_license_mode: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct AuthorizationRequest {
    pub actor: Actor,
    pub organization_id: Option<Uuid>,
    pub distribution_mode: DistributionMode,
    pub license_mode: LicenseMode,
    pub enabled_modules: Vec<String>,
    pub enabled_features: Vec<String>,
    pub resource: Resource,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct AuthorizationDecision {
    pub allowed: bool,
    pub reason: String,
    pub decision_type: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct IdentityChallenge {
    pub challenge_id: Uuid,
    pub subject_id: Uuid,
    pub purpose: String,
    pub expires_in_seconds: u64,
    pub nonce: String,
}

impl IdentityChallenge {
    pub fn new(subject_id: Uuid, purpose: impl Into<String>, expires_in_seconds: u64) -> Self {
        Self {
            challenge_id: Uuid::new_v4(),
            subject_id,
            purpose: purpose.into(),
            expires_in_seconds,
            nonce: Uuid::new_v4().to_string(),
        }
    }

    pub fn payload(&self) -> String {
        format!(
            "meta-org:{}:{}:{}:{}:{}",
            self.challenge_id, self.subject_id, self.purpose, self.expires_in_seconds, self.nonce
        )
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct VerifiedIdentity {
    pub algorithm: String,
    pub fingerprint: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct HashEventInput {
    pub previous_hash: Option<String>,
    pub event_type: String,
    pub subject_id: Uuid,
    pub payload_hash: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct HashEvent {
    pub previous_hash: Option<String>,
    pub event_hash: String,
    pub event_type: String,
    pub subject_id: Uuid,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct VerifyIdentityRequest {
    pub challenge: IdentityChallenge,
    pub algorithm: String,
    pub public_key: String,
    pub signature: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ServiceSignatureInput {
    pub timestamp: String,
    pub body: Vec<u8>,
}

pub fn evaluate_authorization(request: &AuthorizationRequest) -> AuthorizationDecision {
    if !license_allows(
        &request.license_mode,
        &request.resource.required_license_mode,
    ) {
        return deny("license_denied");
    }

    let module_key = request.resource.module_key.trim();
    if !module_key.is_empty()
        && module_key != "general"
        && !request
            .enabled_modules
            .iter()
            .any(|item| item == module_key)
    {
        return deny("module_disabled");
    }

    match request.resource.scope_level.as_str() {
        "saas_global" if !request.actor.is_platform_admin => {
            return deny("platform_admin_required")
        }
        "organization" => {
            if request.resource.organization_id.is_some()
                && request.organization_id != request.resource.organization_id
                && !request.actor.is_platform_admin
            {
                return deny("tenant_mismatch");
            }
        }
        "deployment" | "" => {}
        _ => return deny("unsupported_scope"),
    }

    if !authority_allows(
        &request.actor.authority_tier,
        &request.resource.required_authority_tier,
        request.actor.is_platform_admin,
    ) {
        return deny("authority_denied");
    }

    AuthorizationDecision {
        allowed: true,
        reason: "allowed".to_string(),
        decision_type: "allow".to_string(),
    }
}

pub fn verify_challenge_signature(
    challenge: &IdentityChallenge,
    algorithm: &str,
    public_key_b64: &str,
    signature_b64: &str,
) -> Result<VerifiedIdentity, SecurityError> {
    if algorithm != "ed25519" {
        return Err(SecurityError::new("unsupported signature algorithm"));
    }
    let public_key = STANDARD
        .decode(public_key_b64)
        .map_err(|_| SecurityError::new("invalid public key encoding"))?;
    let signature = STANDARD
        .decode(signature_b64)
        .map_err(|_| SecurityError::new("invalid signature encoding"))?;
    let public_key: [u8; 32] = public_key
        .try_into()
        .map_err(|_| SecurityError::new("invalid ed25519 public key length"))?;
    let signature: [u8; 64] = signature
        .try_into()
        .map_err(|_| SecurityError::new("invalid ed25519 signature length"))?;
    let verifying_key = VerifyingKey::from_bytes(&public_key)
        .map_err(|_| SecurityError::new("invalid ed25519 public key"))?;
    let signature = Signature::from_bytes(&signature);
    verifying_key
        .verify(challenge.payload().as_bytes(), &signature)
        .map_err(|_| SecurityError::new("signature verification failed"))?;

    Ok(VerifiedIdentity {
        algorithm: "ed25519".to_string(),
        fingerprint: hex_sha256(&public_key),
    })
}

pub fn append_hash_event(input: HashEventInput) -> HashEvent {
    let previous = input
        .previous_hash
        .clone()
        .unwrap_or_else(|| "genesis".to_string());
    let material = format!(
        "{}:{}:{}:{}",
        previous, input.event_type, input.subject_id, input.payload_hash
    );
    HashEvent {
        previous_hash: input.previous_hash,
        event_hash: hex_sha256(material.as_bytes()),
        event_type: input.event_type,
        subject_id: input.subject_id,
    }
}

pub fn service_signature(
    secret: &str,
    input: &ServiceSignatureInput,
) -> Result<String, SecurityError> {
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes())
        .map_err(|_| SecurityError::new("invalid service secret"))?;
    mac.update(input.timestamp.as_bytes());
    mac.update(b".");
    mac.update(&input.body);
    Ok(hex_encode(&mac.finalize().into_bytes()))
}

pub fn verify_service_signature(
    secret: &str,
    input: &ServiceSignatureInput,
    signature: &str,
) -> Result<(), SecurityError> {
    let expected = service_signature(secret, input)?;
    if !constant_time_eq(expected.as_bytes(), signature.as_bytes()) {
        return Err(SecurityError::new("service signature verification failed"));
    }
    Ok(())
}

pub fn now_unix_seconds() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or(0)
}

fn deny(reason: &str) -> AuthorizationDecision {
    AuthorizationDecision {
        allowed: false,
        reason: reason.to_string(),
        decision_type: "deny".to_string(),
    }
}

fn authority_allows(actual: &str, required: &str, platform_admin: bool) -> bool {
    if platform_admin {
        return true;
    }
    authority_weight(actual) >= authority_weight(required) && authority_weight(required) > 0
}

fn authority_weight(value: &str) -> i32 {
    match value {
        "executor" => 1,
        "reviewer" => 2,
        "organization_admin" => 3,
        "organization_creator" => 4,
        _ => 0,
    }
}

fn license_allows(actual: &LicenseMode, required: &str) -> bool {
    license_weight(actual) >= license_weight_str(required)
}

fn license_weight(value: &LicenseMode) -> i32 {
    match value {
        LicenseMode::Community => 1,
        LicenseMode::Commercial => 2,
        LicenseMode::Enterprise => 3,
        LicenseMode::PrivateContract => 4,
    }
}

fn license_weight_str(value: &str) -> i32 {
    match value {
        "" | "community" => 1,
        "commercial" => 2,
        "enterprise" => 3,
        "private_contract" => 4,
        _ => 5,
    }
}

fn hex_sha256(input: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(input);
    hex_encode(&hasher.finalize())
}

fn hex_encode(input: &[u8]) -> String {
    const TABLE: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(input.len() * 2);
    for byte in input {
        out.push(TABLE[(byte >> 4) as usize] as char);
        out.push(TABLE[(byte & 0x0f) as usize] as char);
    }
    out
}

fn constant_time_eq(left: &[u8], right: &[u8]) -> bool {
    if left.len() != right.len() {
        return false;
    }
    let mut diff = 0u8;
    for (a, b) in left.iter().zip(right.iter()) {
        diff |= a ^ b;
    }
    diff == 0
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn authorization_request_accepts_saas_distribution_mode() {
        let input = r#"{
            "actor": {
                "actor_id": "123e4567-e89b-12d3-a456-426614174000",
                "actor_type": "human",
                "authority_tier": "organization_creator",
                "is_platform_admin": false
            },
            "organization_id": null,
            "distribution_mode": "saas",
            "license_mode": "commercial",
            "enabled_modules": [],
            "enabled_features": ["owner_attestation"],
            "resource": {
                "module_key": "general",
                "resource_type": "owner_attestation",
                "action": "verify",
                "scope_level": "deployment",
                "organization_id": null,
                "required_authority_tier": "organization_creator",
                "required_license_mode": "community"
            }
        }"#;

        let request: AuthorizationRequest =
            serde_json::from_str(input).expect("deserialize saas authorization request");

        assert_eq!(request.distribution_mode, DistributionMode::SaaS);
    }
}
