package config

import "testing"

func TestLoadUses32ByteDefaultModelSecretKey(t *testing.T) {
	t.Setenv("MODEL_SECRET_KEY", "")

	cfg := Load()

	if len(cfg.ModelSecretKey) != 32 {
		t.Fatalf("ModelSecretKey length = %d, want 32", len(cfg.ModelSecretKey))
	}
}

func TestLoadSecurityKernelConfig(t *testing.T) {
	t.Setenv("SECURITY_KERNEL_URL", "http://security-kernel:8090")
	t.Setenv("SECURITY_KERNEL_SHARED_SECRET", "shared")
	t.Setenv("SECURITY_KERNEL_ENFORCEMENT_MODE", "audit")
	t.Setenv("META_ORG_DISTRIBUTION_MODE", "single_org_commercial")
	t.Setenv("META_ORG_LICENSE_MODE", "enterprise")

	cfg := Load()

	if cfg.SecurityKernelURL != "http://security-kernel:8090" {
		t.Fatalf("SecurityKernelURL = %q", cfg.SecurityKernelURL)
	}
	if cfg.SecurityKernelSharedSecret != "shared" {
		t.Fatalf("SecurityKernelSharedSecret = %q", cfg.SecurityKernelSharedSecret)
	}
	if cfg.SecurityKernelEnforcementMode != "audit" {
		t.Fatalf("SecurityKernelEnforcementMode = %q", cfg.SecurityKernelEnforcementMode)
	}
	if cfg.MetaOrgDistributionMode != "single_org_commercial" {
		t.Fatalf("MetaOrgDistributionMode = %q", cfg.MetaOrgDistributionMode)
	}
	if cfg.MetaOrgLicenseMode != "enterprise" {
		t.Fatalf("MetaOrgLicenseMode = %q", cfg.MetaOrgLicenseMode)
	}
}

func TestLoadMonitoringAgentConfigDefaultsToDisabledScheduler(t *testing.T) {
	t.Setenv("MONITORING_AGENT_SCHEDULER_ENABLED", "")
	t.Setenv("MONITORING_AGENT_DAILY_TIME", "")
	t.Setenv("MONITORING_AGENT_LOOKBACK_HOURS", "")
	t.Setenv("MONITORING_AGENT_MAX_SIGNALS_PER_RUN", "")

	cfg := Load()

	if cfg.MonitoringAgentSchedulerEnabled {
		t.Fatal("MonitoringAgentSchedulerEnabled = true, want false")
	}
	if cfg.MonitoringAgentDailyTime != "02:00" {
		t.Fatalf("MonitoringAgentDailyTime = %q, want 02:00", cfg.MonitoringAgentDailyTime)
	}
	if cfg.MonitoringAgentLookbackHours != 24 {
		t.Fatalf("MonitoringAgentLookbackHours = %d, want 24", cfg.MonitoringAgentLookbackHours)
	}
	if cfg.MonitoringAgentMaxSignalsPerRun != 100 {
		t.Fatalf("MonitoringAgentMaxSignalsPerRun = %d, want 100", cfg.MonitoringAgentMaxSignalsPerRun)
	}
}

func TestLoadMonitoringAgentConfigCanEnableScheduler(t *testing.T) {
	t.Setenv("MONITORING_AGENT_SCHEDULER_ENABLED", "true")
	t.Setenv("MONITORING_AGENT_DAILY_TIME", "03:30")
	t.Setenv("MONITORING_AGENT_LOOKBACK_HOURS", "48")
	t.Setenv("MONITORING_AGENT_MAX_SIGNALS_PER_RUN", "25")

	cfg := Load()

	if !cfg.MonitoringAgentSchedulerEnabled {
		t.Fatal("MonitoringAgentSchedulerEnabled = false, want true")
	}
	if cfg.MonitoringAgentDailyTime != "03:30" {
		t.Fatalf("MonitoringAgentDailyTime = %q, want 03:30", cfg.MonitoringAgentDailyTime)
	}
	if cfg.MonitoringAgentLookbackHours != 48 {
		t.Fatalf("MonitoringAgentLookbackHours = %d, want 48", cfg.MonitoringAgentLookbackHours)
	}
	if cfg.MonitoringAgentMaxSignalsPerRun != 25 {
		t.Fatalf("MonitoringAgentMaxSignalsPerRun = %d, want 25", cfg.MonitoringAgentMaxSignalsPerRun)
	}
}

