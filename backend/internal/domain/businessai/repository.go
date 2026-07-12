package businessai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateRun(ctx context.Context, input AnalyzeInput) (*Run, error) {
	run := &Run{}
	err := scanRun(r.db.QueryRow(ctx, `
		INSERT INTO business_stage_ai_runs (
			organization_id, project_id, requirement_id, stage, status,
			requested_by_id, requested_by_type, provider_type, requested_model, input_context
		) VALUES ($1, $2, $3, $4, 'running', $5, $6, $7, $8, $9)
		RETURNING id, organization_id, project_id, requirement_id, stage, status,
			requested_by_id, requested_by_type, provider_type, requested_model,
			invocation_id, resolved_model, input_context, result, cost_amount::float8,
			currency, input_tokens, output_tokens, error_message, created_at, completed_at
	`, input.OrganizationID, input.ProjectID, input.RequirementID, input.Stage, input.RequestedByID,
		input.RequestedByType, input.ProviderType, input.Model, mustJSON(input.Context)), run)
	if err != nil {
		return nil, fmt.Errorf("create business stage ai run: %w", err)
	}
	return run, nil
}

type CompleteRunInput struct {
	InvocationID  uuid.UUID
	ResolvedModel string
	Analysis      Analysis
	CostAmount    float64
	Currency      string
	InputTokens   int
	OutputTokens  int
}

func (r *Repository) CompleteRun(ctx context.Context, id uuid.UUID, input CompleteRunInput) (*Run, error) {
	run := &Run{}
	err := scanRun(r.db.QueryRow(ctx, `
		UPDATE business_stage_ai_runs
		SET status = 'completed', invocation_id = $2, resolved_model = $3, result = $4,
			cost_amount = $5, currency = $6, input_tokens = $7, output_tokens = $8,
			completed_at = NOW()
		WHERE id = $1
		RETURNING id, organization_id, project_id, requirement_id, stage, status,
			requested_by_id, requested_by_type, provider_type, requested_model,
			invocation_id, resolved_model, input_context, result, cost_amount::float8,
			currency, input_tokens, output_tokens, error_message, created_at, completed_at
	`, id, input.InvocationID, input.ResolvedModel, mustJSON(input.Analysis), input.CostAmount,
		input.Currency, input.InputTokens, input.OutputTokens), run)
	if err != nil {
		return nil, fmt.Errorf("complete business stage ai run: %w", err)
	}
	return run, nil
}

func (r *Repository) FailRun(ctx context.Context, id uuid.UUID, message string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE business_stage_ai_runs
		SET status = 'failed', error_message = $2, completed_at = NOW()
		WHERE id = $1
	`, id, message)
	if err != nil {
		return fmt.Errorf("fail business stage ai run: %w", err)
	}
	return nil
}

func (r *Repository) ListRuns(ctx context.Context, organizationID, projectID uuid.UUID, limit int) ([]Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, project_id, requirement_id, stage, status,
			requested_by_id, requested_by_type, provider_type, requested_model,
			invocation_id, resolved_model, input_context, result, cost_amount::float8,
			currency, input_tokens, output_tokens, error_message, created_at, completed_at
		FROM business_stage_ai_runs
		WHERE organization_id = $1 AND project_id = $2
		ORDER BY created_at DESC LIMIT $3
	`, organizationID, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list business stage ai runs: %w", err)
	}
	defer rows.Close()
	runs := []Run{}
	for rows.Next() {
		var run Run
		if err := scanRun(rows, &run); err != nil {
			return nil, fmt.Errorf("scan business stage ai run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner, run *Run) error {
	var contextJSON, resultJSON []byte
	if err := row.Scan(&run.ID, &run.OrganizationID, &run.ProjectID, &run.RequirementID, &run.Stage,
		&run.Status, &run.RequestedByID, &run.RequestedByType, &run.ProviderType, &run.RequestedModel,
		&run.InvocationID, &run.ResolvedModel, &contextJSON, &resultJSON, &run.CostAmount, &run.Currency,
		&run.InputTokens, &run.OutputTokens, &run.ErrorMessage, &run.CreatedAt, &run.CompletedAt); err != nil {
		return err
	}
	run.InputContext = map[string]any{}
	_ = json.Unmarshal(contextJSON, &run.InputContext)
	if len(resultJSON) > 0 && string(resultJSON) != "null" && string(resultJSON) != "{}" {
		var analysis Analysis
		if err := json.Unmarshal(resultJSON, &analysis); err != nil {
			return err
		}
		run.Analysis = &analysis
	}
	return nil
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}
