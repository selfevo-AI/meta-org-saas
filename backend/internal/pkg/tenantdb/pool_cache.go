package tenantdb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxCachedPools        = 16
	defaultMaxConnectionsPerPool = 4
	defaultIdlePoolTimeout       = 15 * time.Minute
	defaultPoolSweepInterval     = time.Minute
	defaultConnectionIdleTimeout = 5 * time.Minute
	defaultConnectionLifetime    = 30 * time.Minute
)

var (
	ErrPoolRouterClosed = errors.New("tenant pool router is closed")
	ErrPoolCapacity     = errors.New("tenant pool cache capacity reached with no idle pool available")
)

type PoolRouterConfig struct {
	MaxCachedPools        int
	MaxConnectionsPerPool int32
	MinConnectionsPerPool int32
	IdlePoolTimeout       time.Duration
	SweepInterval         time.Duration
	ConnectionIdleTimeout time.Duration
	ConnectionLifetime    time.Duration
}

type PoolRouterStats struct {
	CachedPools              int    `json:"cached_pools"`
	ActivePools              int    `json:"active_pools"`
	ActiveLeases             int64  `json:"active_leases"`
	TotalConnections         int64  `json:"total_connections"`
	AcquiredConnections      int64  `json:"acquired_connections"`
	IdleConnections          int64  `json:"idle_connections"`
	MaxConfiguredConnections int64  `json:"max_configured_connections"`
	CacheHitsTotal           uint64 `json:"cache_hits_total"`
	CacheMissesTotal         uint64 `json:"cache_misses_total"`
	PoolsCreatedTotal        uint64 `json:"pools_created_total"`
	PoolsEvictedTotal        uint64 `json:"pools_evicted_total"`
	IdleEvictionsTotal       uint64 `json:"idle_evictions_total"`
	CapacityEvictionsTotal   uint64 `json:"capacity_evictions_total"`
	CapacityRejectionsTotal  uint64 `json:"capacity_rejections_total"`
	PoolCreationErrorsTotal  uint64 `json:"pool_creation_errors_total"`
	RoutingErrorsTotal       uint64 `json:"routing_errors_total"`
	Closed                   bool   `json:"closed"`
}

type managedPool interface {
	DB
	Close()
}

type poolFactory func(context.Context, string, PoolRouterConfig) (managedPool, error)

type poolEntry struct {
	pool     managedPool
	lastUsed time.Time
	leases   int64
	// ready is closed once creation finished; pool/err are only valid after.
	ready chan struct{}
	err   error
}

type tenantPoolCache struct {
	config  PoolRouterConfig
	factory poolFactory
	now     func() time.Time

	mu     sync.Mutex
	pools  map[string]*poolEntry
	closed bool
	stats  PoolRouterStats

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func newTenantPoolCache(cfg PoolRouterConfig, factory poolFactory) *tenantPoolCache {
	cfg = normalizePoolRouterConfig(cfg)
	cache := &tenantPoolCache{
		config:  cfg,
		factory: factory,
		now:     time.Now,
		pools:   make(map[string]*poolEntry),
		stop:    make(chan struct{}),
	}
	if cfg.SweepInterval > 0 {
		cache.wg.Add(1)
		go cache.runIdleSweeper()
	}
	return cache
}

func normalizePoolRouterConfig(cfg PoolRouterConfig) PoolRouterConfig {
	if cfg.MaxCachedPools <= 0 {
		cfg.MaxCachedPools = defaultMaxCachedPools
	}
	if cfg.MaxConnectionsPerPool <= 0 {
		cfg.MaxConnectionsPerPool = defaultMaxConnectionsPerPool
	}
	if cfg.MinConnectionsPerPool < 0 {
		cfg.MinConnectionsPerPool = 0
	}
	if cfg.MinConnectionsPerPool > cfg.MaxConnectionsPerPool {
		cfg.MinConnectionsPerPool = cfg.MaxConnectionsPerPool
	}
	if cfg.IdlePoolTimeout <= 0 {
		cfg.IdlePoolTimeout = defaultIdlePoolTimeout
	}
	if cfg.SweepInterval == 0 {
		cfg.SweepInterval = defaultPoolSweepInterval
	}
	if cfg.ConnectionIdleTimeout <= 0 {
		cfg.ConnectionIdleTimeout = defaultConnectionIdleTimeout
	}
	if cfg.ConnectionLifetime <= 0 {
		cfg.ConnectionLifetime = defaultConnectionLifetime
	}
	return cfg
}

func defaultPoolFactory(ctx context.Context, databaseURL string, cfg PoolRouterConfig) (managedPool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	poolConfig.MaxConns = cfg.MaxConnectionsPerPool
	poolConfig.MinConns = cfg.MinConnectionsPerPool
	poolConfig.MaxConnIdleTime = cfg.ConnectionIdleTimeout
	poolConfig.MaxConnLifetime = cfg.ConnectionLifetime
	return pgxpool.NewWithConfig(ctx, poolConfig)
}

func (c *tenantPoolCache) Acquire(ctx context.Context, tenantURL string) (DB, func(), error) {
	if c == nil {
		return nil, nil, fmt.Errorf("tenant pool cache is not configured")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, nil, ErrPoolRouterClosed
	}
	if entry, ok := c.pools[tenantURL]; ok {
		entry.leases++
		entry.lastUsed = c.now()
		c.stats.CacheHitsTotal++
		release := c.releaseFunc(entry)
		c.mu.Unlock()
		// The lease taken above keeps the entry safe from eviction while we
		// wait for a concurrent creator to finish.
		if entry.ready != nil {
			select {
			case <-entry.ready:
			case <-ctx.Done():
				release()
				return nil, nil, ctx.Err()
			}
		}
		if entry.err != nil {
			release()
			return nil, nil, entry.err
		}
		return entry.pool, release, nil
	}

	c.stats.CacheMissesTotal++
	if len(c.pools) >= c.config.MaxCachedPools {
		key, entry := c.capacityEvictionCandidateLocked()
		if entry == nil {
			c.stats.CapacityRejectionsTotal++
			c.mu.Unlock()
			return nil, nil, ErrPoolCapacity
		}
		delete(c.pools, key)
		c.stats.PoolsEvictedTotal++
		c.stats.CapacityEvictionsTotal++
		if entry.pool != nil {
			entry.pool.Close()
		}
	}

	entry := &poolEntry{lastUsed: c.now(), leases: 1, ready: make(chan struct{})}
	c.pools[tenantURL] = entry
	release := c.releaseFunc(entry)
	c.mu.Unlock()

	// Establish the connection outside the cache mutex: a slow or unreachable
	// tenant database must not block every other tenant's Acquire.
	pool, err := c.factory(ctx, tenantURL, c.config)

	c.mu.Lock()
	if err == nil && c.closed {
		pool.Close()
		pool = nil
		err = ErrPoolRouterClosed
	}
	if err != nil {
		entry.err = fmt.Errorf("connect tenant database: %w", err)
		c.stats.PoolCreationErrorsTotal++
		if c.pools[tenantURL] == entry {
			delete(c.pools, tenantURL)
		}
		close(entry.ready)
		c.mu.Unlock()
		release()
		return nil, nil, entry.err
	}
	entry.pool = pool
	c.stats.PoolsCreatedTotal++
	close(entry.ready)
	c.mu.Unlock()
	return pool, release, nil
}

func (c *tenantPoolCache) capacityEvictionCandidateLocked() (string, *poolEntry) {
	var selectedKey string
	var selected *poolEntry
	for key, entry := range c.pools {
		if entry.leases != 0 {
			continue
		}
		if selected == nil || entry.lastUsed.Before(selected.lastUsed) {
			selectedKey = key
			selected = entry
		}
	}
	return selectedKey, selected
}

func (c *tenantPoolCache) releaseFunc(entry *poolEntry) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			if entry.leases > 0 {
				entry.leases--
			}
			entry.lastUsed = c.now()
			c.mu.Unlock()
		})
	}
}

