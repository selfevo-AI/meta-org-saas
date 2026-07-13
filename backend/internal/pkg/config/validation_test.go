package config

import (
	"strings"
	"testing"
)

const testBcryptHash = "$2a$10$/Dou0gOhCVFNGMitu8IUu.92HzEaG6iYWGxTTVUrSA1pkFvogvj22"

func TestLoadValidatedRejectsMalformedEnvironmentValues(t *testing.T) {
	t.Setenv("SERVER_PORT", "not-a-number")
	t.Setenv("TENANT_PROJECTION_WORKER_ENABLED", "sometimes")

	_, err := LoadValidated()
	if err == nil || !strings.Contains(err.Error(), "SERVER_PORT must be an integer") || !strings.Contains(err.Error(), "TENANT_PROJECTION_WORKER_ENABLED must be a boolean") {
		t.Fatalf("LoadValidated() error = %v", err)
	}
}

func TestValidateRejectsUnsafeProductionSecrets(t *testing.T) {
	cfg := validTestConfig()
	cfg.Environment = "production"
	cfg.MetaOrgMode = "saas"
	cfg.PlatformAdminEmail = "platform-admin@example.com"
	cfg.PlatformAdminPasswordHash = testBcryptHash
	cfg.SecurityKernelURL = "http://security-kernel:8090"
	cfg.SecurityKernelSharedSecret = developmentKernelKey
	cfg.SecurityKernelEnforcementMode = "blocking"
	cfg.JWTSecret = developmentJWTSecret
	cfg.ModelSecretKey = composeModelSecret

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	for _, expected := range []string{"production JWT_SECRET", "production MODEL_SECRET_KEY", "production SECURITY_KERNEL_SHARED_SECRET"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("Validate() error = %v, missing %q", err, expected)
		}
	}
}

func TestValidateAcceptsProductionSaaSConfiguration(t *testing.T) {
	cfg := validTestConfig()
	cfg.Environment = "production"
	cfg.MetaOrgMode = "saas"
	cfg.PlatformAdminEmail = "platform-admin@example.com"
	cfg.PlatformAdminPasswordHash = testBcryptHash
	cfg.SecurityKernelURL = "https://security-kernel.internal:8090"
	cfg.SecurityKernelSharedSecret = strings.Repeat("k", 40)
	cfg.SecurityKernelEnforcementMode = "blocking"
	cfg.JWTSecret = strings.Repeat("j", 40)
	cfg.ModelSecretKey = strings.Repeat("m", 32)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidRuntimeBounds(t *testing.T) {
	cfg := validTestConfig()
	cfg.TenantDatabasePoolMinConnections = cfg.TenantDatabasePoolMaxConnections + 1
	cfg.TenantProvisioningRetrySeconds = 60
	cfg.TenantProvisioningRetryMaxSeconds = 10
	cfg.AuthRateLimitFailureThreshold = cfg.AuthRateLimitMaxAttempts + 1
	cfg.MonitoringAgentDailyTime = "25:00"
	cfg.AIGatewayStreamTimeoutSeconds = 0
	cfg.AIGatewayReservationStaleSeconds = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	for _, expected := range []string{
		"TENANT_DATABASE_POOL_MIN_CONNECTIONS", "TENANT_PROVISIONING_RETRY_MAX_SECONDS",
		"AUTH_RATE_LIMIT_FAILURE_THRESHOLD", "MONITORING_AGENT_DAILY_TIME",
		"AI_GATEWAY_STREAM_TIMEOUT_SECONDS", "AI_GATEWAY_RESERVATION_STALE_SECONDS",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("Validate() error = %v, missing %q", err, expected)
		}
	}
}

func TestValidateRequiresReservationRecoveryAfterStreamCeiling(t *testing.T) {
	cfg := validTestConfig()
	cfg.AIGatewayStreamTimeoutSeconds = 900
	cfg.AIGatewayReservationStaleSeconds = 900

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "AI_GATEWAY_RESERVATION_STALE_SECONDS must be greater than AI_GATEWAY_STREAM_TIMEOUT_SECONDS") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func validTestConfig() *Config {
	cfg := Load()
	cfg.Environment = "test"
	cfg.MetaOrgMode = "single_org"
	cfg.SecurityKernelURL = ""
	cfg.SecurityKernelSharedSecret = ""
	return cfg
}
