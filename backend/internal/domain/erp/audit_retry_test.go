package erp

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type auditTransactionRepository struct {
	*businessFakeRepository
	transactions  int
	inTransaction bool
	unsafeAudit   bool
	beforeAudit   func()
}

func (r *auditTransactionRepository) RunInTx(ctx context.Context, fn func(Repository) error) error {
	r.transactions++
	if r.transactions == 2 && r.beforeAudit != nil {
		r.beforeAudit()
	}
	r.inTransaction = true
	defer func() { r.inTransaction = false }()
	return r.businessFakeRepository.RunInTx(ctx, func(Repository) error { return fn(r) })
}

func (r *auditTransactionRepository) CreateActionExecution(ctx context.Context, execution ActionExecution) (*ActionExecution, error) {
	if !r.inTransaction {
		r.unsafeAudit = true
	}
	return r.businessFakeRepository.CreateActionExecution(ctx, execution)
}

func (r *auditTransactionRepository) CompleteActionExecution(ctx context.Context, id uuid.UUID, status string, payload map[string]any, failure *ActionFailure) (*ActionExecution, error) {
	if !r.inTransaction {
		r.unsafeAudit = true
	}
	return r.businessFakeRepository.CompleteActionExecution(ctx, id, status, payload, failure)
}

func TestFailedActionAuditIsSerializedWithSuccessfulRetries(t *testing.T) {
	for _, retryCommitted := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed audit", true: "retry already committed"}[retryCommitted], func(t *testing.T) {
			repo := &auditTransactionRepository{businessFakeRepository: newBusinessFakeRepository()}
			repo.seed("MPOR", "order", map[string]any{})
			service := NewService(repo, DefaultCatalog())
			input := ActionInput{IdempotencyKey: "retry-audit"}
			key := service.effectiveIdempotencyKey("MPOR", "order", "submit", input)
			if retryCommitted {
				repo.beforeAudit = func() {
					id := uuid.New()
					repo.executionsByKey[key] = id
					repo.executions[id] = ActionExecution{ID: id, IdempotencyKey: key, Status: ActionExecutionCompleted}
				}
			}
			if _, err := service.RunAction(context.Background(), "MPOR", "order", "submit", input); !errors.Is(err, ErrValidation) {
				t.Fatalf("action error = %v", err)
			}
			if repo.transactions != 2 || repo.unsafeAudit {
				t.Fatalf("audit was not serialized: %#v", repo)
			}
			want := ActionExecutionFailed
			if retryCommitted {
				want = ActionExecutionCompleted
			}
			if got := repo.executions[repo.executionsByKey[key]].Status; got != want {
				t.Fatalf("execution status = %s, want %s", got, want)
			}
		})
	}
}