func TestLoadDatabaseTopologyConfigDefaultsToPhysicalTenantDatabases(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PLATFORM_DATABASE_URL", "")
	t.Setenv("TENANT_DATABASE_ADMIN_URL", "")
	t.Setenv("TENANT_MIGRATIONS_PATH", "")
	t.Setenv("TENANT_DATABASE_NAME_PREFIX", "")
	t.Setenv("TENANT_DATABASE_MODE", "")
	t.Setenv("TENANT_DATABASE_DEFAULT_CLUSTER", "")
	t.Setenv("TENANT_DATABASE_DEFAULT_REGION", "")
	t.Setenv("TENANT_DATABASE_POOL_MAX_ENTRIES", "")
	t.Setenv("TENANT_DATABASE_POOL_MAX_CONNECTIONS", "")
	t.Setenv("TENANT_DATABASE_POOL_MIN_CONNECTIONS", "")
	t.Setenv("TENANT_DATABASE_POOL_IDLE_SECONDS", "")
	t.Setenv("TENANT_DATABASE_POOL_SWEEP_SECONDS", "")
	t.Setenv("TENANT_DATABASE_CONNECTION_IDLE_SECONDS", "")
	t.Setenv("TENANT_DATABASE_CONNECTION_LIFETIME_SECONDS", "")
	t.Setenv("TENANT_PROJECTION_WORKER_ENABLED", "")
	t.Setenv("TENANT_PROJECTION_POLL_SECONDS", "")
	t.Setenv("TENANT_PROJECTION_LEASE_SECONDS", "")
	t.Setenv("TENANT_PROJECTION_RETRY_SECONDS", "")
	t.Setenv("TENANT_PROJECTION_BATCH_SIZE", "")
	t.Setenv("TENANT_PROJECTION_TARGET_LIMIT", "")
	t.Setenv("TENANT_PROJECTION_ACTIVITY_LIMIT", "")
	t.Setenv("TENANT_PROJECTION_MAX_ATTEMPTS", "")

	cfg := Load()

	if cfg.DatabaseURL != "postgres://postgres:postgres@127.0.0.1:5432/meta_org_saas?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.PlatformDatabaseURL != cfg.DatabaseURL {
		t.Fatalf("PlatformDatabaseURL = %q, want DatabaseURL fallback %q", cfg.PlatformDatabaseURL, cfg.DatabaseURL)
	}
	if cfg.TenantDatabaseAdminURL != "postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable" {
		t.Fatalf("TenantDatabaseAdminURL = %q", cfg.TenantDatabaseAdminURL)
	}
	if cfg.TenantMigrationsPath != "migrations/tenant" {
		t.Fatalf("TenantMigrationsPath = %q, want migrations/tenant", cfg.TenantMigrationsPath)
	}
	if cfg.TenantDatabaseNamePrefix != "meta_org_" {
		t.Fatalf("TenantDatabaseNamePrefix = %q, want meta_org_", cfg.TenantDatabaseNamePrefix)
	}
	if cfg.TenantDatabaseMode != "dedicated_database" {
		t.Fatalf("TenantDatabaseMode = %q, want dedicated_database", cfg.TenantDatabaseMode)
	}
	if cfg.TenantDatabaseDefaultCluster != "local-primary" {
		t.Fatalf("TenantDatabaseDefaultCluster = %q, want local-primary", cfg.TenantDatabaseDefaultCluster)
	}
	if cfg.TenantDatabaseDefaultRegion != "local" {
		t.Fatalf("TenantDatabaseDefaultRegion = %q, want local", cfg.TenantDatabaseDefaultRegion)
	}
	if cfg.TenantDatabasePoolMaxEntries != 16 || cfg.TenantDatabasePoolMaxConnections != 4 || cfg.TenantDatabasePoolMinConnections != 0 {
		t.Fatalf("tenant pool capacity defaults = %d/%d/%d", cfg.TenantDatabasePoolMaxEntries, cfg.TenantDatabasePoolMaxConnections, cfg.TenantDatabasePoolMinConnections)
	}
	if cfg.TenantDatabasePoolIdleSeconds != 900 || cfg.TenantDatabasePoolSweepSeconds != 60 {
		t.Fatalf("tenant pool eviction defaults = %d/%d", cfg.TenantDatabasePoolIdleSeconds, cfg.TenantDatabasePoolSweepSeconds)
	}
	if cfg.TenantDatabaseConnIdleSeconds != 300 || cfg.TenantDatabaseConnLifetimeSeconds != 1800 {
		t.Fatalf("tenant connection lifetime defaults = %d/%d", cfg.TenantDatabaseConnIdleSeconds, cfg.TenantDatabaseConnLifetimeSeconds)
	}
	if !cfg.TenantProjectionWorkerEnabled || cfg.TenantProjectionPollSeconds != 2 || cfg.TenantProjectionLeaseSeconds != 60 || cfg.TenantProjectionRetrySeconds != 5 {
		t.Fatalf("tenant projection timing defaults = enabled:%t poll:%d lease:%d retry:%d", cfg.TenantProjectionWorkerEnabled, cfg.TenantProjectionPollSeconds, cfg.TenantProjectionLeaseSeconds, cfg.TenantProjectionRetrySeconds)
	}
	if cfg.TenantProjectionBatchSize != 100 || cfg.TenantProjectionTargetLimit != 100 || cfg.TenantProjectionActivityLimit != 50 || cfg.TenantProjectionMaxAttempts != 20 {
		t.Fatalf("tenant projection capacity defaults = %d/%d/%d/%d", cfg.TenantProjectionBatchSize, cfg.TenantProjectionTargetLimit, cfg.TenantProjectionActivityLimit, cfg.TenantProjectionMaxAttempts)
	}
}

