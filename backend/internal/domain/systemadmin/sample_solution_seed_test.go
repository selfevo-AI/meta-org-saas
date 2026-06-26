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
		"local_manufacturing_demo",
		"Local Manufacturing Demo",
		"platform.capability_packages",
		"platform.marketplace_listings",
		"sample_industry_solution",
		"tenant_database_template",
		"database_name_prefix",
		"sample_work_order",
		"sample_work_order_create",
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
