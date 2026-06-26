package tenantdb

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBootstrapTenantDataUsesPartialIndexConflictTargetForOwnerMembership(t *testing.T) {
	departmentID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174111")
	db := &captureBootstrapDB{tx: &captureBootstrapTx{departmentID: departmentID}}

	err := BootstrapTenantData(context.Background(), db, TenantBootstrapInput{
		OrganizationID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
		OwnerUserID:    uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
		OwnerName:      "Owner",
		OwnerEmail:     "owner@local.test",
		SampleKey:      "unit",
	})
	if err != nil {
		t.Fatalf("BootstrapTenantData() error = %v", err)
	}

	membershipSQL := db.tx.sqlContaining("organization_memberships")
	if membershipSQL == "" {
		t.Fatalf("owner membership insert SQL was not executed; SQL statements = %#v", db.tx.execSQL)
	}
	if strings.Contains(membershipSQL, "ON CONSTRAINT uq_org_membership_internal") {
		t.Fatalf("owner membership SQL uses partial unique index as a constraint: %s", membershipSQL)
	}
	if !strings.Contains(membershipSQL, "ON CONFLICT (department_id, user_id) WHERE user_id IS NOT NULL AND status <> 'archived'") {
		t.Fatalf("owner membership SQL missing partial index conflict target: %s", membershipSQL)
	}
}

type captureBootstrapDB struct {
	tx *captureBootstrapTx
}

func (f *captureBootstrapDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *captureBootstrapDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *captureBootstrapDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeBootstrapRow{}
}

func (f *captureBootstrapDB) Begin(context.Context) (pgx.Tx, error) {
	return f.tx, nil
}

type captureBootstrapTx struct {
	departmentID uuid.UUID
	execSQL      []string
	committed    bool
	rolledBack   bool
}

func (f *captureBootstrapTx) Begin(context.Context) (pgx.Tx, error) {
	return f, nil
}

func (f *captureBootstrapTx) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *captureBootstrapTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

func (f *captureBootstrapTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (f *captureBootstrapTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (f *captureBootstrapTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (f *captureBootstrapTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (f *captureBootstrapTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.execSQL = append(f.execSQL, sql)
	return pgconn.CommandTag{}, nil
}

func (f *captureBootstrapTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *captureBootstrapTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	f.execSQL = append(f.execSQL, sql)
	return fakeBootstrapRow{departmentID: f.departmentID}
}

func (f *captureBootstrapTx) Conn() *pgx.Conn {
	return nil
}

func (f *captureBootstrapTx) sqlContaining(snippet string) string {
	for _, sql := range f.execSQL {
		if strings.Contains(sql, snippet) {
			return sql
		}
	}
	return ""
}

type fakeBootstrapRow struct {
	departmentID uuid.UUID
}

func (r fakeBootstrapRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if value, ok := dest[0].(*uuid.UUID); ok {
			*value = r.departmentID
		}
	}
	return nil
}
