package tenantdb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeManagedPool struct {
	closeCount int
}

func (p *fakeManagedPool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (p *fakeManagedPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (p *fakeManagedPool) QueryRow(context.Context, string, ...any) pgx.Row {
	return errorRow{}
}

func (p *fakeManagedPool) Begin(context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (p *fakeManagedPool) Close() {
	p.closeCount++
}

type fakePoolFactory struct {
	pools  map[string]*fakeManagedPool
	config PoolRouterConfig
	calls  int
}

func (f *fakePoolFactory) create(_ context.Context, url string, cfg PoolRouterConfig) (managedPool, error) {
	if f.pools == nil {
		f.pools = make(map[string]*fakeManagedPool)
	}
	pool := &fakeManagedPool{}
	f.pools[url] = pool
	f.config = cfg
	f.calls++
	return pool, nil
}

func testPoolCache(factory poolFactory, cfg PoolRouterConfig) (*tenantPoolCache, *time.Time) {
	if cfg.SweepInterval == 0 {
		cfg.SweepInterval = -1
	}
	cache := newTenantPoolCache(cfg, factory)
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	return cache, &now
}

func TestNormalizePoolRouterConfigUsesBoundedDefaults(t *testing.T) {
	cfg := normalizePoolRouterConfig(PoolRouterConfig{MinConnectionsPerPool: 99})

	if cfg.MaxCachedPools != defaultMaxCachedPools {
		t.Fatalf("MaxCachedPools = %d, want %d", cfg.MaxCachedPools, defaultMaxCachedPools)
	}
	if cfg.MaxConnectionsPerPool != defaultMaxConnectionsPerPool {
		t.Fatalf("MaxConnectionsPerPool = %d, want %d", cfg.MaxConnectionsPerPool, defaultMaxConnectionsPerPool)
	}
	if cfg.MinConnectionsPerPool != cfg.MaxConnectionsPerPool {
		t.Fatalf("MinConnectionsPerPool = %d, want clamp to %d", cfg.MinConnectionsPerPool, cfg.MaxConnectionsPerPool)
	}
	if cfg.IdlePoolTimeout != defaultIdlePoolTimeout {
		t.Fatalf("IdlePoolTimeout = %s, want %s", cfg.IdlePoolTimeout, defaultIdlePoolTimeout)
	}
}

func TestTenantPoolCacheReusesAndEvictsLeastRecentlyUsedPool(t *testing.T) {
	factory := &fakePoolFactory{}
	cache, now := testPoolCache(factory.create, PoolRouterConfig{
		MaxCachedPools:  2,
		IdlePoolTimeout: time.Hour,
	})
	defer cache.Close()

	_, releaseA, err := cache.Acquire(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("Acquire(tenant-a) error = %v", err)
	}
	releaseA()
	*now = now.Add(time.Minute)

	_, releaseB, err := cache.Acquire(context.Background(), "tenant-b")
	if err != nil {
		t.Fatalf("Acquire(tenant-b) error = %v", err)
	}
	releaseB()
	*now = now.Add(time.Minute)

	_, releaseA2, err := cache.Acquire(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("second Acquire(tenant-a) error = %v", err)
	}
	releaseA2()
	*now = now.Add(time.Minute)

	_, releaseC, err := cache.Acquire(context.Background(), "tenant-c")
	if err != nil {
		t.Fatalf("Acquire(tenant-c) error = %v", err)
	}
	releaseC()

	if factory.pools["tenant-b"].closeCount != 1 {
		t.Fatalf("tenant-b closeCount = %d, want 1", factory.pools["tenant-b"].closeCount)
	}
	if factory.pools["tenant-a"].closeCount != 0 {
		t.Fatalf("tenant-a closeCount = %d, want 0", factory.pools["tenant-a"].closeCount)
	}
	stats := cache.Stats()
	if stats.CachedPools != 2 || stats.CapacityEvictionsTotal != 1 {
		t.Fatalf("stats = %#v, want two cached pools and one capacity eviction", stats)
	}
	if stats.CacheHitsTotal != 1 || stats.CacheMissesTotal != 3 || stats.PoolsCreatedTotal != 3 {
		t.Fatalf("cache counters = %#v", stats)
	}
}

func TestTenantPoolCacheRejectsCapacityWhenEveryPoolIsActive(t *testing.T) {
	factory := &fakePoolFactory{}
	cache, _ := testPoolCache(factory.create, PoolRouterConfig{MaxCachedPools: 1})
	defer cache.Close()

	_, releaseA, err := cache.Acquire(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("Acquire(tenant-a) error = %v", err)
	}
	_, _, err = cache.Acquire(context.Background(), "tenant-b")
	if !errors.Is(err, ErrPoolCapacity) {
		t.Fatalf("Acquire(tenant-b) error = %v, want ErrPoolCapacity", err)
	}
	stats := cache.Stats()
	if stats.ActivePools != 1 || stats.ActiveLeases != 1 || stats.CapacityRejectionsTotal != 1 {
		t.Fatalf("active capacity stats = %#v", stats)
	}

	releaseA()
	_, releaseB, err := cache.Acquire(context.Background(), "tenant-b")
	if err != nil {
		t.Fatalf("Acquire(tenant-b) after release error = %v", err)
	}
	releaseB()
	if factory.pools["tenant-a"].closeCount != 1 {
		t.Fatalf("tenant-a closeCount = %d, want 1", factory.pools["tenant-a"].closeCount)
	}
}

func TestTenantPoolCacheReapIdleProtectsActiveLease(t *testing.T) {
	factory := &fakePoolFactory{}
	cache, now := testPoolCache(factory.create, PoolRouterConfig{
		MaxCachedPools:  2,
		IdlePoolTimeout: time.Minute,
	})
	defer cache.Close()

	_, release, err := cache.Acquire(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	*now = now.Add(2 * time.Minute)
	if evicted := cache.ReapIdle(); evicted != 0 {
		t.Fatalf("ReapIdle() = %d while lease active, want 0", evicted)
	}

	release()
	*now = now.Add(2 * time.Minute)
	if evicted := cache.ReapIdle(); evicted != 1 {
		t.Fatalf("ReapIdle() = %d, want 1", evicted)
	}
	if factory.pools["tenant-a"].closeCount != 1 {
		t.Fatalf("tenant-a closeCount = %d, want 1", factory.pools["tenant-a"].closeCount)
	}
	stats := cache.Stats()
	if stats.CachedPools != 0 || stats.IdleEvictionsTotal != 1 {
		t.Fatalf("idle eviction stats = %#v", stats)
	}
}

func TestTenantPoolCachePassesConnectionBudgetsToFactory(t *testing.T) {
	factory := &fakePoolFactory{}
	want := PoolRouterConfig{
		MaxCachedPools:        5,
		MaxConnectionsPerPool: 7,
		MinConnectionsPerPool: 2,
		IdlePoolTimeout:       3 * time.Minute,
		ConnectionIdleTimeout: 4 * time.Minute,
		ConnectionLifetime:    45 * time.Minute,
	}
	cache, _ := testPoolCache(factory.create, want)
	defer cache.Close()

	_, release, err := cache.Acquire(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	release()
	if factory.config.MaxConnectionsPerPool != want.MaxConnectionsPerPool ||
		factory.config.MinConnectionsPerPool != want.MinConnectionsPerPool ||
		factory.config.ConnectionIdleTimeout != want.ConnectionIdleTimeout ||
		factory.config.ConnectionLifetime != want.ConnectionLifetime {
		t.Fatalf("factory config = %#v, want budgets from %#v", factory.config, want)
	}
}

func TestTenantPoolCacheCloseIsIdempotentAndRejectsNewAcquisitions(t *testing.T) {
	factory := &fakePoolFactory{}
	cache, _ := testPoolCache(factory.create, PoolRouterConfig{MaxCachedPools: 2})

	_, release, err := cache.Acquire(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	release()
	cache.Close()
	cache.Close()

	if factory.pools["tenant-a"].closeCount != 1 {
		t.Fatalf("tenant-a closeCount = %d, want 1", factory.pools["tenant-a"].closeCount)
	}
	if _, _, err := cache.Acquire(context.Background(), "tenant-b"); !errors.Is(err, ErrPoolRouterClosed) {
		t.Fatalf("Acquire() after Close error = %v, want ErrPoolRouterClosed", err)
	}
	stats := cache.Stats()
	if !stats.Closed || stats.CachedPools != 0 {
		t.Fatalf("closed stats = %#v", stats)
	}
}
