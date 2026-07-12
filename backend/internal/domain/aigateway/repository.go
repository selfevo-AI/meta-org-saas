package aigateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type secretBox interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

type PostgresRepository struct {
	db  *pgxpool.Pool
	box secretBox
}

func NewRepository(db *pgxpool.Pool, box secretBox) *PostgresRepository {
	return &PostgresRepository{db: db, box: box}
}

func (r *PostgresRepository) CreateProvider(ctx context.Context, input CreateProviderInput) (*ModelProvider, error) {
	encrypted, err := r.box.Encrypt(input.APIKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt provider api key: %w", err)
	}
	tagsJSON, metaJSON, err := providerJSON(input.Tags, input.Metadata)
	if err != nil {
		return nil, err
	}
	provider := &ModelProvider{}
	err = scanProviderRow(r.db.QueryRow(ctx, `
		INSERT INTO model_providers (
			name, provider_type, base_url, encrypted_api_key, masked_api_key,
			status, timeout_ms, retry_count, risk_level, tags, metadata
		)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'active'), COALESCE(NULLIF($7, 0), 60000), COALESCE(NULLIF($8, 0), 1), COALESCE(NULLIF($9, ''), 'medium'), $10, $11)
		RETURNING id, name, provider_type, base_url, masked_api_key, status, timeout_ms, retry_count,
			risk_level, tags, metadata, last_test_status, last_test_error, last_tested_at, created_at, updated_at
	`, input.Name, input.ProviderType, input.BaseURL, encrypted, maskSecret(input.APIKey), input.Status, input.TimeoutMS, input.RetryCount, input.RiskLevel, tagsJSON, metaJSON), provider)
	if err != nil {
		return nil, fmt.Errorf("create model provider: %w", err)
	}
	return provider, nil
}

func (r *PostgresRepository) ListProviders(ctx context.Context, limit int) ([]ModelProvider, error) {
	limit = normalizeLimit(limit)
	rows, err := r.db.Query(ctx, `
		SELECT id, name, provider_type, base_url, masked_api_key, status, timeout_ms, retry_count,
			risk_level, tags, metadata, last_test_status, last_test_error, last_tested_at, created_at, updated_at
		FROM model_providers
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list model providers: %w", err)
	}
	defer rows.Close()
	return scanProviders(rows)
}

func (r *PostgresRepository) UpdateProvider(ctx context.Context, id uuid.UUID, input UpdateProviderInput) (*ModelProvider, error) {
	tagsJSON, metaJSON, err := providerJSON(input.Tags, input.Metadata)
	if err != nil {
		return nil, err
	}
	provider := &ModelProvider{}
	err = scanProviderRow(r.db.QueryRow(ctx, `
		UPDATE model_providers
		SET name = COALESCE($2, name),
			base_url = COALESCE($3, base_url),
			status = COALESCE($4, status),
			timeout_ms = COALESCE($5, timeout_ms),
			retry_count = COALESCE($6, retry_count),
			risk_level = COALESCE($7, risk_level),
			tags = CASE WHEN $8::jsonb IS NULL THEN tags ELSE $8::jsonb END,
			metadata = CASE WHEN $9::jsonb IS NULL THEN metadata ELSE $9::jsonb END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, provider_type, base_url, masked_api_key, status, timeout_ms, retry_count,
			risk_level, tags, metadata, last_test_status, last_test_error, last_tested_at, created_at, updated_at
	`, id, input.Name, input.BaseURL, input.Status, input.TimeoutMS, input.RetryCount, input.RiskLevel, nullableJSON(tagsJSON, input.Tags != nil), nullableJSON(metaJSON, input.Metadata != nil)), provider)
	if err != nil {
		return nil, fmt.Errorf("update model provider: %w", err)
	}
	return provider, nil
}

func (r *PostgresRepository) RotateProviderKey(ctx context.Context, id uuid.UUID, apiKey string) (*ModelProvider, error) {
	encrypted, err := r.box.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt provider api key: %w", err)
	}
	provider := &ModelProvider{}
	err = scanProviderRow(r.db.QueryRow(ctx, `
		UPDATE model_providers
		SET encrypted_api_key = $2, masked_api_key = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, provider_type, base_url, masked_api_key, status, timeout_ms, retry_count,
			risk_level, tags, metadata, last_test_status, last_test_error, last_tested_at, created_at, updated_at
	`, id, encrypted, maskSecret(apiKey)), provider)
	if err != nil {
		return nil, fmt.Errorf("rotate model provider key: %w", err)
	}
	return provider, nil
}

func (r *PostgresRepository) UpdateProviderTestResult(ctx context.Context, id uuid.UUID, status string, message string) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE model_providers
		SET last_test_status = $2, last_test_error = $3, last_tested_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, status, message); err != nil {
		return fmt.Errorf("update provider test result: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetProviderSecret(ctx context.Context, id uuid.UUID) (ProviderSecret, error) {
	var provider ProviderSecret
	var encrypted string
	err := r.db.QueryRow(ctx, `
		SELECT id, name, provider_type, base_url, encrypted_api_key, status
		FROM model_providers WHERE id = $1
	`, id).Scan(&provider.ID, &provider.Name, &provider.ProviderType, &provider.BaseURL, &encrypted, &provider.Status)
	if err != nil {
		return provider, fmt.Errorf("get provider secret: %w", err)
	}
	apiKey, err := r.box.Decrypt(encrypted)
	if err != nil {
		return provider, fmt.Errorf("decrypt provider api key: %w", err)
	}
	provider.APIKey = apiKey
	return provider, nil
}

func (r *PostgresRepository) CreateModel(ctx context.Context, input CreateModelInput) (*Model, error) {
	capsJSON, metaJSON, err := modelJSON(input.Capabilities, input.Metadata)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create model: %w", err)
	}
	defer tx.Rollback(ctx)

	model := &Model{}
	err = scanModelRow(tx.QueryRow(ctx, `
		INSERT INTO models (
			provider_id, model_key, display_name, context_window, max_output_tokens,
			capabilities, status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, ''), 'active'), $8)
		RETURNING id, provider_id, model_key, display_name, context_window, max_output_tokens,
			capabilities, status, metadata, created_at, updated_at
	`, input.ProviderID, input.ModelKey, input.DisplayName, input.ContextWindow, input.MaxOutputTokens, capsJSON, input.Status, metaJSON), model)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}
	if createPriceForModel(input) {
		currency := currencyOrDefault(input.Currency)
		if _, err := tx.Exec(ctx, `
			INSERT INTO model_price_versions (
				model_id, input_price_per_1k, output_price_per_1k,
				cache_creation_price_per_1k, cache_read_price_per_1k,
				cache_creation_5m_price_per_1k, cache_creation_1h_price_per_1k,
				image_output_price_per_1k, priority_input_price_per_1k,
				priority_output_price_per_1k, priority_cache_read_price_per_1k,
				long_context_threshold, long_context_input_multiplier,
				long_context_output_multiplier, billing_mode, pricing_source, currency
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE(NULLIF($13, 0), 1), COALESCE(NULLIF($14, 0), 1), COALESCE(NULLIF($15, ''), 'token'), COALESCE(NULLIF($16, ''), 'manual'), $17)
		`, model.ID, input.InputPricePer1K, input.OutputPricePer1K, input.CacheCreationPricePer1K, input.CacheReadPricePer1K,
			input.CacheCreation5mPricePer1K, input.CacheCreation1hPricePer1K, input.ImageOutputPricePer1K, input.PriorityInputPricePer1K,
			input.PriorityOutputPricePer1K, input.PriorityCacheReadPricePer1K, input.LongContextThreshold, input.LongContextInputMultiplier,
			input.LongContextOutputMultiplier, input.BillingMode, input.PricingSource, currency); err != nil {
			return nil, fmt.Errorf("create model price version: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create model: %w", err)
	}
	return model, nil
}

func (r *PostgresRepository) ListModels(ctx context.Context, providerID *uuid.UUID, limit int) ([]Model, error) {
	limit = normalizeLimit(limit)
	rows, err := r.db.Query(ctx, `
		SELECT id, provider_id, model_key, display_name, context_window, max_output_tokens,
			capabilities, status, metadata, created_at, updated_at
		FROM models
		WHERE ($1::uuid IS NULL OR provider_id = $1)
		ORDER BY created_at DESC
		LIMIT $2
	`, providerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()
	return scanModels(rows)
}

func (r *PostgresRepository) UpdateModel(ctx context.Context, id uuid.UUID, input UpdateModelInput) (*Model, error) {
	capsJSON, metaJSON, err := modelJSON(input.Capabilities, input.Metadata)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update model: %w", err)
	}
	defer tx.Rollback(ctx)

	model := &Model{}
	err = scanModelRow(tx.QueryRow(ctx, `
		UPDATE models
		SET display_name = COALESCE($2, display_name),
			context_window = COALESCE($3, context_window),
			max_output_tokens = COALESCE($4, max_output_tokens),
			capabilities = CASE WHEN $5::jsonb IS NULL THEN capabilities ELSE $5::jsonb END,
			status = COALESCE($6, status),
			metadata = CASE WHEN $7::jsonb IS NULL THEN metadata ELSE $7::jsonb END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, provider_id, model_key, display_name, context_window, max_output_tokens,
			capabilities, status, metadata, created_at, updated_at
	`, id, input.DisplayName, input.ContextWindow, input.MaxOutputTokens, nullableJSON(capsJSON, input.Capabilities != nil), input.Status, nullableJSON(metaJSON, input.Metadata != nil)), model)
	if err != nil {
		return nil, fmt.Errorf("update model: %w", err)
	}
	if updatePriceForModel(input) {
		if _, err := tx.Exec(ctx, `UPDATE model_price_versions SET effective_to = NOW() WHERE model_id = $1 AND effective_to IS NULL`, id); err != nil {
			return nil, fmt.Errorf("close model price version: %w", err)
		}
		currency := "CNY"
		if input.Currency != nil {
			currency = *input.Currency
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO model_price_versions (
				model_id, input_price_per_1k, output_price_per_1k,
				cache_creation_price_per_1k, cache_read_price_per_1k,
				cache_creation_5m_price_per_1k, cache_creation_1h_price_per_1k,
				image_output_price_per_1k, priority_input_price_per_1k,
				priority_output_price_per_1k, priority_cache_read_price_per_1k,
				long_context_threshold, long_context_input_multiplier,
				long_context_output_multiplier, billing_mode, pricing_source, currency
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE(NULLIF($13, 0), 1), COALESCE(NULLIF($14, 0), 1), COALESCE(NULLIF($15, ''), 'token'), COALESCE(NULLIF($16, ''), 'manual'), $17)
		`, id, floatPtrValue(input.InputPricePer1K), floatPtrValue(input.OutputPricePer1K), floatPtrValue(input.CacheCreationPricePer1K),
			floatPtrValue(input.CacheReadPricePer1K), floatPtrValue(input.CacheCreation5mPricePer1K), floatPtrValue(input.CacheCreation1hPricePer1K),
			floatPtrValue(input.ImageOutputPricePer1K), floatPtrValue(input.PriorityInputPricePer1K), floatPtrValue(input.PriorityOutputPricePer1K),
			floatPtrValue(input.PriorityCacheReadPricePer1K), intPtrValue(input.LongContextThreshold), floatPtrValue(input.LongContextInputMultiplier),
			floatPtrValue(input.LongContextOutputMultiplier), stringPtrValue(input.BillingMode), stringPtrValue(input.PricingSource), currency); err != nil {
			return nil, fmt.Errorf("create model price version: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update model: %w", err)
	}
	return model, nil
}

