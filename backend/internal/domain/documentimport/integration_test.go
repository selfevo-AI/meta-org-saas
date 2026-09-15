package documentimport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/ontology"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func importDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("RUN_DOCUMENT_IMPORT_INTEGRATION_TEST") != "1" {
		t.Skip("set RUN_DOCUMENT_IMPORT_INTEGRATION_TEST=1 for tenant document-import verification")
	}
	adminURL := os.Getenv("MIGRATION_TEST_ADMIN_URL")
	if adminURL == "" {
		t.Fatal("MIGRATION_TEST_ADMIN_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	target := tenantdb.NewDedicatedDatabaseTarget(uuid.New(), "meta_org_", "test", "test")
	for attempt := 0; attempt < 10; attempt++ {
		var exists bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, target.DatabaseName).Scan(&exists); err != nil {
			admin.Close()
			t.Fatal(err)
		}
		if !exists {
			break
		}
		if attempt == 9 {
			admin.Close()
			t.Fatal("unable to allocate an isolated tenant database")
		}
		target = tenantdb.NewDedicatedDatabaseTarget(uuid.New(), "meta_org_", "test", "test")
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
		t.Fatal(err)
	}
	url, err := tenantdb.DatabaseURLForName(adminURL, target.DatabaseName)
	if err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 2
	pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func importHumanContext() context.Context {
	user := uuid.New()
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, middleware.AuthenticatedUser{ID: user.String(), Type: "human"})
	return context.WithValue(ctx, middleware.TenantContextKey, &middleware.TenantContext{Mode: "saas", AuthorityTier: "organization_admin",
		EnabledModules: map[string]bool{"procurement": true, "inventory": true, "finance": true, "sales": true, "project": true}})
}

