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
}
