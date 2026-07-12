package authlimit

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresLimiterStateMachine(t *testing.T) {
	if os.Getenv("RUN_FRESH_DB_MIGRATION_TEST") != "1" {
		t.Skip("set RUN_FRESH_DB_MIGRATION_TEST=1 to run auth rate limit PostgreSQL verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adminURL := strings.TrimSpace(os.Getenv("MIGRATION_TEST_ADMIN_URL"))
	if adminURL == "" {
		adminURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}
	dbName := fmt.Sprintf("meta_org_auth_limit_%d", time.Now().UnixNano())
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin database: %v", err)
	}
	var target *pgxpool.Pool
	t.Cleanup(func() {
		if target != nil {
			target.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupCtx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, dbName)
		_, _ = admin.Exec(cleanupCtx, `DROP DATABASE IF EXISTS "`+dbName+`"`)
		admin.Close()
	})
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+dbName+`"`); err != nil {
		t.Fatalf("create rate limit database: %v", err)
	}
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse admin URL: %v", err)
	}
	parsed.Path = "/" + dbName
	target, err = pgxpool.New(ctx, parsed.String())
	if err != nil {
		t.Fatalf("connect rate limit database: %v", err)
	}
	if _, err := target.Exec(ctx, `
		CREATE SCHEMA platform;
		CREATE TABLE platform.authentication_rate_limit_buckets (
		    bucket_key TEXT PRIMARY KEY,
		    scope TEXT NOT NULL,
		    window_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		    attempt_count INTEGER NOT NULL DEFAULT 0,
		    failure_count INTEGER NOT NULL DEFAULT 0,
		    blocked_until TIMESTAMPTZ,
		    last_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		    metadata JSONB NOT NULL DEFAULT '{}'
		)
	`); err != nil {
		t.Fatalf("create rate limit table: %v", err)
	}

	limiter := NewPostgresLimiter(target)
	policy := Policy{Window: time.Minute, MaxAttempts: 10, FailureThreshold: 2, BlockDuration: time.Minute}
	if decision, err := limiter.Consume(ctx, "user_login_subject", "user@example.com", policy); err != nil || !decision.Allowed {
		t.Fatalf("initial consume = %#v, %v", decision, err)
	}
	if decision, err := limiter.RecordFailure(ctx, "user_login_subject", "user@example.com", policy); err != nil || !decision.Allowed || decision.Failures != 1 {
		t.Fatalf("first failure = %#v, %v", decision, err)
	}
	if _, err := limiter.Consume(ctx, "user_login_subject", "user@example.com", policy); err != nil {
		t.Fatalf("second consume: %v", err)
	}
	blocked, err := limiter.RecordFailure(ctx, "user_login_subject", "user@example.com", policy)
	if err != nil || blocked.Allowed || blocked.Failures != 2 || blocked.RetryAfter <= 0 {
		t.Fatalf("blocking failure = %#v, %v", blocked, err)
	}
	if decision, err := limiter.Consume(ctx, "user_login_subject", "user@example.com", policy); err != nil || decision.Allowed {
		t.Fatalf("blocked consume = %#v, %v", decision, err)
	}
	if err := limiter.Reset(ctx, "user_login_subject", "user@example.com"); err != nil {
		t.Fatalf("reset limiter: %v", err)
	}
	if decision, err := limiter.Consume(ctx, "user_login_subject", "user@example.com", policy); err != nil || !decision.Allowed || decision.Attempts != 1 {
		t.Fatalf("consume after reset = %#v, %v", decision, err)
	}

	attemptPolicy := Policy{Window: time.Minute, MaxAttempts: 2, FailureThreshold: 2, BlockDuration: time.Minute}
	for attempt := 1; attempt <= 3; attempt++ {
		decision, err := limiter.Consume(ctx, "user_login_ip", "127.0.0.1", attemptPolicy)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		if (attempt <= 2) != decision.Allowed {
			t.Fatalf("attempt %d allowed = %t", attempt, decision.Allowed)
		}
	}
	stats := limiter.Stats()
	if stats.RateLimitedTotal < 2 || stats.BlocksApplied < 2 || stats.StoreErrorsTotal != 0 {
		t.Fatalf("limiter stats = %#v", stats)
	}
}
