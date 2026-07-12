package aigateway

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateUsageLedgerPersistsNumericAmounts(t *testing.T) {
	if os.Getenv("RUN_AI_GATEWAY_DB_TEST") != "1" {
		t.Skip("set RUN_AI_GATEWAY_DB_TEST=1 to run AI Gateway database verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := os.Getenv("AI_GATEWAY_TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/meta_org_saas?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	providerID, modelID, invocationID := uuid.New(), uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO model_providers(id, name, provider_type, encrypted_api_key, status)
		VALUES ($1, $2, 'openai', 'integration-test', 'active')
	`, providerID, "ledger-integration-"+providerID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO models(id, provider_id, model_key, display_name, status)
		VALUES ($1, $2, $3, 'Ledger integration model', 'active')
	`, modelID, providerID, "ledger-integration-"+modelID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO ai_invocations(id, provider_id, model_id, mode, status)
		VALUES ($1, $2, $3, 'sync', 'completed')
	`, invocationID, providerID, modelID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_usage_ledger WHERE invocation_id = $1`, invocationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_invocations WHERE id = $1`, invocationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM models WHERE id = $1`, modelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM model_providers WHERE id = $1`, providerID)
	}()

	repo := &PostgresRepository{db: pool}
	if err := repo.CreateUsageLedger(ctx, CreateUsageLedgerInput{
		InvocationID: invocationID,
		LedgerType:   "usage",
		Amount:       0.12,
		ActualAmount: 0,
		Currency:     "CNY",
		Usage:        TokenUsage{InputTokens: 120, OutputTokens: 90},
	}); err != nil {
		t.Fatalf("CreateUsageLedger() error = %v", err)
	}
	var amount, actualAmount float64
	if err := pool.QueryRow(ctx, `
		SELECT amount::float8, actual_amount::float8
		FROM ai_usage_ledger WHERE invocation_id = $1
	`, invocationID).Scan(&amount, &actualAmount); err != nil {
		t.Fatal(err)
	}
	if amount != 0.12 || actualAmount != 0.12 {
		t.Fatalf("amounts = %v/%v, want 0.12/0.12", amount, actualAmount)
	}
}
