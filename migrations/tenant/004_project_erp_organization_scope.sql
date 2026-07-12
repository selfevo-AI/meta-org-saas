-- Ensure ERP-created projects participate in tenant organization scoping.
-- tenantdb:accept-checksum-drift 001_tenant_business_baseline.sql

CREATE OR REPLACE FUNCTION write_mprj_project_projection()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE v_data JSONB; v_metadata JSONB; v_id UUID; v_organization_id UUID; v_status TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'projects cannot be deleted through the ERP compatibility view'
            USING ERRCODE = '55000', HINT = 'Close or cancel the project through the lifecycle API.';
    END IF;
    v_data := COALESCE(NEW."Payload", '{}'::JSONB)
        || CASE WHEN JSONB_TYPEOF(NEW."Payload"->'Payload') = 'object' THEN NEW."Payload"->'Payload' ELSE '{}'::JSONB END;
    v_metadata := v_data - ARRAY['PrjCode','ProjectID','ProjectCode','Name','Description','Status','Priority','RiskLevel','RequiredLevel','BudgetAmount','BudgetCurrency','Active','Locked','DataSource','OrganizationID','Payload'];
    v_status := CASE WHEN v_data->>'Status' IN ('planning','active','paused','delivering','completed','closed','cancelled') THEN v_data->>'Status'
                     WHEN v_data->>'Active' = 'N' THEN 'paused' ELSE 'active' END;
    v_organization_id := erp_try_uuid(v_data->>'OrganizationID');
    IF v_organization_id IS NULL OR NOT EXISTS (SELECT 1 FROM organizations WHERE id = v_organization_id) THEN
        SELECT id INTO v_organization_id FROM organizations WHERE status = 'active' ORDER BY created_at LIMIT 1;
    END IF;
    IF TG_OP = 'INSERT' THEN
        v_id := COALESCE(erp_try_uuid(NEW."PrjCode"), GEN_RANDOM_UUID());
        INSERT INTO projects(id, organization_id, name, description, status, priority, risk_level, required_level, budget_amount, budget_currency, metadata)
        VALUES (v_id, v_organization_id, COALESCE(NULLIF(v_data->>'Name', ''), NEW."PrjCode"), COALESCE(v_data->>'Description', ''), v_status,
            CASE WHEN v_data->>'Priority' IN ('low','medium','high','critical') THEN v_data->>'Priority' ELSE 'medium' END,
            CASE WHEN v_data->>'RiskLevel' IN ('low','medium','high','critical') THEN v_data->>'RiskLevel' ELSE 'low' END,
            CASE WHEN v_data->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN v_data->>'RequiredLevel' ELSE 'L1' END,
            COALESCE(erp_try_numeric(v_data->>'BudgetAmount'), 0), COALESCE(NULLIF(v_data->>'BudgetCurrency', ''), 'CNY'),
            v_metadata || JSONB_BUILD_OBJECT('erp_project_code', NEW."PrjCode"));
        NEW."CreatedAt" := NOW(); NEW."UpdatedAt" := NEW."CreatedAt"; RETURN NEW;
    END IF;
    UPDATE projects p SET organization_id = COALESCE(p.organization_id, v_organization_id),
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

UPDATE projects p
SET organization_id = tenant_org.id, updated_at = NOW()
FROM LATERAL (
    SELECT id FROM organizations WHERE status = 'active' ORDER BY created_at LIMIT 1
) tenant_org
WHERE p.organization_id IS NULL AND NULLIF(p.metadata->>'erp_project_code', '') IS NOT NULL;
