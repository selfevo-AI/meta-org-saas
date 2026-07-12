BEGIN;

CREATE OR REPLACE FUNCTION ensure_source_master_key(p_source_table TEXT, p_key_prefix TEXT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_has_uuid_id BOOLEAN;
BEGIN
    IF NOT module_table_exists(p_source_table) THEN
        RETURN;
    END IF;

    INSERT INTO data_table_catalog(table_name, master_table_name, detail_table_name, key_prefix, display_name, category, is_base_data, is_business_scenario, metadata)
    VALUES (
        p_source_table,
        p_source_table || '_masters',
        p_source_table || '_details',
        p_key_prefix,
        p_source_table,
        'legacy',
        false,
        false,
        jsonb_build_object('deprecated', true, 'canonicalized_by', '011_ai_module_master_detail_runtime_repair.sql')
    )
    ON CONFLICT (table_name) DO UPDATE SET
        key_prefix = COALESCE(NULLIF(data_table_catalog.key_prefix, ''), EXCLUDED.key_prefix),
        category = 'legacy',
        metadata = data_table_catalog.metadata || EXCLUDED.metadata,
        updated_at = NOW();

    SELECT module_column_exists(p_source_table, 'id') AND module_column_udt(p_source_table, 'id') = 'uuid'
    INTO v_has_uuid_id;

    EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS legacy_id UUID', p_source_table);
    IF v_has_uuid_id THEN
        EXECUTE FORMAT('UPDATE %I SET legacy_id = id WHERE legacy_id IS NULL AND id IS NOT NULL', p_source_table);
    END IF;

    EXECUTE FORMAT('ALTER TABLE %I ADD COLUMN IF NOT EXISTS master_key TEXT', p_source_table);
    EXECUTE FORMAT(
        'UPDATE %I SET master_key = next_business_key(%L, %L) WHERE master_key IS NULL',
        p_source_table,
        p_source_table,
        p_key_prefix
    );
    EXECUTE FORMAT(
        'ALTER TABLE %I ALTER COLUMN master_key SET DEFAULT next_business_key(%L, %L)',
        p_source_table,
        p_source_table,
        p_key_prefix
    );
    EXECUTE FORMAT('ALTER TABLE %I ALTER COLUMN master_key SET NOT NULL', p_source_table);
    EXECUTE FORMAT('CREATE UNIQUE INDEX IF NOT EXISTS %I ON %I(master_key)', 'uq_' || p_source_table || '_master_key', p_source_table);
END;
$$;

DELETE FROM module_master_source_catalog
WHERE source_table = 'skill_details';

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT module_name, MIN(key_prefix) AS key_prefix
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
        GROUP BY module_name
        ORDER BY module_name
    LOOP
        PERFORM ensure_module_master_detail_tables(rec.module_name, rec.key_prefix);
    END LOOP;

    FOR rec IN
        SELECT source_table, key_prefix
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
          AND source_table <> 'skill_details'
          AND module_table_exists(source_table)
        ORDER BY source_table
    LOOP
        PERFORM ensure_source_master_key(rec.source_table, rec.key_prefix);
    END LOOP;
END;
$$;

DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN
        SELECT source_table
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
          AND source_table <> 'skill_details'
          AND module_table_exists(source_table)
        ORDER BY relation_mode, source_table
    LOOP
        PERFORM refresh_module_source(rec.source_table);
    END LOOP;
END;
$$;

DO $$
DECLARE
    rec RECORD;
    v_trigger_name TEXT;
BEGIN
    FOR rec IN
        SELECT source_table
        FROM module_master_source_catalog
        WHERE module_name IN ('identity', 'aigateway', 'toolruntime', 'assistant')
          AND source_table <> 'skill_details'
          AND module_table_exists(source_table)
        ORDER BY source_table
    LOOP
        v_trigger_name := 'trg_refresh_' || rec.source_table || '_module_master';
        EXECUTE FORMAT('DROP TRIGGER IF EXISTS %I ON %I', v_trigger_name, rec.source_table);
        EXECUTE FORMAT(
            'CREATE TRIGGER %I
             AFTER INSERT OR UPDATE OR DELETE ON %I
             FOR EACH STATEMENT EXECUTE FUNCTION refresh_module_source_trigger()',
            v_trigger_name,
            rec.source_table
        );
    END LOOP;
END;
$$;

COMMIT;