func (r *PostgresRepository) ResolveInvocationTarget(ctx context.Context, input InvokeInput) (ResolvedModel, error) {
	effectiveInput := input
	if effectiveInput.ProviderID == nil && effectiveInput.PreferredChannelID == nil {
		if err := r.applyRoutingRule(ctx, &effectiveInput); err != nil {
			return ResolvedModel{}, err
		}
	}
	var target ResolvedModel
	var providerEncrypted string
	var inputPrice, outputPrice, cacheCreationPrice, cacheReadPrice, cacheCreation5mPrice, cacheCreation1hPrice float64
	var imageOutputPrice, priorityInputPrice, priorityOutputPrice, priorityCacheReadPrice float64
	var longContextInputMultiplier, longContextOutputMultiplier float64
	var longContextThreshold int
	var billingMode, pricingSource string
	var priceVersionID pgtype.UUID
	var currency string
	err := r.db.QueryRow(ctx, `
		SELECT p.id, m.id, p.provider_type, p.base_url, p.encrypted_api_key, m.model_key,
			pv.id,
			COALESCE(pv.input_price_per_1k, 0)::float8,
			COALESCE(pv.output_price_per_1k, 0)::float8,
			COALESCE(pv.cache_creation_price_per_1k, 0)::float8,
			COALESCE(pv.cache_read_price_per_1k, 0)::float8,
			COALESCE(pv.cache_creation_5m_price_per_1k, 0)::float8,
			COALESCE(pv.cache_creation_1h_price_per_1k, 0)::float8,
			COALESCE(pv.image_output_price_per_1k, 0)::float8,
			COALESCE(pv.priority_input_price_per_1k, 0)::float8,
			COALESCE(pv.priority_output_price_per_1k, 0)::float8,
			COALESCE(pv.priority_cache_read_price_per_1k, 0)::float8,
			COALESCE(pv.long_context_threshold, 0),
			COALESCE(pv.long_context_input_multiplier, 1)::float8,
			COALESCE(pv.long_context_output_multiplier, 1)::float8,
			COALESCE(pv.billing_mode, 'token'),
			COALESCE(pv.pricing_source, 'manual'),
			COALESCE(pv.currency, 'CNY'),
			m.max_output_tokens
		FROM model_providers p
		JOIN models m ON m.provider_id = p.id
		LEFT JOIN LATERAL (
			SELECT id, input_price_per_1k, output_price_per_1k,
				cache_creation_price_per_1k, cache_read_price_per_1k,
				cache_creation_5m_price_per_1k, cache_creation_1h_price_per_1k,
				image_output_price_per_1k, priority_input_price_per_1k,
				priority_output_price_per_1k, priority_cache_read_price_per_1k,
				long_context_threshold, long_context_input_multiplier,
				long_context_output_multiplier, billing_mode, pricing_source, currency
			FROM model_price_versions
			WHERE model_id = m.id AND (effective_to IS NULL OR effective_to > NOW())
			ORDER BY effective_from DESC
			LIMIT 1
		) pv ON true
		WHERE p.status = 'active'
		  AND m.status = 'active'
		  AND (($1::uuid IS NULL AND p.provider_type = $2) OR p.id = $1)
		  AND (($3::uuid IS NULL AND m.model_key = $4) OR m.id = $3)
		LIMIT 1
	`, effectiveInput.ProviderID, effectiveInput.ProviderType, effectiveInput.ModelID, effectiveInput.Model).
		Scan(&target.ProviderID, &target.ModelID, &target.ProviderType, &target.BaseURL, &providerEncrypted, &target.Model, &priceVersionID,
			&inputPrice, &outputPrice, &cacheCreationPrice, &cacheReadPrice, &cacheCreation5mPrice, &cacheCreation1hPrice, &imageOutputPrice,
			&priorityInputPrice, &priorityOutputPrice, &priorityCacheReadPrice, &longContextThreshold, &longContextInputMultiplier,
			&longContextOutputMultiplier, &billingMode, &pricingSource, &currency, &target.MaxOutputTokens)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return target, fmt.Errorf("%w: no active model matches provider/model selection", ErrUnavailable)
		}
		return target, fmt.Errorf("resolve invocation target: %w", err)
	}
	if priceVersionID.Valid {
		id, err := uuid.FromBytes(priceVersionID.Bytes[:])
		if err != nil {
			return target, fmt.Errorf("decode price version id: %w", err)
		}
		target.PriceVersionID = &id
	}
	target.RequestedModel = firstNonEmpty(effectiveInput.Model, target.Model)
	target.UpstreamModel = target.Model
	target.Price = Price{
		InputPer1K:                  inputPrice,
		OutputPer1K:                 outputPrice,
		CacheCreationPer1K:          cacheCreationPrice,
		CacheReadPer1K:              cacheReadPrice,
		CacheCreation5mPer1K:        cacheCreation5mPrice,
		CacheCreation1hPer1K:        cacheCreation1hPrice,
		ImageOutputPer1K:            imageOutputPrice,
		PriorityInputPer1K:          priorityInputPrice,
		PriorityOutputPer1K:         priorityOutputPrice,
		PriorityCacheReadPer1K:      priorityCacheReadPrice,
		LongContextThreshold:        longContextThreshold,
		LongContextInputMultiplier:  longContextInputMultiplier,
		LongContextOutputMultiplier: longContextOutputMultiplier,
		BillingMode:                 billingMode,
		PricingSource:               pricingSource,
	}
	target.Currency = currency
	target.RateMultiplier = 1
	if err := r.applyChannel(ctx, effectiveInput, &target); err != nil {
		return target, err
	}
	if target.APIKey == "" {
		apiKey, err := r.box.Decrypt(providerEncrypted)
		if err != nil {
			return target, fmt.Errorf("decrypt provider api key: %w", err)
		}
		target.APIKey = apiKey
	}
	return target, nil
}

func (r *PostgresRepository) ResolvePricingTarget(ctx context.Context, input EstimateCostInput) (ResolvedModel, error) {
	var target ResolvedModel
	var inputPrice, outputPrice, cacheCreationPrice, cacheReadPrice, cacheCreation5mPrice, cacheCreation1hPrice float64
	var imageOutputPrice, priorityInputPrice, priorityOutputPrice, priorityCacheReadPrice float64
	var longContextInputMultiplier, longContextOutputMultiplier float64
	var longContextThreshold int
	var billingMode, pricingSource string
	var priceVersionID pgtype.UUID
	var currency string
	err := r.db.QueryRow(ctx, `
		SELECT p.id, m.id, p.provider_type, p.base_url, m.model_key,
			pv.id,
			COALESCE(pv.input_price_per_1k, 0)::float8,
			COALESCE(pv.output_price_per_1k, 0)::float8,
			COALESCE(pv.cache_creation_price_per_1k, 0)::float8,
			COALESCE(pv.cache_read_price_per_1k, 0)::float8,
			COALESCE(pv.cache_creation_5m_price_per_1k, 0)::float8,
			COALESCE(pv.cache_creation_1h_price_per_1k, 0)::float8,
			COALESCE(pv.image_output_price_per_1k, 0)::float8,
			COALESCE(pv.priority_input_price_per_1k, 0)::float8,
			COALESCE(pv.priority_output_price_per_1k, 0)::float8,
			COALESCE(pv.priority_cache_read_price_per_1k, 0)::float8,
			COALESCE(pv.long_context_threshold, 0),
			COALESCE(pv.long_context_input_multiplier, 1)::float8,
			COALESCE(pv.long_context_output_multiplier, 1)::float8,
			COALESCE(pv.billing_mode, 'token'),
			COALESCE(pv.pricing_source, 'manual'),
			COALESCE(pv.currency, 'CNY'),
			m.max_output_tokens
		FROM model_providers p
		JOIN models m ON m.provider_id = p.id
		LEFT JOIN LATERAL (
			SELECT id, input_price_per_1k, output_price_per_1k,
				cache_creation_price_per_1k, cache_read_price_per_1k,
				cache_creation_5m_price_per_1k, cache_creation_1h_price_per_1k,
				image_output_price_per_1k, priority_input_price_per_1k,
				priority_output_price_per_1k, priority_cache_read_price_per_1k,
				long_context_threshold, long_context_input_multiplier,
				long_context_output_multiplier, billing_mode, pricing_source, currency
			FROM model_price_versions
			WHERE model_id = m.id AND (effective_to IS NULL OR effective_to > NOW())
			ORDER BY effective_from DESC
			LIMIT 1
		) pv ON true
		WHERE m.status = 'active'
		  AND ($1::uuid IS NULL OR p.id = $1)
		  AND (NULLIF($2, '') IS NULL OR p.provider_type = $2)
		  AND m.model_key = $3
		ORDER BY CASE WHEN p.status = 'active' THEN 0 ELSE 1 END, m.created_at DESC
		LIMIT 1
	`, input.ProviderID, input.ProviderType, input.Model).
		Scan(&target.ProviderID, &target.ModelID, &target.ProviderType, &target.BaseURL, &target.Model, &priceVersionID,
			&inputPrice, &outputPrice, &cacheCreationPrice, &cacheReadPrice, &cacheCreation5mPrice, &cacheCreation1hPrice, &imageOutputPrice,
			&priorityInputPrice, &priorityOutputPrice, &priorityCacheReadPrice, &longContextThreshold, &longContextInputMultiplier,
			&longContextOutputMultiplier, &billingMode, &pricingSource, &currency, &target.MaxOutputTokens)
	if err != nil {
		return target, fmt.Errorf("resolve pricing target: %w", err)
	}
	if priceVersionID.Valid {
		id, err := uuid.FromBytes(priceVersionID.Bytes[:])
		if err != nil {
			return target, fmt.Errorf("decode price version id: %w", err)
		}
		target.PriceVersionID = &id
	}
	target.RequestedModel = input.Model
	target.UpstreamModel = target.Model
	target.Price = Price{
		InputPer1K:                  inputPrice,
		OutputPer1K:                 outputPrice,
		CacheCreationPer1K:          cacheCreationPrice,
		CacheReadPer1K:              cacheReadPrice,
		CacheCreation5mPer1K:        cacheCreation5mPrice,
		CacheCreation1hPer1K:        cacheCreation1hPrice,
		ImageOutputPer1K:            imageOutputPrice,
		PriorityInputPer1K:          priorityInputPrice,
		PriorityOutputPer1K:         priorityOutputPrice,
		PriorityCacheReadPer1K:      priorityCacheReadPrice,
		LongContextThreshold:        longContextThreshold,
		LongContextInputMultiplier:  longContextInputMultiplier,
		LongContextOutputMultiplier: longContextOutputMultiplier,
		BillingMode:                 billingMode,
		PricingSource:               pricingSource,
	}
	target.Currency = currency
	target.RateMultiplier = 1
	return target, nil
}

