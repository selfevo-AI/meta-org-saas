-- Unify ERP requirements with the authoritative requirement lifecycle domain.
-- platformdb:accept-checksum-drift 001_erp_code_baseline.sql

DO $$
DECLARE v_kind "char";
BEGIN
    SELECT relkind INTO v_kind FROM pg_class WHERE oid = TO_REGCLASS('"MREQ"');
    IF v_kind = 'r' AND TO_REGCLASS('"MREQ_legacy"') IS NULL THEN ALTER TABLE "MREQ" RENAME TO "MREQ_legacy"; END IF;
    SELECT relkind INTO v_kind FROM pg_class WHERE oid = TO_REGCLASS('"REQ1"');
    IF v_kind = 'r' AND TO_REGCLASS('"REQ1_legacy"') IS NULL THEN ALTER TABLE "REQ1" RENAME TO "REQ1_legacy"; END IF;
END;
$$;

DO $$
BEGIN
    IF TO_REGCLASS('"MREQ_legacy"') IS NOT NULL THEN
        INSERT INTO requirements(id, title, description, source, status, priority, risk_level, required_level,
            organization_id, budget_amount, budget_currency, analysis, metadata, created_at, updated_at)
        SELECT COALESCE(erp_try_uuid(m."ReqCode"), GEN_RANDOM_UUID()),
               COALESCE(NULLIF(m."Payload"->>'Title', ''), NULLIF(m."Payload"->>'Name', ''), NULLIF(m."Payload"->'Payload'->>'Name', ''), m."ReqCode"),
               COALESCE(m."Payload"->>'Description', m."Payload"->'Payload'->>'Description', ''), COALESCE(NULLIF(m."Payload"->>'Source', ''), 'erp'),
               CASE WHEN COALESCE(m."Payload"->>'Status', m."Payload"->'Payload'->>'Status', '') IN ('draft','analyzed','approved','converted','rejected','archived')
                    THEN COALESCE(m."Payload"->>'Status', m."Payload"->'Payload'->>'Status') ELSE 'draft' END,
               CASE WHEN m."Payload"->>'Priority' IN ('low','medium','high','critical') THEN m."Payload"->>'Priority' ELSE 'medium' END,
               CASE WHEN m."Payload"->>'RiskLevel' IN ('low','medium','high','critical') THEN m."Payload"->>'RiskLevel' ELSE 'low' END,
               CASE WHEN m."Payload"->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN m."Payload"->>'RequiredLevel' ELSE 'L1' END,
               (SELECT id FROM organizations WHERE status = 'active' ORDER BY created_at LIMIT 1),
               COALESCE(erp_try_numeric(m."Payload"->>'BudgetAmount'), 0), COALESCE(NULLIF(m."Payload"->>'BudgetCurrency', ''), 'CNY'),
               CASE WHEN JSONB_TYPEOF(m."Payload"->'Analysis') = 'object' THEN m."Payload"->'Analysis' ELSE '{}'::JSONB END,
               (m."Payload" - ARRAY['Payload','Title','Name','Description','Source','Status','Priority','RiskLevel','RequiredLevel','BudgetAmount','BudgetCurrency','Analysis'])
                   || COALESCE(m."Payload"->'Payload', '{}'::JSONB)
                   || JSONB_BUILD_OBJECT('erp_requirement_code', m."ReqCode", 'legacy_erp_imported', true),
               m."CreatedAt", m."UpdatedAt"
        FROM "MREQ_legacy" m
        WHERE NOT EXISTS (SELECT 1 FROM requirements r WHERE r.id = erp_try_uuid(m."ReqCode") OR r.metadata->>'erp_requirement_code' = m."ReqCode")
        ON CONFLICT (id) DO NOTHING;
    END IF;
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_requirements_erp_requirement_code ON requirements((metadata->>'erp_requirement_code'))
WHERE NULLIF(metadata->>'erp_requirement_code', '') IS NOT NULL;

