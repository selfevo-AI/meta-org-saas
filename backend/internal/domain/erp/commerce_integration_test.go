package erp_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/ontology"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func provisionCommerceDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("RUN_COMMERCE_DB_TEST") != "1" {
		t.Skip("set RUN_COMMERCE_DB_TEST=1 for provisioned-tenant commerce integration tests")
	}
	adminURL := os.Getenv("MIGRATION_TEST_ADMIN_URL")
	if adminURL == "" {
		t.Fatal("MIGRATION_TEST_ADMIN_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	target := tenantdb.NewDedicatedDatabaseTarget(uuid.New(), "meta_org_", "local", "local")
	var exists bool
	for attempts := 0; attempts < 10; attempts++ {
		if err := admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, target.DatabaseName).Scan(&exists); err != nil {
			admin.Close()
			t.Fatal(err)
		}
		if !exists {
			break
		}
		target = tenantdb.NewDedicatedDatabaseTarget(uuid.New(), "meta_org_", "local", "local")
	}
	if exists {
		admin.Close()
		t.Fatal("could not allocate a fresh tenant database name")
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _ = admin.Exec(cleanupCtx, `DROP DATABASE IF EXISTS "`+target.DatabaseName+`" WITH (FORCE)`)
		admin.Close()
	})
	_, source, _, _ := runtime.Caller(0)
	migrations := filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "migrations", "tenant")
	provisioner := tenantdb.NewProvisioner(tenantdb.ProvisionerConfig{AdminURL: adminURL, Creator: tenantdb.NewCatalogDatabaseCreator(tenantdb.NewPGDatabaseCatalog(admin)), Migrator: tenantdb.FileTenantMigrator{MigrationsDir: migrations}})
	if _, err := provisioner.Provision(ctx, target); err != nil {
		t.Fatalf("provision tenant: %v", err)
	}
	url, err := tenantdb.DatabaseURLForName(adminURL, target.DatabaseName)
	if err != nil {
		t.Fatal(err)
	}
	pool, err = pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestPostgresPurchaseToPayAndOrderToCash(t *testing.T) {
	pool := provisionCommerceDatabase(t)
	ctx := context.Background()
	svc := erp.NewService(erp.NewRepository(pool), erp.DefaultCatalog())
	create := func(table, key string, data map[string]any) {
		t.Helper()
		if _, err := svc.CreateRecord(ctx, table, erp.RecordInput{Key: key, Data: data}); err != nil {
			t.Fatalf("create %s/%s: %v", table, key, err)
		}
	}
	line := func(table, key, child, item, warehouse string, quantity, price, tax float64) {
		t.Helper()
		if _, err := svc.CreateChildRecord(ctx, table, key, child, erp.RecordInput{Key: "1", Data: map[string]any{"ItemCode": item, "WhsCode": warehouse, "Quantity": quantity, "Price": price, "TaxRate": tax}}); err != nil {
			t.Fatalf("line %s/%s: %v", table, key, err)
		}
	}
	action := func(table, key, name, idem string, data map[string]any) *erp.ActionResult {
		t.Helper()
		result, err := svc.RunAction(ctx, table, key, name, erp.ActionInput{IdempotencyKey: idem, Data: data})
		if err != nil {
			t.Fatalf("action %s/%s/%s: %v", table, key, name, err)
		}
		return result
	}
	get := func(table, key string) *erp.Record {
		t.Helper()
		record, err := svc.GetRecord(ctx, table, key)
		if err != nil {
			t.Fatalf("get %s/%s: %v", table, key, err)
		}
		return record
	}
	assertNumber := func(record *erp.Record, field string, want float64) {
		t.Helper()
		if got, ok := record.Data[field].(float64); !ok || got != want {
			t.Fatalf("%s/%s %s = %v, want %v", record.TableCode, record.Key, field, record.Data[field], want)
		}
	}
	create("MCRD", "SUP", map[string]any{"CardName": "Supplier", "CardType": "S"})
	create("MCRD", "CUS", map[string]any{"CardName": "Customer", "CardType": "C"})
	create("MITM", "ITEM", map[string]any{"ItemName": "Item"})
	create("MWHS", "MAIN", map[string]any{"WhsName": "Main"})

	for i, price := range []float64{10, 20} {
		key := fmt.Sprintf("PO-%d", i+1)
		create("MPOR", key, map[string]any{"CardCode": "SUP", "DocCur": "CNY"})
		line("MPOR", key, "POR1", "ITEM", "MAIN", 10, price, 13)
		action("MPOR", key, "submit", "", nil)
		action("MPOR", key, "approve", "", nil)
		action("MPOR", key, "receive", "", nil)
		action("MPDN", "GR-"+key, "approve", "", nil)
		posted := action("MPDN", "GR-"+key, "post", "receipt-post", nil)
		replay := action("MPDN", "GR-"+key, "post", "receipt-post", nil)
		if replay.ExecutionID != posted.ExecutionID {
			t.Fatal("idempotent replay created a new action")
		}
		action("MIGN", "IGN-GR-"+key, "post", "different-request", nil)
	}
	assertNumber(get("MITW", "ITEM|MAIN"), "OnHand", 20)
	assertNumber(get("MITW", "ITEM|MAIN"), "InventoryValue", 300)
	assertNumber(get("MITW", "ITEM|MAIN"), "AvgPrice", 15)
	action("MPCH", "AP-GR-PO-1", "post", "", nil)
	create("MVPM", "PAY-1", map[string]any{"CardCode": "SUP", "DocCur": "CNY", "DocTotal": 113})
	for i, amount := range []float64{50, 63} {
		action("MVPM", "PAY-1", "allocate", fmt.Sprint(i), map[string]any{"TargetKey": "AP-GR-PO-1", "Amount": amount})
	}
	assertNumber(get("MPCH", "AP-GR-PO-1"), "PaidToDate", 113)
	assertNumber(get("MVPM", "PAY-1"), "OpenBal", 0)
	if get("MPCH", "AP-GR-PO-1").Data["DocStatus"] != "C" {
		t.Fatal("payable remains open")
	}

	create("MRDR", "SO-1", map[string]any{"CardCode": "CUS", "DocCur": "CNY"})
	line("MRDR", "SO-1", "RDR1", "ITEM", "MAIN", 4, 50, 13)
	action("MRDR", "SO-1", "confirm", "", nil)
	action("MRDR", "SO-1", "approve", "", nil)
	action("MRDR", "SO-1", "deliver", "", nil)
	action("MDLN", "DL-SO-1", "approve", "", nil)
	action("MDLN", "DL-SO-1", "post", "", nil)
	action("MINV", "INV-DL-SO-1", "post", "", nil)
	create("MRCT", "COLLECT-1", map[string]any{"CardCode": "CUS", "DocCur": "CNY", "DocTotal": 226})
	for i, amount := range []float64{100, 126} {
		action("MRCT", "COLLECT-1", "allocate", fmt.Sprint(i), map[string]any{"TargetKey": "INV-DL-SO-1", "Amount": amount})
	}
	assertNumber(get("MITW", "ITEM|MAIN"), "OnHand", 16)
	assertNumber(get("MITW", "ITEM|MAIN"), "InventoryValue", 240)
	assertNumber(get("MINV", "INV-DL-SO-1"), "PaidToDate", 226)
	assertNumber(get("MRCT", "COLLECT-1"), "OpenBal", 0)
	if _, err := svc.RunAction(ctx, "MRCT", "COLLECT-1", "allocate", erp.ActionInput{IdempotencyKey: "overpay", Data: map[string]any{"TargetKey": "INV-DL-SO-1", "Amount": 1}}); !errors.Is(err, erp.ErrValidation) {
		t.Fatalf("overpayment error = %v", err)
	}
	if _, err := svc.UpdateRecord(ctx, "MINV", "INV-DL-SO-1", erp.RecordInput{Data: map[string]any{"DocTotal": 1}}); !errors.Is(err, erp.ErrConflict) {
		t.Fatalf("posted mutation error = %v", err)
	}
	if _, err := svc.CreateChildRecord(ctx, "MVPM", "PAY-1", "VPM1", erp.RecordInput{Key: "3"}); !errors.Is(err, erp.ErrValidation) {
		t.Fatalf("forged allocation error = %v", err)
	}
	if _, err := svc.RunAction(ctx, "MRCT", "COLLECT-1", "allocate", erp.ActionInput{IdempotencyKey: "0", Data: map[string]any{"TargetKey": "INV-DL-SO-1", "Amount": 99}}); !errors.Is(err, erp.ErrConflict) {
		t.Fatalf("changed replay parameters error = %v", err)
	}

	balance, err := svc.TrialBalance(ctx, erp.TrialBalanceInput{Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	if balance.TotalDebit != 1038 || balance.TotalCredit != 1038 || balance.JournalCount != 9 {
		t.Fatalf("trial balance = %#v", balance)
	}
	nets := map[string]float64{}
	for _, row := range balance.Rows {
		nets[row.AccountCode] = row.NetAmount
	}
	for account, want := range map[string]float64{"1002": 113, "1405": 240, "1122": 0, "2202": 0, "2202-GRNI": -200, "2221-IN": 13, "2221-OUT": -26, "6001": -200, "6401": 60} {
		if nets[account] != want {
			t.Fatalf("account %s = %v, want %v", account, nets[account], want)
		}
	}
	ledger := finance.NewRepository(pool, nil)
	financeBalance, err := ledger.GetGLTrialBalance(ctx, finance.GLTrialBalanceInput{Currency: "CNY"})
	if err != nil || financeBalance.TotalDebit != balance.TotalDebit {
		t.Fatalf("finance ledger diverges: %#v, %v", financeBalance, err)
	}
	journals, err := ledger.ListGLJournalEntries(ctx, 100)
	if err != nil || len(journals) != 9 {
		t.Fatalf("finance journals = %d, %v", len(journals), err)
	}
	entry, err := ledger.GetGLJournalEntry(ctx, journals[0].ID)
	if err != nil || len(entry.Lines) < 2 || entry.Status != "posted" {
		t.Fatalf("finance entry = %#v, %v", entry, err)
	}
	created, err := ledger.CreateGLJournalEntry(ctx, finance.CreateGLJournalEntryInput{EntryNumber: "MANUAL-1", Currency: "CNY", Lines: []finance.CreateGLJournalEntryLineInput{{AccountCode: "1002", Debit: 1}, {AccountCode: "6001", Credit: 1}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.PostGLJournalEntry(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if get("MJDT", "MANUAL-1").Data["BtfStatus"] != "P" {
		t.Fatal("finance API did not post the ERP journal")
	}

	graph := ontology.NewService(svc)
	for _, test := range []struct{ typ, key, relation, target string }{
		{"purchase_order", "PO-1", "fulfillment", "GR-PO-1"},
		{"goods_receipt", "GR-PO-1", "invoice", "AP-GR-PO-1"},
		{"payable_invoice", "AP-GR-PO-1", "payments", "PAY-1"},
		{"receivable_invoice", "INV-DL-SO-1", "payments", "COLLECT-1"},
	} {
		links, err := graph.Links(ctx, test.typ, test.key)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, link := range links.Links {
			found = found || link.Type == test.relation && link.Object.Key == test.target
		}
		if !found {
			t.Fatalf("missing %s/%s %s -> %s: %#v", test.typ, test.key, test.relation, test.target, links)
		}
	}
	page, err := graph.Query(ctx, "purchase_order", ontology.QueryInput{Filters: map[string]any{"partner": "SUP"}, Limit: 1})
	if err != nil || len(page.Objects) != 1 || page.NextCursor == "" {
		t.Fatalf("object query page = %#v, %v", page, err)
	}
	second, err := graph.Query(ctx, "purchase_order", ontology.QueryInput{Filters: map[string]any{"partner": "SUP"}, Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(second.Objects) != 1 || second.Objects[0].Key == page.Objects[0].Key {
		t.Fatalf("object cursor = %#v, %v", second, err)
	}

	t.Run("failed posting rolls back all effects and can retry", func(t *testing.T) {
		create("MPDN", "ROLLBACK", map[string]any{"CardCode": "SUP", "DocCur": "CNY"})
		line("MPDN", "ROLLBACK", "PDN1", "ITEM", "MAIN", 2, 10, 0)
		action("MPDN", "ROLLBACK", "approve", "", nil)
		if _, err := svc.UpdateRecord(ctx, "MACT", "1405", erp.RecordInput{Data: map[string]any{"Postable": "N"}}); err != nil {
			t.Fatal(err)
		}
		_, err := svc.RunAction(ctx, "MPDN", "ROLLBACK", "post", erp.ActionInput{IdempotencyKey: "retry"})
		if !errors.Is(err, erp.ErrValidation) {
			t.Fatalf("posting error = %v", err)
		}
		assertNumber(get("MITW", "ITEM|MAIN"), "OnHand", 16)
		for _, record := range []struct{ table, key string }{{"MIGN", "IGN-ROLLBACK"}, {"MPCH", "AP-ROLLBACK"}, {"MJDT", "JE-STOCK-MPDN-ROLLBACK"}} {
			if _, err := svc.GetRecord(ctx, record.table, record.key); !errors.Is(err, erp.ErrNotFound) {
				t.Fatalf("failed posting left %s/%s: %v", record.table, record.key, err)
			}
		}
		if _, err := svc.UpdateRecord(ctx, "MACT", "1405", erp.RecordInput{Data: map[string]any{"Postable": "Y"}}); err != nil {
			t.Fatal(err)
		}
		action("MPDN", "ROLLBACK", "post", "retry", map[string]any{"retry_failed": true})
		assertNumber(get("MITW", "ITEM|MAIN"), "OnHand", 18)
	})

	t.Run("concurrent allocations cannot overspend", func(t *testing.T) {
		create("MINV", "CONCURRENT", map[string]any{"CardCode": "CUS", "DocCur": "CNY", "DocTotal": 100})
		action("MINV", "CONCURRENT", "post", "", nil)
		create("MRCT", "CONCURRENT", map[string]any{"CardCode": "CUS", "DocCur": "CNY", "DocTotal": 100})
		var wait sync.WaitGroup
		errs := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wait.Add(1)
			go func(i int) {
				defer wait.Done()
				_, err := svc.RunAction(ctx, "MRCT", "CONCURRENT", "allocate", erp.ActionInput{IdempotencyKey: fmt.Sprint(i), Data: map[string]any{"TargetKey": "CONCURRENT", "Amount": 80}})
				errs <- err
			}(i)
		}
		wait.Wait()
		close(errs)
		succeeded := 0
		for err := range errs {
			if err == nil {
				succeeded++
			} else if !errors.Is(err, erp.ErrValidation) {
				t.Fatal(err)
			}
		}
		if succeeded != 1 {
			t.Fatalf("successful allocations = %d", succeeded)
		}
		assertNumber(get("MRCT", "CONCURRENT"), "OpenBal", 20)
		assertNumber(get("MINV", "CONCURRENT"), "PaidToDate", 80)
	})

	t.Run("concurrent replay posts only once", func(t *testing.T) {
		create("MPDN", "PARALLEL", map[string]any{"CardCode": "SUP", "DocCur": "CNY"})
		line("MPDN", "PARALLEL", "PDN1", "ITEM", "MAIN", 1, 10, 0)
		action("MPDN", "PARALLEL", "approve", "", nil)
		var wait sync.WaitGroup
		ids := make(chan uuid.UUID, 6)
		for i := 0; i < 6; i++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				result, err := svc.RunAction(ctx, "MPDN", "PARALLEL", "post", erp.ActionInput{IdempotencyKey: "same"})
				if err != nil {
					t.Error(err)
					return
				}
				ids <- result.ExecutionID
			}()
		}
		wait.Wait()
		close(ids)
		unique := map[uuid.UUID]bool{}
		for id := range ids {
			unique[id] = true
		}
		if len(unique) != 1 {
			t.Fatalf("replay execution count = %d", len(unique))
		}
		assertNumber(get("MITW", "ITEM|MAIN"), "OnHand", 19)
	})

	t.Run("module permissions apply to linked reads and downstream writes", func(t *testing.T) {
		limited := context.WithValue(ctx, middleware.TenantContextKey, &middleware.TenantContext{Mode: "saas", AuthorityTier: "organization_admin", EnabledModules: map[string]bool{"procurement": true}})
		if _, err := svc.RunAction(limited, "MPDN", "GR-PO-1", "post", erp.ActionInput{}); !errors.Is(err, erp.ErrForbidden) {
			t.Fatalf("cross-module write error = %v", err)
		}
		links, err := graph.Links(limited, "goods_receipt", "GR-PO-1")
		if err != nil {
			t.Fatal(err)
		}
		for _, link := range links.Links {
			if link.Object.Type != "purchase_order" {
				t.Fatalf("disabled module object leaked: %#v", link)
			}
		}
	})

	t.Run("trial balance aggregates beyond the UI page limit", func(t *testing.T) {
		_, err := pool.Exec(ctx, `INSERT INTO "MJDT"("TransId","Payload") SELECT 'BULK-' || n, jsonb_build_object('BtfStatus','P','Currency','USD','RefDate','2000-01-01') FROM generate_series(1,501) n;
		INSERT INTO "JDT1"("TransId","LineNum","Payload") SELECT 'BULK-' || n, l, jsonb_build_object('AccountCode', CASE WHEN l=1 THEN '1002' ELSE '6001' END, 'Debit', CASE WHEN l=1 THEN 1 ELSE 0 END, 'Credit', CASE WHEN l=2 THEN 1 ELSE 0 END) FROM generate_series(1,501) n CROSS JOIN generate_series(1,2) l;`)
		if err != nil {
			t.Fatal(err)
		}
		balance, err := svc.TrialBalance(ctx, erp.TrialBalanceInput{Currency: "USD", PeriodStart: "2000-01-01", PeriodEnd: "2000-01-01"})
		if err != nil || balance.JournalCount != 501 || balance.TotalDebit != 501 || balance.TotalCredit != 501 {
			t.Fatalf("large ledger = %#v, %v", balance, err)
		}
	})
	var constraints int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_constraint WHERE NOT convalidated`).Scan(&constraints); err != nil || constraints != 0 {
		t.Fatalf("unvalidated constraints = %d, %v", constraints, err)
	}
	if strings.Contains(pool.Config().ConnConfig.Database, "migration_check") {
		t.Fatal("tenant name was not assigned by provisioner")
	}
}