func (r *PostgresRepository) CreateInvocation(ctx context.Context, input CreateInvocationInput) (*Invocation, error) {
	inv := &Invocation{}
	err := scanInvocationRow(r.db.QueryRow(ctx, `
		INSERT INTO ai_invocations (
			provider_id, model_id, channel_id, access_token_id, model_group_id, mode, status, organization_id, department_id, project_id,
			requirement_id, workflow_id, task_id, agent_id, user_id, capability_id,
			source_surface, requested_model, upstream_model, model_mapping_chain, service_tier,
			reasoning_effort, request_hash, estimated_input_tokens, estimated_output_tokens, reserved_amount, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27)
		RETURNING id, provider_id, model_id, channel_id, mode, status,
			organization_id, department_id, project_id, requirement_id, workflow_id, task_id,
			agent_id, user_id, capability_id, source_surface, requested_model, upstream_model,
			model_mapping_chain, service_tier, reasoning_effort,
			request_hash, provider_request_id,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			cache_creation_5m_tokens, cache_creation_1h_tokens, image_output_tokens,
			estimated_input_tokens, estimated_output_tokens,
			cost_amount::float8, cost_breakdown, currency, first_token_ms, duration_ms, error_type, error_message,
			metadata, created_at, completed_at
	`, input.ProviderID, input.ModelID, input.ChannelID, input.AccessTokenID, input.ModelGroupID, input.Mode, input.Status, input.Attribution.OrganizationID, input.Attribution.DepartmentID, input.Attribution.ProjectID,
		input.Attribution.RequirementID, input.Attribution.WorkflowID, input.Attribution.TaskID, input.Attribution.AgentID, input.Attribution.UserID,
		input.Attribution.CapabilityID, input.Attribution.SourceSurface, input.RequestedModel, input.UpstreamModel, input.ModelMappingChain,
		input.ServiceTier, input.ReasoningEffort, input.RequestHash, input.EstimatedInputTokens, input.EstimatedOutputTokens, input.ReservedAmount, mustJSON(input.Metadata)), inv)
	if err != nil {
		return nil, fmt.Errorf("create ai invocation: %w", err)
	}
	inv.Attribution = input.Attribution
	return inv, nil
}

func (r *PostgresRepository) CompleteInvocation(ctx context.Context, id uuid.UUID, input CompleteInvocationInput) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE ai_invocations
		SET status = $2,
			provider_request_id = COALESCE(NULLIF($3, ''), provider_request_id),
			input_tokens = $4,
			output_tokens = $5,
			cache_creation_tokens = $6,
			cache_read_tokens = $7,
			cache_creation_5m_tokens = $8,
			cache_creation_1h_tokens = $9,
			image_output_tokens = $10,
			cost_amount = $11,
			settled_amount = $11,
			cost_breakdown = $12,
			currency = $13,
			duration_ms = $14,
			completed_at = NOW()
		WHERE id = $1
	`, id, StatusCompleted, input.ProviderRequestID, input.Usage.InputTokens, input.Usage.OutputTokens, input.Usage.CacheCreationTokens, input.Usage.CacheReadTokens,
		input.Usage.CacheCreation5mTokens, input.Usage.CacheCreation1hTokens, input.Usage.ImageOutputTokens, input.CostAmount, mustJSONAny(input.CostBreakdown),
		currencyOrDefault(input.Currency), input.DurationMS); err != nil {
		return fmt.Errorf("complete ai invocation: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FailInvocation(ctx context.Context, id uuid.UUID, input FailInvocationInput) error {
	status := StatusFailed
	if input.Cancelled {
		status = StatusCancelled
	}
	if _, err := r.db.Exec(ctx, `
		UPDATE ai_invocations
		SET status = $2, error_type = $3, error_message = $4, duration_ms = $5, completed_at = NOW()
		WHERE id = $1
	`, id, status, input.ErrorType, input.Message, input.DurationMS); err != nil {
		return fmt.Errorf("fail ai invocation: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreateUsageLedger(ctx context.Context, input CreateUsageLedgerInput) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO ai_usage_ledger (
			invocation_id, access_token_id, model_group_id, channel_id, model_price_version_id, ledger_type, amount, actual_amount, currency,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			cache_creation_5m_tokens, cache_creation_1h_tokens, image_output_tokens,
			input_cost, output_cost, cache_creation_cost, cache_read_cost, image_output_cost,
			rate_multiplier, service_tier, reasoning_effort, requested_model, upstream_model, metadata, reason
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, COALESCE(NULLIF($8::numeric, 0::numeric), $7::numeric), $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28)
	`, input.InvocationID, input.AccessTokenID, input.ModelGroupID, input.ChannelID, input.ModelPriceVersionID, input.LedgerType, input.Amount, input.ActualAmount, currencyOrDefault(input.Currency),
		input.Usage.InputTokens, input.Usage.OutputTokens, input.Usage.CacheCreationTokens, input.Usage.CacheReadTokens,
		input.Usage.CacheCreation5mTokens, input.Usage.CacheCreation1hTokens, input.Usage.ImageOutputTokens,
		input.CostBreakdown.InputCost, input.CostBreakdown.OutputCost, input.CostBreakdown.CacheCreationCost, input.CostBreakdown.CacheReadCost,
		input.CostBreakdown.ImageOutputCost, input.CostBreakdown.RateMultiplier, input.ServiceTier, input.ReasoningEffort,
		input.RequestedModel, input.UpstreamModel, mustJSON(input.Metadata), input.Reason); err != nil {
		return fmt.Errorf("create ai usage ledger: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListInvocations(ctx context.Context, limit int) ([]Invocation, error) {
	orgID := nullableUUID(currentTenantOrganizationID(ctx))
	rows, err := r.db.Query(ctx, `
		SELECT id, provider_id, model_id, channel_id, mode, status,
			organization_id, department_id, project_id, requirement_id, workflow_id, task_id,
			agent_id, user_id, capability_id, source_surface, requested_model, upstream_model,
			model_mapping_chain, service_tier, reasoning_effort,
			request_hash, provider_request_id,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			cache_creation_5m_tokens, cache_creation_1h_tokens, image_output_tokens,
			estimated_input_tokens, estimated_output_tokens,
			cost_amount::float8, cost_breakdown, currency, first_token_ms, duration_ms, error_type, error_message,
			metadata, created_at, completed_at
		FROM ai_invocations
		WHERE $2::uuid IS NULL OR organization_id = $2
		ORDER BY created_at DESC
		LIMIT $1
	`, normalizeLimit(limit), orgID)
	if err != nil {
		return nil, fmt.Errorf("list ai invocations: %w", err)
	}
	defer rows.Close()
	return scanInvocations(rows)
}

func (r *PostgresRepository) GetInvocation(ctx context.Context, id uuid.UUID) (*Invocation, error) {
	inv := &Invocation{}
	orgID := nullableUUID(currentTenantOrganizationID(ctx))
	err := scanInvocationRow(r.db.QueryRow(ctx, `
		SELECT id, provider_id, model_id, channel_id, mode, status,
			organization_id, department_id, project_id, requirement_id, workflow_id, task_id,
			agent_id, user_id, capability_id, source_surface, requested_model, upstream_model,
			model_mapping_chain, service_tier, reasoning_effort,
			request_hash, provider_request_id,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			cache_creation_5m_tokens, cache_creation_1h_tokens, image_output_tokens,
			estimated_input_tokens, estimated_output_tokens,
			cost_amount::float8, cost_breakdown, currency, first_token_ms, duration_ms, error_type, error_message,
			metadata, created_at, completed_at
		FROM ai_invocations
		WHERE id = $1 AND ($2::uuid IS NULL OR organization_id = $2)
	`, id, orgID), inv)
	if err != nil {
		return nil, fmt.Errorf("get ai invocation: %w", err)
	}
	return inv, nil
}

func (r *PostgresRepository) CostSummary(ctx context.Context) (*GatewayCostSummary, error) {
	summary := &GatewayCostSummary{Currency: "CNY", ByProvider: map[string]float64{}, ByChannel: map[string]float64{}}
	orgID := nullableUUID(currentTenantOrganizationID(ctx))
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(l.amount), 0)::float8,
			COALESCE(SUM(l.amount) FILTER (WHERE l.finance_export_line_id IS NULL), 0)::float8,
			COALESCE(MAX(l.currency), 'CNY')
		FROM ai_usage_ledger l
		JOIN ai_invocations i ON i.id = l.invocation_id
		WHERE $1::uuid IS NULL OR i.organization_id = $1
	`, orgID).Scan(&summary.Total, &summary.Unexported, &summary.Currency); err != nil {
		return nil, fmt.Errorf("query ai cost summary: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT p.provider_type, COALESCE(SUM(l.amount), 0)::float8
		FROM ai_usage_ledger l
		JOIN ai_invocations i ON i.id = l.invocation_id
		JOIN model_providers p ON p.id = i.provider_id
		WHERE $1::uuid IS NULL OR i.organization_id = $1
		GROUP BY p.provider_type
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query ai cost by provider: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var amount float64
		if err := rows.Scan(&provider, &amount); err != nil {
			return nil, fmt.Errorf("scan ai cost by provider: %w", err)
		}
		summary.ByProvider[provider] = amount
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai cost by provider: %w", err)
	}
	channelRows, err := r.db.Query(ctx, `
		SELECT COALESCE(c.name, 'provider_default'), COALESCE(SUM(l.amount), 0)::float8
		FROM ai_usage_ledger l
		JOIN ai_invocations i ON i.id = l.invocation_id
		LEFT JOIN model_provider_channels c ON c.id = l.channel_id
		WHERE $1::uuid IS NULL OR i.organization_id = $1
		GROUP BY COALESCE(c.name, 'provider_default')
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query ai cost by channel: %w", err)
	}
	defer channelRows.Close()
	for channelRows.Next() {
		var channel string
		var amount float64
		if err := channelRows.Scan(&channel, &amount); err != nil {
			return nil, fmt.Errorf("scan ai cost by channel: %w", err)
		}
		summary.ByChannel[channel] = amount
	}
	if err := channelRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai cost by channel: %w", err)
	}
	return summary, nil
}

