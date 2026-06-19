use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer, SigningKey};
use rand_core::OsRng;
use security_kernel::{
    append_hash_event, evaluate_authorization, verify_challenge_signature, Actor,
    AuthorizationRequest, DistributionMode, HashEventInput, IdentityChallenge, LicenseMode,
    Resource,
};
use uuid::Uuid;

#[test]
fn verifies_ed25519_challenge_signature() {
    let signing_key = SigningKey::generate(&mut OsRng);
    let verifying_key = signing_key.verifying_key();
    let challenge = IdentityChallenge::new(Uuid::new_v4(), "register_user", 300);
    let signature = signing_key.sign(challenge.payload().as_bytes());

    let verified = verify_challenge_signature(
        &challenge,
        "ed25519",
        &STANDARD.encode(verifying_key.as_bytes()),
        &STANDARD.encode(signature.to_bytes()),
    )
    .expect("signature should verify");

    assert_eq!(verified.algorithm, "ed25519");
    assert_eq!(verified.fingerprint.len(), 64);
}

#[test]
fn rejects_tampered_challenge_signature() {
    let signing_key = SigningKey::generate(&mut OsRng);
    let verifying_key = signing_key.verifying_key();
    let challenge = IdentityChallenge::new(Uuid::new_v4(), "owner_attestation", 300);
    let mut tampered = challenge.clone();
    tampered.purpose = "register_user".to_string();
    let signature = signing_key.sign(challenge.payload().as_bytes());

    let err = verify_challenge_signature(
        &tampered,
        "ed25519",
        &STANDARD.encode(verifying_key.as_bytes()),
        &STANDARD.encode(signature.to_bytes()),
    )
    .expect_err("tampered challenge must be rejected");

    assert!(err.to_string().contains("signature"));
}

#[test]
fn denies_disabled_commercial_feature() {
    let request = AuthorizationRequest {
        actor: Actor {
            actor_id: Uuid::new_v4(),
            actor_type: "human".to_string(),
            authority_tier: "organization_creator".to_string(),
            is_platform_admin: false,
        },
        organization_id: Some(Uuid::new_v4()),
        distribution_mode: DistributionMode::SingleOrgCommercial,
        license_mode: LicenseMode::Community,
        enabled_modules: vec!["organization".to_string()],
        enabled_features: vec![],
        resource: Resource {
            module_key: "ai_gateway".to_string(),
            resource_type: "model_provider_channel".to_string(),
            action: "use".to_string(),
            scope_level: "organization".to_string(),
            organization_id: None,
            required_authority_tier: "executor".to_string(),
            required_license_mode: "commercial".to_string(),
        },
    };

    let decision = evaluate_authorization(&request);

    assert!(!decision.allowed);
    assert_eq!(decision.reason, "license_denied");
}

#[test]
fn allows_owner_to_activate_organization_skill() {
    let org_id = Uuid::new_v4();
    let request = AuthorizationRequest {
        actor: Actor {
            actor_id: Uuid::new_v4(),
            actor_type: "human".to_string(),
            authority_tier: "organization_creator".to_string(),
            is_platform_admin: false,
        },
        organization_id: Some(org_id),
        distribution_mode: DistributionMode::SaaS,
        license_mode: LicenseMode::Commercial,
        enabled_modules: vec!["assistant".to_string()],
        enabled_features: vec!["skill_governance".to_string()],
        resource: Resource {
            module_key: "assistant".to_string(),
            resource_type: "skill".to_string(),
            action: "activate".to_string(),
            scope_level: "organization".to_string(),
            organization_id: Some(org_id),
            required_authority_tier: "organization_admin".to_string(),
            required_license_mode: "commercial".to_string(),
        },
    };

    let decision = evaluate_authorization(&request);

    assert!(decision.allowed, "unexpected denial: {}", decision.reason);
}

#[test]
fn appends_tamper_evident_hash_events() {
    let first = append_hash_event(HashEventInput {
        previous_hash: None,
        event_type: "identity_verified".to_string(),
        subject_id: Uuid::new_v4(),
        payload_hash: "abc".to_string(),
    });
    let second = append_hash_event(HashEventInput {
        previous_hash: Some(first.event_hash.clone()),
        event_type: "owner_attested".to_string(),
        subject_id: Uuid::new_v4(),
        payload_hash: "def".to_string(),
    });

    assert_eq!(second.previous_hash, Some(first.event_hash));
    assert_eq!(second.event_hash.len(), 64);
}
