package dashboard

import (
	"strings"
	"testing"
)

func TestScopedObservabilityQueryFiltersFinanceBatchesThroughFinanceExportLines(t *testing.T) {
	query := scopedObservabilityQuery()

	if strings.Contains(query, "FROM finance_export_batches WHERE organization_id = $1") {
		t.Fatalf("scoped observability query filters finance_export_batches by missing organization_id column:\n%s", query)
	}
	if !strings.Contains(query, "FROM finance_export_batches b") {
		t.Fatalf("scoped observability query should alias finance_export_batches as b:\n%s", query)
	}
	if !strings.Contains(query, "EXISTS (SELECT 1 FROM finance_export_lines l WHERE l.batch_id = b.id AND l.organization_id = $1)") {
		t.Fatalf("scoped observability query should filter finance batches through finance_export_lines:\n%s", query)
	}
}
