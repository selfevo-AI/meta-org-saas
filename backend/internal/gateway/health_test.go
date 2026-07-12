package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/auditretention"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/tenantprojection"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/authlimit"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type fakeTenantPoolStatsProvider struct {
	stats tenantdb.PoolRouterStats
}

func (p fakeTenantPoolStatsProvider) Stats() tenantdb.PoolRouterStats {
	return p.stats
}

type fakeTenantProjectionStatsProvider struct {
	stats tenantprojection.WorkerStats
}

func (p fakeTenantProjectionStatsProvider) Stats() tenantprojection.WorkerStats {
	return p.stats
}

type fakeAuthenticationRateLimitStatsProvider struct {
	stats authlimit.Stats
}

type fakeAuditRetentionStatsProvider struct {
	stats auditretention.WorkerStats
}

func (p fakeAuditRetentionStatsProvider) Stats() auditretention.WorkerStats {
	return p.stats
}

func (p fakeAuthenticationRateLimitStatsProvider) Stats() authlimit.Stats {
	return p.stats
}

func TestHealthCheckIncludesTenantPoolStats(t *testing.T) {
	provider := fakeTenantPoolStatsProvider{stats: tenantdb.PoolRouterStats{
		CachedPools:             3,
		ActivePools:             1,
		CapacityRejectionsTotal: 2,
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	projectionProvider := fakeTenantProjectionStatsProvider{stats: tenantprojection.WorkerStats{
		WorkerID:            "projection-test",
		EventsProjected:     7,
		LastProjectionLagMs: 12,
		Running:             true,
	}}
	authLimitProvider := fakeAuthenticationRateLimitStatsProvider{stats: authlimit.Stats{
		ChecksTotal:      9,
		RateLimitedTotal: 2,
		BlocksApplied:    1,
	}}
	retentionProvider := fakeAuditRetentionStatsProvider{stats: auditretention.WorkerStats{
		Running: true, RunsTotal: 3, RowsRedactedTotal: 42,
	}}
	healthCheck(provider, projectionProvider, authLimitProvider, retentionProvider).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response healthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" {
		t.Fatalf("status = %q, want ok", response.Status)
	}
	if response.TenantDatabasePools == nil {
		t.Fatal("tenant_database_pools missing")
	}
	if response.TenantDatabasePools.CachedPools != 3 || response.TenantDatabasePools.ActivePools != 1 {
		t.Fatalf("tenant pool stats = %#v", response.TenantDatabasePools)
	}
	if response.TenantDatabasePools.CapacityRejectionsTotal != 2 {
		t.Fatalf("capacity rejections = %d, want 2", response.TenantDatabasePools.CapacityRejectionsTotal)
	}
	if response.TenantProjectionWorker == nil {
		t.Fatal("tenant_projection_worker missing")
	}
	if response.TenantProjectionWorker.WorkerID != "projection-test" || response.TenantProjectionWorker.EventsProjected != 7 {
		t.Fatalf("projection worker stats = %#v", response.TenantProjectionWorker)
	}
	if !response.TenantProjectionWorker.Running || response.TenantProjectionWorker.LastProjectionLagMs != 12 {
		t.Fatalf("projection worker health = %#v", response.TenantProjectionWorker)
	}
	if response.AuthenticationRateLimits == nil {
		t.Fatal("authentication_rate_limits missing")
	}
	if response.RequestRateLimits == nil || response.RequestRateLimits.ChecksTotal != 9 || response.RequestRateLimits.RateLimitedTotal != 2 || response.RequestRateLimits.BlocksApplied != 1 {
		t.Fatalf("request rate limit stats = %#v", response.RequestRateLimits)
	}
	if response.AuthenticationRateLimits.ChecksTotal != 9 || response.AuthenticationRateLimits.RateLimitedTotal != 2 || response.AuthenticationRateLimits.BlocksApplied != 1 {
		t.Fatalf("authentication rate limit stats = %#v", response.AuthenticationRateLimits)
	}
	if response.AuditRetentionWorker == nil || !response.AuditRetentionWorker.Running || response.AuditRetentionWorker.RunsTotal != 3 || response.AuditRetentionWorker.RowsRedactedTotal != 42 {
		t.Fatalf("audit retention worker stats = %#v", response.AuditRetentionWorker)
	}
}
