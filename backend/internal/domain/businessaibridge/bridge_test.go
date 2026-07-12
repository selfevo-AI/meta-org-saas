package businessaibridge

import (
	"testing"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/businessai"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
)

func TestProposalStatusMapsToolRuntimeTerminalStates(t *testing.T) {
	tests := map[string]string{
		toolruntime.ExecutionApprovalRequired: businessai.ProposalApprovalRequired,
		toolruntime.ExecutionCompleted:        businessai.ProposalCompleted,
		toolruntime.ExecutionRejected:         businessai.ProposalRejected,
		toolruntime.ExecutionDenied:           businessai.ProposalDenied,
		toolruntime.ExecutionFailed:           businessai.ProposalFailed,
	}
	for input, want := range tests {
		if got := proposalStatus(input); got != want {
			t.Fatalf("proposalStatus(%q) = %q, want %q", input, got, want)
		}
	}
}
