package tenantdb

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Begin(context.Context) (pgx.Tx, error)
}

type PoolRouter struct {
	platform *pgxpool.Pool
	adminURL string
	pools    sync.Map
}

func NewPoolRouter(platform *pgxpool.Pool, adminURL string) *PoolRouter {
	return &PoolRouter{platform: platform, adminURL: adminURL}
}

func TenantDatabaseURLFromContext(ctx context.Context, adminURL string) (string, bool, error) {
	tenant, ok := middleware.TenantFromContext(ctx)
	if !ok || tenant == nil {
		return "", false, nil
	}
	if tenant.TenantDatabaseDeploymentMode != DeploymentModeDedicatedDatabase {
		return "", false, nil
	}
	if tenant.TenantDatabaseStatus != TargetStatusProvisioned {
		return "", true, fmt.Errorf("tenant database %q is not provisioned: %s", tenant.TenantDatabaseName, tenant.TenantDatabaseStatus)
	}
	if tenant.TenantDatabaseName == "" {
		return "", true, fmt.Errorf("tenant database name is required for dedicated tenant database")
	}
	url, err := DatabaseURLForName(adminURL, tenant.TenantDatabaseName)
	if err != nil {
		return "", true, err
	}
	return url, true, nil
}

func (r *PoolRouter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	db, err := r.dbForContext(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	return db.Exec(ctx, sql, args...)
}

func (r *PoolRouter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	db, err := r.dbForContext(ctx)
	if err != nil {
		return nil, err
	}
	return db.Query(ctx, sql, args...)
}

func (r *PoolRouter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	db, err := r.dbForContext(ctx)
	if err != nil {
		return errorRow{err: err}
	}
	return db.QueryRow(ctx, sql, args...)
}

func (r *PoolRouter) Begin(ctx context.Context) (pgx.Tx, error) {
	db, err := r.dbForContext(ctx)
	if err != nil {
		return nil, err
	}
	return db.Begin(ctx)
}

func (r *PoolRouter) Close() {
	r.pools.Range(func(_, value any) bool {
		if pool, ok := value.(*pgxpool.Pool); ok {
			pool.Close()
		}
		return true
	})
}

func (r *PoolRouter) dbForContext(ctx context.Context) (DB, error) {
	if r == nil || r.platform == nil {
		return nil, fmt.Errorf("tenant pool router platform database is not configured")
	}
	tenantURL, useTenant, err := TenantDatabaseURLFromContext(ctx, r.adminURL)
	if err != nil {
		return nil, err
	}
	if !useTenant {
		return r.platform, nil
	}
	if cached, ok := r.pools.Load(tenantURL); ok {
		return cached.(*pgxpool.Pool), nil
	}
	pool, err := pgxpool.New(ctx, tenantURL)
	if err != nil {
		return nil, fmt.Errorf("connect tenant database: %w", err)
	}
	actual, loaded := r.pools.LoadOrStore(tenantURL, pool)
	if loaded {
		pool.Close()
		return actual.(*pgxpool.Pool), nil
	}
	return pool, nil
}

type errorRow struct {
	err error
}

func (r errorRow) Scan(...any) error {
	return r.err
}
