package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database URL: %w", err)
	}
	applyPoolDefaults(config, databaseURL)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}
	return pool, nil
}

// applyPoolDefaults sizes the shared platform pool for its real load —
// tenant resolution on every request plus all AI-gateway billing writes.
// pgx's default (max(4, NumCPU) connections, no lifetimes) throttles global
// throughput. Every value can still be pinned per deployment through the
// standard pgx URL parameters (pool_max_conns, pool_min_conns,
// pool_max_conn_lifetime, pool_max_conn_idle_time).
func applyPoolDefaults(config *pgxpool.Config, databaseURL string) {
	if !strings.Contains(databaseURL, "pool_max_conns") {
		config.MaxConns = 20
	}
	if !strings.Contains(databaseURL, "pool_min_conns") {
		config.MinConns = 2
	}
	if !strings.Contains(databaseURL, "pool_max_conn_lifetime") {
		config.MaxConnLifetime = 30 * time.Minute
	}
	if !strings.Contains(databaseURL, "pool_max_conn_idle_time") {
		config.MaxConnIdleTime = 5 * time.Minute
	}
}
