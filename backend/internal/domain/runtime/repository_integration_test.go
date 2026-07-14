package runtime

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func TestRuntimeOperationRepositoryAcceptsOptionalEntityKey(t *testing.T) {
	if os.Getenv("RUN_RUNTIME_DB_TEST") != "1" {
		t.Skip("set RUN_RUNTIME_DB_TEST=1 to run runtime database verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := os.Getenv("RUNTIME_TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/meta_org_saas?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := NewRepository(pool)

	operation, err := repo.GetOperation(ctx, "erp.finance.trial_balance.run")
	if err != nil {
		t.Fatalf("GetOperation() error = %v", err)
	}
	if operation.EntityKey != "" || operation.ActionType != "finance.gl.trial_balance" {
		t.Fatalf("operation entity/action = %q/%q", operation.EntityKey, operation.ActionType)
	}

	operations, err := repo.ListOperations(ctx)
	if err != nil {
		t.Fatalf("ListOperations() error = %v", err)
	}
	found := false
	for _, item := range operations {
		if item.ID == operation.ID {
			found = true
			if item.EntityKey != "" {
				t.Fatalf("listed operation entity_key = %q, want empty", item.EntityKey)
			}
			break
		}
	}
	if !found {
		t.Fatalf("ListOperations() did not include %q", operation.ID)
	}

	financeService := &integrationTrialBalanceService{balance: &finance.GLTrialBalance{Rows: []finance.GLTrialBalanceRow{}, Currency: "CNY"}}
	service := NewService(repo, WithOperationAdapter(ActionFinanceGLTrialBalance, NewFinanceTrialBalanceAdapter(financeService)))
	organizationID := uuid.New()
	tenantCtx := context.WithValue(ctx, middleware.TenantContextKey, &middleware.TenantContext{
		Mode: "saas", OrganizationID: &organizationID, EnabledModules: map[string]bool{"finance": true},
	})
	result, err := service.ExecuteAssistantOperation(tenantCtx, operation.ID, RuntimeExecutionRequest{Query: map[string]string{"currency": "CNY"}})
	if err != nil {
		t.Fatalf("ExecuteAssistantOperation() error = %v", err)
	}
	if result.Status != "ok" || financeService.input.Currency != "CNY" {
		t.Fatalf("assistant runtime result/input = %#v/%#v", result, financeService.input)
	}

	tenantURL := os.Getenv("RUNTIME_TENANT_TEST_DATABASE_URL")
	if tenantURL == "" {
		t.Log("RUNTIME_TENANT_TEST_DATABASE_URL is not set; skipping live tenant finance adapter verification")
		return
	}
	tenantPool, err := pgxpool.New(ctx, tenantURL)
	if err != nil {
		t.Fatal(err)
	}
	defer tenantPool.Close()
	liveFinanceService := finance.NewService(finance.NewRepository(tenantPool, nil))
	liveService := NewService(repo, WithOperationAdapter(ActionFinanceGLTrialBalance, NewFinanceTrialBalanceAdapter(liveFinanceService)))
	liveResult, err := liveService.ExecuteAssistantOperation(tenantCtx, operation.ID, RuntimeExecutionRequest{Query: map[string]string{"currency": "CNY"}})
	if err != nil {
		t.Fatalf("live ExecuteAssistantOperation() error = %v", err)
	}
	liveBalance, ok := liveResult.Data.(*finance.GLTrialBalance)
	if !ok || liveBalance.Rows == nil || liveBalance.Currency != "CNY" {
		t.Fatalf("live trial balance result = %#v", liveResult)
	}
}

type integrationTrialBalanceService struct {
	input   finance.GLTrialBalanceInput
	balance *finance.GLTrialBalance
}

func (s *integrationTrialBalanceService) GetGLTrialBalance(_ context.Context, input finance.GLTrialBalanceInput) (*finance.GLTrialBalance, error) {
	s.input = input
	return s.balance, nil
}
