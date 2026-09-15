-- tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql
-- tenantdb:accept-checksum-drift 002_tenant_projection_outbox.sql

-- Normalize historical Windows checksums, including the expanded ERP baseline.
-- No tenant rows or schema objects are changed by this repair.
SELECT 1;
