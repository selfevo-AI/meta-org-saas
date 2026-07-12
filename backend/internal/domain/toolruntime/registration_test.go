package toolruntime

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestContextProposalToolsOnlyRegistersIncrementalAdapter(t *testing.T) {
	tools := ContextProposalTools(fakeContextProposalService{})
	if len(tools) != 1 || tools["context.proposal.apply"] == nil {
		t.Fatalf("tools = %#v, want only context.proposal.apply", tools)
	}
}

type fakeContextProposalService struct{}

func (fakeContextProposalService) ApplyContextProposal(context.Context, uuid.UUID, uuid.UUID, string) (map[string]any, error) {
	return map[string]any{}, nil
}
