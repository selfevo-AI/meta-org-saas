package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ServerPort                      int
	DatabaseURL                     string
	JWTSecret                       string
	ModelSecretKey                  string
	CorsOrigins                     []string
	MigrationsPath                  string
	MetaOrgMode                     string
	MetaOrgDistributionMode         string
	MetaOrgLicenseMode              string
	PlatformAdminEmail              string
	PlatformAdminPasswordHash       string
	SecurityKernelURL               string
	SecurityKernelSharedSecret      string
	SecurityKernelEnforcementMode   string
	MonitoringAgentSchedulerEnabled bool
	MonitoringAgentDailyTime        string
	MonitoringAgentLookbackHours    int
	MonitoringAgentMaxSignalsPerRun int
}

func Load() *Config {
	mode := strings.ToLower(strings.TrimSpace(getEnv("META_ORG_MODE", "single_org")))
	if mode != "saas" {
		mode = "single_org"
	}
	return &Config{
		ServerPort:                      getEnvInt("SERVER_PORT", 8080),
		DatabaseURL:                     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meta_org?sslmode=disable"),
		JWTSecret:                       getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		ModelSecretKey:                  getEnv("MODEL_SECRET_KEY", "dev-model-secret-key-32-bytes!!!"),
		CorsOrigins:                     getEnvSlice("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"),
		MigrationsPath:                  getEnv("MIGRATIONS_PATH", "migrations"),
		MetaOrgMode:                     mode,
		MetaOrgDistributionMode:         normalizedDistributionMode(getEnv("META_ORG_DISTRIBUTION_MODE", mode)),
		MetaOrgLicenseMode:              normalizedLicenseMode(getEnv("META_ORG_LICENSE_MODE", "commercial")),
		PlatformAdminEmail:              strings.ToLower(strings.TrimSpace(getEnv("META_ORG_PLATFORM_ADMIN_EMAIL", ""))),
		PlatformAdminPasswordHash:       getEnv("META_ORG_PLATFORM_ADMIN_PASSWORD_HASH", ""),
		SecurityKernelURL:               strings.TrimSpace(getEnv("SECURITY_KERNEL_URL", "")),
		SecurityKernelSharedSecret:      getEnv("SECURITY_KERNEL_SHARED_SECRET", ""),
		SecurityKernelEnforcementMode:   normalizedEnforcementMode(getEnv("SECURITY_KERNEL_ENFORCEMENT_MODE", "blocking")),
		MonitoringAgentSchedulerEnabled: getEnvBool("MONITORING_AGENT_SCHEDULER_ENABLED", false),
		MonitoringAgentDailyTime:        strings.TrimSpace(getEnv("MONITORING_AGENT_DAILY_TIME", "02:00")),
		MonitoringAgentLookbackHours:    getEnvInt("MONITORING_AGENT_LOOKBACK_HOURS", 24),
		MonitoringAgentMaxSignalsPerRun: getEnvInt("MONITORING_AGENT_MAX_SIGNALS_PER_RUN", 100),
	}
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
