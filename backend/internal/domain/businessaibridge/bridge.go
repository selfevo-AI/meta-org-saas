package businessaibridge

import (
	"context"
	"fmt"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/businessai"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
)

type ToolService interface {
	ExecuteTool(context.Context, toolruntime.ExecuteToolInput) (*toolruntime.ExecuteToolOutput, error)
}

type Bridge struct {
	businessAI *businessai.Service
	tools      ToolService
}

func New(businessAI *businessai.Service) *Bridge {
	return &Bridge{businessAI: businessAI}
}

func (b *Bridge) SetToolService(tools ToolService) {
	b.tools = tools
}

func (b *Bridge) ExecuteProposal(ctx context.Context, input businessai.ProposalExecutionRequest) (*businessai.ProposalExecutionOutput, error) {
	if b.tools == nil {
		return nil, fmt.Errorf("business AI proposal tool service is not configured")
	}
	organizationID := input.OrganizationID
	projectID := input.ProjectID
	output, err := b.tools.ExecuteTool(ctx, toolruntime.ExecuteToolInput{
		ToolName: input.ToolName, InvocationID: input.InvocationID, ActorID: input.ActorID, ActorType: input.ActorType,
		OrganizationID: &organizationID, ProjectID: &projectID, IdempotencyKey: input.IdempotencyKey,
		RequireApproval: input.RequireApproval, Arguments: input.Arguments,
	})
	if err != nil {
		return nil, err
	}
	if output == nil || output.Execution == nil {
		return nil, fmt.Errorf("tool runtime returned no execution")
	}
	result := &businessai.ProposalExecutionOutput{
		ExecutionID: output.Execution.ID,
		Status:      proposalStatus(output.Execution.Status),
		Result:      output.Execution.Result,
		Error:       output.Execution.ErrorMessage,
	}
	if output.Approval != nil {
		id := output.Approval.ID
		result.ApprovalID = &id
	}
	return result, nil
}

func (b *Bridge) ToolExecutionUpdated(ctx context.Context, execution *toolruntime.ToolExecution) error {
	if b.businessAI == nil || execution == nil {
		return nil
	}
	return b.businessAI.UpdateProposalExecution(ctx, businessai.ProposalExecutionUpdate{
		ExecutionID: execution.ID,
		Status:      proposalStatus(execution.Status),
		Result:      execution.Result,
		Error:       execution.ErrorMessage,
	})
}

func proposalStatus(status string) string {
	switch status {
	case toolruntime.ExecutionApprovalRequired, toolruntime.ExecutionApproved, toolruntime.ExecutionRunning:
		return businessai.ProposalApprovalRequired
	case toolruntime.ExecutionCompleted:
		return businessai.ProposalCompleted
	case toolruntime.ExecutionRejected:
		return businessai.ProposalRejected
	case toolruntime.ExecutionDenied:
		return businessai.ProposalDenied
	case toolruntime.ExecutionFailed:
		return businessai.ProposalFailed
	default:
		return businessai.ProposalSubmitting
	}
}
