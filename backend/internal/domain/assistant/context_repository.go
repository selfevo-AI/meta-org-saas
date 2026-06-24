package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

func (r *PostgresContextRepository) GetContextChangeProposal(ctx context.Context, id uuid.UUID) (*ContextChangeProposal, error) {
	var proposal ContextChangeProposal
	var payloadJSON, applyResultJSON []byte
	var reviewerID pgtype.UUID
	err := r.db.QueryRow(ctx, `
		SELECT id, dictionary_version_id, proposal_type, title, summary, payload, status,
			reviewer_id, review_reason, apply_result
		FROM context_change_proposals
		WHERE id = $1
	`, id).Scan(&proposal.ID, &proposal.DictionaryVersionID, &proposal.ProposalType, &proposal.Title, &proposal.Summary,
		&payloadJSON, &proposal.Status, &reviewerID, &proposal.ReviewReason, &applyResultJSON)
	if err != nil {
		return nil, fmt.Errorf("get context change proposal: %w", err)
	}
	proposal.ReviewerID = uuidPointer(reviewerID)
	proposal.Payload = unmarshalMap(payloadJSON)
	proposal.ApplyResult = unmarshalMap(applyResultJSON)
	return &proposal, nil
}

func (r *PostgresContextRepository) ActivateContextRules(ctx context.Context, proposal *ContextChangeProposal, reviewerID uuid.UUID, rules []ContextRuleRecord) ([]ContextRuleRecord, error) {
	if proposal == nil {
		return nil, fmt.Errorf("%w: context proposal is required", ErrValidation)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin activate context rules: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE context_dictionary_versions
		SET status = 'active', updated_at = NOW()
		WHERE id = $1
	`, proposal.DictionaryVersionID); err != nil {
		return nil, fmt.Errorf("activate context dictionary version: %w", err)
	}
	activated := make([]ContextRuleRecord, 0, len(rules))
	for _, rule := range rules {
		if rule.ID == uuid.Nil {
			rule.ID = uuid.New()
		}
		if rule.DictionaryVersionID == uuid.Nil {
			rule.DictionaryVersionID = proposal.DictionaryVersionID
		}
		if _, err := tx.Exec(ctx, `
			UPDATE context_rules
			SET status = 'archived'
			WHERE dictionary_version_id = $1
				AND module_key = $2
				AND entity_key = $3
				AND field_key = $4
				AND rule_type = $5
				AND status = 'active'
				AND id <> $6
		`, rule.DictionaryVersionID, rule.ModuleKey, rule.EntityKey, rule.FieldKey, rule.RuleType, rule.ID); err != nil {
			return nil, fmt.Errorf("archive replaced context rule: %w", err)
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO context_rules (
				id, dictionary_version_id, module_key, entity_key, field_key, rule_type, rule,
				status, approved_by, approved_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', $8, NOW())
			ON CONFLICT (id) DO UPDATE
			SET dictionary_version_id = EXCLUDED.dictionary_version_id,
				module_key = EXCLUDED.module_key,
				entity_key = EXCLUDED.entity_key,
				field_key = EXCLUDED.field_key,
				rule_type = EXCLUDED.rule_type,
				rule = EXCLUDED.rule,
				status = 'active',
				approved_by = EXCLUDED.approved_by,
				approved_at = EXCLUDED.approved_at
			RETURNING id
		`, rule.ID, rule.DictionaryVersionID, rule.ModuleKey, rule.EntityKey, rule.FieldKey, rule.RuleType, mustJSON(rule.Rule), reviewerID).Scan(&rule.ID)
		if err != nil {
			return nil, fmt.Errorf("activate context rule: %w", err)
		}
		rule.Status = DictionaryStatusActive
		activated = append(activated, rule)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit activate context rules: %w", err)
	}
	return activated, nil
}