func (r *PostgresRepository) CreateChannel(ctx context.Context, input CreateChannelInput) (*ProviderChannel, error) {
	encrypted, err := r.box.Encrypt(input.APIKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt channel api key: %w", err)
	}
	patternsJSON, mappingJSON, metaJSON, err := channelJSON(input.SupportedModelPatterns, input.ModelMapping, input.Metadata)
	if err != nil {
		return nil, err
	}
	channel := &ProviderChannel{}
	err = scanChannelRow(r.db.QueryRow(ctx, `
		INSERT INTO model_provider_channels (
			provider_id, name, base_url, encrypted_api_key, masked_api_key, owner_type,
			user_id, agent_id, status, priority, concurrency_limit, load_factor,
			rate_multiplier, quota_amount, quota_currency, supported_model_patterns,
			model_mapping, metadata
		)
		VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), ''), $7, $8,
			COALESCE(NULLIF($9, ''), 'active'), COALESCE(NULLIF($10, 0), 50),
			$11, COALESCE(NULLIF($12, 0), 1), COALESCE($13, 1),
			$14, COALESCE(NULLIF($15, ''), 'CNY'), $16, $17, $18)
		RETURNING id, provider_id, name, base_url, masked_api_key, owner_type, user_id, agent_id,
			status, priority, concurrency_limit, inflight_requests, load_factor, rate_multiplier::float8,
			quota_amount::float8, quota_used::float8, quota_currency, supported_model_patterns, model_mapping,
			health_status, last_error, last_tested_at, last_used_at, metadata, created_at, updated_at
	`, input.ProviderID, input.Name, input.BaseURL, encrypted, maskSecret(input.APIKey), input.OwnerType, input.UserID, input.AgentID,
		input.Status, input.Priority, input.ConcurrencyLimit, input.LoadFactor, nullableFloat64(input.RateMultiplier), input.QuotaAmount, input.QuotaCurrency,
		patternsJSON, mappingJSON, metaJSON), channel)
	if err != nil {
		return nil, fmt.Errorf("create provider channel: %w", err)
	}
	return channel, nil
}

func (r *PostgresRepository) ListChannels(ctx context.Context, providerID *uuid.UUID, limit int) ([]ProviderChannel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, provider_id, name, base_url, masked_api_key, owner_type, user_id, agent_id,
			status, priority, concurrency_limit, inflight_requests, load_factor, rate_multiplier::float8,
			quota_amount::float8, quota_used::float8, quota_currency, supported_model_patterns, model_mapping,
			health_status, last_error, last_tested_at, last_used_at, metadata, created_at, updated_at
		FROM model_provider_channels
		WHERE ($1::uuid IS NULL OR provider_id = $1)
		ORDER BY priority ASC, created_at DESC
		LIMIT $2
	`, providerID, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list provider channels: %w", err)
	}
	defer rows.Close()
	return scanChannels(rows)
}

func (r *PostgresRepository) UpdateChannel(ctx context.Context, id uuid.UUID, input UpdateChannelInput) (*ProviderChannel, error) {
	patternsJSON, mappingJSON, metaJSON, err := channelJSON(input.SupportedModelPatterns, input.ModelMapping, input.Metadata)
	if err != nil {
		return nil, err
	}
	channel := &ProviderChannel{}
	err = scanChannelRow(r.db.QueryRow(ctx, `
		UPDATE model_provider_channels
		SET name = COALESCE($2, name),
			base_url = COALESCE($3, base_url),
			owner_type = COALESCE($4, owner_type),
			user_id = COALESCE($5, user_id),
			agent_id = COALESCE($6, agent_id),
			status = COALESCE($7, status),
			priority = COALESCE($8, priority),
			concurrency_limit = COALESCE($9, concurrency_limit),
			load_factor = COALESCE($10, load_factor),
			rate_multiplier = COALESCE($11, rate_multiplier),
			quota_amount = COALESCE($12, quota_amount),
			quota_currency = COALESCE($13, quota_currency),
			supported_model_patterns = CASE WHEN $14::jsonb IS NULL THEN supported_model_patterns ELSE $14::jsonb END,
			model_mapping = CASE WHEN $15::jsonb IS NULL THEN model_mapping ELSE $15::jsonb END,
			metadata = CASE WHEN $16::jsonb IS NULL THEN metadata ELSE $16::jsonb END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, provider_id, name, base_url, masked_api_key, owner_type, user_id, agent_id,
			status, priority, concurrency_limit, inflight_requests, load_factor, rate_multiplier::float8,
			quota_amount::float8, quota_used::float8, quota_currency, supported_model_patterns, model_mapping,
			health_status, last_error, last_tested_at, last_used_at, metadata, created_at, updated_at
	`, id, input.Name, input.BaseURL, input.OwnerType, input.UserID, input.AgentID, input.Status, input.Priority, input.ConcurrencyLimit,
		input.LoadFactor, input.RateMultiplier, input.QuotaAmount, input.QuotaCurrency, nullableJSON(patternsJSON, input.SupportedModelPatterns != nil),
		nullableJSON(mappingJSON, input.ModelMapping != nil), nullableJSON(metaJSON, input.Metadata != nil)), channel)
	if err != nil {
		return nil, fmt.Errorf("update provider channel: %w", err)
	}
	return channel, nil
}

func (r *PostgresRepository) RotateChannelKey(ctx context.Context, id uuid.UUID, apiKey string) (*ProviderChannel, error) {
	encrypted, err := r.box.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt channel api key: %w", err)
	}
	channel := &ProviderChannel{}
	err = scanChannelRow(r.db.QueryRow(ctx, `
		UPDATE model_provider_channels
		SET encrypted_api_key = $2, masked_api_key = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, provider_id, name, base_url, masked_api_key, owner_type, user_id, agent_id,
			status, priority, concurrency_limit, inflight_requests, load_factor, rate_multiplier::float8,
			quota_amount::float8, quota_used::float8, quota_currency, supported_model_patterns, model_mapping,
			health_status, last_error, last_tested_at, last_used_at, metadata, created_at, updated_at
	`, id, encrypted, maskSecret(apiKey)), channel)
	if err != nil {
		return nil, fmt.Errorf("rotate provider channel key: %w", err)
	}
	return channel, nil
}

func (r *PostgresRepository) GetChannelSecret(ctx context.Context, id uuid.UUID) (ChannelSecret, error) {
	var channel ChannelSecret
	var encrypted string
	err := r.db.QueryRow(ctx, `
		SELECT c.id, c.provider_id, p.provider_type, c.name, COALESCE(NULLIF(c.base_url, ''), p.base_url), c.encrypted_api_key, c.status
		FROM model_provider_channels c
		JOIN model_providers p ON p.id = c.provider_id
		WHERE c.id = $1
	`, id).Scan(&channel.ID, &channel.ProviderID, &channel.ProviderType, &channel.Name, &channel.BaseURL, &encrypted, &channel.Status)
	if err != nil {
		return channel, fmt.Errorf("get channel secret: %w", err)
	}
	apiKey, err := r.box.Decrypt(encrypted)
	if err != nil {
		return channel, fmt.Errorf("decrypt channel api key: %w", err)
	}
	channel.APIKey = apiKey
	return channel, nil
}

func (r *PostgresRepository) UpdateChannelTestResult(ctx context.Context, id uuid.UUID, status string, message string) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE model_provider_channels
		SET health_status = $2, last_error = $3, last_tested_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, status, message); err != nil {
		return fmt.Errorf("update channel test result: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ReleaseChannel(ctx context.Context, id *uuid.UUID, amount float64) error {
	if id == nil {
		return nil
	}
	if _, err := r.db.Exec(ctx, `
		UPDATE model_provider_channels
		SET inflight_requests = GREATEST(inflight_requests - 1, 0),
			quota_used = quota_used + GREATEST($2::numeric, 0),
			last_used_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, *id, amount); err != nil {
		return fmt.Errorf("release provider channel: %w", err)
	}
	return nil
}

