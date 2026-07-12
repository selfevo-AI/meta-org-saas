package authlimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Policy struct {
	Window           time.Duration
	MaxAttempts      int
	FailureThreshold int
	BlockDuration    time.Duration
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Attempts   int
	Failures   int
}

type Stats struct {
	ChecksTotal      uint64 `json:"checks_total"`
	AllowedTotal     uint64 `json:"allowed_total"`
	RateLimitedTotal uint64 `json:"rate_limited_total"`
	FailuresRecorded uint64 `json:"failures_recorded_total"`
	BlocksApplied    uint64 `json:"blocks_applied_total"`
	SuccessfulResets uint64 `json:"successful_resets_total"`
	StoreErrorsTotal uint64 `json:"store_errors_total"`
}

type Limiter interface {
	Consume(context.Context, string, string, Policy) (Decision, error)
	RecordFailure(context.Context, string, string, Policy) (Decision, error)
	Reset(context.Context, string, string) error
	Stats() Stats
}

type PostgresLimiter struct {
	db *pgxpool.Pool

	checksTotal      atomic.Uint64
	allowedTotal     atomic.Uint64
	rateLimitedTotal atomic.Uint64
	failuresRecorded atomic.Uint64
	blocksApplied    atomic.Uint64
	successfulResets atomic.Uint64
	storeErrorsTotal atomic.Uint64
}

func NewPostgresLimiter(db *pgxpool.Pool) *PostgresLimiter {
	return &PostgresLimiter{db: db}
}

func (l *PostgresLimiter) Consume(ctx context.Context, scope, rawKey string, policy Policy) (Decision, error) {
	policy = normalizePolicy(policy)
	checkNumber := l.checksTotal.Add(1)
	key := HashKey(scope, rawKey)
	var decision Decision
	var blockedUntil *time.Time
	var windowStarted time.Time
	err := l.db.QueryRow(ctx, `
		INSERT INTO platform.authentication_rate_limit_buckets(
		    bucket_key, scope, attempt_count, failure_count, metadata
		)
		VALUES ($1, $2, 1, 0, jsonb_build_object('key_format', 'sha256-v1'))
		ON CONFLICT (bucket_key) DO UPDATE SET
		    scope = EXCLUDED.scope,
		    window_started_at = CASE
		        WHEN platform.authentication_rate_limit_buckets.window_started_at <= NOW() - make_interval(secs => $3)
		        THEN NOW() ELSE platform.authentication_rate_limit_buckets.window_started_at END,
		    attempt_count = CASE
		        WHEN platform.authentication_rate_limit_buckets.window_started_at <= NOW() - make_interval(secs => $3)
		        THEN 1 ELSE platform.authentication_rate_limit_buckets.attempt_count + 1 END,
		    failure_count = CASE
		        WHEN platform.authentication_rate_limit_buckets.window_started_at <= NOW() - make_interval(secs => $3)
		        THEN 0 ELSE platform.authentication_rate_limit_buckets.failure_count END,
		    blocked_until = CASE
		        WHEN platform.authentication_rate_limit_buckets.blocked_until > NOW()
		        THEN platform.authentication_rate_limit_buckets.blocked_until
		        WHEN (
		            CASE WHEN platform.authentication_rate_limit_buckets.window_started_at <= NOW() - make_interval(secs => $3)
		            THEN 1 ELSE platform.authentication_rate_limit_buckets.attempt_count + 1 END
		        ) > $4 THEN NOW() + make_interval(secs => $5)
		        ELSE NULL END,
		    last_attempt_at = NOW(),
		    updated_at = NOW(),
		    metadata = platform.authentication_rate_limit_buckets.metadata || EXCLUDED.metadata
		RETURNING attempt_count, failure_count, blocked_until, window_started_at
	`, key, scope, durationSeconds(policy.Window), policy.MaxAttempts, durationSeconds(policy.BlockDuration)).Scan(
		&decision.Attempts, &decision.Failures, &blockedUntil, &windowStarted,
	)
	if err != nil {
		l.storeErrorsTotal.Add(1)
		return Decision{}, fmt.Errorf("consume authentication rate limit: %w", err)
	}
	decision.Allowed = blockedUntil == nil
	if blockedUntil != nil {
		decision.RetryAfter = time.Until(*blockedUntil)
		if decision.RetryAfter < 0 {
			decision.RetryAfter = 0
		}
		l.rateLimitedTotal.Add(1)
		if decision.Attempts == policy.MaxAttempts+1 {
			l.blocksApplied.Add(1)
		}
	} else {
		l.allowedTotal.Add(1)
	}
	_ = windowStarted
	if checkNumber%1000 == 0 {
		if err := l.cleanupExpired(ctx, policy); err != nil {
			l.storeErrorsTotal.Add(1)
		}
	}
	return decision, nil
}

