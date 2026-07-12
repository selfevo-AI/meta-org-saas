package config

import (
	"reflect"
	"testing"
)

func TestLoadAuthenticationProtectionConfig(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.168.0.0/16")
	t.Setenv("AUTH_RATE_LIMIT_WINDOW_SECONDS", "90")
	t.Setenv("AUTH_RATE_LIMIT_MAX_ATTEMPTS", "12")
	t.Setenv("AUTH_RATE_LIMIT_FAILURE_THRESHOLD", "4")
	t.Setenv("AUTH_RATE_LIMIT_BLOCK_SECONDS", "600")
	t.Setenv("AUTH_REGISTRATION_MAX_ATTEMPTS", "3")
	t.Setenv("SENSITIVE_RATE_LIMIT_WINDOW_SECONDS", "120")
	t.Setenv("SENSITIVE_RATE_LIMIT_MAX_ATTEMPTS", "8")
	t.Setenv("SENSITIVE_RATE_LIMIT_BLOCK_SECONDS", "900")
	t.Setenv("INVITATION_ACCEPT_RATE_LIMIT_MAX_ATTEMPTS", "4")
	t.Setenv("AI_GATEWAY_RATE_LIMIT_WINDOW_SECONDS", "30")
	t.Setenv("AI_GATEWAY_RATE_LIMIT_MAX_REQUESTS", "55")
	t.Setenv("AI_GATEWAY_RATE_LIMIT_BLOCK_SECONDS", "45")

	cfg := Load()

	if !reflect.DeepEqual(cfg.TrustedProxyCIDRs, []string{"10.0.0.0/8", "192.168.0.0/16"}) {
		t.Fatalf("TrustedProxyCIDRs = %#v", cfg.TrustedProxyCIDRs)
	}
	if cfg.AuthRateLimitWindowSeconds != 90 || cfg.AuthRateLimitMaxAttempts != 12 || cfg.AuthRateLimitFailureThreshold != 4 {
		t.Fatalf("auth rate limit window/max/failure = %d/%d/%d", cfg.AuthRateLimitWindowSeconds, cfg.AuthRateLimitMaxAttempts, cfg.AuthRateLimitFailureThreshold)
	}
	if cfg.AuthRateLimitBlockSeconds != 600 || cfg.AuthRegistrationMaxAttempts != 3 {
		t.Fatalf("auth block/registration max = %d/%d", cfg.AuthRateLimitBlockSeconds, cfg.AuthRegistrationMaxAttempts)
	}
	if cfg.SensitiveRateLimitWindowSeconds != 120 || cfg.SensitiveRateLimitMaxAttempts != 8 || cfg.SensitiveRateLimitBlockSeconds != 900 || cfg.InvitationAcceptMaxAttempts != 4 {
		t.Fatalf("sensitive rate limit config = %d/%d/%d invitation=%d", cfg.SensitiveRateLimitWindowSeconds, cfg.SensitiveRateLimitMaxAttempts, cfg.SensitiveRateLimitBlockSeconds, cfg.InvitationAcceptMaxAttempts)
	}
	if cfg.AIGatewayRateLimitWindowSeconds != 30 || cfg.AIGatewayRateLimitMaxRequests != 55 || cfg.AIGatewayRateLimitBlockSeconds != 45 {
		t.Fatalf("AI gateway rate limit config = %d/%d/%d", cfg.AIGatewayRateLimitWindowSeconds, cfg.AIGatewayRateLimitMaxRequests, cfg.AIGatewayRateLimitBlockSeconds)
	}
}