func (r *PostgresContextRepository) MarkContextChangeProposalApplied(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID, result map[string]any) (*ContextChangeProposal, error) {
	var proposal ContextChangeProposal
	var payloadJSON, applyResultJSON []byte
	var scannedReviewerID pgtype.UUID
	err := r.db.QueryRow(ctx, `
		UPDATE context_change_proposals
		SET status = 'applied',
			reviewer_id = $2,
			review_reason = CASE WHEN review_reason = '' THEN 'applied via verified tool loop' ELSE review_reason END,
			apply_result = $3,
			applied_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, dictionary_version_id, proposal_type, title, summary, payload, status,
			reviewer_id, review_reason, apply_result
	`, id, reviewerID, mustJSON(result)).Scan(&proposal.ID, &proposal.DictionaryVersionID, &proposal.ProposalType, &proposal.Title, &proposal.Summary,
		&payloadJSON, &proposal.Status, &scannedReviewerID, &proposal.ReviewReason, &applyResultJSON)
	if err != nil {
		return nil, fmt.Errorf("mark context change proposal applied: %w", err)
	}
	proposal.ReviewerID = uuidPointer(scannedReviewerID)
	proposal.Payload = unmarshalMap(payloadJSON)
	proposal.ApplyResult = unmarshalMap(applyResultJSON)
	return &proposal, nil
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

func (r *PostgresContextRepository) GetContextPackage(ctx context.Context, id uuid.UUID, organizationID *uuid.UUID) (*ContextPackage, error) {
	var pkg ContextPackage
	var sessionID, dictionaryVersionID pgtype.UUID
	var attentionCoreJSON, supportingContextJSON, riskAndSignalsJSON, omissionsJSON, weightsJSON, validationsJSON, provenanceJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, session_id, dictionary_version_id, attention_core, supporting_context,
			risk_and_signals, omissions, weights, validations, provenance, token_budget
		FROM context_packages
		WHERE id = $1
			AND ($2::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $2::uuid)
	`, id, nullableUUID(organizationID)).Scan(&pkg.ID, &sessionID, &dictionaryVersionID, &attentionCoreJSON, &supportingContextJSON,
		&riskAndSignalsJSON, &omissionsJSON, &weightsJSON, &validationsJSON, &provenanceJSON, &pkg.TokenBudget)
	if err != nil {
		return nil, fmt.Errorf("get context package: %w", err)
	}
	if value := uuidPointer(sessionID); value != nil {
		pkg.SessionID = *value
	}
	pkg.DictionaryVersionID = uuidPointer(dictionaryVersionID)
	pkg.AttentionCore = unmarshalContextItems(attentionCoreJSON)
	pkg.SupportingContext = unmarshalContextItems(supportingContextJSON)
	pkg.RiskAndSignals = unmarshalContextItems(riskAndSignalsJSON)
	pkg.Omissions = unmarshalContextOmissions(omissionsJSON)
	pkg.Weights = unmarshalFloatMap(weightsJSON)
	pkg.Validations = unmarshalMap(validationsJSON)
	pkg.Provenance = unmarshalMap(provenanceJSON)
	return &pkg, nil
}

func (r *PostgresContextRepository) GetContextHealth(ctx context.Context, organizationID *uuid.UUID) (*ContextHealthSummary, error) {
	strictModules := []string{"erp", "finance", "governance"}
	health := &ContextHealthSummary{
		StrictModules:        strictModules,
		StrictModuleCoverage: map[string]int{"erp": 0, "finance": 0, "governance": 0},
	}
	if organizationID != nil {
		id := *organizationID
		health.OrganizationID = &id
	}
	orgArg := nullableUUID(organizationID)
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM context_rules cr
		JOIN context_dictionary_versions cdv ON cdv.id = cr.dictionary_version_id
		WHERE cr.status = 'active'
			AND cdv.status = 'active'
			AND ($1::uuid IS NULL OR cdv.organization_id IS NULL OR cdv.organization_id IS NOT DISTINCT FROM $1::uuid)
	`, orgArg).Scan(&health.ActiveRuleCount); err != nil {
		return nil, fmt.Errorf("count active context rules: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT cr.module_key, COUNT(*)
		FROM context_rules cr
		JOIN context_dictionary_versions cdv ON cdv.id = cr.dictionary_version_id
		WHERE cr.status = 'active'
			AND cdv.status = 'active'
			AND cr.module_key IN ('erp', 'finance', 'governance')
			AND ($1::uuid IS NULL OR cdv.organization_id IS NULL OR cdv.organization_id IS NOT DISTINCT FROM $1::uuid)
		GROUP BY cr.module_key
	`, orgArg)
	if err != nil {
		return nil, fmt.Errorf("count strict context rule coverage: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var moduleKey string
		var count int
		if err := rows.Scan(&moduleKey, &count); err != nil {
			return nil, fmt.Errorf("scan strict context rule coverage: %w", err)
		}
		health.StrictModuleCoverage[moduleKey] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate strict context rule coverage: %w", err)
	}
	for _, moduleKey := range strictModules {
		if health.StrictModuleCoverage[moduleKey] == 0 {
			health.MissingStrictModules = append(health.MissingStrictModules, moduleKey)
		}
	}
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM context_packages
		WHERE created_at >= NOW() - INTERVAL '24 hours'
			AND ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1::uuid)
	`, orgArg).Scan(&health.RecentPackageCount); err != nil {
		return nil, fmt.Errorf("count recent context packages: %w", err)
	}
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM context_packages
		WHERE created_at >= NOW() - INTERVAL '24 hours'
			AND COALESCE(provenance->>'source', '') <> 'context_dictionary'
			AND ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1::uuid)
	`, orgArg).Scan(&health.FallbackPackageCount); err != nil {
		return nil, fmt.Errorf("count fallback context packages: %w", err)
	}
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM assistant_steps
		WHERE step_type = 'error'
			AND created_at >= NOW() - INTERVAL '24 hours'
			AND (summary ILIKE '%context%' OR data::text ILIKE '%context%')
			AND ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1::uuid)
	`, orgArg).Scan(&health.ContextBuildFailureCount); err != nil {
		return nil, fmt.Errorf("count context build failures: %w", err)
	}
	if err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE ccp.status = 'pending'),
			COUNT(*) FILTER (WHERE ccp.status = 'approved'),
			COUNT(*) FILTER (WHERE ccp.status = 'applied')
		FROM context_change_proposals ccp
		JOIN context_dictionary_versions cdv ON cdv.id = ccp.dictionary_version_id
		WHERE ($1::uuid IS NULL OR cdv.organization_id IS NULL OR cdv.organization_id IS NOT DISTINCT FROM $1::uuid)
	`, orgArg).Scan(&health.PendingProposalCount, &health.ApprovedProposalCount, &health.AppliedProposalCount); err != nil {
		return nil, fmt.Errorf("count context change proposals: %w", err)
	}
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM tool_approvals ta
		JOIN tool_executions te ON te.id = ta.execution_id
		WHERE ta.status = 'pending'
			AND ($1::uuid IS NULL OR te.organization_id IS NOT DISTINCT FROM $1::uuid)
	`, orgArg).Scan(&health.ToolApprovalBacklog); err != nil {
		return nil, fmt.Errorf("count tool approval backlog: %w", err)
	}
	return health, nil
}

func unmarshalContextItems(data []byte) []ContextItem {
	items := []ContextItem{}
	if len(data) == 0 {
		return items
	}
	_ = json.Unmarshal(data, &items)
	return items
}

func unmarshalContextOmissions(data []byte) []ContextOmission {
	items := []ContextOmission{}
	if len(data) == 0 {
		return items
	}
	_ = json.Unmarshal(data, &items)
	return items
}

func unmarshalFloatMap(data []byte) map[string]float64 {
	items := map[string]float64{}
	if len(data) == 0 {
		return items
	}
	_ = json.Unmarshal(data, &items)
	if items == nil {
		return map[string]float64{}
	}
	return items
}
