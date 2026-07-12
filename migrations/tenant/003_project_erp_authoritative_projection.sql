-- Repair existing tenant databases so project lifecycle tables become the
-- single source of truth behind the ERP MPRJ/APRJ contract.
-- tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql

CREATE OR REPLACE FUNCTION erp_try_uuid(p_value TEXT)
RETURNS UUID LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN
    RETURN NULLIF(BTRIM(p_value), '')::UUID;
EXCEPTION WHEN invalid_text_representation THEN RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION erp_try_numeric(p_value TEXT)
RETURNS NUMERIC LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN
    RETURN NULLIF(BTRIM(p_value), '')::NUMERIC;
EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range THEN RETURN NULL;
END;
$$;

DO $$
DECLARE v_kind "char";
BEGIN
    SELECT relkind INTO v_kind FROM pg_class WHERE oid = TO_REGCLASS('"MPRJ"');
    IF v_kind = 'r' AND TO_REGCLASS('"MPRJ_legacy"') IS NULL THEN ALTER TABLE "MPRJ" RENAME TO "MPRJ_legacy"; END IF;
    SELECT relkind INTO v_kind FROM pg_class WHERE oid = TO_REGCLASS('"APRJ"');
    IF v_kind = 'r' AND TO_REGCLASS('"APRJ_legacy"') IS NULL THEN ALTER TABLE "APRJ" RENAME TO "APRJ_legacy"; END IF;
END;
$$;

DO $$
BEGIN
    IF TO_REGCLASS('"MPRJ_legacy"') IS NOT NULL THEN
        INSERT INTO projects(id, name, description, status, priority, risk_level, required_level, budget_amount, budget_currency, metadata, created_at, updated_at)
        SELECT COALESCE(erp_try_uuid(m."PrjCode"), GEN_RANDOM_UUID()),
               COALESCE(NULLIF(m."Payload"->>'Name', ''), NULLIF(m."Payload"->'Payload'->>'Name', ''), m."PrjCode"),
               COALESCE(m."Payload"->>'Description', m."Payload"->'Payload'->>'Description', ''),
               CASE WHEN COALESCE(m."Payload"->>'Status', m."Payload"->'Payload'->>'Status', '') IN ('planning','active','paused','delivering','completed','closed','cancelled')
                    THEN COALESCE(m."Payload"->>'Status', m."Payload"->'Payload'->>'Status')
                    WHEN m."Payload"->>'Active' = 'N' THEN 'paused' ELSE 'active' END,
               CASE WHEN m."Payload"->>'Priority' IN ('low','medium','high','critical') THEN m."Payload"->>'Priority' ELSE 'medium' END,
               CASE WHEN m."Payload"->>'RiskLevel' IN ('low','medium','high','critical') THEN m."Payload"->>'RiskLevel' ELSE 'low' END,
               CASE WHEN m."Payload"->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN m."Payload"->>'RequiredLevel' ELSE 'L1' END,
               COALESCE(erp_try_numeric(m."Payload"->>'BudgetAmount'), 0), COALESCE(NULLIF(m."Payload"->>'BudgetCurrency', ''), 'CNY'),
               (m."Payload" - ARRAY['Payload','Name','Description','Status','Priority','RiskLevel','RequiredLevel','BudgetAmount','BudgetCurrency'])
                   || COALESCE(m."Payload"->'Payload', '{}'::JSONB)
                   || JSONB_BUILD_OBJECT('erp_project_code', m."PrjCode", 'legacy_erp_imported', true),
               m."CreatedAt", m."UpdatedAt"
        FROM "MPRJ_legacy" m
        WHERE NOT EXISTS (SELECT 1 FROM projects p WHERE p.id = erp_try_uuid(m."PrjCode") OR p.metadata->>'erp_project_code' = m."PrjCode")
        ON CONFLICT (id) DO NOTHING;
    END IF;
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_erp_project_code ON projects((metadata->>'erp_project_code'))
WHERE NULLIF(metadata->>'erp_project_code', '') IS NOT NULL;

CREATE OR REPLACE VIEW "MPRJ" AS
SELECT COALESCE(NULLIF(p.metadata->>'erp_project_code', ''), p.id::TEXT) AS "PrjCode",
       (p.metadata - 'erp_project_code') || JSONB_BUILD_OBJECT(
           'PrjCode', COALESCE(NULLIF(p.metadata->>'erp_project_code', ''), p.id::TEXT), 'ProjectID', p.id::TEXT,
           'ProjectCode', COALESCE(NULLIF(p.master_key, ''), p.id::TEXT), 'Name', p.name, 'Description', p.description,
           'Status', p.status, 'Priority', p.priority, 'RiskLevel', p.risk_level, 'RequiredLevel', p.required_level,
           'BudgetAmount', p.budget_amount, 'BudgetCurrency', p.budget_currency,
           'Active', CASE WHEN p.status IN ('completed','closed','cancelled') THEN 'N' ELSE 'Y' END,
           'Locked', CASE WHEN p.status IN ('closed','cancelled') THEN 'Y' ELSE 'N' END, 'DataSource', 'P') AS "Payload",
       p.created_at AS "CreatedAt", p.updated_at AS "UpdatedAt"
