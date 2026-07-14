package aigateway

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/secretbox"
)

func TestProviderChannelCircuitBreakerAndAtomicReservation(t *testing.T) {
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
	box, err := secretbox.New(strings.Repeat("c", 32))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := box.Encrypt("sk-circuit-integration")
	if err != nil {
		t.Fatal(err)
	}
	providerID, modelID, channelID := uuid.New(), uuid.New(), uuid.New()
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM model_provider_channels WHERE id = $1`, channelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM models WHERE id = $1`, modelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM model_providers WHERE id = $1`, providerID)
	}()
	if _, err := pool.Exec(ctx, `
		INSERT INTO model_providers(id, name, provider_type, encrypted_api_key, status)
		VALUES ($1, $2, 'openai', $3, 'active')
	`, providerID, "circuit-integration-"+providerID.String(), encrypted); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO models(id, provider_id, model_key, status)
		VALUES ($1, $2, $3, 'active')
	`, modelID, providerID, "circuit-integration-"+modelID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO model_provider_channels(id, provider_id, name, encrypted_api_key, status, concurrency_limit, supported_model_patterns)
		VALUES ($1, $2, 'circuit', $3, 'active', 1, '["*"]')
	`, channelID, providerID, encrypted); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, box, WithChannelCircuitPolicy(2, time.Minute))
	invoke := InvokeInput{ProviderID: &providerID, ModelID: &modelID}
	for failure := 0; failure < 2; failure++ {
		target, err := repo.ResolveInvocationTarget(ctx, invoke)
		if err != nil {
			t.Fatalf("ResolveInvocationTarget() failure %d error = %v", failure+1, err)
		}
		if target.ChannelID == nil || *target.ChannelID != channelID {
			t.Fatalf("resolved channel = %v, want %s", target.ChannelID, channelID)
		}
		if err := repo.ReleaseChannel(ctx, target.ChannelID, ReleaseChannelInput{Outcome: ChannelOutcomeFailure, Error: "upstream unavailable", ResponseMS: 50}); err != nil {
			t.Fatalf("ReleaseChannel() failure %d error = %v", failure+1, err)
		}
	}

	var consecutive int
	var health string
	var circuitOpen bool
	if err := pool.QueryRow(ctx, `
		SELECT consecutive_failure_count, health_status, circuit_open_until > NOW()
		FROM model_provider_channels WHERE id = $1
	`, channelID).Scan(&consecutive, &health, &circuitOpen); err != nil {
		t.Fatal(err)
	}
	if consecutive != 2 || health != "circuit_open" || !circuitOpen {
		t.Fatalf("open circuit state = consecutive:%d health:%q open:%t", consecutive, health, circuitOpen)
	}
	if _, err := repo.ResolveInvocationTarget(ctx, invoke); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ResolveInvocationTarget() during open circuit error = %v, want ErrUnavailable", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE model_provider_channels SET circuit_open_until = NOW() - INTERVAL '1 second' WHERE id = $1`, channelID); err != nil {
		t.Fatal(err)
	}
	probe, err := repo.ResolveInvocationTarget(ctx, invoke)
	if err != nil {
		t.Fatalf("ResolveInvocationTarget() recovery probe error = %v", err)
	}
	if _, err := repo.ResolveInvocationTarget(ctx, invoke); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("second recovery probe error = %v, want ErrUnavailable", err)
	}
	if err := repo.ReleaseChannel(ctx, probe.ChannelID, ReleaseChannelInput{Outcome: ChannelOutcomeSuccess, ResponseMS: 25}); err != nil {
		t.Fatalf("ReleaseChannel() successful probe error = %v", err)
	}
	var successCount, failureCount int64
	var circuitUntil *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT success_count, failure_count, consecutive_failure_count, health_status, circuit_open_until
		FROM model_provider_channels WHERE id = $1
	`, channelID).Scan(&successCount, &failureCount, &consecutive, &health, &circuitUntil); err != nil {
		t.Fatal(err)
	}
	if successCount != 1 || failureCount != 2 || consecutive != 0 || health != "healthy" || circuitUntil != nil {
		t.Fatalf("closed circuit state = success:%d failure:%d consecutive:%d health:%q until:%v", successCount, failureCount, consecutive, health, circuitUntil)
	}
	channels, err := repo.ListChannels(ctx, &providerID, 10)
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if len(channels) != 1 || channels[0].SuccessCount != 1 || channels[0].FailureCount != 2 || channels[0].ConsecutiveFailureCount != 0 || channels[0].CircuitOpenUntil != nil {
		t.Fatalf("listed channel circuit state = %#v", channels)
	}

	const contenders = 8
	start := make(chan struct{})
	hold := make(chan struct{})
	results := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			target, err := repo.ResolveInvocationTarget(context.Background(), invoke)
			results <- err
			if err == nil {
				<-hold
				_ = repo.ReleaseChannel(context.Background(), target.ChannelID, ReleaseChannelInput{Outcome: ChannelOutcomeNeutral})
			}
		}()
	}
	close(start)
	successes := 0
	for i := 0; i < contenders; i++ {
		if err := <-results; err == nil {
			successes++
		} else if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("concurrent reservation error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent successful reservations = %d, want 1", successes)
	}
	var inflight int
	if err := pool.QueryRow(ctx, `SELECT inflight_requests FROM model_provider_channels WHERE id = $1`, channelID).Scan(&inflight); err != nil {
		t.Fatal(err)
	}
	if inflight != 1 {
		t.Fatalf("inflight_requests = %d, want 1", inflight)
	}
	close(hold)
	wg.Wait()
}

func TestResolveInvocationTargetLoadsProviderRetryPolicy(t *testing.T) {
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
	box, err := secretbox.New(strings.Repeat("r", 32))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := box.Encrypt("sk-retry-integration")
	if err != nil {
		t.Fatal(err)
	}
	providerID, modelID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO model_providers(id, name, provider_type, encrypted_api_key, status, timeout_ms, retry_count)
		VALUES ($1, $2, 'openai', $3, 'active', 45000, 4)
	`, providerID, "retry-integration-"+providerID.String(), encrypted); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO models(id, provider_id, model_key, status)
		VALUES ($1, $2, $3, 'active')
	`, modelID, providerID, "retry-integration-"+modelID.String()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM models WHERE id = $1`, modelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM model_providers WHERE id = $1`, providerID)
	}()

	repo := NewRepository(pool, box)
	target, err := repo.ResolveInvocationTarget(ctx, InvokeInput{ProviderID: &providerID, ModelID: &modelID})
	if err != nil {
		t.Fatalf("ResolveInvocationTarget() error = %v", err)
	}
	if target.TimeoutMS != 45000 || target.RetryCount != 4 || target.APIKey != "sk-retry-integration" {
		t.Fatalf("resolved provider policy = timeout:%d retries:%d key:%q", target.TimeoutMS, target.RetryCount, target.APIKey)
	}
}

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

func TestReservationRecoveryRefundsOrphansAndCancelsAbandonedInvocations(t *testing.T) {
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

	orgID, tokenID := uuid.New(), uuid.New()
	providerID, modelID, channelID, invocationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	orphanReservationID, attachedReservationID := uuid.New(), uuid.New()
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_usage_ledger WHERE invocation_id = $1`, invocationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_gateway_balance_transactions WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_gateway_balances WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_access_tokens WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_invocations WHERE id = $1`, invocationID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM model_provider_channels WHERE id = $1`, channelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM models WHERE id = $1`, modelID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM model_providers WHERE id = $1`, providerID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
	}()

	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO organizations(id, name) VALUES ($1, $2)`, []any{orgID, "reservation-recovery-" + orgID.String()}},
		{`INSERT INTO ai_access_tokens(id, organization_id, name, token_hash, quota_amount, quota_used) VALUES ($1, $2, 'recovery', $3, 100, 5)`, []any{tokenID, orgID, "recovery-" + tokenID.String()}},
		{`INSERT INTO ai_gateway_balances(organization_id, balance_amount, reserved_amount) VALUES ($1, 100, 5)`, []any{orgID}},
		{`INSERT INTO model_providers(id, name, provider_type, encrypted_api_key) VALUES ($1, $2, 'openai', 'integration-test')`, []any{providerID, "recovery-" + providerID.String()}},
		{`INSERT INTO models(id, provider_id, model_key) VALUES ($1, $2, $3)`, []any{modelID, providerID, "recovery-" + modelID.String()}},
		{`INSERT INTO model_provider_channels(id, provider_id, name, encrypted_api_key, inflight_requests) VALUES ($1, $2, 'recovery', 'integration-test', 1)`, []any{channelID, providerID}},
		{`INSERT INTO ai_invocations(id, provider_id, model_id, channel_id, mode, status, organization_id, cost_amount) VALUES ($1, $2, $3, $4, 'stream', 'streaming', $5, 0)`, []any{invocationID, providerID, modelID, channelID, orgID}},
		{`INSERT INTO ai_usage_ledger(invocation_id, channel_id, ledger_type, amount, actual_amount, currency) VALUES ($1, $2, 'usage', 0.25, 0.25, 'CNY')`, []any{invocationID, channelID}},
		{`INSERT INTO ai_gateway_balance_transactions(id, organization_id, access_token_id, transaction_type, amount, currency, created_at) VALUES ($1, $2, $3, 'reserve', 2, 'CNY', NOW() - INTERVAL '2 hours')`, []any{orphanReservationID, orgID, tokenID}},
		{`INSERT INTO ai_gateway_balance_transactions(id, organization_id, access_token_id, invocation_id, transaction_type, amount, currency, created_at) VALUES ($1, $2, $3, $4, 'reserve', 3, 'CNY', NOW() - INTERVAL '2 hours')`, []any{attachedReservationID, orgID, tokenID, invocationID}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}

	repo := &PostgresRepository{db: pool}
	reservations, err := repo.ClaimStaleBalanceReservations(ctx, "integration-worker", time.Now().Add(-time.Hour), time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimStaleBalanceReservations() error = %v", err)
	}
	if len(reservations) != 2 {
		t.Fatalf("claimed reservations = %#v", reservations)
	}
	outcomes := map[string]int{}
	for _, reservation := range reservations {
		outcome, err := repo.RecoverStaleBalanceReservation(ctx, "integration-worker", reservation)
		if err != nil {
			t.Fatalf("RecoverStaleBalanceReservation() error = %v", err)
		}
		outcomes[outcome]++
	}
	if outcomes["refunded"] != 1 || outcomes["abandoned"] != 1 {
		t.Fatalf("recovery outcomes = %#v", outcomes)
	}

	var balance, reserved, quotaUsed float64
	if err := pool.QueryRow(ctx, `SELECT balance_amount::float8, reserved_amount::float8 FROM ai_gateway_balances WHERE organization_id = $1`, orgID).Scan(&balance, &reserved); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT quota_used::float8 FROM ai_access_tokens WHERE id = $1`, tokenID).Scan(&quotaUsed); err != nil {
		t.Fatal(err)
	}
	var invocationStatus string
	var inflight int
	if err := pool.QueryRow(ctx, `SELECT status FROM ai_invocations WHERE id = $1`, invocationID).Scan(&invocationStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT inflight_requests FROM model_provider_channels WHERE id = $1`, channelID).Scan(&inflight); err != nil {
		t.Fatal(err)
	}
	if balance != 99.75 || reserved != 0 || quotaUsed != 0.25 || invocationStatus != StatusCancelled || inflight != 0 {
		t.Fatalf("recovered state = balance:%v reserved:%v quota:%v invocation:%s inflight:%d", balance, reserved, quotaUsed, invocationStatus, inflight)
	}
}

// TestFinishReservationIsIdempotent guards the double-finish path that keeps
// the reservation-recovery worker and the live invoke goroutine from both
// settling the same reservation (which would double-charge the balance).
func TestFinishReservationIsIdempotent(t *testing.T) {
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

	orgID, tokenID := uuid.New(), uuid.New()
	settleReservationID, refundReservationID := uuid.New(), uuid.New()
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_gateway_balance_transactions WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_gateway_balances WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_access_tokens WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
	}()

	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO organizations(id, name) VALUES ($1, $2)`, []any{orgID, "double-finish-" + orgID.String()}},
		{`INSERT INTO ai_access_tokens(id, organization_id, name, token_hash, quota_amount, quota_used) VALUES ($1, $2, 'double-finish', $3, 100, 10)`, []any{tokenID, orgID, "double-finish-" + tokenID.String()}},
		{`INSERT INTO ai_gateway_balances(organization_id, balance_amount, reserved_amount) VALUES ($1, 100, 10)`, []any{orgID}},
		{`INSERT INTO ai_gateway_balance_transactions(id, organization_id, access_token_id, transaction_type, amount, currency) VALUES ($1, $2, $3, 'reserve', 4, 'CNY')`, []any{settleReservationID, orgID, tokenID}},
		{`INSERT INTO ai_gateway_balance_transactions(id, organization_id, access_token_id, transaction_type, amount, currency) VALUES ($1, $2, $3, 'reserve', 6, 'CNY')`, []any{refundReservationID, orgID, tokenID}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}

	repo := &PostgresRepository{db: pool}

	// Settle once, then settle again: the second call must be a no-op.
	settle := SettleBalanceInput{ReservationID: settleReservationID, AccessTokenID: tokenID, ActualAmount: 3, Currency: "CNY", Reason: "first"}
	if err := repo.SettleAccessTokenBalance(ctx, settle); err != nil {
		t.Fatalf("first settle error = %v", err)
	}
	settle.ActualAmount = 3
	settle.Reason = "duplicate"
	if err := repo.SettleAccessTokenBalance(ctx, settle); err != nil {
		t.Fatalf("duplicate settle error = %v", err)
	}
	// Refund the second reservation, then attempt to settle it: must be a no-op.
	if err := repo.RefundAccessTokenBalance(ctx, refundReservationID, "refunded"); err != nil {
		t.Fatalf("refund error = %v", err)
	}
	if err := repo.SettleAccessTokenBalance(ctx, SettleBalanceInput{ReservationID: refundReservationID, AccessTokenID: tokenID, ActualAmount: 6, Currency: "CNY", Reason: "settle-after-refund"}); err != nil {
		t.Fatalf("settle-after-refund error = %v", err)
	}

	var balance, reserved float64
	if err := pool.QueryRow(ctx, `SELECT balance_amount::float8, reserved_amount::float8 FROM ai_gateway_balances WHERE organization_id = $1`, orgID).Scan(&balance, &reserved); err != nil {
		t.Fatal(err)
	}
	// Started at balance 100 / reserved 10. Settle 3 of a 4 reservation: -3
	// balance, -4 reserved. Refund the 6 reservation: -6 reserved. So the
	// balance must move exactly once (to 97) and reserved unwind exactly to 0 —
	// duplicate settle and settle-after-refund must not double-charge.
	if balance != 97 || reserved != 0 {
		t.Fatalf("balance after idempotent finishes = balance:%v reserved:%v, want 97/0", balance, reserved)
	}
	var settleCount, refundCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_gateway_balance_transactions WHERE reservation_id = $1 AND transaction_type = 'settle'`, settleReservationID).Scan(&settleCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_gateway_balance_transactions WHERE reservation_id = $1 AND transaction_type = 'refund'`, refundReservationID).Scan(&refundCount); err != nil {
		t.Fatal(err)
	}
	if settleCount != 1 || refundCount != 1 {
		t.Fatalf("finish transaction counts = settle:%d refund:%d, want 1/1", settleCount, refundCount)
	}
}
