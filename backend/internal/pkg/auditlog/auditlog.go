// Package auditlog appends tenant-side audit events for governance-relevant
// mutations. Events live in each tenant's physical database (table
// `audit_events`, tenant migration 012) so a tenant's trail travels with its
// data; the platform database keeps no copy.
package auditlog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

// Event describes one audited mutation.
type Event struct {
	Action     string         // stable verb, e.g. "workflow.template_created"
	EntityType string         // e.g. "workflow_template"
	EntityKey  string         // human-recognizable key (id or code)
	Detail     map[string]any // small, non-secret context (status, counts, ...)
}

// Record appends the event to the tenant audit trail and returns any write
// error. Call sites currently audit after the business mutation commits and
// log failures without failing the request; wiring the insert into the same
// transaction would let callers fail closed instead.
func Record(ctx context.Context, db tenantdb.DB, event Event) error {
	detail := event.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail: %w", err)
	}

	var actorID *uuid.UUID
	actorType := "system"
	if user, ok := middleware.UserFromContext(ctx); ok {
		actorType = user.Type
		if parsed, err := uuid.Parse(user.ID); err == nil {
			actorID = &parsed
		}
	}
	var organizationID *uuid.UUID
	if tenant, ok := middleware.TenantFromContext(ctx); ok {
		organizationID = tenant.OrganizationID
	}

	if _, err := db.Exec(ctx,
		`INSERT INTO audit_events (actor_id, actor_type, action, entity_type, entity_key, organization_id, detail)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		actorID, actorType, event.Action, event.EntityType, event.EntityKey, organizationID, payload,
	); err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}
