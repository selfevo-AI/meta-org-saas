-- platformdb:accept-checksum-drift 000_saas_platform_management_baseline.sql
-- platformdb:accept-checksum-drift 001_erp_code_baseline.sql
-- platformdb:accept-checksum-drift 002_erp_platform_integration_baseline.sql
-- platformdb:accept-checksum-drift 004_ai_capability_baseline.sql
-- platformdb:accept-checksum-drift 006_saas_manufacturing_module_seed.sql
-- platformdb:accept-checksum-drift 007_saas_runtime_organization_target_repair.sql
-- platformdb:accept-checksum-drift 012_tenant_database_target_state_repair.sql
-- platformdb:accept-checksum-drift 013_tenant_event_projection_infrastructure.sql
-- platformdb:accept-checksum-drift 014_platform_migration_checksum_governance.sql
-- platformdb:accept-checksum-drift 015_authentication_rate_limit_buckets.sql

-- These files previously contained CRLF or mixed line endings on Windows.
-- Their SQL is unchanged; .gitattributes now fixes migration files to LF.
-- The migrator records the one-time checksum reconciliation atomically.
SELECT 1;
