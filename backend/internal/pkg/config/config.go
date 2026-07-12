package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ServerPort                        int
	DatabaseURL                       string
	PlatformDatabaseURL               string
	TenantDatabaseAdminURL            string
	TenantMigrationsPath              string
	TenantDatabaseNamePrefix          string
	TenantDatabaseMode                string
	TenantDatabaseDefaultCluster      string
	TenantDatabaseDefaultRegion       string
	TenantDatabasePoolMaxEntries      int
	TenantDatabasePoolMaxConnections  int
	TenantDatabasePoolMinConnections  int
	TenantDatabasePoolIdleSeconds     int
	TenantDatabasePoolSweepSeconds    int
	TenantDatabaseConnIdleSeconds     int
	TenantDatabaseConnLifetimeSeconds int
	TenantProvisioningWorkerEnabled   bool
	TenantProvisioningPollSeconds     int
	TenantProvisioningLeaseSeconds    int
	TenantProvisioningRetrySeconds    int
	TenantProvisioningRetryMaxSeconds int
	TenantProjectionWorkerEnabled     bool
	TenantProjectionPollSeconds       int
	TenantProjectionLeaseSeconds      int
	TenantProjectionRetrySeconds      int
	TenantProjectionBatchSize         int
	TenantProjectionTargetLimit       int
	TenantProjectionActivityLimit     int
	TenantProjectionMaxAttempts       int
	JWTSecret                         string
	ModelSecretKey                    string
	CorsOrigins                       []string
	TrustedProxyCIDRs                 []string
	AuthRateLimitWindowSeconds        int
	AuthRateLimitMaxAttempts          int
	AuthRateLimitFailureThreshold     int
	AuthRateLimitBlockSeconds         int
	AuthRegistrationMaxAttempts       int
	MigrationsPath                    string
	MetaOrgMode                       string
	MetaOrgDistributionMode           string
	MetaOrgLicenseMode                string
	PlatformAdminEmail                string
	PlatformAdminPasswordHash         string
	SecurityKernelURL                 string
	SecurityKernelSharedSecret        string
	SecurityKernelEnforcementMode     string
	MonitoringAgentSchedulerEnabled   bool
	MonitoringAgentDailyTime          string
	MonitoringAgentLookbackHours      int
	MonitoringAgentMaxSignalsPerRun   int
}