func (c *tenantPoolCache) ReapIdle() int {
	if c == nil {
		return 0
	}
	now := c.now()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0
	}
	evicted := make([]managedPool, 0)
	for key, entry := range c.pools {
		if entry.leases == 0 && now.Sub(entry.lastUsed) >= c.config.IdlePoolTimeout {
			delete(c.pools, key)
			evicted = append(evicted, entry.pool)
		}
	}
	c.stats.PoolsEvictedTotal += uint64(len(evicted))
	c.stats.IdleEvictionsTotal += uint64(len(evicted))
	c.mu.Unlock()

	for _, pool := range evicted {
		pool.Close()
	}
	return len(evicted)
}

func (c *tenantPoolCache) Stats() PoolRouterStats {
	if c == nil {
		return PoolRouterStats{Closed: true}
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := c.stats
	stats.CachedPools = len(c.pools)
	for _, entry := range c.pools {
		stats.ActiveLeases += entry.leases
		if entry.leases > 0 {
			stats.ActivePools++
		}
		if statser, ok := entry.pool.(interface{ Stat() *pgxpool.Stat }); ok {
			poolStats := statser.Stat()
			stats.TotalConnections += int64(poolStats.TotalConns())
			stats.AcquiredConnections += int64(poolStats.AcquiredConns())
			stats.IdleConnections += int64(poolStats.IdleConns())
			stats.MaxConfiguredConnections += int64(poolStats.MaxConns())
		}
	}
	return stats
}

func (c *tenantPoolCache) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.stats.Closed = true
	c.stopOnce.Do(func() { close(c.stop) })
	pools := make([]managedPool, 0, len(c.pools))
	for key, entry := range c.pools {
		delete(c.pools, key)
		if entry.pool != nil {
			pools = append(pools, entry.pool)
		}
	}
	c.mu.Unlock()

	c.wg.Wait()
	for _, pool := range pools {
		pool.Close()
	}
}

func (c *tenantPoolCache) recordRoutingError() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.stats.RoutingErrorsTotal++
	c.mu.Unlock()
}

func (c *tenantPoolCache) runIdleSweeper() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.ReapIdle()
		case <-c.stop:
			return
		}
	}
}

type leasedRows struct {
	pgx.Rows
	release func()
	once    sync.Once
}

func (r *leasedRows) Close() {
	r.Rows.Close()
	r.once.Do(r.release)
}

func (r *leasedRows) Next() bool {
	next := r.Rows.Next()
	if !next {
		r.once.Do(r.release)
	}
	return next
}

type leasedRow struct {
	pgx.Row
	release func()
	once    sync.Once
}

func (r *leasedRow) Scan(dest ...any) error {
	defer r.once.Do(r.release)
	return r.Row.Scan(dest...)
}

type leasedTx struct {
	pgx.Tx
	release func()
	once    sync.Once
}

func (tx *leasedTx) Commit(ctx context.Context) error {
	defer tx.once.Do(tx.release)
	return tx.Tx.Commit(ctx)
}

func (tx *leasedTx) Rollback(ctx context.Context) error {
	defer tx.once.Do(tx.release)
	return tx.Tx.Rollback(ctx)
}