CREATE OR REPLACE VIEW "MREQ" AS
SELECT COALESCE(NULLIF(r.metadata->>'erp_requirement_code', ''), r.id::TEXT) AS "ReqCode",
       (r.metadata - 'erp_requirement_code') || JSONB_BUILD_OBJECT(
           'ReqCode', COALESCE(NULLIF(r.metadata->>'erp_requirement_code', ''), r.id::TEXT), 'RequirementID', r.id::TEXT,
           'RequirementCode', r.id::TEXT, 'Name', r.title, 'Title', r.title, 'Description', r.description, 'Source', r.source,
           'Status', r.status, 'Priority', r.priority, 'RiskLevel', r.risk_level, 'RequiredLevel', r.required_level,
           'BudgetAmount', r.budget_amount, 'BudgetCurrency', r.budget_currency, 'Analysis', r.analysis) AS "Payload",
       r.created_at AS "CreatedAt", r.updated_at AS "UpdatedAt"
FROM requirements r;

CREATE OR REPLACE FUNCTION write_mreq_requirement_projection()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE v_data JSONB; v_metadata JSONB; v_id UUID; v_organization_id UUID; v_status TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'requirements cannot be deleted through the ERP compatibility view'
            USING ERRCODE = '55000', HINT = 'Archive the requirement through the lifecycle API.';
    END IF;
    v_data := COALESCE(NEW."Payload", '{}'::JSONB)
        || CASE WHEN JSONB_TYPEOF(NEW."Payload"->'Payload') = 'object' THEN NEW."Payload"->'Payload' ELSE '{}'::JSONB END;
    v_metadata := v_data - ARRAY['ReqCode','RequirementID','RequirementCode','Name','Title','Description','Source','Status','Priority','RiskLevel','RequiredLevel','BudgetAmount','BudgetCurrency','Analysis','OrganizationID','Payload'];
    v_status := CASE WHEN v_data->>'Status' IN ('draft','analyzed','approved','converted','rejected','archived') THEN v_data->>'Status' ELSE 'draft' END;
    v_organization_id := erp_try_uuid(v_data->>'OrganizationID');
    IF v_organization_id IS NULL OR NOT EXISTS (SELECT 1 FROM organizations WHERE id = v_organization_id) THEN
        SELECT id INTO v_organization_id FROM organizations WHERE status = 'active' ORDER BY created_at LIMIT 1;
    END IF;
    IF TG_OP = 'INSERT' THEN
        v_id := COALESCE(erp_try_uuid(NEW."ReqCode"), GEN_RANDOM_UUID());
        INSERT INTO requirements(id, organization_id, title, description, source, status, priority, risk_level, required_level,
            budget_amount, budget_currency, analysis, metadata)
        VALUES (v_id, v_organization_id, COALESCE(NULLIF(v_data->>'Title', ''), NULLIF(v_data->>'Name', ''), NEW."ReqCode"),
            COALESCE(v_data->>'Description', ''), COALESCE(NULLIF(v_data->>'Source', ''), 'erp'), v_status,
            CASE WHEN v_data->>'Priority' IN ('low','medium','high','critical') THEN v_data->>'Priority' ELSE 'medium' END,
            CASE WHEN v_data->>'RiskLevel' IN ('low','medium','high','critical') THEN v_data->>'RiskLevel' ELSE 'low' END,
            CASE WHEN v_data->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN v_data->>'RequiredLevel' ELSE 'L1' END,
            COALESCE(erp_try_numeric(v_data->>'BudgetAmount'), 0), COALESCE(NULLIF(v_data->>'BudgetCurrency', ''), 'CNY'),
            CASE WHEN JSONB_TYPEOF(v_data->'Analysis') = 'object' THEN v_data->'Analysis' ELSE '{}'::JSONB END,
            v_metadata || JSONB_BUILD_OBJECT('erp_requirement_code', NEW."ReqCode"));
        NEW."CreatedAt" := NOW(); NEW."UpdatedAt" := NEW."CreatedAt"; RETURN NEW;
    END IF;
    UPDATE requirements r SET organization_id = COALESCE(r.organization_id, v_organization_id),
        title = COALESCE(NULLIF(v_data->>'Title', ''), NULLIF(v_data->>'Name', ''), r.title),
        description = COALESCE(v_data->>'Description', r.description), source = COALESCE(NULLIF(v_data->>'Source', ''), r.source), status = v_status,
        priority = CASE WHEN v_data->>'Priority' IN ('low','medium','high','critical') THEN v_data->>'Priority' ELSE r.priority END,
        risk_level = CASE WHEN v_data->>'RiskLevel' IN ('low','medium','high','critical') THEN v_data->>'RiskLevel' ELSE r.risk_level END,
        required_level = CASE WHEN v_data->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN v_data->>'RequiredLevel' ELSE r.required_level END,
        budget_amount = COALESCE(erp_try_numeric(v_data->>'BudgetAmount'), r.budget_amount), budget_currency = COALESCE(NULLIF(v_data->>'BudgetCurrency', ''), r.budget_currency),
        analysis = CASE WHEN JSONB_TYPEOF(v_data->'Analysis') = 'object' THEN v_data->'Analysis' ELSE r.analysis END,
        metadata = r.metadata || v_metadata || JSONB_BUILD_OBJECT('erp_requirement_code', OLD."ReqCode"), updated_at = NOW()
    WHERE COALESCE(NULLIF(r.metadata->>'erp_requirement_code', ''), r.id::TEXT) = OLD."ReqCode" RETURNING r.id INTO v_id;
    IF v_id IS NULL THEN RAISE EXCEPTION 'requirement % was not found', OLD."ReqCode" USING ERRCODE = 'P0002'; END IF;
    NEW."UpdatedAt" := NOW(); RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_write_mreq_requirement_projection ON "MREQ";
