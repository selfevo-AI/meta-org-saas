package tenantdb

import (
	"context"
	"fmt"

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
	cache    *tenantPoolCache
}

func NewPoolRouter(platform *pgxpool.Pool, adminURL string) *PoolRouter {
	return NewPoolRouterWithConfig(platform, adminURL, PoolRouterConfig{})
}

func NewPoolRouterWithConfig(platform *pgxpool.Pool, adminURL string, cfg PoolRouterConfig) *PoolRouter {
	return &PoolRouter{platform: platform, adminURL: adminURL, cache: newTenantPoolCache(cfg, defaultPoolFactory)}
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
	db, release, err := r.acquireDB(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	if release != nil {
		defer release()
	}
	return db.Exec(ctx, sql, args...)
}

func (r *PoolRouter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	db, release, err := r.acquireDB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		if release != nil {
			release()
		}
		return nil, err
	}
	if release == nil {
		return rows, nil
	}
	return &leasedRows{Rows: rows, release: release}, nil
}

func (r *PoolRouter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	db, release, err := r.acquireDB(ctx)
	if err != nil {
		return errorRow{err: err}
	}
	row := db.QueryRow(ctx, sql, args...)
	if release == nil {
		return row
	}
	return &leasedRow{Row: row, release: release}
}

func (r *PoolRouter) Begin(ctx context.Context) (pgx.Tx, error) {
	db, release, err := r.acquireDB(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		if release != nil {
			release()
		}
		return nil, err
	}
	if release == nil {
		return tx, nil
	}
	return &leasedTx{Tx: tx, release: release}, nil
}

func (r *PoolRouter) Close() {
	if r != nil && r.cache != nil {
		r.cache.Close()
	}
}

func (r *PoolRouter) acquireDB(ctx context.Context) (DB, func(), error) {
	if r == nil || r.platform == nil {
		return nil, nil, fmt.Errorf("tenant pool router platform database is not configured")
	}
	tenantURL, useTenant, err := TenantDatabaseURLFromContext(ctx, r.adminURL)
	if err != nil {
		if r.cache != nil {
			r.cache.recordRoutingError()
		}
		return nil, nil, err
	}
	if !useTenant {
		return r.platform, nil, nil
	}
	if r.cache == nil {
		return nil, nil, fmt.Errorf("tenant pool router cache is not configured")
	}
	return r.cache.Acquire(ctx, tenantURL)
}

func (r *PoolRouter) AcquireTarget(ctx context.Context, target Target) (DB, func(), error) {
	if r == nil || r.cache == nil {
		return nil, nil, fmt.Errorf("tenant pool router cache is not configured")
	}
	if target.DeploymentMode != DeploymentModeDedicatedDatabase {
		return nil, nil, fmt.Errorf("tenant database target %s is not dedicated", target.OrganizationID)
	}
	if target.Status != TargetStatusProvisioned {
		return nil, nil, fmt.Errorf("tenant database %q is not provisioned: %s", target.DatabaseName, target.Status)
	}
	if target.DatabaseName == "" {
		return nil, nil, fmt.Errorf("tenant database name is required for dedicated tenant database")
	}
	tenantURL, err := DatabaseURLForName(r.adminURL, target.DatabaseName)
	if err != nil {
		return nil, nil, err
	}
	return r.cache.Acquire(ctx, tenantURL)
}

func (r *PoolRouter) ReapIdle() int {
	if r == nil || r.cache == nil {
		return 0
	}
	return r.cache.ReapIdle()
}

func (r *PoolRouter) Stats() PoolRouterStats {
	if r == nil || r.cache == nil {
		return PoolRouterStats{Closed: true}
	}
	return r.cache.Stats()
}

type errorRow struct {
	err error
}

func (r errorRow) Scan(...any) error {
	return r.err
}
