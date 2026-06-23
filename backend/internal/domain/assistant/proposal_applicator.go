package assistant

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBProposalApplicator struct {
	db *pgxpool.Pool
}

func NewDBProposalApplicator(db *pgxpool.Pool) *DBProposalApplicator {
	return &DBProposalApplicator{db: db}
}

func (a *DBProposalApplicator) ApplyProposal(ctx context.Context, proposal *Proposal) (map[string]any, error) {
	if a == nil || a.db == nil {
		return nil, fmt.Errorf("%w: proposal applicator database is not configured", ErrValidation)
	}
	if target, ok, err := erpProposalTargetFromPayload(proposal); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return a.applyERPProposal(ctx, proposal, target)
	}
	if proposal == nil || proposal.TargetID == nil {
		return nil, fmt.Errorf("%w: proposal target is required", ErrValidation)
	}
	table, err := proposalTargetTable(proposal.ModuleKey, proposal.TargetType)
	if err != nil {
		return nil, err
	}
	entry := map[string]any{
		"proposal_id":   proposal.ID.String(),
		"session_id":    proposal.SessionID.String(),
		"proposal_type": proposal.ProposalType,
		"title":         proposal.Title,
		"summary":       proposal.Summary,
		"payload":       proposal.Payload,
	}
	command, err := a.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET metadata = jsonb_set(
			COALESCE(metadata, '{}'::jsonb),
			'{assistant_confirmed_proposals}',
			COALESCE(metadata->'assistant_confirmed_proposals', '[]'::jsonb) || jsonb_build_array($2::jsonb),
			true
		)
		WHERE id = $1
	`, table), *proposal.TargetID, mustJSON(entry))
	if err != nil {
		return nil, fmt.Errorf("apply assistant proposal: %w", err)
	}
	if command.RowsAffected() == 0 {
		return nil, fmt.Errorf("%w: proposal target not found", ErrNotFound)
	}
	return map[string]any{
		"target_table": table,
		"target_id":    proposal.TargetID.String(),
		"writeback":    "metadata.assistant_confirmed_proposals",
	}, nil
}

type erpProposalTarget struct {
	tableCode  string
	primaryKey string
	key        string
	action     string
}

var erpPrimaryKeys = map[string]string{
	"MREQ": "ReqCode",
	"MPRJ": "PrjCode",
	"MCST": "CostCode",
	"MFDB": "FeedbackCode",
	"MCRD": "CardCode",
	"MITM": "ItemCode",
	"MITW": "ItemCode",
	"MWHS": "WhsCode",
	"MPOR": "DocEntry",
	"MPDN": "DocEntry",
	"MPCH": "DocEntry",
	"MRDR": "DocEntry",
	"MDLN": "DocEntry",
	"MINV": "DocEntry",
	"MRCT": "DocEntry",
	"MIGN": "DocEntry",
	"MIGE": "DocEntry",
	"MJDT": "TransId",
}

func erpProposalTargetFromPayload(proposal *Proposal) (erpProposalTarget, bool, error) {
	if proposal == nil {
		return erpProposalTarget{}, false, nil
	}
	if !strings.EqualFold(strings.TrimSpace(proposal.ModuleKey), "erp") && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(proposal.TargetType)), "erp_") {
		return erpProposalTarget{}, false, nil
	}
	tableCode := strings.ToUpper(strings.TrimSpace(stringPayloadValue(proposal.Payload, "table_code")))
	key := strings.TrimSpace(firstPayloadValue(proposal.Payload, "key", "record_key", "target_key"))
	action := strings.TrimSpace(stringPayloadValue(proposal.Payload, "action"))
	if tableCode == "" || key == "" {
		return erpProposalTarget{}, true, fmt.Errorf("%w: ERP proposal requires table_code and key payload fields", ErrValidation)
	}
	primaryKey, ok := erpPrimaryKeys[tableCode]
	if !ok {
		return erpProposalTarget{}, true, fmt.Errorf("%w: unsupported ERP proposal table %q", ErrValidation, tableCode)
	}
	return erpProposalTarget{tableCode: tableCode, primaryKey: primaryKey, key: key, action: action}, true, nil
}

func (a *DBProposalApplicator) applyERPProposal(ctx context.Context, proposal *Proposal, target erpProposalTarget) (map[string]any, error) {
	entry := map[string]any{
		"proposal_id":   proposal.ID.String(),
		"session_id":    proposal.SessionID.String(),
		"proposal_type": proposal.ProposalType,
		"title":         proposal.Title,
		"summary":       proposal.Summary,
		"payload":       proposal.Payload,
		"action":        target.action,
	}
	command, err := a.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET "Payload" = jsonb_set(
			COALESCE("Payload", '{}'::jsonb),
			'{assistant_confirmed_proposals}',
			COALESCE("Payload"->'assistant_confirmed_proposals', '[]'::jsonb) || jsonb_build_array($2::jsonb),
			true
		)
		WHERE %s::TEXT = $1
	`, quoteERPIdent(target.tableCode), quoteERPIdent(target.primaryKey)), target.key, mustJSON(entry))
	if err != nil {
		return nil, fmt.Errorf("apply assistant ERP proposal: %w", err)
	}
	if command.RowsAffected() == 0 {
		return nil, fmt.Errorf("%w: ERP proposal target not found", ErrNotFound)
	}
	return map[string]any{
		"target_table": target.tableCode,
		"target_key":   target.key,
		"action":       target.action,
		"writeback":    "Payload.assistant_confirmed_proposals",
	}, nil
}

func stringPayloadValue(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	return fmt.Sprint(payload[key])
}

func firstPayloadValue(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value := stringPayloadValue(payload, key)
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func quoteERPIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func proposalTargetTable(moduleKey string, targetType string) (string, error) {
	key := targetType
	if key == "" {
		key = moduleKey
	}
	switch key {
	case "requirement":
		return "requirements", nil
	case "project":
		return "projects", nil
	case "deliverable", "delivery":
		return "deliverables", nil
	case "project_cost", "cost":
		return "project_cost_entries", nil
	case "project_evaluation", "feedback":
		return "project_evaluations", nil
	case "workflow", "workflow_instance":
		return "workflow_instances", nil
	case "task":
		return "tasks", nil
	case "finance_batch", "finance", "finance_accounting":
		return "finance_export_batches", nil
	case "finance_settlement", "settlement", "settlement_order":
		return "finance_settlement_orders", nil
	case "finance_receivable", "receivable":
		return "finance_receivables", nil
	case "finance_payable", "payable":
		return "finance_payables", nil
	case "cost_budget", "budget":
		return "cost_budgets", nil
	case "cost_rate_card", "rate_card":
		return "cost_rate_cards", nil
	case "cost_ledger_entry", "ledger_entry":
		return "cost_ledger_entries", nil
	default:
		return "", fmt.Errorf("%w: unsupported proposal target type %q", ErrValidation, key)
	}
}
