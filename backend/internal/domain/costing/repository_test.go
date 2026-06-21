package costing

import (
	"testing"

	"github.com/google/uuid"
)

func TestScopeWhereUsesLedgerDimensionColumn(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	where, args := scopeWhere("organization", &id)

	if where != " AND organization_id = $1" {
		t.Fatalf("where = %q, want organization_id condition", where)
	}
	if len(args) != 1 || args[0] != id {
		t.Fatalf("args = %#v, want organization id", args)
	}
}

func TestBudgetScopeWhereUsesScopeColumns(t *testing.T) {
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	where, args := budgetScopeWhere("organization", &id)

	if where != " AND scope_type = $1 AND scope_id = $2" {
		t.Fatalf("where = %q, want budget scope condition", where)
	}
	if len(args) != 2 || args[0] != "organization" || args[1] != id {
		t.Fatalf("args = %#v, want scope type and id", args)
	}
}
