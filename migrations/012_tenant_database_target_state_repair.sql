BEGIN;

CREATE OR REPLACE FUNCTION platform.guard_tenant_database_target_state()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = 'provisioned'
       AND NEW.status = 'provisioning'
       AND OLD.deployment_mode = NEW.deployment_mode
       AND OLD.cluster_key = NEW.cluster_key
       AND OLD.region = NEW.region
       AND OLD.database_name IS NOT DISTINCT FROM NEW.database_name
       AND OLD.schema_name IS NOT DISTINCT FROM NEW.schema_name THEN
        NEW.status := 'provisioned';
        NEW.last_provisioned_at := OLD.last_provisioned_at;
        IF NEW.connection_secret_ref = '' THEN
            NEW.connection_secret_ref := OLD.connection_secret_ref;
        END IF;
        IF NEW.migration_version = '' THEN
            NEW.migration_version := OLD.migration_version;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_guard_tenant_database_target_state ON platform.tenant_database_targets;
CREATE TRIGGER trg_guard_tenant_database_target_state
BEFORE UPDATE ON platform.tenant_database_targets
FOR EACH ROW
EXECUTE FUNCTION platform.guard_tenant_database_target_state();

UPDATE platform.tenant_database_targets AS target
SET status = 'provisioned',
    migration_version = COALESCE(
        NULLIF(target.migration_version, ''),
        (
            SELECT migration.migration_key
            FROM platform.tenant_database_migrations AS migration
            WHERE migration.tenant_database_id = target.id
              AND migration.status IN ('applied', 'skipped')
            ORDER BY migration.applied_at DESC NULLS LAST, migration.updated_at DESC
            LIMIT 1
        ),
        ''
    ),
    last_provisioned_at = COALESCE(
        target.last_provisioned_at,
        (
            SELECT MAX(job.completed_at)
            FROM platform.tenant_database_provisioning_jobs AS job
            WHERE job.tenant_database_id = target.id
              AND job.status = 'succeeded'
        ),
        NOW()
    ),
    metadata = target.metadata || jsonb_build_object(
        'state_repaired_by', '012_tenant_database_target_state_repair',
        'state_repaired_at', NOW()
    ),
    updated_at = NOW()
WHERE target.deployment_mode = 'dedicated_database'
  AND target.status IN ('provisioning', 'failed')
  AND EXISTS (SELECT 1 FROM pg_database WHERE datname = target.database_name)
  AND EXISTS (
      SELECT 1
      FROM platform.tenant_database_provisioning_jobs AS job
      WHERE job.tenant_database_id = target.id
        AND job.status = 'succeeded'
  );

COMMIT;
