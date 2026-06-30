package systemadmin

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBaselineSeedsLocalManufacturingSampleIndustrySolution(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	migrationPath := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "migrations", "000_saas_platform_management_baseline.sql")
	data, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration baseline: %v", err)
	}
	sql := string(data)

	required := []string{
		"erpnext_manufacturing_demo",
		"ERPNext Manufacturing Demo",
		"platform.capability_packages",
		"platform.marketplace_listings",
		"sample_industry_solution",
		"tenant_database_template",
		"database_name_prefix",
		`"table_code": "MBOM"`,
		`"table_code": "MWOR"`,
		"runtime_operation.erpnext_work_order_complete",
		"('erp', 'ERP'",
		"('inventory', 'Inventory'",
		"('procurement', 'Procurement'",
		"('sales', 'Sales'",
	}
	for _, text := range required {
		if !strings.Contains(sql, text) {
			t.Fatalf("baseline missing sample industry solution seed text %q", text)
		}
	}
}
