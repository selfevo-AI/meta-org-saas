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
