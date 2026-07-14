package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func runtimeTenantContext(enabledModules map[string]bool) context.Context {
	orgID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	return context.WithValue(context.Background(), middleware.TenantContextKey, &middleware.TenantContext{
		Mode:           "saas",
		OrganizationID: &orgID,
		EnabledModules: enabledModules,
	})
}

func TestValidateEntityDefinitionRejectsDuplicateFields(t *testing.T) {
	def := EntityDefinition{
		EntityKey:    "organization.department",
		ModuleKey:    "organization",
		StorageTable: "organization_masters",
		EntityType:   "department",
		Status:       StatusActive,
		Fields: []FieldDefinition{
			{FieldKey: "name", DataType: "text", Required: true},
			{FieldKey: "name", DataType: "text"},
		},
	}

	err := ValidateEntityDefinition(def)

	if err == nil {
		t.Fatal("ValidateEntityDefinition() succeeded, want duplicate field error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("ValidateEntityDefinition() error = %v, want ErrValidation", err)
	}
}

func TestListOperationsReturnsOnlyActiveOperations(t *testing.T) {
	repo := &fakeRepository{
		operations: []OperationDefinition{
			{
				ID:         "organization-list",
				Domain:     "Organization",
				Title:      "operation.organization.list",
				Method:     "GET",
				Path:       "/runtime/entities/organization.organization/records",
				Status:     StatusActive,
				ActionType: ActionCRUDList,
				EntityKey:  "organization.organization",
			},
			{
				ID:         "disabled-operation",
				Domain:     "Organization",
				Title:      "Disabled",
				Method:     "GET",
				Path:       "/disabled",
				Status:     StatusDisabled,
				ActionType: ActionCRUDList,
				EntityKey:  "organization.organization",
			},
		},
	}
	service := NewService(repo)

	ops, err := service.ListOperations(context.Background())

	if err != nil {
		t.Fatalf("ListOperations() error = %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("ListOperations() returned %d operations, want 1", len(ops))
	}
	if ops[0].ID != "organization-list" {
		t.Fatalf("ListOperations()[0].ID = %q, want organization-list", ops[0].ID)
	}
}

func TestListOperationsFiltersDisabledTenantModules(t *testing.T) {
	repo := &fakeRepository{
		operations: []OperationDefinition{
			{
				ID:         "inventory.item.list",
				Domain:     "inventory",
				Title:      "operation.runtime.inventory.item.list",
				Method:     "GET",
				Path:       "/runtime/entities/inventory.item/records",
				Status:     StatusActive,
				ActionType: ActionCRUDList,
				EntityKey:  "inventory.item",
			},
			{
				ID:         "procurement.purchase_order.list",
				Domain:     "procurement",
				Title:      "operation.runtime.procurement.purchase_order.list",
				Method:     "GET",
				Path:       "/runtime/entities/procurement.purchase_order/records",
				Status:     StatusActive,
				ActionType: ActionCRUDList,
				EntityKey:  "procurement.purchase_order",
			},
		},
	}
	service := NewService(repo)

	ops, err := service.ListOperations(runtimeTenantContext(map[string]bool{"inventory": true}))

	if err != nil {
		t.Fatalf("ListOperations() error = %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("ListOperations() returned %d operations, want 1", len(ops))
	}
	if ops[0].ID != "inventory.item.list" {
		t.Fatalf("ListOperations()[0].ID = %q, want inventory.item.list", ops[0].ID)
	}
}

func TestExecuteOperationDispatchesConfiguredCRUDCreate(t *testing.T) {
	repo := &fakeRepository{
		operationByID: map[string]OperationDefinition{
			"organization-create": {
				ID:         "organization-create",
				Domain:     "Organization",
				Title:      "operation.organization.create",
				Method:     "POST",
				Path:       "/runtime/operations/organization-create/execute",
				Status:     StatusActive,
				ActionType: ActionCRUDCreate,
				EntityKey:  "organization.organization",
			},
		},
		entityByKey: map[string]EntityDefinition{
			"organization.organization": {
				EntityKey:    "organization.organization",
				ModuleKey:    "organization",
				StorageTable: "organization_masters",
				EntityType:   "organization",
				Status:       StatusActive,
				Fields: []FieldDefinition{
					{FieldKey: "title", DataType: "text", Required: true},
					{FieldKey: "status", DataType: "text"},
				},
			},
		},
		createdRecord: &RuntimeRecord{
			MasterKey:  "ORG-20260621-000001",
			EntityKey:  "organization.organization",
			EntityType: "organization",
			Title:      "Acme",
			Status:     "active",
			Data:       map[string]any{"title": "Acme", "status": "active"},
			Metadata:   map[string]any{},
		},
	}
	service := NewService(repo)

	result, err := service.ExecuteOperation(context.Background(), "organization-create", RuntimeExecutionRequest{
		Body: map[string]any{"title": "Acme", "status": "active"},
	})

	if err != nil {
		t.Fatalf("ExecuteOperation() error = %v", err)
	}
	if repo.createdEntityKey != "organization.organization" {
		t.Fatalf("created entity = %q, want organization.organization", repo.createdEntityKey)
	}
	if result.Record == nil || result.Record.MasterKey != "ORG-20260621-000001" {
		t.Fatalf("ExecuteOperation() record = %#v, want created runtime record", result.Record)
	}
	if result.Status != "created" {
		t.Fatalf("ExecuteOperation() status = %q, want created", result.Status)
	}
}

func TestExecuteAssistantOperationRunsFinanceTrialBalanceAdapter(t *testing.T) {
	repo := &fakeRepository{operationByID: map[string]OperationDefinition{
		"erp.finance.trial_balance.run": {
			ID: "erp.finance.trial_balance.run", Domain: "ERP", Status: StatusActive,
			ActionType: ActionFinanceGLTrialBalance, AssistantEligible: true,
			Metadata: map[string]any{"workspace": map[string]any{"module": "finance"}},
		},
	}}
	financeService := &fakeFinanceTrialBalanceService{balance: &finance.GLTrialBalance{
		Rows: []finance.GLTrialBalanceRow{{AccountCode: "1001", Debit: 1200}}, TotalDebit: 1200, TotalCredit: 1200, Currency: "CNY",
	}}
	service := NewService(repo, WithOperationAdapter(ActionFinanceGLTrialBalance, NewFinanceTrialBalanceAdapter(financeService)))

	result, err := service.ExecuteAssistantOperation(runtimeTenantContext(map[string]bool{"finance": true}), "erp.finance.trial_balance.run", RuntimeExecutionRequest{
		Query: map[string]string{"period_start": "2026-01-01", "period_end": "2026-06-30", "currency": "CNY"},
	})
	if err != nil {
		t.Fatalf("ExecuteAssistantOperation() error = %v", err)
	}
	if financeService.input.PeriodStart != "2026-01-01" || financeService.input.PeriodEnd != "2026-06-30" || financeService.input.Currency != "CNY" {
		t.Fatalf("finance adapter input = %#v", financeService.input)
	}
	balance, ok := result.Data.(*finance.GLTrialBalance)
	if !ok || balance.TotalDebit != 1200 || result.Status != "ok" {
		t.Fatalf("runtime result = %#v", result)
	}
}

func TestExecuteAssistantOperationRejectsIneligibleCatalogOperation(t *testing.T) {
	repo := &fakeRepository{operationByID: map[string]OperationDefinition{
		"internal-only": {ID: "internal-only", Domain: "finance", Status: StatusActive, ActionType: ActionFinanceGLTrialBalance},
	}}
	service := NewService(repo, WithOperationAdapter(ActionFinanceGLTrialBalance, NewFinanceTrialBalanceAdapter(&fakeFinanceTrialBalanceService{})))

	_, err := service.ExecuteAssistantOperation(runtimeTenantContext(map[string]bool{"finance": true}), "internal-only", RuntimeExecutionRequest{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ExecuteAssistantOperation() error = %v, want ErrForbidden", err)
	}
}

func TestExecuteOperationHumanPathHonorsTenantModuleGate(t *testing.T) {
	// The human path falls back to the operation Domain (case-insensitive)
	// when no workspace.module metadata is present; it must keep working for
	// enabled modules and reject disabled ones.
	repo := &fakeRepository{operationByID: map[string]OperationDefinition{
		"finance.trial_balance.run": {
			ID: "finance.trial_balance.run", Domain: "Finance", Status: StatusActive,
			ActionType: ActionFinanceGLTrialBalance,
		},
	}}
	financeService := &fakeFinanceTrialBalanceService{balance: &finance.GLTrialBalance{Currency: "CNY"}}
	service := NewService(repo, WithOperationAdapter(ActionFinanceGLTrialBalance, NewFinanceTrialBalanceAdapter(financeService)))

	if _, err := service.ExecuteOperation(runtimeTenantContext(map[string]bool{"finance": true}), "finance.trial_balance.run", RuntimeExecutionRequest{}); err != nil {
		t.Fatalf("ExecuteOperation() with enabled module error = %v, want nil", err)
	}
	_, err := service.ExecuteOperation(runtimeTenantContext(map[string]bool{"inventory": true}), "finance.trial_balance.run", RuntimeExecutionRequest{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ExecuteOperation() with disabled module error = %v, want ErrForbidden", err)
	}
}

func TestExecuteAssistantOperationPropagatesFinanceAdapterError(t *testing.T) {
	repo := &fakeRepository{operationByID: map[string]OperationDefinition{
		"erp.finance.trial_balance.run": {
			ID: "erp.finance.trial_balance.run", Domain: "ERP", Status: StatusActive,
			ActionType: ActionFinanceGLTrialBalance, AssistantEligible: true,
			Metadata: map[string]any{"workspace": map[string]any{"module": "finance"}},
		},
	}}
	wantErr := errors.New("trial balance unavailable")
	financeService := &fakeFinanceTrialBalanceService{err: wantErr}
	service := NewService(repo, WithOperationAdapter(ActionFinanceGLTrialBalance, NewFinanceTrialBalanceAdapter(financeService)))

	result, err := service.ExecuteAssistantOperation(runtimeTenantContext(map[string]bool{"finance": true}), "erp.finance.trial_balance.run", RuntimeExecutionRequest{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ExecuteAssistantOperation() error = %v, want %v", err, wantErr)
	}
	if result != nil {
		t.Fatalf("ExecuteAssistantOperation() result = %#v, want nil on adapter error", result)
	}
}

func TestCreateRecordAcceptsTopLevelTitleForRequiredTitleField(t *testing.T) {
	repo := &fakeRepository{
		entityByKey: map[string]EntityDefinition{
			"organization.organization": {
				EntityKey:    "organization.organization",
				ModuleKey:    "organization",
				StorageTable: "organization_masters",
				EntityType:   "organization",
				Status:       StatusActive,
				Fields: []FieldDefinition{
					{FieldKey: "title", DataType: "text", Required: true},
				},
			},
		},
		createdRecord: &RuntimeRecord{
			MasterKey:  "ORG-20260621-000001",
			EntityKey:  "organization.organization",
			EntityType: "organization",
			Title:      "Acme",
			Status:     StatusActive,
			Data:       map[string]any{"title": "Acme"},
			Metadata:   map[string]any{},
		},
	}
	service := NewService(repo)

	_, err := service.CreateRecord(context.Background(), "organization.organization", RuntimeRecordInput{Title: "Acme"})

	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
}

func TestCreateRecordRejectsDisabledTenantModule(t *testing.T) {
	repo := &fakeRepository{
		entityByKey: map[string]EntityDefinition{
			"procurement.purchase_order": {
				EntityKey:    "procurement.purchase_order",
				ModuleKey:    "procurement",
				StorageTable: "procurement_masters",
				EntityType:   "purchase_order",
				Status:       StatusActive,
				Fields:       []FieldDefinition{{FieldKey: "title", DataType: "text", Required: true}},
			},
		},
		createdRecord: &RuntimeRecord{MasterKey: "PROCUR-20260621-000001"},
	}
	service := NewService(repo)

	_, err := service.CreateRecord(runtimeTenantContext(map[string]bool{"inventory": true}), "procurement.purchase_order", RuntimeRecordInput{Title: "PO-1"})

	if err == nil {
		t.Fatal("CreateRecord() succeeded, want forbidden error")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateRecord() error = %v, want ErrForbidden", err)
	}
	if repo.createdEntityKey != "" {
		t.Fatalf("CreateRecord() created entity = %q, want no create", repo.createdEntityKey)
	}
}

type fakeRepository struct {
	operations        []OperationDefinition
	operationByID     map[string]OperationDefinition
	entityByKey       map[string]EntityDefinition
	createdEntityKey  string
	createdRecord     *RuntimeRecord
	listRecordsResult []RuntimeRecord
}

type fakeFinanceTrialBalanceService struct {
	input   finance.GLTrialBalanceInput
	balance *finance.GLTrialBalance
	err     error
}

func (f *fakeFinanceTrialBalanceService) GetGLTrialBalance(_ context.Context, input finance.GLTrialBalanceInput) (*finance.GLTrialBalance, error) {
	f.input = input
	return f.balance, f.err
}

func (f *fakeRepository) ListOperations(context.Context) ([]OperationDefinition, error) {
	return f.operations, nil
}

func (f *fakeRepository) GetOperation(_ context.Context, operationID string) (*OperationDefinition, error) {
	if operation, ok := f.operationByID[operationID]; ok {
		return &operation, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepository) GetEntity(_ context.Context, entityKey string) (*EntityDefinition, error) {
	if entity, ok := f.entityByKey[entityKey]; ok {
		return &entity, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepository) ListRecords(context.Context, EntityDefinition, int) ([]RuntimeRecord, error) {
	return f.listRecordsResult, nil
}

func (f *fakeRepository) GetRecord(context.Context, EntityDefinition, string) (*RuntimeRecord, error) {
	return nil, ErrNotFound
}

func (f *fakeRepository) CreateRecord(_ context.Context, entity EntityDefinition, input RuntimeRecordInput) (*RuntimeRecord, error) {
	f.createdEntityKey = entity.EntityKey
	return f.createdRecord, nil
}

func (f *fakeRepository) UpdateRecord(context.Context, EntityDefinition, string, RuntimeRecordInput) (*RuntimeRecord, error) {
	return nil, ErrNotFound
}

func (f *fakeRepository) DeleteRecord(context.Context, EntityDefinition, string) error {
	return ErrNotFound
}
