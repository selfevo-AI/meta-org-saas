package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkRecord struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Status    string         `json:"status"`
	CreatedAt string         `json:"created_at"`
	Data      map[string]any `json:"data,omitempty"`
}

type WorkRecordContext struct {
	ModuleKey string
	Records   []WorkRecord
	Error     string
}

type ContextResolver interface {
	Resolve(context.Context, *Session) WorkRecordContext
}

var erpContextTargetTypes = map[string]string{
	"MREQ": "requirement",
	"MPRJ": "project",
	"MPOR": "purchase_order",
	"MRDR": "sales_order",
	"MINV": "ar_invoice",
	"MPCH": "ap_invoice",
	"MPDN": "goods_receipt_po",
	"MDLN": "delivery",
	"MIGN": "goods_receipt",
	"MIGE": "goods_issue",
	"MJDT": "journal_entry",
}

func erpContextTargetsForCatalog(tableCodes []string) []string {
	targets := []string{}
	seen := map[string]bool{}
	for _, tableCode := range tableCodes {
		target, ok := erpContextTargetTypes[strings.ToUpper(strings.TrimSpace(tableCode))]
		if !ok || seen[target] {
			continue
		}
		seen[target] = true
		targets = append(targets, target)
	}
	return targets
}

type DBContextResolver struct {
	db *pgxpool.Pool
}

func NewDBContextResolver(db *pgxpool.Pool) *DBContextResolver {
	return &DBContextResolver{db: db}
}

func (r *DBContextResolver) Resolve(ctx context.Context, session *Session) WorkRecordContext {
	result := WorkRecordContext{ModuleKey: session.ModuleKey}
	if r == nil || r.db == nil || session == nil {
		return result
	}
	records, err := r.queryRecords(ctx, session.ModuleKey, session.TargetType, session.OrganizationID)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if session.TargetID != nil {
		if selected, err := r.queryTargetRecord(ctx, session.ModuleKey, session.TargetType, *session.TargetID, session.OrganizationID); err == nil {
			records = mergeSelectedRecord(selected, records)
		} else {
			result.Error = err.Error()
			return result
		}
	}
	result.Records = records
	return result
}