func TestLoadDatabaseTopologyConfigAllowsExplicitControlPlaneAndTenantAdminURLs(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/legacy?sslmode=disable")
	t.Setenv("PLATFORM_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meta_org_saas_explicit?sslmode=disable")
	t.Setenv("TENANT_DATABASE_ADMIN_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	t.Setenv("TENANT_MIGRATIONS_PATH", "../migrations/tenant")
	t.Setenv("TENANT_DATABASE_NAME_PREFIX", "customer_org_")
	t.Setenv("TENANT_DATABASE_MODE", "shared_schema")
	t.Setenv("TENANT_DATABASE_DEFAULT_CLUSTER", "cn-east-1-a")
	t.Setenv("TENANT_DATABASE_DEFAULT_REGION", "cn-east-1")
	t.Setenv("TENANT_DATABASE_POOL_MAX_ENTRIES", "24")
	t.Setenv("TENANT_DATABASE_POOL_MAX_CONNECTIONS", "8")
	t.Setenv("TENANT_DATABASE_POOL_MIN_CONNECTIONS", "2")
	t.Setenv("TENANT_DATABASE_POOL_IDLE_SECONDS", "600")
	t.Setenv("TENANT_DATABASE_POOL_SWEEP_SECONDS", "30")
	t.Setenv("TENANT_DATABASE_CONNECTION_IDLE_SECONDS", "120")
	t.Setenv("TENANT_DATABASE_CONNECTION_LIFETIME_SECONDS", "900")
	t.Setenv("TENANT_PROJECTION_WORKER_ENABLED", "false")
	t.Setenv("TENANT_PROJECTION_POLL_SECONDS", "7")
	t.Setenv("TENANT_PROJECTION_LEASE_SECONDS", "90")
	t.Setenv("TENANT_PROJECTION_RETRY_SECONDS", "11")
	t.Setenv("TENANT_PROJECTION_BATCH_SIZE", "25")
	t.Setenv("TENANT_PROJECTION_TARGET_LIMIT", "40")
	t.Setenv("TENANT_PROJECTION_ACTIVITY_LIMIT", "30")
	t.Setenv("TENANT_PROJECTION_MAX_ATTEMPTS", "12")

	cfg := Load()

	if cfg.PlatformDatabaseURL != "postgres://postgres:postgres@localhost:5432/meta_org_saas_explicit?sslmode=disable" {
		t.Fatalf("PlatformDatabaseURL = %q", cfg.PlatformDatabaseURL)
	}
	if cfg.TenantDatabaseAdminURL != "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" {
		t.Fatalf("TenantDatabaseAdminURL = %q", cfg.TenantDatabaseAdminURL)
	}
	if cfg.TenantMigrationsPath != "../migrations/tenant" {
		t.Fatalf("TenantMigrationsPath = %q", cfg.TenantMigrationsPath)
	}
	if cfg.TenantDatabaseNamePrefix != "customer_org_" {
		t.Fatalf("TenantDatabaseNamePrefix = %q", cfg.TenantDatabaseNamePrefix)
	}
	if cfg.TenantDatabaseMode != "shared_schema" {
		t.Fatalf("TenantDatabaseMode = %q, want shared_schema", cfg.TenantDatabaseMode)
	}
	if cfg.TenantDatabaseDefaultCluster != "cn-east-1-a" {
		t.Fatalf("TenantDatabaseDefaultCluster = %q", cfg.TenantDatabaseDefaultCluster)
	}
	if cfg.TenantDatabaseDefaultRegion != "cn-east-1" {
		t.Fatalf("TenantDatabaseDefaultRegion = %q", cfg.TenantDatabaseDefaultRegion)
	}
	if cfg.TenantDatabasePoolMaxEntries != 24 || cfg.TenantDatabasePoolMaxConnections != 8 || cfg.TenantDatabasePoolMinConnections != 2 {
		t.Fatalf("tenant pool capacity = %d/%d/%d", cfg.TenantDatabasePoolMaxEntries, cfg.TenantDatabasePoolMaxConnections, cfg.TenantDatabasePoolMinConnections)
	}
	if cfg.TenantDatabasePoolIdleSeconds != 600 || cfg.TenantDatabasePoolSweepSeconds != 30 {
		t.Fatalf("tenant pool eviction = %d/%d", cfg.TenantDatabasePoolIdleSeconds, cfg.TenantDatabasePoolSweepSeconds)
	}
	if cfg.TenantDatabaseConnIdleSeconds != 120 || cfg.TenantDatabaseConnLifetimeSeconds != 900 {
		t.Fatalf("tenant connection lifetime = %d/%d", cfg.TenantDatabaseConnIdleSeconds, cfg.TenantDatabaseConnLifetimeSeconds)
	}
	if cfg.TenantProjectionWorkerEnabled || cfg.TenantProjectionPollSeconds != 7 || cfg.TenantProjectionLeaseSeconds != 90 || cfg.TenantProjectionRetrySeconds != 11 {
		t.Fatalf("tenant projection timing = enabled:%t poll:%d lease:%d retry:%d", cfg.TenantProjectionWorkerEnabled, cfg.TenantProjectionPollSeconds, cfg.TenantProjectionLeaseSeconds, cfg.TenantProjectionRetrySeconds)
	}
	if cfg.TenantProjectionBatchSize != 25 || cfg.TenantProjectionTargetLimit != 40 || cfg.TenantProjectionActivityLimit != 30 || cfg.TenantProjectionMaxAttempts != 12 {
		t.Fatalf("tenant projection capacity = %d/%d/%d/%d", cfg.TenantProjectionBatchSize, cfg.TenantProjectionTargetLimit, cfg.TenantProjectionActivityLimit, cfg.TenantProjectionMaxAttempts)
	}
}