func (r *PostgresRepository) AuthenticateAccessToken(ctx context.Context, token string) (AccessTokenContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AccessTokenContext{}, fmt.Errorf("%w: bearer token is required", ErrForbidden)
	}
	var result AccessTokenContext
	var modelGroupID pgtype.UUID
	var allowedModelsJSON, allowedPatternsJSON, metaJSON []byte
	err := r.db.QueryRow(ctx, `
		UPDATE ai_access_tokens
		SET last_used_at = NOW(), updated_at = NOW()
		WHERE token_hash = $1
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > NOW())
		RETURNING id, organization_id, model_group_id, allowed_models, allowed_model_patterns,
			allow_channel_override, status, metadata
	`, hashAccessToken(token)).Scan(&result.ID, &result.OrganizationID, &modelGroupID, &allowedModelsJSON, &allowedPatternsJSON,
		&result.AllowChannelOverride, &result.Status, &metaJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AccessTokenContext{}, fmt.Errorf("%w: invalid or expired ai access token", ErrForbidden)
		}
		return AccessTokenContext{}, fmt.Errorf("authenticate ai access token: %w", err)
	}
	result.ModelGroupID = uuidPointer(modelGroupID)
	if err := json.Unmarshal(allowedModelsJSON, &result.AllowedModels); err != nil {
		return AccessTokenContext{}, fmt.Errorf("unmarshal access token allowed models: %w", err)
	}
	if err := json.Unmarshal(allowedPatternsJSON, &result.AllowedModelPatterns); err != nil {
		return AccessTokenContext{}, fmt.Errorf("unmarshal access token allowed model patterns: %w", err)
	}
	if err := json.Unmarshal(metaJSON, &result.Metadata); err != nil {
		return AccessTokenContext{}, fmt.Errorf("unmarshal access token metadata: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) ReserveAccessTokenBalance(ctx context.Context, input ReserveBalanceInput) (BalanceReservation, error) {
	amount := roundCost(input.EstimatedAmount)
	if amount < 0 {
		amount = 0
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return BalanceReservation{}, fmt.Errorf("begin reserve ai gateway balance: %w", err)
	}
	defer tx.Rollback(ctx)

	currency := currencyOrDefault(input.Currency)
	tokenTag, err := tx.Exec(ctx, `
		UPDATE ai_access_tokens
		SET quota_used = quota_used + $3,
			updated_at = NOW()
		WHERE id = $1
		  AND quota_currency = $2
		  AND organization_id = $4
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > NOW())
		  AND (quota_amount <= 0 OR quota_used + $3 <= quota_amount)
	`, input.AccessTokenID, currency, amount, input.OrganizationID)
	if err != nil {
		return BalanceReservation{}, fmt.Errorf("reserve ai access token quota: %w", err)
	}
	if tokenTag.RowsAffected() == 0 {
		return BalanceReservation{}, fmt.Errorf("%w: ai access token quota is insufficient", ErrForbidden)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_gateway_balances(organization_id, currency)
		VALUES ($1, $2)
		ON CONFLICT (organization_id, currency) DO NOTHING
	`, input.OrganizationID, currency); err != nil {
		return BalanceReservation{}, fmt.Errorf("ensure ai gateway balance: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE ai_gateway_balances
		SET reserved_amount = reserved_amount + $3,
			updated_at = NOW()
		WHERE organization_id = $1
		  AND currency = $2
		  AND balance_amount - reserved_amount >= $3
	`, input.OrganizationID, currency, amount)
	if err != nil {
		return BalanceReservation{}, fmt.Errorf("reserve ai gateway balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return BalanceReservation{}, fmt.Errorf("%w: ai gateway balance is insufficient", ErrForbidden)
	}
	reservation := BalanceReservation{ReservedAmount: amount, Currency: currency}
	if err := tx.QueryRow(ctx, `
		INSERT INTO ai_gateway_balance_transactions(
			organization_id, access_token_id, model_group_id, transaction_type,
			amount, currency, reason, metadata
		)
		VALUES ($1, $2, $3, 'reserve', $4, $5, $6, $7)
		RETURNING id
	`, input.OrganizationID, input.AccessTokenID, input.ModelGroupID, amount, currency, input.Reason, mustJSON(map[string]any{"estimated": true})).Scan(&reservation.ID); err != nil {
		return BalanceReservation{}, fmt.Errorf("record ai gateway balance reservation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return BalanceReservation{}, fmt.Errorf("commit reserve ai gateway balance: %w", err)
	}
	return reservation, nil
}

func (r *PostgresRepository) SettleAccessTokenBalance(ctx context.Context, input SettleBalanceInput) error {
	return r.finishAccessTokenReservation(ctx, input.ReservationID, "settle", input.ActualAmount, input.Reason, input.AccessTokenID)
}

func (r *PostgresRepository) RefundAccessTokenBalance(ctx context.Context, reservationID uuid.UUID, reason string) error {
	return r.finishAccessTokenReservation(ctx, reservationID, "refund", 0, reason, uuid.Nil)
}

func (r *PostgresRepository) finishAccessTokenReservation(ctx context.Context, reservationID uuid.UUID, transactionType string, actualAmount float64, reason string, accessTokenID uuid.UUID) error {
	if reservationID == uuid.Nil {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finish ai gateway balance reservation: %w", err)
	}
	defer tx.Rollback(ctx)

	var orgID uuid.UUID
	var tokenID, modelGroupID pgtype.UUID
	var reservedAmount float64
	var currency string
	err = tx.QueryRow(ctx, `
		SELECT organization_id, access_token_id, model_group_id, amount::float8, currency
		FROM ai_gateway_balance_transactions
		WHERE id = $1 AND transaction_type = 'reserve'
		FOR UPDATE
	`, reservationID).Scan(&orgID, &tokenID, &modelGroupID, &reservedAmount, &currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: balance reservation not found", ErrNotFound)
		}
		return fmt.Errorf("get ai gateway balance reservation: %w", err)
	}
	var existingFinish uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM ai_gateway_balance_transactions
		WHERE reservation_id = $1
		  AND transaction_type IN ('settle', 'refund')
		FOR UPDATE
	`, reservationID).Scan(&existingFinish)
	if err == nil {
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check ai gateway balance reservation finish: %w", err)
	}
	settledAmount := 0.0
	transactionAmount := -reservedAmount
	if transactionType == "settle" {
		settledAmount = roundCost(actualAmount)
		transactionAmount = settledAmount
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ai_gateway_balances
		SET reserved_amount = GREATEST(reserved_amount - $3, 0),
			balance_amount = balance_amount - $4,
			updated_at = NOW()
		WHERE organization_id = $1 AND currency = $2
	`, orgID, currency, reservedAmount, settledAmount); err != nil {
		return fmt.Errorf("finish ai gateway balance reservation: %w", err)
	}
	if !tokenID.Valid && accessTokenID != uuid.Nil {
		copy(tokenID.Bytes[:], accessTokenID[:])
		tokenID.Valid = true
	}
	if tokenID.Valid {
		quotaDelta := -reservedAmount
		if transactionType == "settle" {
			quotaDelta = settledAmount - reservedAmount
		}
		if _, err := tx.Exec(ctx, `
			UPDATE ai_access_tokens
			SET quota_used = GREATEST(quota_used + $2, 0),
				updated_at = NOW()
			WHERE id = $1
		`, uuidPointer(tokenID), quotaDelta); err != nil {
			return fmt.Errorf("reconcile ai access token quota: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_gateway_balance_transactions(
			organization_id, access_token_id, model_group_id, reservation_id,
			transaction_type, amount, currency, reason, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, orgID, uuidPointer(tokenID), uuidPointer(modelGroupID), reservationID, transactionType,
		transactionAmount, currency, reason, mustJSON(map[string]any{"reserved_amount": reservedAmount})); err != nil {
		return fmt.Errorf("record ai gateway balance %s: %w", transactionType, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finish ai gateway balance reservation: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListRoutingRules(ctx context.Context, limit int) ([]RoutingRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, provider_id, channel_id, match_scope, match_value, model_pattern,
			priority, status, metadata, created_at, updated_at
		FROM ai_routing_rules
		ORDER BY priority ASC, created_at DESC
		LIMIT $1
	`, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list routing rules: %w", err)
	}
	defer rows.Close()
	return scanRoutingRules(rows)
}

func (r *PostgresRepository) CreateRoutingRule(ctx context.Context, input CreateRoutingRuleInput) (*RoutingRule, error) {
	rule := &RoutingRule{}
	err := scanRoutingRuleRow(r.db.QueryRow(ctx, `
		INSERT INTO ai_routing_rules (
			name, provider_id, channel_id, match_scope, match_value, model_pattern,
			priority, status, metadata
		)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'global'), $5, $6,
			COALESCE(NULLIF($7, 0), 100), COALESCE(NULLIF($8, ''), 'active'), $9)
		RETURNING id, name, provider_id, channel_id, match_scope, match_value, model_pattern,
			priority, status, metadata, created_at, updated_at
	`, input.Name, input.ProviderID, input.ChannelID, input.MatchScope, input.MatchValue, input.ModelPattern, input.Priority, input.Status, mustJSON(input.Metadata)), rule)
	if err != nil {
		return nil, fmt.Errorf("create routing rule: %w", err)
	}
	return rule, nil
}

func (r *PostgresRepository) CreateAccessToken(ctx context.Context, input CreateAccessTokenStoreInput) (*AccessToken, error) {
	allowedModelsJSON, err := json.Marshal(input.AllowedModels)
	if err != nil {
		return nil, fmt.Errorf("marshal access token allowed models: %w", err)
	}
	allowedPatternsJSON, err := json.Marshal(input.AllowedModelPatterns)
	if err != nil {
		return nil, fmt.Errorf("marshal access token allowed model patterns: %w", err)
	}
	token := &AccessToken{}
	err = scanAccessTokenRow(r.db.QueryRow(ctx, `
		INSERT INTO ai_access_tokens(
			organization_id, model_group_id, name, token_hash, masked_token,
			allowed_models, allowed_model_patterns, allow_channel_override,
			quota_amount, quota_currency, expires_at, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, organization_id, model_group_id, name, token_hash, masked_token, status,
			allowed_models, allowed_model_patterns, allow_channel_override,
			quota_amount::float8, quota_used::float8, quota_currency,
			expires_at, last_used_at, metadata, created_at, updated_at
	`, input.OrganizationID, input.ModelGroupID, input.Name, input.TokenHash, input.MaskedToken,
		allowedModelsJSON, allowedPatternsJSON, input.AllowChannelOverride, input.QuotaAmount,
		currencyOrDefault(input.QuotaCurrency), input.ExpiresAt, mustJSON(input.Metadata)), token)
	if err != nil {
		return nil, fmt.Errorf("create ai access token: %w", err)
	}
	return token, nil
}

func (r *PostgresRepository) ListAccessTokens(ctx context.Context, organizationID *uuid.UUID, limit int) ([]AccessToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, model_group_id, name, token_hash, masked_token, status,
			allowed_models, allowed_model_patterns, allow_channel_override,
			quota_amount::float8, quota_used::float8, quota_currency,
			expires_at, last_used_at, metadata, created_at, updated_at
		FROM ai_access_tokens
		WHERE $1::uuid IS NULL OR organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, organizationID, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list ai access tokens: %w", err)
	}
	defer rows.Close()
	items := []AccessToken{}
	for rows.Next() {
		var item AccessToken
		if err := scanAccessTokenRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan ai access token: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateModelGroup(ctx context.Context, input CreateModelGroupInput) (*ModelGroup, error) {
	group := &ModelGroup{}
	err := scanModelGroupRow(r.db.QueryRow(ctx, `
		INSERT INTO ai_model_groups(
			organization_id, department_id, project_id, agent_id, name, group_key,
			status, rate_multiplier, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, ''), 'active'), COALESCE(NULLIF($8, 0), 1), $9)
		RETURNING id, organization_id, department_id, project_id, agent_id, name, group_key,
			status, rate_multiplier::float8, metadata, created_at, updated_at
	`, input.OrganizationID, input.DepartmentID, input.ProjectID, input.AgentID, input.Name, input.GroupKey,
		input.Status, input.RateMultiplier, mustJSON(input.Metadata)), group)
	if err != nil {
		return nil, fmt.Errorf("create ai model group: %w", err)
	}
	return group, nil
}

func (r *PostgresRepository) ListModelGroups(ctx context.Context, organizationID *uuid.UUID, limit int) ([]ModelGroup, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, department_id, project_id, agent_id, name, group_key,
			status, rate_multiplier::float8, metadata, created_at, updated_at
		FROM ai_model_groups
		WHERE $1::uuid IS NULL OR organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, organizationID, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list ai model groups: %w", err)
	}
	defer rows.Close()
	items := []ModelGroup{}
	for rows.Next() {
		var item ModelGroup
		if err := scanModelGroupRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan ai model group: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateModelChannelAbility(ctx context.Context, input CreateModelChannelAbilityInput) (*ModelChannelAbility, error) {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	ability := &ModelChannelAbility{}
	err := scanModelChannelAbilityRow(r.db.QueryRow(ctx, `
		INSERT INTO ai_model_channel_abilities(
			model_group_id, channel_id, requested_model, model_pattern,
			upstream_model, priority, enabled, metadata
		)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), '*'), $5, COALESCE(NULLIF($6, 0), 100), $7, $8)
		RETURNING id, model_group_id, channel_id, requested_model, model_pattern,
			upstream_model, priority, enabled, metadata, created_at, updated_at
	`, input.ModelGroupID, input.ChannelID, input.RequestedModel, input.ModelPattern, input.UpstreamModel,
		input.Priority, enabled, mustJSON(input.Metadata)), ability)
	if err != nil {
		return nil, fmt.Errorf("create ai model channel ability: %w", err)
	}
	return ability, nil
}

func (r *PostgresRepository) ListModelChannelAbilities(ctx context.Context, modelGroupID *uuid.UUID, limit int) ([]ModelChannelAbility, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, model_group_id, channel_id, requested_model, model_pattern,
			upstream_model, priority, enabled, metadata, created_at, updated_at
		FROM ai_model_channel_abilities
		WHERE $1::uuid IS NULL OR model_group_id = $1
		ORDER BY priority ASC, created_at DESC
		LIMIT $2
	`, modelGroupID, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list ai model channel abilities: %w", err)
	}
	defer rows.Close()
	items := []ModelChannelAbility{}
	for rows.Next() {
		var item ModelChannelAbility
		if err := scanModelChannelAbilityRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan ai model channel ability: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetGatewayBalance(ctx context.Context, organizationID uuid.UUID, currency string) (*GatewayBalance, error) {
	balance := &GatewayBalance{}
	err := scanGatewayBalanceRow(r.db.QueryRow(ctx, `
		SELECT id, organization_id, balance_amount::float8, reserved_amount::float8,
			currency, metadata, created_at, updated_at
		FROM ai_gateway_balances
		WHERE organization_id = $1 AND currency = $2
	`, organizationID, currencyOrDefault(currency)), balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &GatewayBalance{OrganizationID: organizationID, Currency: currencyOrDefault(currency), Metadata: map[string]any{}}, nil
		}
		return nil, fmt.Errorf("get ai gateway balance: %w", err)
	}
	return balance, nil
}

