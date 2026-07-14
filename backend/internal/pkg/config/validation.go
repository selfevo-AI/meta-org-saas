package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	developmentJWTSecret   = "dev-secret-change-in-production"
	developmentModelSecret = "dev-model-secret-key-32-bytes!!!"
	composeModelSecret     = "0123456789abcdef0123456789abcdef"
	developmentKernelKey   = "dev-security-kernel-shared-secret"
)

var integerEnvironmentKeys = []string{
	"SERVER_PORT",
	"TENANT_DATABASE_POOL_MAX_ENTRIES", "TENANT_DATABASE_POOL_MAX_CONNECTIONS", "TENANT_DATABASE_POOL_MIN_CONNECTIONS",
	"TENANT_DATABASE_POOL_IDLE_SECONDS", "TENANT_DATABASE_POOL_SWEEP_SECONDS", "TENANT_DATABASE_CONNECTION_IDLE_SECONDS", "TENANT_DATABASE_CONNECTION_LIFETIME_SECONDS",
	"TENANT_PROVISIONING_POLL_SECONDS", "TENANT_PROVISIONING_LEASE_SECONDS", "TENANT_PROVISIONING_RETRY_SECONDS", "TENANT_PROVISIONING_RETRY_MAX_SECONDS",
	"TENANT_PROJECTION_POLL_SECONDS", "TENANT_PROJECTION_LEASE_SECONDS", "TENANT_PROJECTION_RETRY_SECONDS", "TENANT_PROJECTION_BATCH_SIZE",
	"TENANT_PROJECTION_TARGET_LIMIT", "TENANT_PROJECTION_ACTIVITY_LIMIT", "TENANT_PROJECTION_MAX_ATTEMPTS",
	"AUTH_RATE_LIMIT_WINDOW_SECONDS", "AUTH_RATE_LIMIT_MAX_ATTEMPTS", "AUTH_RATE_LIMIT_FAILURE_THRESHOLD", "AUTH_RATE_LIMIT_BLOCK_SECONDS", "AUTH_REGISTRATION_MAX_ATTEMPTS",
	"SENSITIVE_RATE_LIMIT_WINDOW_SECONDS", "SENSITIVE_RATE_LIMIT_MAX_ATTEMPTS", "SENSITIVE_RATE_LIMIT_BLOCK_SECONDS", "INVITATION_ACCEPT_RATE_LIMIT_MAX_ATTEMPTS",
	"AI_GATEWAY_RATE_LIMIT_WINDOW_SECONDS", "AI_GATEWAY_RATE_LIMIT_MAX_REQUESTS", "AI_GATEWAY_RATE_LIMIT_BLOCK_SECONDS",
	"AI_GATEWAY_INVOKE_TIMEOUT_SECONDS", "AI_GATEWAY_STREAM_TIMEOUT_SECONDS",
	"AI_GATEWAY_MAX_RETRIES", "AI_GATEWAY_CHANNEL_FAILURE_THRESHOLD", "AI_GATEWAY_CHANNEL_COOLDOWN_SECONDS",
	"AI_GATEWAY_RESERVATION_STALE_SECONDS", "AI_GATEWAY_RESERVATION_POLL_SECONDS", "AI_GATEWAY_RESERVATION_LEASE_SECONDS", "AI_GATEWAY_RESERVATION_BATCH_SIZE",
	"AUDIT_RETENTION_DAYS", "AUDIT_RETENTION_POLL_SECONDS", "AUDIT_RETENTION_BATCH_SIZE", "AUDIT_RETENTION_MAX_BATCHES",
	"BUSINESS_AI_MAX_TOKENS", "MONITORING_AGENT_LOOKBACK_HOURS", "MONITORING_AGENT_MAX_SIGNALS_PER_RUN",
}

var booleanEnvironmentKeys = []string{
	"TENANT_PROVISIONING_WORKER_ENABLED", "TENANT_PROJECTION_WORKER_ENABLED", "AI_GATEWAY_RESERVATION_RECOVERY_ENABLED", "AUDIT_RETENTION_WORKER_ENABLED", "MONITORING_AGENT_SCHEDULER_ENABLED",
}

