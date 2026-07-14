package runtime

import (
	"context"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
)

const ActionFinanceGLTrialBalance = "finance.gl.trial_balance"

type FinanceTrialBalanceService interface {
	GetGLTrialBalance(context.Context, finance.GLTrialBalanceInput) (*finance.GLTrialBalance, error)
}

func NewFinanceTrialBalanceAdapter(service FinanceTrialBalanceService) OperationAdapter {
	return OperationAdapterFunc(func(ctx context.Context, _ OperationDefinition, input RuntimeExecutionRequest) (*RuntimeExecutionResult, error) {
		balance, err := service.GetGLTrialBalance(ctx, finance.GLTrialBalanceInput{
			PeriodStart: firstNonEmpty(input.Query["period_start"], stringFromBody(input.Body, "period_start")),
			PeriodEnd:   firstNonEmpty(input.Query["period_end"], stringFromBody(input.Body, "period_end")),
			Currency:    firstNonEmpty(input.Query["currency"], stringFromBody(input.Body, "currency")),
		})
		if err != nil {
			return nil, err
		}
		return &RuntimeExecutionResult{Status: "ok", Data: balance}, nil
	})
}
