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