func (r *PostgresRepository) ListBalanceTransactions(ctx context.Context, organizationID uuid.UUID, limit int) ([]BalanceTransaction, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, access_token_id, model_group_id, invocation_id, reservation_id,
			transaction_type, amount::float8, currency, reason, metadata, created_at
		FROM ai_gateway_balance_transactions
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, organizationID, normalizeLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list ai gateway balance transactions: %w", err)
	}
	defer rows.Close()
	items := []BalanceTransaction{}
	for rows.Next() {
		var item BalanceTransaction
		if err := scanBalanceTransactionRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan ai gateway balance transaction: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) AdjustGatewayBalance(ctx context.Context, input AdjustGatewayBalanceInput) (*GatewayBalance, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin adjust ai gateway balance: %w", err)
	}
	defer tx.Rollback(ctx)

	currency := currencyOrDefault(input.Currency)
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_gateway_balances(organization_id, currency)
		VALUES ($1, $2)
		ON CONFLICT (organization_id, currency) DO NOTHING
	`, input.OrganizationID, currency); err != nil {
		return nil, fmt.Errorf("ensure ai gateway balance: %w", err)
	}
	balance := &GatewayBalance{}
	err = scanGatewayBalanceRow(tx.QueryRow(ctx, `
		UPDATE ai_gateway_balances
		SET balance_amount = balance_amount + $3,
			metadata = metadata || $4::jsonb,
			updated_at = NOW()
		WHERE organization_id = $1
		  AND currency = $2
		  AND balance_amount + $3 >= reserved_amount
		RETURNING id, organization_id, balance_amount::float8, reserved_amount::float8,
			currency, metadata, created_at, updated_at
	`, input.OrganizationID, currency, roundCost(input.Amount), mustJSON(input.Metadata)), balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: ai gateway balance adjustment would fall below reserved amount", ErrForbidden)
		}
		return nil, fmt.Errorf("adjust ai gateway balance: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_gateway_balance_transactions(
			organization_id, transaction_type, amount, currency, reason, metadata
		)
		VALUES ($1, 'adjustment', $2, $3, $4, $5)
	`, input.OrganizationID, roundCost(input.Amount), currency, input.Reason, mustJSON(input.Metadata)); err != nil {
		return nil, fmt.Errorf("record ai gateway balance adjustment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit adjust ai gateway balance: %w", err)
	}
	return balance, nil
}

func (r *PostgresRepository) UsageAnalysis(ctx context.Context, filter UsageAnalysisFilter) (*UsageAnalysis, error) {
	analysis := &UsageAnalysis{
		Currency:   "CNY",
		ByProvider: map[string]float64{},
		ByChannel:  map[string]float64{},
		ByModel:    map[string]float64{},
		ByActor:    map[string]float64{},
	}
	orgID := nullableUUID(currentTenantOrganizationID(ctx))
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_amount), 0)::float8, COUNT(*), COALESCE(MAX(currency), 'CNY')
		FROM ai_invocations
		WHERE $1::uuid IS NULL OR organization_id = $1
	`, orgID).Scan(&analysis.TotalCost, &analysis.InvocationCount, &analysis.Currency); err != nil {
		return nil, fmt.Errorf("query usage analysis totals: %w", err)
	}
	if err := r.fillUsageBreakdown(ctx, analysis.ByProvider, `
		SELECT p.provider_type, COALESCE(SUM(i.cost_amount), 0)::float8
		FROM ai_invocations i JOIN model_providers p ON p.id = i.provider_id
		WHERE $1::uuid IS NULL OR i.organization_id = $1
		GROUP BY p.provider_type
	`, orgID); err != nil {
		return nil, err
	}
	if err := r.fillUsageBreakdown(ctx, analysis.ByChannel, `
		SELECT COALESCE(c.name, 'provider_default'), COALESCE(SUM(i.cost_amount), 0)::float8
		FROM ai_invocations i LEFT JOIN model_provider_channels c ON c.id = i.channel_id
		WHERE $1::uuid IS NULL OR i.organization_id = $1
		GROUP BY COALESCE(c.name, 'provider_default')
	`, orgID); err != nil {
		return nil, err
	}
	if err := r.fillUsageBreakdown(ctx, analysis.ByModel, `
		SELECT COALESCE(NULLIF(i.requested_model, ''), m.model_key), COALESCE(SUM(i.cost_amount), 0)::float8
		FROM ai_invocations i JOIN models m ON m.id = i.model_id
		WHERE $1::uuid IS NULL OR i.organization_id = $1
		GROUP BY COALESCE(NULLIF(i.requested_model, ''), m.model_key)
	`, orgID); err != nil {
		return nil, err
	}
	if err := r.fillUsageBreakdown(ctx, analysis.ByActor, `
		SELECT COALESCE(i.agent_id::text, i.user_id::text, 'unattributed'), COALESCE(SUM(i.cost_amount), 0)::float8
		FROM ai_invocations i
		WHERE $1::uuid IS NULL OR i.organization_id = $1
		GROUP BY COALESCE(i.agent_id::text, i.user_id::text, 'unattributed')
	`, orgID); err != nil {
		return nil, err
	}
	recent, err := r.ListInvocations(ctx, filter.Limit)
	if err != nil {
		return nil, err
	}
	analysis.Recent = recent
	return analysis, nil
}

func (r *PostgresRepository) fillUsageBreakdown(ctx context.Context, target map[string]float64, query string, args ...any) error {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query usage breakdown: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var amount float64
		if err := rows.Scan(&key, &amount); err != nil {
			return fmt.Errorf("scan usage breakdown: %w", err)
		}
		target[key] = amount
	}
	return rows.Err()
}

func (r *PostgresRepository) applyRoutingRule(ctx context.Context, input *InvokeInput) error {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(rule.provider_id, channel.provider_id), rule.channel_id,
			COALESCE(rule.match_scope, 'global'), COALESCE(rule.match_value, ''),
			COALESCE(rule.model_pattern, '*')
		FROM ai_routing_rules rule
		LEFT JOIN model_provider_channels channel ON channel.id = rule.channel_id
		WHERE rule.status = 'active'
		ORDER BY rule.priority ASC, rule.created_at ASC
		LIMIT 100
	`)
	if err != nil {
		return fmt.Errorf("query ai routing rules: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var providerID, channelID pgtype.UUID
		var scope, value, modelPattern string
		if err := rows.Scan(&providerID, &channelID, &scope, &value, &modelPattern); err != nil {
			return fmt.Errorf("scan ai routing rule: %w", err)
		}
		if !routingRuleMatches(input.Attribution, scope, value) || !wildcardMatch(firstNonEmpty(modelPattern, "*"), input.Model) {
			continue
		}
		if providerID.Valid {
			id, err := uuid.FromBytes(providerID.Bytes[:])
			if err != nil {
				return fmt.Errorf("decode routing rule provider id: %w", err)
			}
			input.ProviderID = &id
		}
		if channelID.Valid {
			id, err := uuid.FromBytes(channelID.Bytes[:])
			if err != nil {
				return fmt.Errorf("decode routing rule channel id: %w", err)
			}
			input.PreferredChannelID = &id
		}
		return nil
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ai routing rules: %w", err)
	}
	return nil
}

func routingRuleMatches(attribution Attribution, scope string, value string) bool {
	scope = strings.TrimSpace(strings.ToLower(scope))
	value = strings.TrimSpace(value)
	if scope == "" || scope == "global" || value == "" {
		return true
	}
	return attributionValue(attribution, scope) == value
}