func (l *PostgresLimiter) RecordFailure(ctx context.Context, scope, rawKey string, policy Policy) (Decision, error) {
	policy = normalizePolicy(policy)
	key := HashKey(scope, rawKey)
	var decision Decision
	var blockedUntil *time.Time
	err := l.db.QueryRow(ctx, `
		UPDATE platform.authentication_rate_limit_buckets
		SET failure_count = failure_count + 1,
		    blocked_until = CASE
		        WHEN failure_count + 1 >= $2
		        THEN GREATEST(COALESCE(blocked_until, NOW()), NOW() + make_interval(secs => $3))
		        ELSE blocked_until END,
		    updated_at = NOW()
		WHERE bucket_key = $1
		RETURNING attempt_count, failure_count, blocked_until
	`, key, policy.FailureThreshold, durationSeconds(policy.BlockDuration)).Scan(
		&decision.Attempts, &decision.Failures, &blockedUntil,
	)
	if err != nil {
		l.storeErrorsTotal.Add(1)
		return Decision{}, fmt.Errorf("record authentication failure: %w", err)
	}
	l.failuresRecorded.Add(1)
	decision.Allowed = blockedUntil == nil
	if blockedUntil != nil {
		decision.RetryAfter = time.Until(*blockedUntil)
		if decision.RetryAfter < 0 {
			decision.RetryAfter = 0
		}
		if decision.Failures == policy.FailureThreshold {
			l.blocksApplied.Add(1)
		}
	}
	return decision, nil
}

func (l *PostgresLimiter) Reset(ctx context.Context, scope, rawKey string) error {
	key := HashKey(scope, rawKey)
	if _, err := l.db.Exec(ctx, `
		DELETE FROM platform.authentication_rate_limit_buckets
		WHERE bucket_key = $1 AND scope = $2
	`, key, scope); err != nil {
		l.storeErrorsTotal.Add(1)
		return fmt.Errorf("reset authentication rate limit: %w", err)
	}
	l.successfulResets.Add(1)
	return nil
}

func (l *PostgresLimiter) cleanupExpired(ctx context.Context, policy Policy) error {
	retention := policy.Window + policy.BlockDuration
	if retention < time.Hour {
		retention = time.Hour
	}
	_, err := l.db.Exec(ctx, `
		DELETE FROM platform.authentication_rate_limit_buckets
		WHERE updated_at < NOW() - make_interval(secs => $1)
		  AND (blocked_until IS NULL OR blocked_until < NOW())
	`, durationSeconds(retention))
	if err != nil {
		return fmt.Errorf("cleanup authentication rate limits: %w", err)
	}
	return nil
}

func (l *PostgresLimiter) Stats() Stats {
	if l == nil {
		return Stats{}
	}
	return Stats{
		ChecksTotal:      l.checksTotal.Load(),
		AllowedTotal:     l.allowedTotal.Load(),
		RateLimitedTotal: l.rateLimitedTotal.Load(),
		FailuresRecorded: l.failuresRecorded.Load(),
		BlocksApplied:    l.blocksApplied.Load(),
		SuccessfulResets: l.successfulResets.Load(),
		StoreErrorsTotal: l.storeErrorsTotal.Load(),
	}
}

func HashKey(scope, rawKey string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + rawKey))
	return hex.EncodeToString(sum[:])
}

func normalizePolicy(policy Policy) Policy {
	if policy.Window <= 0 {
		policy.Window = time.Minute
	}
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 10
	}
	if policy.FailureThreshold <= 0 || policy.FailureThreshold > policy.MaxAttempts {
		policy.FailureThreshold = policy.MaxAttempts
	}
	if policy.BlockDuration <= 0 {
		policy.BlockDuration = 5 * time.Minute
	}
	return policy
}

func durationSeconds(value time.Duration) int64 {
	seconds := int64(value / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