func Load() *Config {
	mode := strings.ToLower(strings.TrimSpace(getEnv("META_ORG_MODE", "single_org")))
	if mode != "saas" {
		mode = "single_org"
	}
	databaseURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@127.0.0.1:5432/meta_org_saas?sslmode=disable")
	platformDatabaseURL := getEnv("PLATFORM_DATABASE_URL", databaseURL)
	return &Config{
		ServerPort:                        getEnvInt("SERVER_PORT", 8080),
		DatabaseURL:                       databaseURL,
		PlatformDatabaseURL:               platformDatabaseURL,
		TenantDatabaseAdminURL:            getEnv("TENANT_DATABASE_ADMIN_URL", defaultTenantDatabaseAdminURL(platformDatabaseURL)),
		TenantMigrationsPath:              getEnv("TENANT_MIGRATIONS_PATH", "migrations/tenant"),
		TenantDatabaseNamePrefix:          normalizedTenantDatabasePrefix(getEnv("TENANT_DATABASE_NAME_PREFIX", "meta_org_")),
		TenantDatabaseMode:                normalizedTenantDatabaseMode(getEnv("TENANT_DATABASE_MODE", "dedicated_database")),
		TenantDatabaseDefaultCluster:      strings.TrimSpace(getEnv("TENANT_DATABASE_DEFAULT_CLUSTER", "local-primary")),
		TenantDatabaseDefaultRegion:       strings.TrimSpace(getEnv("TENANT_DATABASE_DEFAULT_REGION", "local")),
		TenantDatabasePoolMaxEntries:      getEnvInt("TENANT_DATABASE_POOL_MAX_ENTRIES", 16),
		TenantDatabasePoolMaxConnections:  getEnvInt("TENANT_DATABASE_POOL_MAX_CONNECTIONS", 4),
		TenantDatabasePoolMinConnections:  getEnvInt("TENANT_DATABASE_POOL_MIN_CONNECTIONS", 0),
		TenantDatabasePoolIdleSeconds:     getEnvInt("TENANT_DATABASE_POOL_IDLE_SECONDS", 900),
		TenantDatabasePoolSweepSeconds:    getEnvInt("TENANT_DATABASE_POOL_SWEEP_SECONDS", 60),
		TenantDatabaseConnIdleSeconds:     getEnvInt("TENANT_DATABASE_CONNECTION_IDLE_SECONDS", 300),
		TenantDatabaseConnLifetimeSeconds: getEnvInt("TENANT_DATABASE_CONNECTION_LIFETIME_SECONDS", 1800),
		TenantProvisioningWorkerEnabled:   getEnvBool("TENANT_PROVISIONING_WORKER_ENABLED", true),
		TenantProvisioningPollSeconds:     getEnvInt("TENANT_PROVISIONING_POLL_SECONDS", 2),
		TenantProvisioningLeaseSeconds:    getEnvInt("TENANT_PROVISIONING_LEASE_SECONDS", 1800),
		TenantProvisioningRetrySeconds:    getEnvInt("TENANT_PROVISIONING_RETRY_SECONDS", 5),
		TenantProvisioningRetryMaxSeconds: getEnvInt("TENANT_PROVISIONING_RETRY_MAX_SECONDS", 900),
		TenantProjectionWorkerEnabled:     getEnvBool("TENANT_PROJECTION_WORKER_ENABLED", true),
		TenantProjectionPollSeconds:       getEnvInt("TENANT_PROJECTION_POLL_SECONDS", 2),
		TenantProjectionLeaseSeconds:      getEnvInt("TENANT_PROJECTION_LEASE_SECONDS", 60),
		TenantProjectionRetrySeconds:      getEnvInt("TENANT_PROJECTION_RETRY_SECONDS", 5),
		TenantProjectionBatchSize:         getEnvInt("TENANT_PROJECTION_BATCH_SIZE", 100),
		TenantProjectionTargetLimit:       getEnvInt("TENANT_PROJECTION_TARGET_LIMIT", 100),
		TenantProjectionActivityLimit:     getEnvInt("TENANT_PROJECTION_ACTIVITY_LIMIT", 50),
		TenantProjectionMaxAttempts:       getEnvInt("TENANT_PROJECTION_MAX_ATTEMPTS", 20),
		JWTSecret:                         getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		ModelSecretKey:                    getEnv("MODEL_SECRET_KEY", "dev-model-secret-key-32-bytes!!!"),
		CorsOrigins:                       getEnvSlice("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"),
		TrustedProxyCIDRs:                 getEnvSlice("TRUSTED_PROXY_CIDRS", ""),
		AuthRateLimitWindowSeconds:        getEnvInt("AUTH_RATE_LIMIT_WINDOW_SECONDS", 60),
		AuthRateLimitMaxAttempts:          getEnvInt("AUTH_RATE_LIMIT_MAX_ATTEMPTS", 10),
		AuthRateLimitFailureThreshold:     getEnvInt("AUTH_RATE_LIMIT_FAILURE_THRESHOLD", 5),
		AuthRateLimitBlockSeconds:         getEnvInt("AUTH_RATE_LIMIT_BLOCK_SECONDS", 300),
		AuthRegistrationMaxAttempts:       getEnvInt("AUTH_REGISTRATION_MAX_ATTEMPTS", 5),
		MigrationsPath:                    getEnv("MIGRATIONS_PATH", "migrations"),
		MetaOrgMode:                       mode,
		MetaOrgDistributionMode:           normalizedDistributionMode(getEnv("META_ORG_DISTRIBUTION_MODE", mode)),
		MetaOrgLicenseMode:                normalizedLicenseMode(getEnv("META_ORG_LICENSE_MODE", "commercial")),
		PlatformAdminEmail:                strings.ToLower(strings.TrimSpace(getEnv("META_ORG_PLATFORM_ADMIN_EMAIL", ""))),
		PlatformAdminPasswordHash:         getEnv("META_ORG_PLATFORM_ADMIN_PASSWORD_HASH", ""),
		SecurityKernelURL:                 strings.TrimSpace(getEnv("SECURITY_KERNEL_URL", "")),
		SecurityKernelSharedSecret:        getEnv("SECURITY_KERNEL_SHARED_SECRET", ""),
		SecurityKernelEnforcementMode:     normalizedEnforcementMode(getEnv("SECURITY_KERNEL_ENFORCEMENT_MODE", "blocking")),
		MonitoringAgentSchedulerEnabled:   getEnvBool("MONITORING_AGENT_SCHEDULER_ENABLED", false),
		MonitoringAgentDailyTime:          strings.TrimSpace(getEnv("MONITORING_AGENT_DAILY_TIME", "02:00")),
		MonitoringAgentLookbackHours:      getEnvInt("MONITORING_AGENT_LOOKBACK_HOURS", 24),
		MonitoringAgentMaxSignalsPerRun:   getEnvInt("MONITORING_AGENT_MAX_SIGNALS_PER_RUN", 100),
	}
}

func normalizedTenantDatabaseMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "shared_schema":
		return "shared_schema"
	default:
		return "dedicated_database"
	}
}

func defaultTenantDatabaseAdminURL(platformDatabaseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(platformDatabaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return platformDatabaseURL
	}
	parsed.Path = "/postgres"
	return parsed.String()
}

func normalizedTenantDatabasePrefix(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "meta_org_"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "meta_org_"
	}
	return out + "_"
}

func normalizedDistributionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "saas", "saas_org_private", "single_org_commercial", "private_deployment":
		return strings.ToLower(strings.TrimSpace(value))
	case "single_org":
		return "private_deployment"
	default:
		return "saas"
	}
}

func normalizedLicenseMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "community", "commercial", "enterprise", "private_contract":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "commercial"
	}
}

func normalizedEnforcementMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "audit":
		return "audit"
	default:
		return "blocking"
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func getEnvSlice(key, fallback string) []string {
	v := getEnv(key, fallback)
	parts := strings.Split(v, ",")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.TrimSpace(p)
	}
	return result
}