func attributionValue(attribution Attribution, scope string) string {
	switch scope {
	case "organization", "organization_id":
		return uuidString(attribution.OrganizationID)
	case "department", "department_id":
		return uuidString(attribution.DepartmentID)
	case "project", "project_id":
		return uuidString(attribution.ProjectID)
	case "requirement", "requirement_id":
		return uuidString(attribution.RequirementID)
	case "workflow", "workflow_id":
		return uuidString(attribution.WorkflowID)
	case "task", "task_id":
		return uuidString(attribution.TaskID)
	case "agent", "agent_id":
		return uuidString(attribution.AgentID)
	case "user", "user_id":
		return uuidString(attribution.UserID)
	case "capability", "capability_id":
		return uuidString(attribution.CapabilityID)
	case "source_surface":
		return strings.TrimSpace(attribution.SourceSurface)
	default:
		return ""
	}
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func currentTenantOrganizationID(ctx context.Context) *uuid.UUID {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant.OrganizationID == nil {
		return nil
	}
	id := *tenant.OrganizationID
	return &id
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func (r *PostgresRepository) applyChannel(ctx context.Context, input InvokeInput, target *ResolvedModel) error {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, COALESCE(NULLIF(c.base_url, ''), $2), c.encrypted_api_key,
			c.rate_multiplier::float8, c.supported_model_patterns, c.model_mapping,
			COALESCE(a.priority, c.priority) AS ability_priority,
			COALESCE(a.requested_model, ''), COALESCE(a.model_pattern, ''), COALESCE(a.upstream_model, '')
		FROM model_provider_channels c
		LEFT JOIN ai_model_channel_abilities a
		  ON $4::uuid IS NOT NULL
		 AND a.channel_id = c.id
		 AND a.enabled = TRUE
		 AND (a.model_group_id = $4 OR a.model_group_id IS NULL)
		WHERE c.provider_id = $1
		  AND c.status = 'active'
		  AND ($3::uuid IS NULL OR c.id = $3)
		  AND ($4::uuid IS NULL OR a.id IS NOT NULL)
		  AND (c.quota_amount <= 0 OR c.quota_used < c.quota_amount)
		  AND (c.concurrency_limit <= 0 OR c.inflight_requests < c.concurrency_limit)
		ORDER BY ability_priority ASC,
			(c.inflight_requests::float8 / GREATEST(c.load_factor, 1)) ASC,
			c.last_used_at ASC NULLS FIRST,
			c.created_at ASC
		LIMIT 50
	`, target.ProviderID, target.BaseURL, input.PreferredChannelID, input.ModelGroupID)
	if err != nil {
		return fmt.Errorf("query provider channels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var name, baseURL, encrypted string
		var rateMultiplier float64
		var patternsJSON, mappingJSON []byte
		var abilityPriority int
		var abilityRequestedModel, abilityPattern, abilityUpstreamModel string
		if err := rows.Scan(&id, &name, &baseURL, &encrypted, &rateMultiplier, &patternsJSON, &mappingJSON,
			&abilityPriority, &abilityRequestedModel, &abilityPattern, &abilityUpstreamModel); err != nil {
			return fmt.Errorf("scan provider channel candidate: %w", err)
		}
		if input.ModelGroupID != nil {
			if abilityRequestedModel != "" && !strings.EqualFold(abilityRequestedModel, target.RequestedModel) {
				continue
			}
			if !modelSupported([]string{firstNonEmpty(abilityPattern, "*")}, target.RequestedModel) {
				continue
			}
		}
		patterns, mapping, err := parseChannelRouting(patternsJSON, mappingJSON)
		if err != nil {
			return err
		}
		if !modelSupported(patterns, target.RequestedModel) {
			continue
		}
		upstreamModel, mapped := resolveMappedModel(mapping, target.RequestedModel)
		if strings.TrimSpace(abilityUpstreamModel) != "" {
			upstreamModel = strings.TrimSpace(abilityUpstreamModel)
			mapped = !strings.EqualFold(upstreamModel, target.RequestedModel)
		}
		apiKey, err := r.box.Decrypt(encrypted)
		if err != nil {
			return fmt.Errorf("decrypt channel api key: %w", err)
		}
		if _, err := r.db.Exec(ctx, `
			UPDATE model_provider_channels
			SET inflight_requests = inflight_requests + 1, last_used_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, id); err != nil {
			return fmt.Errorf("reserve provider channel: %w", err)
		}
		target.ChannelID = &id
		target.ChannelName = name
		target.BaseURL = baseURL
		target.APIKey = apiKey
		target.RateMultiplier = rateMultiplier
		target.UpstreamModel = upstreamModel
		target.Model = upstreamModel
		if mapped {
			target.ModelMappingChain = target.RequestedModel + " -> " + upstreamModel
		}
		return nil
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate provider channels: %w", err)
	}
	if input.PreferredChannelID != nil {
		return fmt.Errorf("%w: preferred channel is not available", ErrNotFound)
	}
	if input.ModelGroupID != nil {
		return fmt.Errorf("%w: no active channel ability matches model group", ErrUnavailable)
	}
	return nil
}

func parseChannelRouting(patternsJSON, mappingJSON []byte) ([]string, map[string]string, error) {
	patterns := []string{}
	if len(patternsJSON) > 0 {
		if err := json.Unmarshal(patternsJSON, &patterns); err != nil {
			return nil, nil, fmt.Errorf("unmarshal channel model patterns: %w", err)
		}
	}
	mapping := map[string]string{}
	if len(mappingJSON) > 0 {
		if err := json.Unmarshal(mappingJSON, &mapping); err != nil {
			return nil, nil, fmt.Errorf("unmarshal channel model mapping: %w", err)
		}
	}
	return patterns, mapping, nil
}

func modelSupported(patterns []string, model string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if wildcardMatch(pattern, model) {
			return true
		}
	}
	return false
}

func resolveMappedModel(mapping map[string]string, model string) (string, bool) {
	if mapped, ok := mapping[model]; ok && strings.TrimSpace(mapped) != "" {
		return mapped, true
	}
	bestPattern := ""
	bestMapped := ""
	for pattern, mapped := range mapping {
		if wildcardMatch(pattern, model) && len(pattern) > len(bestPattern) {
			bestPattern = pattern
			bestMapped = mapped
		}
	}
	if bestMapped != "" {
		return bestMapped, true
	}
	return model, false
}

func wildcardMatch(pattern, value string) bool {
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	value = strings.TrimSpace(strings.ToLower(value))
	if pattern == "" {
		return false
	}
	if pattern == "*" || pattern == value {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(value[pos:], part)
		if found < 0 {
			return false
		}
		if index == 0 && !strings.HasPrefix(pattern, "*") && found != 0 {
			return false
		}
		pos += found + len(part)
	}
	last := parts[len(parts)-1]
	return strings.HasSuffix(pattern, "*") || last == "" || strings.HasSuffix(value, last)
}

func nullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProviderRow(row scanner, provider *ModelProvider) error {
	var tagsJSON, metaJSON []byte
	var lastTestedAt pgtype.Timestamptz
	if err := row.Scan(&provider.ID, &provider.Name, &provider.ProviderType, &provider.BaseURL, &provider.MaskedAPIKey, &provider.Status,
		&provider.TimeoutMS, &provider.RetryCount, &provider.RiskLevel, &tagsJSON, &metaJSON, &provider.LastTestStatus,
		&provider.LastTestError, &lastTestedAt, &provider.CreatedAt, &provider.UpdatedAt); err != nil {
		return err
	}
	if lastTestedAt.Valid {
		t := lastTestedAt.Time
		provider.LastTestedAt = &t
	}
	return hydrateProviderJSON(provider, tagsJSON, metaJSON)
}

func scanModelRow(row scanner, model *Model) error {
	var capJSON, metaJSON []byte
	if err := row.Scan(&model.ID, &model.ProviderID, &model.ModelKey, &model.DisplayName, &model.ContextWindow,
		&model.MaxOutputTokens, &capJSON, &model.Status, &metaJSON, &model.CreatedAt, &model.UpdatedAt); err != nil {
		return err
	}
	return hydrateModelJSON(model, capJSON, metaJSON)
}

func scanInvocationRow(row scanner, inv *Invocation) error {
	var metaJSON, costBreakdownJSON []byte
	var completedAt pgtype.Timestamptz
	var channelID, organizationID, departmentID, projectID, requirementID, workflowID, taskID pgtype.UUID
	var agentID, userID, capabilityID pgtype.UUID
	if err := row.Scan(&inv.ID, &inv.ProviderID, &inv.ModelID, &channelID, &inv.Mode, &inv.Status,
		&organizationID, &departmentID, &projectID, &requirementID, &workflowID, &taskID,
		&agentID, &userID, &capabilityID, &inv.Attribution.SourceSurface,
		&inv.RequestedModel, &inv.UpstreamModel, &inv.ModelMappingChain, &inv.ServiceTier, &inv.ReasoningEffort,
		&inv.RequestHash, &inv.ProviderRequestID,
		&inv.InputTokens, &inv.OutputTokens, &inv.CacheCreationTokens, &inv.CacheReadTokens,
		&inv.CacheCreation5mTokens, &inv.CacheCreation1hTokens, &inv.ImageOutputTokens,
		&inv.EstimatedInputTokens, &inv.EstimatedOutputTokens,
		&inv.CostAmount, &costBreakdownJSON, &inv.Currency, &inv.FirstTokenMS, &inv.DurationMS, &inv.ErrorType, &inv.ErrorMessage,
		&metaJSON, &inv.CreatedAt, &completedAt); err != nil {
		return err
	}
	inv.ChannelID = uuidPointer(channelID)
	inv.Attribution.OrganizationID = uuidPointer(organizationID)
	inv.Attribution.DepartmentID = uuidPointer(departmentID)
	inv.Attribution.ProjectID = uuidPointer(projectID)
	inv.Attribution.RequirementID = uuidPointer(requirementID)
	inv.Attribution.WorkflowID = uuidPointer(workflowID)
	inv.Attribution.TaskID = uuidPointer(taskID)
	inv.Attribution.AgentID = uuidPointer(agentID)
	inv.Attribution.UserID = uuidPointer(userID)
	inv.Attribution.CapabilityID = uuidPointer(capabilityID)
	if completedAt.Valid {
		t := completedAt.Time
		inv.CompletedAt = &t
	}
	if err := json.Unmarshal(metaJSON, &inv.Metadata); err != nil {
		return fmt.Errorf("unmarshal invocation metadata: %w", err)
	}
	if len(costBreakdownJSON) > 0 {
		if err := json.Unmarshal(costBreakdownJSON, &inv.CostBreakdown); err != nil {
			return fmt.Errorf("unmarshal invocation cost breakdown: %w", err)
		}
	}
	return nil
}

func scanChannelRow(row scanner, channel *ProviderChannel) error {
	var patternsJSON, mappingJSON, metaJSON []byte
	var userID, agentID pgtype.UUID
	var lastTestedAt, lastUsedAt pgtype.Timestamptz
	if err := row.Scan(&channel.ID, &channel.ProviderID, &channel.Name, &channel.BaseURL, &channel.MaskedAPIKey, &channel.OwnerType,
		&userID, &agentID, &channel.Status, &channel.Priority, &channel.ConcurrencyLimit, &channel.InflightRequests,
		&channel.LoadFactor, &channel.RateMultiplier, &channel.QuotaAmount, &channel.QuotaUsed, &channel.QuotaCurrency,
		&patternsJSON, &mappingJSON, &channel.HealthStatus, &channel.LastError, &lastTestedAt, &lastUsedAt,
		&metaJSON, &channel.CreatedAt, &channel.UpdatedAt); err != nil {
		return err
	}
	channel.UserID = uuidPointer(userID)
	channel.AgentID = uuidPointer(agentID)
	if lastTestedAt.Valid {
		t := lastTestedAt.Time
		channel.LastTestedAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		channel.LastUsedAt = &t
	}
	if err := json.Unmarshal(patternsJSON, &channel.SupportedModelPatterns); err != nil {
		return fmt.Errorf("unmarshal channel patterns: %w", err)
	}
	if err := json.Unmarshal(mappingJSON, &channel.ModelMapping); err != nil {
		return fmt.Errorf("unmarshal channel model mapping: %w", err)
	}
	if err := json.Unmarshal(metaJSON, &channel.Metadata); err != nil {
		return fmt.Errorf("unmarshal channel metadata: %w", err)
	}
	return nil
}

