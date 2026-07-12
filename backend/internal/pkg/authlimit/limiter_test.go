package authlimit

import (
	"strings"
	"testing"
	"time"
)

func TestHashKeyDoesNotExposeRawIdentity(t *testing.T) {
	raw := "user@example.com"
	hash := HashKey("user_login_subject", raw)
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
	if strings.Contains(hash, raw) {
		t.Fatalf("hash %q exposes raw key", hash)
	}
	if hash == HashKey("agent_login_subject", raw) {
		t.Fatal("hash must be scope-separated")
	}
}

func TestNormalizePolicyAppliesSafeDefaults(t *testing.T) {
	policy := normalizePolicy(Policy{})
	if policy.Window != time.Minute || policy.MaxAttempts != 10 || policy.FailureThreshold != 10 || policy.BlockDuration != 5*time.Minute {
		t.Fatalf("normalized policy = %#v", policy)
	}
}
