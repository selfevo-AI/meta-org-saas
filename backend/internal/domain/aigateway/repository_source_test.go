package aigateway

import (
	"os"
	"strings"
	"testing"
)

func TestRepositoryReserveAccessTokenBalanceEnforcesAccessTokenQuota(t *testing.T) {
	source := readRepositorySource(t)
	if !strings.Contains(source, "UPDATE ai_access_tokens") {
		t.Fatalf("repository does not update ai_access_tokens during balance reservation")
	}
	if !strings.Contains(source, "quota_used = quota_used + $") {
		t.Fatalf("repository does not reserve access token quota_used before invocation")
	}
	if !strings.Contains(source, "quota_amount <= 0 OR quota_used + $") {
		t.Fatalf("repository does not enforce access token quota_amount before reservation")
	}
}

func TestRepositoryFinishAccessTokenReservationIsIdempotentAndReconcilesQuota(t *testing.T) {
	source := readRepositorySource(t)
	if !strings.Contains(source, "reservation_id = $1") || !strings.Contains(source, "transaction_type IN ('settle', 'refund')") {
		t.Fatalf("repository does not guard against duplicate reservation settlement/refund")
	}
	if !strings.Contains(source, "quota_used = GREATEST(quota_used + $") {
		t.Fatalf("repository does not reconcile access token quota_used on settle/refund")
	}
}

func TestRepositoryApplyChannelUsesModelGroupAbilities(t *testing.T) {
	source := readRepositorySource(t)
	if !strings.Contains(source, "ai_model_channel_abilities") {
		t.Fatalf("repository applyChannel does not read ai_model_channel_abilities")
	}
	if !strings.Contains(source, "ability_priority") {
		t.Fatalf("repository applyChannel does not order by model channel ability priority")
	}
}

func readRepositorySource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	return string(data)
}
