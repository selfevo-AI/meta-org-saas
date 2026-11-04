package auditlog

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type recordingDB struct {
	sql  string
	args []any
	err  error
}

func (d *recordingDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.sql, d.args = sql, args
	if d.err != nil {
		return pgconn.CommandTag{}, d.err
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (d *recordingDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

func (d *recordingDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (d *recordingDB) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}

var _ tenantdb.DB = (*recordingDB)(nil)

func TestRecordDerivesActorAndOrganizationFromContext(t *testing.T) {
	actorID := uuid.New()
	orgID := uuid.New()
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, middleware.AuthenticatedUser{
		ID: actorID.String(), Type: "human", Name: "Auditor",
	})
	ctx = context.WithValue(ctx, middleware.TenantContextKey, &middleware.TenantContext{OrganizationID: &orgID})

	db := &recordingDB{}
	err := Record(ctx, db, Event{
		Action:     "workflow.template_created",
		EntityType: "workflow_template",
		EntityKey:  "tpl-1",
		Detail:     map[string]any{"status": "active"},
	})
	if err != nil {
		t.Fatalf("Record error = %v", err)
	}

	if len(db.args) != 7 {
		t.Fatalf("args = %d, want 7", len(db.args))
	}
	gotActor, ok := db.args[0].(*uuid.UUID)
	if !ok || gotActor == nil || *gotActor != actorID {
		t.Fatalf("actor arg = %v, want %v", db.args[0], actorID)
	}
	if db.args[1] != "human" {
		t.Fatalf("actor type = %v, want human", db.args[1])
	}
	if db.args[2] != "workflow.template_created" || db.args[3] != "workflow_template" || db.args[4] != "tpl-1" {
		t.Fatalf("action/entity args = %v", db.args[2:5])
	}
	gotOrg, ok := db.args[5].(*uuid.UUID)
	if !ok || gotOrg == nil || *gotOrg != orgID {
		t.Fatalf("organization arg = %v, want %v", db.args[5], orgID)
	}
	if detail, ok := db.args[6].([]byte); !ok || string(detail) != `{"status":"active"}` {
		t.Fatalf("detail arg = %v, want marshalled JSON", db.args[6])
	}
}

func TestRecordWithoutContextFallsBackToSystemActor(t *testing.T) {
	db := &recordingDB{}
	if err := Record(context.Background(), db, Event{Action: "costing.budget_created", EntityType: "cost_budget"}); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	if db.args[0] != (*uuid.UUID)(nil) {
		t.Fatalf("actor arg = %v, want nil without an authenticated user", db.args[0])
	}
	if db.args[1] != "system" {
		t.Fatalf("actor type = %v, want system", db.args[1])
	}
	if db.args[4] != "" {
		t.Fatalf("entity key = %v, want empty string default", db.args[4])
	}
	if detail, ok := db.args[6].([]byte); !ok || string(detail) != "{}" {
		t.Fatalf("detail arg = %v, want empty JSON object", db.args[6])
	}
}

func TestRecordWrapsDatabaseErrors(t *testing.T) {
	db := &recordingDB{err: errors.New("connection refused")}
	err := Record(context.Background(), db, Event{Action: "project.requirement_approved", EntityType: "requirement"})
	if err == nil || !errors.Is(err, db.err) {
		t.Fatalf("Record error = %v, want the database error wrapped", err)
	}
}