func TestPostgresDocumentImportReviewAndQuery(t *testing.T) {
	pool := importDatabase(t)
	ctx, cancel := context.WithTimeout(importHumanContext(), 2*time.Minute)
	defer cancel()
	business := erp.NewService(erp.NewRepository(pool), erp.DefaultCatalog())
	objects := ontology.NewService(business, ontology.NewRepository(pool))
	service := NewService(NewRepository(pool), objects, business, NewGatewayRecognizer(nil, RecognitionConfig{}))
	for _, fixture := range []struct {
		table, key string
		data       map[string]any
	}{
		{"MCRD", "SUP", map[string]any{"CardName": "Supplier", "CardType": "S", "ValidFor": "Y"}},
		{"MCRD", "CUSTOMER", map[string]any{"CardName": "Customer", "CardType": "C"}},
		{"MITM", "ITEM", map[string]any{"ItemName": "Part", "validFor": "Y"}},
		{"MITM", "INACTIVE", map[string]any{"ItemName": "Retired part", "validFor": "N"}},
		{"MWHS", "MAIN", map[string]any{"WhsName": "Main warehouse"}},
	} {
		if _, err := business.CreateRecord(ctx, fixture.table, erp.RecordInput{Key: fixture.key, Data: fixture.data}); err != nil {
			t.Fatal(err)
		}
	}
	types, err := objects.Types(ctx)
	if err != nil || len(types) != 19 {
		t.Fatalf("catalog has %d types: %v", len(types), err)
	}
	for _, typ := range types {
		if typ.SchemaVersion != 1 || len(typ.Properties) == 0 {
			t.Fatalf("incomplete persisted type: %#v", typ)
		}
	}
	var unvalidated int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE NOT convalidated`).Scan(&unvalidated); err != nil || unvalidated != 0 {
		t.Fatalf("unvalidated=%d: %v", unvalidated, err)
	}

	upload := func(external string) *Import {
		t.Helper()
		csv := "external_number,partner,date,currency,item,warehouse,quantity,unit_price,tax_rate\n" + external + ",SUP,2026-09-15,CNY,ITEM,MAIN,10,12.5,13\n"
		item, err := service.Upload(ctx, "purchase_order", []SourceFile{{Name: "invoice.csv", Content: []byte(csv)}})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	item := upload("EXT-ONE")
	duplicate := upload("EXT-ONE")
	if duplicate.ID != item.ID {
		t.Fatal("identical upload created a second import")
	}
	if _, err := business.GetRecord(ctx, "MPOR", item.Draft.Key); !errors.Is(err, erp.ErrNotFound) {
		t.Fatal("upload created a business object")
	}
	item, err = service.Recognize(ctx, item.ID, item.Version)
	if err != nil || item.Status != StatusNeedsReview || len(item.Extraction.Evidence) == 0 {
		t.Fatalf("recognition: %#v %v", item, err)
	}
	if _, err := business.GetRecord(ctx, "MPOR", item.Draft.Key); !errors.Is(err, erp.ErrNotFound) {
		t.Fatal("recognition created a business object")
	}

	oldVersion := item.Version
	review := item.Draft
	review.Properties["note"] = "Reviewed against the supplier original"
	item, err = service.Review(ctx, item.ID, ReviewInput{Version: item.Version, Draft: review})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Review(ctx, item.ID, ReviewInput{Version: oldVersion, Draft: review}); !errors.Is(err, erp.ErrConflict) {
		t.Fatalf("stale review accepted: %v", err)
	}
	item, err = service.Recognize(ctx, item.ID, item.Version)
	if err != nil || item.Draft.Properties["note"] != "Reviewed against the supplier original" {
		t.Fatal("recognition overwrote a saved human correction")
	}
	input := ReviewInput{Version: item.Version, Draft: item.Draft, Confirmed: true}
	var wg sync.WaitGroup
	errorsChannel := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := service.Confirm(ctx, item.ID, input); errorsChannel <- err }()
	}
	wg.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent confirmation: %v", err)
		}
	}
	confirmed, err := service.Get(ctx, item.ID)
	if err != nil || confirmed.Status != StatusConfirmed || confirmed.ReviewedBy == nil {
		t.Fatalf("confirmation: %#v %v", confirmed, err)
	}
	record, err := business.GetRecord(ctx, "MPOR", confirmed.ConfirmedKey)
	if err != nil || record.Data["Posted"] == "Y" || record.Data["WddStatus"] == "A" {
		t.Fatalf("import bypassed draft lifecycle: %#v %v", record, err)
	}
	lines, err := business.ListChildRecords(ctx, "MPOR", record.Key, "POR1", 200)
	if err != nil || len(lines) != 1 {
		t.Fatalf("import lines: %d %v", len(lines), err)
	}
	if record.Data["DocTotal"] != 141.25 || lines[0].Data["Price"] != 12.5 || lines[0].Data["Quantity"] != float64(10) {
		t.Fatalf("import total: %#v", record.Data["DocTotal"])
	}
	filtered, err := objects.Query(ctx, "purchase_order", ontology.QueryInput{Filters: map[string]any{"total": json.Number("141.25")}})
	if err != nil || filtered.Total != 1 || filtered.Objects[0].Key != record.Key {
		t.Fatalf("import decimals did not retain numeric query semantics: %#v %v", filtered, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ontology_import_events WHERE import_id=$1 AND event='confirmed'`, item.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("confirmation events=%d err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ontology_document_sources WHERE object_type='purchase_order' AND object_key=$1`, record.Key).Scan(&count); err != nil || count != 1 {
		t.Fatalf("source edges=%d err=%v", count, err)
	}
	history, err := objects.History(ctx, "purchase_order", record.Key, 50)
	if err != nil || len(history) != 1 || history[0].Action != "import" || history[0].Status != "completed" {
		t.Fatalf("import action provenance: %#v %v", history, err)
	}
	input.Draft.Key = "CHANGED"
	if _, err := service.Confirm(ctx, item.ID, input); !errors.Is(err, erp.ErrConflict) {
		t.Fatalf("changed confirmation replay: %v", err)
	}
	agent := context.WithValue(ctx, middleware.UserContextKey, middleware.AuthenticatedUser{ID: uuid.NewString(), Type: "agent"})
	if _, err := service.Confirm(agent, item.ID, input); !errors.Is(err, erp.ErrForbidden) {
		t.Fatal("agent bypassed human review")
	}
	disabled := context.WithValue(ctx, middleware.TenantContextKey, &middleware.TenantContext{Mode: "saas", AuthorityTier: "organization_admin", EnabledModules: map[string]bool{}})
	if _, err := service.File(disabled, item.ID, item.Files[0].ID); !errors.Is(err, erp.ErrForbidden) {
		t.Fatal("file escaped module permission check")
	}

	t.Run("rollback after audit failure", func(t *testing.T) {
		candidate := upload("EXT-ROLLBACK")
		candidate, err := service.Recognize(ctx, candidate.ID, candidate.Version)
		if err != nil {
			t.Fatal(err)
		}
		candidate.Draft.Key = "ROLLBACK-IMPORT"
		_, err = pool.Exec(ctx, `CREATE FUNCTION fail_import_confirmation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
			IF NEW.event='confirmed' THEN RAISE EXCEPTION 'injected audit failure'; END IF; RETURN NEW; END $$;
			CREATE TRIGGER import_confirmation_failure BEFORE INSERT ON ontology_import_events FOR EACH ROW EXECUTE FUNCTION fail_import_confirmation();`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Confirm(ctx, candidate.ID, ReviewInput{Version: candidate.Version, Draft: candidate.Draft, Confirmed: true}); err == nil {
			t.Fatal("injected failure was ignored")
		}
		if _, err := business.GetRecord(ctx, "MPOR", candidate.Draft.Key); !errors.Is(err, erp.ErrNotFound) {
			t.Fatal("failed import left a header")
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM "POR1" WHERE "DocEntry"=$1`, candidate.Draft.Key).Scan(&count); err != nil || count != 0 {
			t.Fatal("failed import left lines")
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM "MAEX" WHERE "RecordKey"=$1`, candidate.Draft.Key).Scan(&count); err != nil || count != 0 {
			t.Fatal("failed import left action history")
		}
		unchanged, err := service.Get(ctx, candidate.ID)
		if err != nil || unchanged.Version != candidate.Version || unchanged.Status != StatusNeedsReview {
			t.Fatal("failed confirmation changed import state")
		}
		if _, err := pool.Exec(ctx, `DROP TRIGGER import_confirmation_failure ON ontology_import_events`); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Confirm(ctx, candidate.ID, ReviewInput{Version: candidate.Version, Draft: candidate.Draft, Confirmed: true}); err != nil {
			t.Fatalf("retry after rollback: %v", err)
		}
	})

	t.Run("references and rejection", func(t *testing.T) {
		candidate := upload("EXT-INVALID")
		candidate, err := service.Recognize(ctx, candidate.ID, candidate.Version)
		if err != nil {
			t.Fatal(err)
		}
		candidate.Draft.Properties["partner"] = "CUSTOMER"
		if _, err := service.Confirm(ctx, candidate.ID, ReviewInput{Version: candidate.Version, Draft: candidate.Draft, Confirmed: true}); !errors.Is(err, erp.ErrValidation) {
			t.Fatalf("wrong partner accepted: %v", err)
		}
		candidate.Draft.Properties["partner"] = "SUP"
		candidate.Draft.Lines[0]["item"] = "MISSING"
		if _, err := service.Confirm(ctx, candidate.ID, ReviewInput{Version: candidate.Version, Draft: candidate.Draft, Confirmed: true}); !errors.Is(err, erp.ErrValidation) {
			t.Fatalf("unknown item accepted: %v", err)
		}
		candidate.Draft.Lines[0]["item"] = "INACTIVE"
		_, err = service.Confirm(ctx, candidate.ID, ReviewInput{Version: candidate.Version, Draft: candidate.Draft, Confirmed: true})
		var validation *ValidationError
		if !errors.As(err, &validation) || len(validation.Issues) != 1 || validation.Issues[0].Code != "inactive_reference" {
			t.Fatalf("inactive item accepted: %v", err)
		}
		rejected, err := service.Reject(ctx, candidate.ID, candidate.Version)
		if err != nil || rejected.Status != StatusRejected {
			t.Fatal(err)
		}
		if _, err := service.Recognize(ctx, candidate.ID, rejected.Version); !errors.Is(err, erp.ErrConflict) {
			t.Fatal("rejected import was reopened")
		}
		file, err := service.File(ctx, candidate.ID, candidate.Files[0].ID)
		if err != nil || len(file.Content) == 0 {
			t.Fatal("rejection removed evidence")
		}
	})

	t.Run("unavailable recognition still permits human review", func(t *testing.T) {
		candidate, err := service.Upload(ctx, "purchase_order", []SourceFile{{Name: "manual.txt", Content: []byte("Supplier original requiring manual review")}})
		if err != nil {
			t.Fatal(err)
		}
		candidate, err = service.Recognize(ctx, candidate.ID, candidate.Version)
		if err != nil || candidate.Status != StatusFailed || candidate.ErrorCode != "recognition_unavailable" {
			t.Fatalf("unavailable recognition: %#v %v", candidate, err)
		}
		if _, err := business.GetRecord(ctx, "MPOR", candidate.Draft.Key); !errors.Is(err, erp.ErrNotFound) {
			t.Fatal("failed recognition created a business object")
		}
		draft := validDraft()
		draft.Key = "MANUAL-REVIEW"
		candidate, err = service.Review(ctx, candidate.ID, ReviewInput{Version: candidate.Version, Draft: draft})
		if err != nil {
			t.Fatal(err)
		}
		candidate, err = service.Confirm(ctx, candidate.ID, ReviewInput{Version: candidate.Version, Draft: candidate.Draft, Confirmed: true})
		if err != nil || candidate.Status != StatusConfirmed || candidate.ConfirmedKey != draft.Key {
			t.Fatalf("manual confirmation: %#v %v", candidate, err)
		}
	})

	t.Run("active lease conflicts and expired lease recovers", func(t *testing.T) {
		candidate := upload("EXT-LEASE")
		if _, err := pool.Exec(ctx, `UPDATE ontology_document_imports SET status='recognizing',recognition_token=$2,recognition_started_at=NOW() WHERE id=$1`, candidate.ID, uuid.New()); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Recognize(ctx, candidate.ID, candidate.Version); !errors.Is(err, erp.ErrConflict) {
			t.Fatalf("active lease was replaced: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE ontology_document_imports SET recognition_started_at=NOW()-INTERVAL '4 minutes' WHERE id=$1`, candidate.ID); err != nil {
			t.Fatal(err)
		}
		candidate, err := service.Recognize(ctx, candidate.ID, candidate.Version)
		if err != nil || candidate.Status != StatusNeedsReview || candidate.RecognitionToken != nil {
			t.Fatalf("expired lease did not recover: %#v %v", candidate, err)
		}
	})

	t.Run("semantic status sorting and cursor", func(t *testing.T) {
		for index, total := range []any{100, 9, 9, 1, nil} {
			data := map[string]any{"CardCode": "SUP", "DocDate": "2026-09-15", "DocCur": "CNY"}
			if total != nil {
				data["DocTotal"] = total
			}
			if _, err := business.CreateRecord(ctx, "MPOR", erp.RecordInput{Key: fmt.Sprintf("QUERY-%d", index), Data: data}); err != nil {
				t.Fatal(err)
			}
		}
		keys := []string{}
		input := ontology.QueryInput{Search: "QUERY-", Status: "draft", Sort: "total", Direction: "asc", Limit: 2}
		for {
			page, err := objects.Query(ctx, "purchase_order", input)
			if err != nil || page.Total != 5 {
				t.Fatalf("page: %#v %v", page, err)
			}
			for _, object := range page.Objects {
				keys = append(keys, object.Key)
			}
			if page.NextCursor == "" {
				break
			}
			input.Cursor = page.NextCursor
		}
		if fmt.Sprint(keys) != "[QUERY-3 QUERY-1 QUERY-2 QUERY-0 QUERY-4]" {
			t.Fatalf("numeric or null ordering: %v", keys)
		}
		encoded, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		if err != nil {
			t.Fatal(err)
		}
		var cursor map[string]any
		if err := json.Unmarshal(encoded, &cursor); err != nil {
			t.Fatal(err)
		}
		cursor["value"] = "not-a-number"
		encoded, _ = json.Marshal(cursor)
		forged := input
		forged.Cursor = base64.RawURLEncoding.EncodeToString(encoded)
		if _, err := objects.Query(ctx, "purchase_order", forged); !errors.Is(err, erp.ErrValidation) {
			t.Fatalf("invalid numeric cursor was not rejected before the database cast: %v", err)
		}
		input.Status = "posted"
		if _, err := objects.Query(ctx, "purchase_order", input); !errors.Is(err, erp.ErrValidation) {
			t.Fatalf("cursor reused for another filter: %v", err)
		}
	})
}