func LoadValidated() (*Config, error) {
	cfg := Load()
	return cfg, errors.Join(validateEnvironmentSyntax(), cfg.Validate())
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	var errs []error
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		errs = append(errs, fmt.Errorf("META_ORG_ENVIRONMENT must be development, test, or production"))
	}
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		errs = append(errs, fmt.Errorf("SERVER_PORT must be between 1 and 65535"))
	}
	errs = append(errs, validatePostgresURL("PLATFORM_DATABASE_URL", c.PlatformDatabaseURL, true))
	if c.MetaOrgMode == "saas" && c.TenantDatabaseMode == "dedicated_database" {
		errs = append(errs, validatePostgresURL("TENANT_DATABASE_ADMIN_URL", c.TenantDatabaseAdminURL, true))
	}
	if strings.TrimSpace(c.MigrationsPath) == "" {
		errs = append(errs, fmt.Errorf("MIGRATIONS_PATH is required"))
	}
	if strings.TrimSpace(c.TenantMigrationsPath) == "" {
		errs = append(errs, fmt.Errorf("TENANT_MIGRATIONS_PATH is required"))
	}
	if len(c.ModelSecretKey) != 32 {
		errs = append(errs, fmt.Errorf("MODEL_SECRET_KEY must contain exactly 32 bytes"))
	}
	if len(c.JWTSecret) < 16 {
		errs = append(errs, fmt.Errorf("JWT_SECRET must contain at least 16 characters"))
	}
	// Repository-public development secrets must be rejected in production, and
	// also in saas mode outside an explicit test environment: the default
	// Environment is "development", so a saas deployment that forgets to set
	// META_ORG_ENVIRONMENT=production would otherwise silently sign JWTs and
	// encrypt provider keys with secrets that are committed to this repository.
	// The "test" environment is exempt so CI can run saas flows with throwaway keys.
	requireHardenedSecrets := c.Environment == "production" || (c.MetaOrgMode == "saas" && c.Environment != "test")
	if requireHardenedSecrets {
		if len(c.JWTSecret) < 32 || c.JWTSecret == developmentJWTSecret {
			errs = append(errs, fmt.Errorf("JWT_SECRET must contain at least 32 characters and must not use the development default in production or saas mode"))
		}
		if c.ModelSecretKey == developmentModelSecret || c.ModelSecretKey == composeModelSecret {
			errs = append(errs, fmt.Errorf("MODEL_SECRET_KEY must not use a repository development key in production or saas mode"))
		}
		if c.SecurityKernelSharedSecret == developmentKernelKey {
			errs = append(errs, fmt.Errorf("SECURITY_KERNEL_SHARED_SECRET must not use the Compose development key in production or saas mode"))
		}
	}
	if c.MetaOrgMode == "saas" {
		if strings.TrimSpace(c.PlatformAdminEmail) == "" || strings.TrimSpace(c.PlatformAdminPasswordHash) == "" {
			errs = append(errs, fmt.Errorf("META_ORG_PLATFORM_ADMIN_EMAIL and META_ORG_PLATFORM_ADMIN_PASSWORD_HASH are required in saas mode"))
		} else if _, err := bcrypt.Cost([]byte(c.PlatformAdminPasswordHash)); err != nil {
			errs = append(errs, fmt.Errorf("META_ORG_PLATFORM_ADMIN_PASSWORD_HASH must be a bcrypt hash"))
		}
	}
	if c.MetaOrgMode == "saas" && strings.TrimSpace(c.SecurityKernelURL) == "" {
		errs = append(errs, fmt.Errorf("SECURITY_KERNEL_URL is required in saas mode"))
	}
	if strings.TrimSpace(c.SecurityKernelURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(c.SecurityKernelURL))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			errs = append(errs, fmt.Errorf("SECURITY_KERNEL_URL must be an absolute HTTP or HTTPS URL"))
		}
		if len(strings.TrimSpace(c.SecurityKernelSharedSecret)) < 32 {
			errs = append(errs, fmt.Errorf("SECURITY_KERNEL_SHARED_SECRET must contain at least 32 characters"))
		}
	}
	if c.MetaOrgMode == "saas" && c.SecurityKernelEnforcementMode != "blocking" {
		errs = append(errs, fmt.Errorf("SECURITY_KERNEL_ENFORCEMENT_MODE must be blocking in saas mode"))
	}

	errs = append(errs, validatePositive("TENANT_DATABASE_POOL_MAX_ENTRIES", c.TenantDatabasePoolMaxEntries))
	errs = append(errs, validatePositive("TENANT_DATABASE_POOL_MAX_CONNECTIONS", c.TenantDatabasePoolMaxConnections))
	if c.TenantDatabasePoolMinConnections < 0 || c.TenantDatabasePoolMinConnections > c.TenantDatabasePoolMaxConnections {
		errs = append(errs, fmt.Errorf("TENANT_DATABASE_POOL_MIN_CONNECTIONS must be between 0 and TENANT_DATABASE_POOL_MAX_CONNECTIONS"))
	}
	for name, value := range map[string]int{
		"TENANT_DATABASE_POOL_IDLE_SECONDS": c.TenantDatabasePoolIdleSeconds, "TENANT_DATABASE_POOL_SWEEP_SECONDS": c.TenantDatabasePoolSweepSeconds,
		"TENANT_DATABASE_CONNECTION_IDLE_SECONDS": c.TenantDatabaseConnIdleSeconds, "TENANT_DATABASE_CONNECTION_LIFETIME_SECONDS": c.TenantDatabaseConnLifetimeSeconds,
		"TENANT_PROVISIONING_POLL_SECONDS": c.TenantProvisioningPollSeconds, "TENANT_PROVISIONING_LEASE_SECONDS": c.TenantProvisioningLeaseSeconds,
		"TENANT_PROVISIONING_RETRY_SECONDS": c.TenantProvisioningRetrySeconds, "TENANT_PROVISIONING_RETRY_MAX_SECONDS": c.TenantProvisioningRetryMaxSeconds,
		"TENANT_PROJECTION_POLL_SECONDS": c.TenantProjectionPollSeconds, "TENANT_PROJECTION_LEASE_SECONDS": c.TenantProjectionLeaseSeconds,
		"TENANT_PROJECTION_RETRY_SECONDS": c.TenantProjectionRetrySeconds, "TENANT_PROJECTION_BATCH_SIZE": c.TenantProjectionBatchSize,
		"TENANT_PROJECTION_TARGET_LIMIT": c.TenantProjectionTargetLimit, "TENANT_PROJECTION_ACTIVITY_LIMIT": c.TenantProjectionActivityLimit,
		"TENANT_PROJECTION_MAX_ATTEMPTS": c.TenantProjectionMaxAttempts, "AUTH_RATE_LIMIT_WINDOW_SECONDS": c.AuthRateLimitWindowSeconds,
		"AUTH_RATE_LIMIT_MAX_ATTEMPTS": c.AuthRateLimitMaxAttempts, "AUTH_RATE_LIMIT_FAILURE_THRESHOLD": c.AuthRateLimitFailureThreshold,
		"AUTH_RATE_LIMIT_BLOCK_SECONDS": c.AuthRateLimitBlockSeconds, "AUTH_REGISTRATION_MAX_ATTEMPTS": c.AuthRegistrationMaxAttempts,
		"SENSITIVE_RATE_LIMIT_WINDOW_SECONDS": c.SensitiveRateLimitWindowSeconds, "SENSITIVE_RATE_LIMIT_MAX_ATTEMPTS": c.SensitiveRateLimitMaxAttempts,
		"SENSITIVE_RATE_LIMIT_BLOCK_SECONDS": c.SensitiveRateLimitBlockSeconds, "INVITATION_ACCEPT_RATE_LIMIT_MAX_ATTEMPTS": c.InvitationAcceptMaxAttempts,
		"AI_GATEWAY_RATE_LIMIT_WINDOW_SECONDS": c.AIGatewayRateLimitWindowSeconds, "AI_GATEWAY_RATE_LIMIT_MAX_REQUESTS": c.AIGatewayRateLimitMaxRequests,
		"AI_GATEWAY_RATE_LIMIT_BLOCK_SECONDS": c.AIGatewayRateLimitBlockSeconds, "AUDIT_RETENTION_DAYS": c.AuditRetentionDays,
		"AI_GATEWAY_INVOKE_TIMEOUT_SECONDS": c.AIGatewayInvokeTimeoutSeconds, "AI_GATEWAY_STREAM_TIMEOUT_SECONDS": c.AIGatewayStreamTimeoutSeconds,
		"AI_GATEWAY_CHANNEL_FAILURE_THRESHOLD": c.AIGatewayChannelFailureThreshold, "AI_GATEWAY_CHANNEL_COOLDOWN_SECONDS": c.AIGatewayChannelCooldownSeconds,
		"AI_GATEWAY_RESERVATION_STALE_SECONDS": c.AIGatewayReservationStaleSeconds, "AI_GATEWAY_RESERVATION_POLL_SECONDS": c.AIGatewayReservationPollSeconds,
		"AI_GATEWAY_RESERVATION_LEASE_SECONDS": c.AIGatewayReservationLeaseSeconds, "AI_GATEWAY_RESERVATION_BATCH_SIZE": c.AIGatewayReservationBatchSize,
		"AUDIT_RETENTION_POLL_SECONDS": c.AuditRetentionPollSeconds, "AUDIT_RETENTION_BATCH_SIZE": c.AuditRetentionBatchSize,
		"AUDIT_RETENTION_MAX_BATCHES": c.AuditRetentionMaxBatches, "BUSINESS_AI_MAX_TOKENS": c.BusinessAIMaxTokens,
		"MONITORING_AGENT_LOOKBACK_HOURS": c.MonitoringAgentLookbackHours, "MONITORING_AGENT_MAX_SIGNALS_PER_RUN": c.MonitoringAgentMaxSignalsPerRun,
	} {
		errs = append(errs, validatePositive(name, value))
	}
	if c.TenantProvisioningRetryMaxSeconds < c.TenantProvisioningRetrySeconds {
		errs = append(errs, fmt.Errorf("TENANT_PROVISIONING_RETRY_MAX_SECONDS must be greater than or equal to TENANT_PROVISIONING_RETRY_SECONDS"))
	}
	if c.AuthRateLimitFailureThreshold > c.AuthRateLimitMaxAttempts {
		errs = append(errs, fmt.Errorf("AUTH_RATE_LIMIT_FAILURE_THRESHOLD must not exceed AUTH_RATE_LIMIT_MAX_ATTEMPTS"))
	}
	if c.AuditRetentionBatchSize > 5000 {
		errs = append(errs, fmt.Errorf("AUDIT_RETENTION_BATCH_SIZE must not exceed 5000"))
	}
	if c.AuditRetentionMaxBatches > 1000 {
		errs = append(errs, fmt.Errorf("AUDIT_RETENTION_MAX_BATCHES must not exceed 1000"))
	}
	if c.AIGatewayReservationStaleSeconds <= c.AIGatewayStreamTimeoutSeconds {
		errs = append(errs, fmt.Errorf("AI_GATEWAY_RESERVATION_STALE_SECONDS must be greater than AI_GATEWAY_STREAM_TIMEOUT_SECONDS"))
	}
	if c.AIGatewayReservationLeaseSeconds > c.AIGatewayReservationStaleSeconds {
		errs = append(errs, fmt.Errorf("AI_GATEWAY_RESERVATION_LEASE_SECONDS must not exceed AI_GATEWAY_RESERVATION_STALE_SECONDS"))
	}
	if c.AIGatewayReservationBatchSize > 1000 {
		errs = append(errs, fmt.Errorf("AI_GATEWAY_RESERVATION_BATCH_SIZE must not exceed 1000"))
	}
	if c.AIGatewayMaxRetries < 0 || c.AIGatewayMaxRetries > 10 {
		errs = append(errs, fmt.Errorf("AI_GATEWAY_MAX_RETRIES must be between 0 and 10"))
	}
	if _, err := time.Parse("15:04", c.MonitoringAgentDailyTime); err != nil {
		errs = append(errs, fmt.Errorf("MONITORING_AGENT_DAILY_TIME must use 24-hour HH:MM format"))
	}
	return errors.Join(compactErrors(errs)...)
}

func validateEnvironmentSyntax() error {
	var errs []error
	for _, key := range integerEnvironmentKeys {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			if _, err := strconv.Atoi(value); err != nil {
				errs = append(errs, fmt.Errorf("%s must be an integer", key))
			}
		}
	}
	for _, key := range booleanEnvironmentKeys {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "1", "true", "yes", "on", "0", "false", "no", "off":
			default:
				errs = append(errs, fmt.Errorf("%s must be a boolean", key))
			}
		}
	}
	return errors.Join(errs...)
}

func validatePostgresURL(name, value string, requireDatabase bool) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute PostgreSQL URL", name)
	}
	if requireDatabase && strings.Trim(parsed.Path, "/") == "" {
		return fmt.Errorf("%s must include a database name", name)
	}
	return nil
}

func validatePositive(name string, value int) error {
	if value <= 0 {
		return fmt.Errorf("%s must be greater than zero", name)
	}
	return nil
}

func compactErrors(values []error) []error {
	result := make([]error, 0, len(values))
	for _, err := range values {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}
