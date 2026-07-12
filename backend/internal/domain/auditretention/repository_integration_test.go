package auditretention

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryRedactsPayloadAndPreservesUsageLedger(t *testing.T) {
	databaseURL := os.Getenv("AUDIT_RETENTION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set AUDIT_RETENTION_TEST_DATABASE_URL to run audit retention integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect audit retention database: %v", err)
	}
	t.Cleanup(pool.Close)

	providerID := uuid.New()
	modelID := uuid.New()
	organizationID := uuid.New()
	projectID := uuid.New()
	invocationID := uuid.New()
	runID := uuid.New()
	ledgerID := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM business_stage_ai_runs WHERE id = $1`, runID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_usage_ledger WHERE id = $1`, ledgerID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_invocations WHERE id = $1`, invocationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM models WHERE id = $1`, modelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM model_providers WHERE id = $1`, providerID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, organizationID)
	})

	if _, err := pool.Exec(ctx, `INSERT INTO organizations(id, name) VALUES ($1, $2)`, organizationID, "Retention integration "+organizationID.String()); err != nil {
		t.Fatalf("create retention organization: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO model_providers(id, name, provider_type, encrypted_api_key, status)
		VALUES ($1, $2, 'openai', 'integration-secret', 'active')
	`, providerID, "Retention integration "+providerID.String()); err != nil {
		t.Fatalf("create retention provider: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO models(id, provider_id, model_key, display_name, status)
		VALUES ($1, $2, $3, 'Retention integration model', 'active')
	`, modelID, providerID, "retention-"+modelID.String()); err != nil {
		t.Fatalf("create retention model: %v", err)
	}
	old := time.Now().UTC().AddDate(0, 0, -400)
	if _, err := pool.Exec(ctx, `
		INSERT INTO ai_invocations(
		    id, provider_id, model_id, mode, status, organization_id, project_id,
		    provider_request_id, input_tokens, output_tokens, cost_amount, error_type,
		    error_message, metadata, created_at, completed_at
		)
		VALUES ($1, $2, $3, 'sync', 'completed', $4, $5, 'provider-private-id', 10, 20, 1.25,
		        'private_error', 'sensitive provider error', '{"prompt":"sensitive"}', $6, $6)
	`, invocationID, providerID, modelID, organizationID, projectID, old); err != nil {
		t.Fatalf("insert retention invocation: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO ai_usage_ledger(id, invocation_id, amount, currency, input_tokens, output_tokens, created_at)
		VALUES ($1, $2, 1.25, 'CNY', 10, 20, $3)
	`, ledgerID, invocationID, old); err != nil {
		t.Fatalf("insert retention usage ledger: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO business_stage_ai_runs(
		    id, organization_id, project_id, stage, status, provider_type, requested_model,
		    invocation_id, resolved_model, input_context, result, cost_amount, input_tokens,
		    output_tokens, proposal_result, error_message, proposal_error, created_at, completed_at
		)
		VALUES ($1, $2, $3, 'plan', 'completed', 'openai', 'retention-model', $4, 'retention-model',
		        '{"customer":"sensitive"}', '{"summary":"preserved summary","confidence":0.9,"findings":[{"secret":true}]}',
		        1.25, 10, 20, '{"private":"result"}', 'sensitive run error', 'sensitive proposal error', $5, $5)
	`, runID, organizationID, projectID, invocationID, old); err != nil {
		t.Fatalf("insert retention business run: %v", err)
	}

	repository := NewRepository(pool)
	counts, err := repository.RedactBatch(ctx, time.Now().UTC().AddDate(0, 0, -365), 100)
	if err != nil {
		t.Fatalf("RedactBatch() error = %v", err)
	}
	if counts.AIInvocations != 1 || counts.BusinessAIRuns != 1 {
		t.Fatalf("redaction counts = %#v", counts)
	}

	var providerRequestID, invocationError, metadata string
	var invocationRedacted bool
	if err := pool.QueryRow(ctx, `
		SELECT provider_request_id, error_message, metadata::text, retention_redacted_at IS NOT NULL
		FROM ai_invocations WHERE id = $1
	`, invocationID).Scan(&providerRequestID, &invocationError, &metadata, &invocationRedacted); err != nil {
		t.Fatalf("read redacted invocation: %v", err)
	}
	if providerRequestID != "" || invocationError != "" || !invocationRedacted || metadata != `{"retention_redacted": true}` {
		t.Fatalf("redacted invocation = provider_request_id=%q error=%q metadata=%s marker=%v", providerRequestID, invocationError, metadata, invocationRedacted)
	}

	var inputContext, result, proposalResult string
	var runRedacted bool
	if err := pool.QueryRow(ctx, `
		SELECT input_context::text, result::text, proposal_result::text, retention_redacted_at IS NOT NULL
		FROM business_stage_ai_runs WHERE id = $1
	`, runID).Scan(&inputContext, &result, &proposalResult, &runRedacted); err != nil {
		t.Fatalf("read redacted business run: %v", err)
	}
	if inputContext != `{}` || proposalResult != `{}` || !runRedacted {
		t.Fatalf("redacted business run = input=%s result=%s proposal=%s marker=%v", inputContext, result, proposalResult, runRedacted)
	}
	if result == "" || !strings.Contains(result, "retention_redacted") || strings.Contains(result, "preserved summary") {
		t.Fatalf("redacted business result = %s", result)
	}

	var ledgerCount, invocationCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_usage_ledger WHERE id = $1`, ledgerID).Scan(&ledgerCount); err != nil {
		t.Fatalf("count usage ledger: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_invocations WHERE id = $1`, invocationID).Scan(&invocationCount); err != nil {
		t.Fatalf("count invocation: %v", err)
	}
	if ledgerCount != 1 || invocationCount != 1 {
		t.Fatalf("preserved ledger/invocation = %d/%d", ledgerCount, invocationCount)
	}
}