func (r *DBContextResolver) queryRecords(ctx context.Context, moduleKey string, targetType string, organizationID *uuid.UUID) ([]WorkRecord, error) {
	query := ""
	if strings.EqualFold(strings.TrimSpace(moduleKey), "erp") {
		query = erpTargetContextQuery(targetType)
	}
	if query == "" {
		query = targetContextQuery(targetType)
	}
	if query == "" {
		query = contextQuery(moduleKey)
	}
	if query == "" {
		query = contextQuery("meta_org")
	}
	var (
		rows pgx.Rows
		err  error
	)
	if strings.Contains(query, "$1") {
		rows, err = r.db.Query(ctx, query, nullableUUID(organizationID))
	} else {
		rows, err = r.db.Query(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve assistant context: %w", err)
	}
	defer rows.Close()

	records := []WorkRecord{}
	for rows.Next() {
		var record WorkRecord
		if err := rows.Scan(&record.ID, &record.Type, &record.Title, &record.Status, &record.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan assistant context: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read assistant context: %w", err)
	}
	return records, nil
}

func (r *DBContextResolver) queryTargetRecord(ctx context.Context, moduleKey string, targetType string, targetID uuid.UUID, organizationID *uuid.UUID) (WorkRecord, error) {
	table, err := proposalTargetTable(moduleKey, targetType)
	if err != nil {
		return WorkRecord{}, err
	}
	predicate := proposalTargetOrganizationPredicate(table)
	var record WorkRecord
	var dataJSON []byte
	err = r.db.QueryRow(ctx, fmt.Sprintf(`
		WITH target AS (
			SELECT to_jsonb(t) AS data
			FROM %s t
			WHERE t.id = $1 AND %s
			LIMIT 1
		)
		SELECT
			data->>'id',
			$3::text,
			COALESCE(
				NULLIF(data->>'title', ''),
				NULLIF(data->>'name', ''),
				NULLIF(data->>'settlement_number', ''),
				NULLIF(data->>'invoice_number', ''),
				NULLIF(data->>'description', ''),
				data->>'id',
				''
			),
			COALESCE(NULLIF(data->>'status', ''), ''),
			COALESCE(NULLIF(data->>'created_at', ''), ''),
			data
		FROM target
	`, table, predicate), targetID, nullableUUID(organizationID), firstNonEmpty(targetType, moduleKey)).Scan(
		&record.ID, &record.Type, &record.Title, &record.Status, &record.CreatedAt, &dataJSON,
	)
	if err != nil {
		return WorkRecord{}, fmt.Errorf("resolve assistant target context: %w", err)
	}
	record.Data = unmarshalRecordData(dataJSON)
	return record, nil
}

func mergeSelectedRecord(selected WorkRecord, records []WorkRecord) []WorkRecord {
	merged := []WorkRecord{selected}
	for _, record := range records {
		if record.ID == selected.ID && record.Type == selected.Type {
			continue
		}
		merged = append(merged, record)
	}
	return merged
}

func unmarshalRecordData(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	return value
}

func proposalTargetOrganizationPredicate(table string) string {
	switch table {
	case "requirements", "projects", "workflow_instances", "finance_receivables", "finance_payables", "cost_ledger_entries":
		return "($2::uuid IS NULL OR t.organization_id IS NOT DISTINCT FROM $2)"
	case "deliverables":
		return `($2::uuid IS NULL OR EXISTS (
			SELECT 1 FROM projects p
			WHERE p.id = t.project_id AND p.organization_id IS NOT DISTINCT FROM $2
		))`
	case "project_cost_entries", "project_evaluations":
		return `($2::uuid IS NULL OR EXISTS (
			SELECT 1 FROM projects p
			WHERE p.id = t.project_id AND p.organization_id IS NOT DISTINCT FROM $2
		))`
	case "tasks":
		return `($2::uuid IS NULL OR EXISTS (
			SELECT 1 FROM workflow_instances wi
			WHERE wi.id = t.workflow_id AND wi.organization_id IS NOT DISTINCT FROM $2
		))`
	case "finance_export_batches":
		return `($2::uuid IS NULL OR EXISTS (
			SELECT 1 FROM finance_export_lines l
			WHERE l.batch_id = t.id AND l.organization_id IS NOT DISTINCT FROM $2
		))`
	case "finance_settlement_orders":
		return `($2::uuid IS NULL
			OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.organization_id IS NOT DISTINCT FROM $2)
			OR EXISTS (SELECT 1 FROM requirements req WHERE req.id = t.requirement_id AND req.organization_id IS NOT DISTINCT FROM $2)
			OR EXISTS (
				SELECT 1
				FROM deliverables d
				JOIN projects p ON p.id = d.project_id
				WHERE d.id = t.deliverable_id AND p.organization_id IS NOT DISTINCT FROM $2
			))`
	case "cost_budgets":
		return `($2::uuid IS NULL
			OR (t.scope_type = 'organization' AND t.scope_id IS NOT DISTINCT FROM $2)
			OR EXISTS (SELECT 1 FROM departments d WHERE t.scope_type = 'department' AND d.id = t.scope_id AND d.organization_id IS NOT DISTINCT FROM $2)
			OR EXISTS (SELECT 1 FROM requirements req WHERE t.scope_type = 'requirement' AND req.id = t.scope_id AND req.organization_id IS NOT DISTINCT FROM $2)
			OR EXISTS (SELECT 1 FROM projects p WHERE t.scope_type = 'project' AND p.id = t.scope_id AND p.organization_id IS NOT DISTINCT FROM $2)
			OR EXISTS (SELECT 1 FROM workflow_instances wi WHERE t.scope_type = 'workflow' AND wi.id = t.scope_id AND wi.organization_id IS NOT DISTINCT FROM $2)
			OR EXISTS (
				SELECT 1
				FROM tasks task
				JOIN workflow_instances wi ON wi.id = task.workflow_id
				WHERE t.scope_type = 'task' AND task.id = t.scope_id AND wi.organization_id IS NOT DISTINCT FROM $2
			))`
	case "cost_rate_cards":
		return `($2::uuid IS NULL
			OR (t.scope_type = 'organization' AND t.scope_id IS NOT DISTINCT FROM $2)
			OR EXISTS (SELECT 1 FROM departments d WHERE t.scope_type = 'department' AND d.id = t.scope_id AND d.organization_id IS NOT DISTINCT FROM $2))`
	default:
		return "$2::uuid IS NULL"
	}
}

func targetContextQuery(targetType string) string {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "requirement":
		return `SELECT id::text, 'requirement', title, status, created_at::text FROM requirements WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	case "project":
		return `SELECT id::text, 'project', name, status, created_at::text FROM projects WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	case "deliverable", "delivery":
		return `SELECT d.id::text, 'deliverable', d.name, d.status, d.created_at::text FROM deliverables d JOIN projects p ON p.id = d.project_id WHERE ($1::uuid IS NULL OR p.organization_id IS NOT DISTINCT FROM $1) ORDER BY d.created_at DESC LIMIT 12`
	case "project_cost", "cost":
		return `SELECT c.id::text, 'project_cost', COALESCE(NULLIF(c.description, ''), c.source_type), 'posted', c.created_at::text FROM project_cost_entries c JOIN projects p ON p.id = c.project_id WHERE ($1::uuid IS NULL OR p.organization_id IS NOT DISTINCT FROM $1) ORDER BY c.created_at DESC LIMIT 12`
	case "project_evaluation", "feedback":
		return `SELECT e.id::text, 'project_evaluation', e.evaluator_type, 'completed', e.created_at::text FROM project_evaluations e JOIN projects p ON p.id = e.project_id WHERE ($1::uuid IS NULL OR p.organization_id IS NOT DISTINCT FROM $1) ORDER BY e.created_at DESC LIMIT 12`
	case "workflow", "workflow_instance":
		return `SELECT id::text, 'workflow_instance', status, status, created_at::text FROM workflow_instances WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	case "task":
		return `SELECT t.id::text, 'task', t.stage_type::text, t.status::text, t.created_at::text FROM tasks t JOIN workflow_instances wi ON wi.id = t.workflow_id WHERE ($1::uuid IS NULL OR wi.organization_id IS NOT DISTINCT FROM $1) ORDER BY t.created_at DESC LIMIT 12`
	case "finance_settlement", "settlement", "settlement_order", "finance_accounting":
		return `
			SELECT f.id::text, 'finance_settlement', COALESCE(NULLIF(f.title, ''), f.settlement_number), f.status, f.created_at::text
			FROM finance_settlement_orders f
			LEFT JOIN projects p ON p.id = f.project_id
			LEFT JOIN requirements req ON req.id = f.requirement_id
			LEFT JOIN deliverables d ON d.id = f.deliverable_id
			LEFT JOIN projects dp ON dp.id = d.project_id
			WHERE ($1::uuid IS NULL OR p.organization_id IS NOT DISTINCT FROM $1 OR req.organization_id IS NOT DISTINCT FROM $1 OR dp.organization_id IS NOT DISTINCT FROM $1)
			ORDER BY f.created_at DESC LIMIT 12`
	case "finance_receivable", "receivable":
		return `SELECT id::text, 'finance_receivable', COALESCE(NULLIF(invoice_number, ''), customer_name), status, created_at::text FROM finance_receivables WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	case "finance_payable", "payable":
		return `SELECT id::text, 'finance_payable', COALESCE(NULLIF(invoice_number, ''), vendor_name), status, created_at::text FROM finance_payables WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	case "cost_budget", "budget":
		return `
			SELECT b.id::text, 'cost_budget', b.scope_type, b.status, b.created_at::text
			FROM cost_budgets b
			WHERE ($1::uuid IS NULL
				OR (b.scope_type = 'organization' AND b.scope_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM departments d WHERE b.scope_type = 'department' AND d.id = b.scope_id AND d.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM requirements req WHERE b.scope_type = 'requirement' AND req.id = b.scope_id AND req.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM projects p WHERE b.scope_type = 'project' AND p.id = b.scope_id AND p.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM workflow_instances wi WHERE b.scope_type = 'workflow' AND wi.id = b.scope_id AND wi.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (
					SELECT 1
					FROM tasks task
					JOIN workflow_instances wi ON wi.id = task.workflow_id
					WHERE b.scope_type = 'task' AND task.id = b.scope_id AND wi.organization_id IS NOT DISTINCT FROM $1
				))
			ORDER BY b.created_at DESC LIMIT 12`
	case "cost_rate_card", "rate_card":
		return `
			SELECT c.id::text, 'cost_rate_card', c.subject_type || ':' || c.rate_type, c.status, c.created_at::text
			FROM cost_rate_cards c
			WHERE ($1::uuid IS NULL
				OR (c.scope_type = 'organization' AND c.scope_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM departments d WHERE c.scope_type = 'department' AND d.id = c.scope_id AND d.organization_id IS NOT DISTINCT FROM $1))
			ORDER BY c.created_at DESC LIMIT 12`
	case "cost_ledger_entry", "ledger_entry":
		return `SELECT id::text, 'cost_ledger_entry', cost_category || ':' || source_type, status, created_at::text FROM cost_ledger_entries WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	default:
		return ""
	}
}

func erpTargetContextQuery(targetType string) string {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "requirement", "erp_requirement":
		return `SELECT "ReqCode"::text, 'requirement', COALESCE(NULLIF("Payload"->>'Name', ''), "ReqCode"::text), COALESCE(NULLIF("Payload"->>'Status', ''), ''), "CreatedAt"::text FROM "MREQ" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "project", "erp_project":
		return `SELECT "PrjCode"::text, 'project', COALESCE(NULLIF("Payload"->>'Name', ''), "PrjCode"::text), COALESCE(NULLIF("Payload"->>'Status', ''), ''), "CreatedAt"::text FROM "MPRJ" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "purchase_order":
		return `SELECT "DocEntry"::text, 'purchase_order', COALESCE(NULLIF("CardCode", ''), "DocEntry"::text), COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MPOR" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "goods_receipt_po":
		return `SELECT "DocEntry"::text, 'goods_receipt_po', COALESCE(NULLIF("CardCode", ''), "DocEntry"::text), COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MPDN" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "sales_order":
		return `SELECT "DocEntry"::text, 'sales_order', COALESCE(NULLIF("CardCode", ''), "DocEntry"::text), COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MRDR" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "delivery":
		return `SELECT "DocEntry"::text, 'delivery', COALESCE(NULLIF("CardCode", ''), "DocEntry"::text), COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MDLN" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "ar_invoice":
		return `SELECT "DocEntry"::text, 'ar_invoice', COALESCE(NULLIF("CardCode", ''), "DocEntry"::text), COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MINV" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "ap_invoice":
		return `SELECT "DocEntry"::text, 'ap_invoice', COALESCE(NULLIF("CardCode", ''), "DocEntry"::text), COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MPCH" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "goods_receipt":
		return `SELECT "DocEntry"::text, 'goods_receipt', "DocEntry"::text, COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MIGN" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "goods_issue":
		return `SELECT "DocEntry"::text, 'goods_issue', "DocEntry"::text, COALESCE(NULLIF("DocStatus", ''), ''), "CreatedAt"::text FROM "MIGE" ORDER BY "CreatedAt" DESC LIMIT 12`
	case "journal_entry":
		return `SELECT "TransId"::text, 'journal_entry', "TransId"::text, COALESCE(NULLIF("BtfStatus", ''), ''), "CreatedAt"::text FROM "MJDT" ORDER BY "CreatedAt" DESC LIMIT 12`
	default:
		return ""
	}
}

func contextQuery(moduleKey string) string {
	switch strings.ToLower(moduleKey) {
	case "requirement":
		return `
			SELECT id::text, 'requirement', title, status, created_at::text FROM requirements WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	case "project", "delivery", "feedback":
		return `
			SELECT id::text, 'project', name, status, created_at::text FROM projects WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 12`
	case "project_cost", "cost":
		return `
			SELECT c.id::text, 'project_cost', COALESCE(NULLIF(c.description, ''), c.source_type), 'posted', c.created_at::text
			FROM project_cost_entries c
			JOIN projects p ON p.id = c.project_id
			WHERE ($1::uuid IS NULL OR p.organization_id IS NOT DISTINCT FROM $1)
			ORDER BY c.created_at DESC LIMIT 12`
	case "meta_resource":
		return `
			(SELECT id::text, 'meta_resource', name, status, created_at::text FROM meta_resources WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 8)
			UNION ALL
			(SELECT c.id::text, 'pdca_cycle', COALESCE(NULLIF(c.summary, ''), c.current_stage), c.status, c.created_at::text
			 FROM pdca_cycles c
			 LEFT JOIN requirements req ON req.id = c.requirement_id
			 LEFT JOIN projects p ON p.id = c.project_id
			 WHERE ($1::uuid IS NULL OR req.organization_id IS NOT DISTINCT FROM $1 OR p.organization_id IS NOT DISTINCT FROM $1)
			 ORDER BY c.created_at DESC LIMIT 4)
			LIMIT 12`
	case "organization":
		return `
			(SELECT id::text, 'department', name, status, created_at::text FROM departments WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 6)
			UNION ALL
			(SELECT id::text, 'position', name, status, created_at::text FROM positions WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 6)
			LIMIT 12`
	case "workflow":
		return `
			(SELECT id::text, 'workflow_instance', status, status, created_at::text FROM workflow_instances WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 6)
			UNION ALL
			(SELECT t.id::text, 'task', t.stage_type::text, t.status::text, t.created_at::text
			 FROM tasks t
			 JOIN workflow_instances wi ON wi.id = t.workflow_id
			 WHERE ($1::uuid IS NULL OR wi.organization_id IS NOT DISTINCT FROM $1)
			 ORDER BY t.created_at DESC LIMIT 6)
			LIMIT 12`
	case "capability":
		return `
			SELECT id::text, 'capability', name, CASE WHEN is_active THEN 'active' ELSE 'inactive' END, created_at::text FROM capabilities ORDER BY created_at DESC LIMIT 12`
	case "governance":
		return `
			(SELECT id::text, 'principle', name, CASE WHEN is_active THEN 'active' ELSE 'inactive' END, created_at::text FROM principles ORDER BY created_at DESC LIMIT 6)
			UNION ALL
			(SELECT id::text, 'access_decision', action, decision, created_at::text FROM access_decisions WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 6)
			LIMIT 12`
	case "self_evolution":
		return `
			(SELECT id::text, 'signal', signal_type, CASE WHEN acknowledged THEN 'acknowledged' ELSE 'open' END, created_at::text FROM signals ORDER BY created_at DESC LIMIT 6)
			UNION ALL
			(SELECT id::text, 'knowledge', title, source, created_at::text FROM knowledge_entries ORDER BY created_at DESC LIMIT 6)
			LIMIT 12`
	case "verification":
		return `
			SELECT id::text, 'verification_report', conclusion, 'completed', created_at::text FROM verification_reports ORDER BY created_at DESC LIMIT 12`
	case "model_settings":
		return `
			(SELECT id::text, 'model', model_key, status, created_at::text FROM models ORDER BY created_at DESC LIMIT 6)
			UNION ALL
			(SELECT id::text, 'ai_invocation', requested_model, status, created_at::text FROM ai_invocations WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 6)
			LIMIT 12`
	case "costing":
		return `
			(SELECT c.id::text, 'rate_card', c.subject_type || ':' || c.rate_type, c.status, c.created_at::text
			 FROM cost_rate_cards c
			 WHERE ($1::uuid IS NULL
				OR (c.scope_type = 'organization' AND c.scope_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM departments d WHERE c.scope_type = 'department' AND d.id = c.scope_id AND d.organization_id IS NOT DISTINCT FROM $1))
			 ORDER BY c.created_at DESC LIMIT 6)
			UNION ALL
			(SELECT b.id::text, 'budget', b.scope_type, b.status, b.created_at::text
			 FROM cost_budgets b
			 WHERE ($1::uuid IS NULL
				OR (b.scope_type = 'organization' AND b.scope_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM departments d WHERE b.scope_type = 'department' AND d.id = b.scope_id AND d.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM requirements req WHERE b.scope_type = 'requirement' AND req.id = b.scope_id AND req.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM projects p WHERE b.scope_type = 'project' AND p.id = b.scope_id AND p.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (SELECT 1 FROM workflow_instances wi WHERE b.scope_type = 'workflow' AND wi.id = b.scope_id AND wi.organization_id IS NOT DISTINCT FROM $1)
				OR EXISTS (
					SELECT 1
					FROM tasks task
					JOIN workflow_instances wi ON wi.id = task.workflow_id
					WHERE b.scope_type = 'task' AND task.id = b.scope_id AND wi.organization_id IS NOT DISTINCT FROM $1
				))
			 ORDER BY b.created_at DESC LIMIT 6)
			UNION ALL
			(SELECT id::text, 'cost_ledger_entry', cost_category || ':' || source_type, status, created_at::text FROM cost_ledger_entries WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 6)
			LIMIT 12`
	case "finance":
		return `
			(SELECT f.id::text, 'finance_settlement', COALESCE(NULLIF(f.title, ''), f.settlement_number), f.status, f.created_at::text
			 FROM finance_settlement_orders f
			 LEFT JOIN projects p ON p.id = f.project_id
			 LEFT JOIN requirements req ON req.id = f.requirement_id
			 LEFT JOIN deliverables d ON d.id = f.deliverable_id
			 LEFT JOIN projects dp ON dp.id = d.project_id
			 WHERE ($1::uuid IS NULL OR p.organization_id IS NOT DISTINCT FROM $1 OR req.organization_id IS NOT DISTINCT FROM $1 OR dp.organization_id IS NOT DISTINCT FROM $1)
			 ORDER BY f.created_at DESC LIMIT 4)
			UNION ALL
			(SELECT id::text, 'finance_receivable', COALESCE(NULLIF(invoice_number, ''), customer_name), status, created_at::text FROM finance_receivables WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 4)
			UNION ALL
			(SELECT id::text, 'finance_payable', COALESCE(NULLIF(invoice_number, ''), vendor_name), status, created_at::text FROM finance_payables WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 4)
			LIMIT 12`
	case "meta_org":
		return `
			(SELECT id::text, 'requirement', title, status, created_at::text FROM requirements WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 4)
			UNION ALL
			(SELECT id::text, 'project', name, status, created_at::text FROM projects WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 4)
			UNION ALL
			(SELECT id::text, 'ai_invocation', requested_model, status, created_at::text FROM ai_invocations WHERE ($1::uuid IS NULL OR organization_id IS NOT DISTINCT FROM $1) ORDER BY created_at DESC LIMIT 4)
			LIMIT 12`
	default:
		return ""
	}
}