func scanRoutingRuleRow(row scanner, rule *RoutingRule) error {
	var metaJSON []byte
	var providerID, channelID pgtype.UUID
	if err := row.Scan(&rule.ID, &rule.Name, &providerID, &channelID, &rule.MatchScope, &rule.MatchValue,
		&rule.ModelPattern, &rule.Priority, &rule.Status, &metaJSON, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return err
	}
	rule.ProviderID = uuidPointer(providerID)
	rule.ChannelID = uuidPointer(channelID)
	if err := json.Unmarshal(metaJSON, &rule.Metadata); err != nil {
		return fmt.Errorf("unmarshal routing rule metadata: %w", err)
	}
	return nil
}

func scanAccessTokenRow(row scanner, token *AccessToken) error {
	var groupID pgtype.UUID
	var allowedModelsJSON, allowedPatternsJSON, metaJSON []byte
	var expiresAt, lastUsedAt pgtype.Timestamptz
	if err := row.Scan(&token.ID, &token.OrganizationID, &groupID, &token.Name, &token.TokenHash, &token.MaskedToken, &token.Status,
		&allowedModelsJSON, &allowedPatternsJSON, &token.AllowChannelOverride, &token.QuotaAmount, &token.QuotaUsed, &token.QuotaCurrency,
		&expiresAt, &lastUsedAt, &metaJSON, &token.CreatedAt, &token.UpdatedAt); err != nil {
		return err
	}
	token.ModelGroupID = uuidPointer(groupID)
	if expiresAt.Valid {
		t := expiresAt.Time
		token.ExpiresAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		token.LastUsedAt = &t
	}
	if err := json.Unmarshal(allowedModelsJSON, &token.AllowedModels); err != nil {
		return fmt.Errorf("unmarshal access token allowed models: %w", err)
	}
	if err := json.Unmarshal(allowedPatternsJSON, &token.AllowedModelPatterns); err != nil {
		return fmt.Errorf("unmarshal access token allowed model patterns: %w", err)
	}
	if err := json.Unmarshal(metaJSON, &token.Metadata); err != nil {
		return fmt.Errorf("unmarshal access token metadata: %w", err)
	}
	return nil
}

func scanModelGroupRow(row scanner, group *ModelGroup) error {
	var orgID, departmentID, projectID, agentID pgtype.UUID
	var metaJSON []byte
	if err := row.Scan(&group.ID, &orgID, &departmentID, &projectID, &agentID, &group.Name, &group.GroupKey,
		&group.Status, &group.RateMultiplier, &metaJSON, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return err
	}
	group.OrganizationID = uuidPointer(orgID)
	group.DepartmentID = uuidPointer(departmentID)
	group.ProjectID = uuidPointer(projectID)
	group.AgentID = uuidPointer(agentID)
	if err := json.Unmarshal(metaJSON, &group.Metadata); err != nil {
		return fmt.Errorf("unmarshal model group metadata: %w", err)
	}
	return nil
}

func scanModelChannelAbilityRow(row scanner, ability *ModelChannelAbility) error {
	var modelGroupID pgtype.UUID
	var metaJSON []byte
	if err := row.Scan(&ability.ID, &modelGroupID, &ability.ChannelID, &ability.RequestedModel, &ability.ModelPattern,
		&ability.UpstreamModel, &ability.Priority, &ability.Enabled, &metaJSON, &ability.CreatedAt, &ability.UpdatedAt); err != nil {
		return err
	}
	ability.ModelGroupID = uuidPointer(modelGroupID)
	if err := json.Unmarshal(metaJSON, &ability.Metadata); err != nil {
		return fmt.Errorf("unmarshal model channel ability metadata: %w", err)
	}
	return nil
}

func scanGatewayBalanceRow(row scanner, balance *GatewayBalance) error {
	var metaJSON []byte
	if err := row.Scan(&balance.ID, &balance.OrganizationID, &balance.BalanceAmount, &balance.ReservedAmount,
		&balance.Currency, &metaJSON, &balance.CreatedAt, &balance.UpdatedAt); err != nil {
		return err
	}
	if err := json.Unmarshal(metaJSON, &balance.Metadata); err != nil {
		return fmt.Errorf("unmarshal gateway balance metadata: %w", err)
	}
	return nil
}

func scanBalanceTransactionRow(row scanner, tx *BalanceTransaction) error {
	var accessTokenID, modelGroupID, invocationID, reservationID pgtype.UUID
	var metaJSON []byte
	if err := row.Scan(&tx.ID, &tx.OrganizationID, &accessTokenID, &modelGroupID, &invocationID, &reservationID,
		&tx.TransactionType, &tx.Amount, &tx.Currency, &tx.Reason, &metaJSON, &tx.CreatedAt); err != nil {
		return err
	}
	tx.AccessTokenID = uuidPointer(accessTokenID)
	tx.ModelGroupID = uuidPointer(modelGroupID)
	tx.InvocationID = uuidPointer(invocationID)
	tx.ReservationID = uuidPointer(reservationID)
	if err := json.Unmarshal(metaJSON, &tx.Metadata); err != nil {
		return fmt.Errorf("unmarshal balance transaction metadata: %w", err)
	}
	return nil
}

func uuidPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id, err := uuid.FromBytes(value.Bytes[:])
	if err != nil {
		return nil
	}
	return &id
}

func scanProviders(rows pgx.Rows) ([]ModelProvider, error) {
	items := []ModelProvider{}
	for rows.Next() {
		var item ModelProvider
		if err := scanProviderRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan model provider: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanModels(rows pgx.Rows) ([]Model, error) {
	items := []Model{}
	for rows.Next() {
		var item Model
		if err := scanModelRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanInvocations(rows pgx.Rows) ([]Invocation, error) {
	items := []Invocation{}
	for rows.Next() {
		var item Invocation
		if err := scanInvocationRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan invocation: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanChannels(rows pgx.Rows) ([]ProviderChannel, error) {
	items := []ProviderChannel{}
	for rows.Next() {
		var item ProviderChannel
		if err := scanChannelRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan provider channel: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanRoutingRules(rows pgx.Rows) ([]RoutingRule, error) {
	items := []RoutingRule{}
	for rows.Next() {
		var item RoutingRule
		if err := scanRoutingRuleRow(rows, &item); err != nil {
			return nil, fmt.Errorf("scan routing rule: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func providerJSON(tags []string, metadata map[string]any) ([]byte, []byte, error) {
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal provider tags: %w", err)
	}
	metaJSON, err := json.Marshal(nonNilMap(metadata))
	if err != nil {
		return nil, nil, fmt.Errorf("marshal provider metadata: %w", err)
	}
	return tagsJSON, metaJSON, nil
}

func modelJSON(capabilities []string, metadata map[string]any) ([]byte, []byte, error) {
	if capabilities == nil {
		capabilities = []string{}
	}
	capJSON, err := json.Marshal(capabilities)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal model capabilities: %w", err)
	}
	metaJSON, err := json.Marshal(nonNilMap(metadata))
	if err != nil {
		return nil, nil, fmt.Errorf("marshal model metadata: %w", err)
	}
	return capJSON, metaJSON, nil
}

func channelJSON(patterns []string, mapping map[string]string, metadata map[string]any) ([]byte, []byte, []byte, error) {
	if patterns == nil {
		patterns = []string{}
	}
	if mapping == nil {
		mapping = map[string]string{}
	}
	patternsJSON, err := json.Marshal(patterns)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal channel patterns: %w", err)
	}
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal channel model mapping: %w", err)
	}
	metaJSON, err := json.Marshal(nonNilMap(metadata))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal channel metadata: %w", err)
	}
	return patternsJSON, mappingJSON, metaJSON, nil
}

func hydrateProviderJSON(provider *ModelProvider, tagsJSON, metaJSON []byte) error {
	if err := json.Unmarshal(tagsJSON, &provider.Tags); err != nil {
		return fmt.Errorf("unmarshal provider tags: %w", err)
	}
	if err := json.Unmarshal(metaJSON, &provider.Metadata); err != nil {
		return fmt.Errorf("unmarshal provider metadata: %w", err)
	}
	return nil
}

func hydrateModelJSON(model *Model, capJSON, metaJSON []byte) error {
	if err := json.Unmarshal(capJSON, &model.Capabilities); err != nil {
		return fmt.Errorf("unmarshal model capabilities: %w", err)
	}
	if err := json.Unmarshal(metaJSON, &model.Metadata); err != nil {
		return fmt.Errorf("unmarshal model metadata: %w", err)
	}
	return nil
}

func nonNilMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	return input
}

func nullableJSON(data []byte, include bool) any {
	if !include {
		return nil
	}
	return data
}

func mustJSON(input map[string]any) []byte {
	data, err := json.Marshal(nonNilMap(input))
	if err != nil {
		return []byte("{}")
	}
	return data
}

func mustJSONAny(input any) []byte {
	data, err := json.Marshal(input)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func createPriceForModel(input CreateModelInput) bool {
	return input.InputPricePer1K > 0 ||
		input.OutputPricePer1K > 0 ||
		input.CacheCreationPricePer1K > 0 ||
		input.CacheReadPricePer1K > 0 ||
		input.CacheCreation5mPricePer1K > 0 ||
		input.CacheCreation1hPricePer1K > 0 ||
		input.ImageOutputPricePer1K > 0 ||
		input.PriorityInputPricePer1K > 0 ||
		input.PriorityOutputPricePer1K > 0 ||
		input.PriorityCacheReadPricePer1K > 0 ||
		input.LongContextThreshold > 0 ||
		input.BillingMode != "" ||
		input.PricingSource != "" ||
		input.Currency != ""
}

func updatePriceForModel(input UpdateModelInput) bool {
	return input.InputPricePer1K != nil ||
		input.OutputPricePer1K != nil ||
		input.CacheCreationPricePer1K != nil ||
		input.CacheReadPricePer1K != nil ||
		input.CacheCreation5mPricePer1K != nil ||
		input.CacheCreation1hPricePer1K != nil ||
		input.ImageOutputPricePer1K != nil ||
		input.PriorityInputPricePer1K != nil ||
		input.PriorityOutputPricePer1K != nil ||
		input.PriorityCacheReadPricePer1K != nil ||
		input.LongContextThreshold != nil ||
		input.LongContextInputMultiplier != nil ||
		input.LongContextOutputMultiplier != nil ||
		input.BillingMode != nil ||
		input.PricingSource != nil ||
		input.Currency != nil
}

func floatPtrValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

func hashAccessToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