FROM projects p;

CREATE OR REPLACE FUNCTION write_mprj_project_projection()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE v_data JSONB; v_metadata JSONB; v_id UUID; v_status TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'projects cannot be deleted through the ERP compatibility view'
            USING ERRCODE = '55000', HINT = 'Close or cancel the project through the lifecycle API.';
    END IF;
    v_data := COALESCE(NEW."Payload", '{}'::JSONB)
        || CASE WHEN JSONB_TYPEOF(NEW."Payload"->'Payload') = 'object' THEN NEW."Payload"->'Payload' ELSE '{}'::JSONB END;
    v_metadata := v_data - ARRAY['PrjCode','ProjectID','ProjectCode','Name','Description','Status','Priority','RiskLevel','RequiredLevel','BudgetAmount','BudgetCurrency','Active','Locked','DataSource','Payload'];
    v_status := CASE WHEN v_data->>'Status' IN ('planning','active','paused','delivering','completed','closed','cancelled') THEN v_data->>'Status'
                     WHEN v_data->>'Active' = 'N' THEN 'paused' ELSE 'active' END;
    IF TG_OP = 'INSERT' THEN
        v_id := COALESCE(erp_try_uuid(NEW."PrjCode"), GEN_RANDOM_UUID());
        INSERT INTO projects(id, name, description, status, priority, risk_level, required_level, budget_amount, budget_currency, metadata)
        VALUES (v_id, COALESCE(NULLIF(v_data->>'Name', ''), NEW."PrjCode"), COALESCE(v_data->>'Description', ''), v_status,
            CASE WHEN v_data->>'Priority' IN ('low','medium','high','critical') THEN v_data->>'Priority' ELSE 'medium' END,
            CASE WHEN v_data->>'RiskLevel' IN ('low','medium','high','critical') THEN v_data->>'RiskLevel' ELSE 'low' END,
            CASE WHEN v_data->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN v_data->>'RequiredLevel' ELSE 'L1' END,
            COALESCE(erp_try_numeric(v_data->>'BudgetAmount'), 0), COALESCE(NULLIF(v_data->>'BudgetCurrency', ''), 'CNY'),
            v_metadata || JSONB_BUILD_OBJECT('erp_project_code', NEW."PrjCode"));
        NEW."CreatedAt" := NOW(); NEW."UpdatedAt" := NEW."CreatedAt"; RETURN NEW;
    END IF;
    UPDATE projects p SET
        name = COALESCE(NULLIF(v_data->>'Name', ''), p.name), description = COALESCE(v_data->>'Description', p.description), status = v_status,
        priority = CASE WHEN v_data->>'Priority' IN ('low','medium','high','critical') THEN v_data->>'Priority' ELSE p.priority END,
        risk_level = CASE WHEN v_data->>'RiskLevel' IN ('low','medium','high','critical') THEN v_data->>'RiskLevel' ELSE p.risk_level END,
        required_level = CASE WHEN v_data->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN v_data->>'RequiredLevel' ELSE p.required_level END,
        budget_amount = COALESCE(erp_try_numeric(v_data->>'BudgetAmount'), p.budget_amount),
        budget_currency = COALESCE(NULLIF(v_data->>'BudgetCurrency', ''), p.budget_currency),
        metadata = p.metadata || v_metadata || JSONB_BUILD_OBJECT('erp_project_code', OLD."PrjCode"), updated_at = NOW()
    WHERE COALESCE(NULLIF(p.metadata->>'erp_project_code', ''), p.id::TEXT) = OLD."PrjCode" RETURNING p.id INTO v_id;
    IF v_id IS NULL THEN RAISE EXCEPTION 'project % was not found', OLD."PrjCode" USING ERRCODE = 'P0002'; END IF;
    NEW."UpdatedAt" := NOW(); RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_write_mprj_project_projection ON "MPRJ";
CREATE TRIGGER trg_write_mprj_project_projection INSTEAD OF INSERT OR UPDATE OR DELETE ON "MPRJ"
FOR EACH ROW EXECUTE FUNCTION write_mprj_project_projection();

CREATE OR REPLACE VIEW "APRJ" AS
SELECT COALESCE(NULLIF(p.metadata->>'erp_project_code', ''), p.id::TEXT) AS "PrjCode",
       ROW_NUMBER() OVER (PARTITION BY pm.project_id ORDER BY pm.created_at, pm.id)::BIGINT AS "LineNum",
       CASE WHEN pm.status = 'active' THEN 'O' ELSE 'C' END::VARCHAR(1) AS "LineStatus",
       JSONB_BUILD_OBJECT('ProjectMemberID', pm.id::TEXT, 'ActorID', pm.actor_id::TEXT, 'ActorType', pm.actor_type,
           'Role', pm.role, 'Title', pm.title, 'AllocationPercent', pm.allocation_percent, 'CostRate', pm.cost_rate,
           'PermissionLevel', pm.permission_level, 'Capabilities', pm.capabilities, 'Status', pm.status) || pm.metadata AS "Payload",
       pm.created_at AS "CreatedAt", pm.updated_at AS "UpdatedAt"
FROM project_members pm JOIN projects p ON p.id = pm.project_id;
