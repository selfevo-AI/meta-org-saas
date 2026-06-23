package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresContextRepository struct {
	db *pgxpool.Pool
}

func NewContextRepository(db *pgxpool.Pool) *PostgresContextRepository {
	return &PostgresContextRepository{db: db}
}

func (r *PostgresContextRepository) CreateDictionaryVersion(ctx context.Context, model DictionaryImportModel, importedBy *uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_dictionary_versions (scope_level, organization_id, module_key, version_key, source_type, source_name, imported_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, model.ScopeLevel, model.OrganizationID, model.ModuleKey, model.VersionKey, model.SourceType, model.SourceName, importedBy, mustJSON(map[string]any{"field_count": len(model.Fields)})).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create context dictionary version: %w", err)
	}
	return id, nil
}

func (r *PostgresContextRepository) CreateContextChangeProposal(ctx context.Context, input ContextChangeProposalInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_change_proposals (dictionary_version_id, proposal_type, title, summary, payload, status)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'pending'))
		RETURNING id
	`, input.DictionaryVersionID, input.ProposalType, input.Title, input.Summary, mustJSON(input.Payload), input.Status).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create context change proposal: %w", err)
	}
	return id, nil
}

func (r *PostgresContextRepository) CreateContextMigrationDraft(ctx context.Context, input ContextMigrationDraftInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_migration_drafts (dictionary_version_id, title, summary, sql_up, sql_down, risk_level, metadata)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'medium'), $7)
		RETURNING id
	`, input.DictionaryVersionID, input.Title, input.Summary, input.SQLUp, input.SQLDown, input.RiskLevel, mustJSON(input.Metadata)).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create context migration draft: %w", err)
	}
	return id, nil
}

func (r *PostgresContextRepository) ListActiveContextRules(ctx context.Context, request ContextRequest) ([]ContextRuleRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cr.id, cr.dictionary_version_id, cr.module_key, cr.entity_key, cr.field_key, cr.rule_type, cr.rule, cr.status
		FROM context_rules cr
		JOIN context_dictionary_versions cdv ON cdv.id = cr.dictionary_version_id
		WHERE cr.status = 'active'
			AND cdv.status = 'active'
			AND (cr.module_key = '' OR cr.module_key = $1 OR $1 = '')
			AND (cdv.organization_id IS NULL OR cdv.organization_id IS NOT DISTINCT FROM $2)
		ORDER BY
			CASE WHEN cdv.organization_id IS NOT NULL THEN 0 ELSE 1 END,
			cr.created_at DESC
	`, request.ModuleKey, request.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list active context rules: %w", err)
	}
	defer rows.Close()
	rules := []ContextRuleRecord{}
	for rows.Next() {
		var item ContextRuleRecord
		var ruleJSON []byte
		if err := rows.Scan(&item.ID, &item.DictionaryVersionID, &item.ModuleKey, &item.EntityKey, &item.FieldKey, &item.RuleType, &ruleJSON, &item.Status); err != nil {
			return nil, fmt.Errorf("scan active context rule: %w", err)
		}
		item.Rule = map[string]any{}
		if len(ruleJSON) > 0 {
			if err := json.Unmarshal(ruleJSON, &item.Rule); err != nil {
				return nil, fmt.Errorf("decode active context rule: %w", err)
			}
		}
		rules = append(rules, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active context rules: %w", err)
	}
	return rules, nil
}

func (r *PostgresContextRepository) CreateContextPackage(ctx context.Context, request ContextRequest, pkg ContextPackage) (*ContextPackage, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO context_packages (
			session_id, dictionary_version_id, actor_id, actor_type, organization_id, module_key,
			target_type, target_id, workflow_id, task_id, attention_core, supporting_context,
			risk_and_signals, omissions, weights, validations, provenance, token_budget
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`, request.SessionID, pkg.DictionaryVersionID, request.ActorID, request.ActorType, request.OrganizationID, request.ModuleKey,
		request.TargetType, request.TargetID, request.WorkflowID, request.TaskID, mustJSONValue(pkg.AttentionCore), mustJSONValue(pkg.SupportingContext),
		mustJSONValue(pkg.RiskAndSignals), mustJSONValue(pkg.Omissions), mustJSONValue(pkg.Weights), mustJSONValue(pkg.Validations),
		mustJSONValue(pkg.Provenance), pkg.TokenBudget).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create context package: %w", err)
	}
	pkg.ID = id
	return &pkg, nil
}