CREATE TRIGGER trg_write_mreq_requirement_projection INSTEAD OF INSERT OR UPDATE OR DELETE ON "MREQ"
FOR EACH ROW EXECUTE FUNCTION write_mreq_requirement_projection();

CREATE OR REPLACE VIEW "REQ1" AS
SELECT COALESCE(NULLIF(r.metadata->>'erp_requirement_code', ''), r.id::TEXT) AS "ReqCode",
       ROW_NUMBER() OVER (PARTITION BY d.requirement_id ORDER BY d.created_at, d.id)::BIGINT AS "LineNum", 'O'::VARCHAR(1) AS "LineStatus",
       JSONB_BUILD_OBJECT('RequirementDocumentID', d.id::TEXT, 'FileName', d.file_name, 'ContentType', d.content_type,
           'SizeBytes', d.size_bytes, 'UploadedByID', d.uploaded_by_id, 'UploadedByType', d.uploaded_by_type) || d.metadata AS "Payload",
       d.created_at AS "CreatedAt", d.created_at AS "UpdatedAt"
FROM requirement_documents d JOIN requirements r ON r.id = d.requirement_id;

CREATE OR REPLACE FUNCTION write_mprj_project_projection()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE v_data JSONB; v_metadata JSONB; v_id UUID; v_organization_id UUID; v_requirement_id UUID; v_status TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'projects cannot be deleted through the ERP compatibility view'
        USING ERRCODE = '55000', HINT = 'Close or cancel the project through the lifecycle API.'; END IF;
    v_data := COALESCE(NEW."Payload", '{}'::JSONB)
        || CASE WHEN JSONB_TYPEOF(NEW."Payload"->'Payload') = 'object' THEN NEW."Payload"->'Payload' ELSE '{}'::JSONB END;
    v_metadata := v_data - ARRAY['PrjCode','ProjectID','ProjectCode','Name','Description','Status','Priority','RiskLevel','RequiredLevel','BudgetAmount','BudgetCurrency','Active','Locked','DataSource','OrganizationID','RequirementID','RequirementCode','Payload'];
    v_status := CASE WHEN v_data->>'Status' IN ('planning','active','paused','delivering','completed','closed','cancelled') THEN v_data->>'Status'
                     WHEN v_data->>'Active' = 'N' THEN 'paused' ELSE 'active' END;
    v_organization_id := erp_try_uuid(v_data->>'OrganizationID');
    IF v_organization_id IS NULL OR NOT EXISTS (SELECT 1 FROM organizations WHERE id = v_organization_id) THEN
        SELECT id INTO v_organization_id FROM organizations WHERE status = 'active' ORDER BY created_at LIMIT 1;
    END IF;
    v_requirement_id := COALESCE(erp_try_uuid(v_data->>'RequirementID'), erp_try_uuid(v_data->>'RequirementCode'));
    IF v_requirement_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM requirements WHERE id = v_requirement_id) THEN v_requirement_id := NULL; END IF;
    IF TG_OP = 'INSERT' THEN
        v_id := COALESCE(erp_try_uuid(NEW."PrjCode"), GEN_RANDOM_UUID());
        INSERT INTO projects(id, requirement_id, organization_id, name, description, status, priority, risk_level, required_level, budget_amount, budget_currency, metadata)
        VALUES (v_id, v_requirement_id, v_organization_id, COALESCE(NULLIF(v_data->>'Name', ''), NEW."PrjCode"), COALESCE(v_data->>'Description', ''), v_status,
            CASE WHEN v_data->>'Priority' IN ('low','medium','high','critical') THEN v_data->>'Priority' ELSE 'medium' END,
            CASE WHEN v_data->>'RiskLevel' IN ('low','medium','high','critical') THEN v_data->>'RiskLevel' ELSE 'low' END,
            CASE WHEN v_data->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN v_data->>'RequiredLevel' ELSE 'L1' END,
            COALESCE(erp_try_numeric(v_data->>'BudgetAmount'), 0), COALESCE(NULLIF(v_data->>'BudgetCurrency', ''), 'CNY'),
            v_metadata || JSONB_BUILD_OBJECT('erp_project_code', NEW."PrjCode"));
        NEW."CreatedAt" := NOW(); NEW."UpdatedAt" := NEW."CreatedAt"; RETURN NEW;
    END IF;
    UPDATE projects p SET requirement_id = COALESCE(p.requirement_id, v_requirement_id), organization_id = COALESCE(p.organization_id, v_organization_id),
        name = COALESCE(NULLIF(v_data->>'Name', ''), p.name), description = COALESCE(v_data->>'Description', p.description), status = v_status,
        priority = CASE WHEN v_data->>'Priority' IN ('low','medium','high','critical') THEN v_data->>'Priority' ELSE p.priority END,
        risk_level = CASE WHEN v_data->>'RiskLevel' IN ('low','medium','high','critical') THEN v_data->>'RiskLevel' ELSE p.risk_level END,
        required_level = CASE WHEN v_data->>'RequiredLevel' IN ('L1','L2','L3','L4') THEN v_data->>'RequiredLevel' ELSE p.required_level END,
        budget_amount = COALESCE(erp_try_numeric(v_data->>'BudgetAmount'), p.budget_amount), budget_currency = COALESCE(NULLIF(v_data->>'BudgetCurrency', ''), p.budget_currency),
        metadata = p.metadata || v_metadata || JSONB_BUILD_OBJECT('erp_project_code', OLD."PrjCode"), updated_at = NOW()
    WHERE COALESCE(NULLIF(p.metadata->>'erp_project_code', ''), p.id::TEXT) = OLD."PrjCode" RETURNING p.id INTO v_id;
    IF v_id IS NULL THEN RAISE EXCEPTION 'project % was not found', OLD."PrjCode" USING ERRCODE = 'P0002'; END IF;
    NEW."UpdatedAt" := NOW(); RETURN NEW;
END;
$$;

UPDATE projects p SET requirement_id = r.id, updated_at = NOW()
FROM requirements r
WHERE p.requirement_id IS NULL
  AND r.id = COALESCE(erp_try_uuid(p.metadata->>'RequirementID'), erp_try_uuid(p.metadata->>'RequirementCode'));
