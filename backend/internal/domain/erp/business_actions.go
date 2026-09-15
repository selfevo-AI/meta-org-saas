package erp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func (s *Service) runBusinessAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	switch tableCode + ":" + action {
	case "MREQ:analyze":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "analyzed"})
	case "MREQ:approve":
		_, check, err := s.requireRecordField(ctx, tableCode, key, "Status", "analyzed", "requirement must be analyzed before approval")
		if err != nil {
			return nil, err
		}
		result, err := s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "approved", "ApprovedBy": input.Data["approver"]})
		if err != nil {
			return nil, err
		}
		return attachPreconditions(result, check), nil
	case "MREQ:convert-to-project":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.convertRequirementToProject(ctx, key, input)
		})
	case "MPRJ:refresh-cost":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.refreshProjectCost(ctx, key, input)
		})
	case "MPRJ:close-feedback":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.closeProjectFeedback(ctx, key, input)
		})
	case "MPOR:submit":
		return s.prepareOrder(ctx, tableCode, key, action)
	case "MPOR:approve":
		if err := s.requireDocumentField(ctx, tableCode, key, "DocStatus", "S", "purchase order must be submitted before approval"); err != nil {
			return nil, err
		}
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
	case "MPOR:receive", "MRDR:deliver":
		return s.convertOrderToFulfillment(ctx, tableCode, key, input)
	case "MPDN:approve", "MDLN:approve":
		return s.approveFulfillment(ctx, tableCode, key)
	case "MPDN:post":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postGoodsReceiptPO(ctx, key)
		})
	case "MRDR:confirm":
		return s.prepareOrder(ctx, tableCode, key, action)
	case "MRDR:approve":
		_, check, err := s.requireRecordField(ctx, tableCode, key, "Confirmed", "Y", "sales order must be confirmed before approval")
		if err != nil {
			return nil, err
		}
		result, err := s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
		if err != nil {
			return nil, err
		}
		return attachPreconditions(result, check), nil
	case "MDLN:post":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postDelivery(ctx, key)
		})
	case "MINV:post":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postInvoice(ctx, key)
		})
	case "MPCH:post":
		return s.postFinancialInvoice(ctx, tableCode, key)
	case "MVPM:allocate":
		return s.allocatePayment(ctx, tableCode, key, input)
	case "MRCT:allocate":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.allocateIncomingPayment(ctx, key, input)
		})
	case "MIGN:post":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postInventoryDocument(ctx, tableCode, key, 1)
		})
	case "MIGE:post":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postInventoryDocument(ctx, tableCode, key, -1)
		})
	case "MJDT:post":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postJournal(ctx, key)
		})
	case "MGLR:run":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.runTrialBalance(ctx, key, input)
		})
	default:
		return nil, fmt.Errorf("%w: %s for %s", errUnsupportedERPAction, action, tableCode)
	}
}

func (s *Service) mergeStatusAction(ctx context.Context, tableCode string, key string, action string, data map[string]any) (*ActionResult, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: data})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: action, Status: actionStatus(record), Record: record}, nil
}

func (s *Service) convertRequirementToProject(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	reqTable, _ := s.table("MREQ")
	projectTable, _ := s.table("MPRJ")
	req, err := s.repo.GetRecord(ctx, reqTable, key)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(req.Data["Status"]) != "approved" {
		return nil, fmt.Errorf("%w: requirement must be approved before conversion", ErrValidation)
	}
	projectKey := stringValue(input.Data, "PrjCode", "PRJ-"+key)
	projectPayload := withActionProvenance(ctx, "MREQ", key, "convert-to-project", map[string]any{
		"Name":            stringValue(req.Data, "Name", projectKey),
		"RequirementCode": key,
		"Status":          "active",
	})
	project, err := s.repo.CreateRecord(ctx, projectTable, RecordInput{
		Key: projectKey,
		Data: map[string]any{
			"PrjCode":    projectKey,
			"Active":     "Y",
			"Payload":    projectPayload,
			"provenance": projectPayload["provenance"],
		},
	})
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateRecord(ctx, reqTable, key, RecordInput{Data: map[string]any{"Status": "converted", "ProjectCode": projectKey}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MREQ", Key: key, Action: "convert-to-project", Status: "converted", Record: updated, GeneratedRecords: []Record{*project}}, nil
}

func (s *Service) refreshProjectCost(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	projectTable, _ := s.table("MPRJ")
	costTable, _ := s.table("MCST")
	costKey := stringValue(input.Data, "CostCode", "COST-"+key)
	costPayload := withActionProvenance(ctx, "MPRJ", key, "refresh-cost", map[string]any{"ProjectCode": key})
	cost, err := s.repo.CreateRecord(ctx, costTable, RecordInput{
		Key: costKey,
		Data: map[string]any{
			"CostCode":   costKey,
			"Name":       "Project cost " + key,
			"Status":     "refreshed",
			"Payload":    costPayload,
			"provenance": costPayload["provenance"],
		},
	})
	if err != nil {
		return nil, err
	}
	project, err := s.repo.UpdateRecord(ctx, projectTable, key, RecordInput{Data: map[string]any{"LastCostCode": costKey}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MPRJ", Key: key, Action: "refresh-cost", Status: "refreshed", Record: project, GeneratedRecords: []Record{*cost}}, nil
}

func (s *Service) closeProjectFeedback(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	project, check, err := s.requireProjectCostRefreshed(ctx, key)
	if err != nil {
		return nil, err
	}
	projectTable, _ := s.table("MPRJ")
	feedbackTable, _ := s.table("MFDB")
	feedbackKey := stringValue(input.Data, "FeedbackCode", "FDB-"+key)
	feedbackPayload := withActionProvenance(ctx, "MPRJ", key, "close-feedback", map[string]any{"ProjectCode": key, "Result": input.Data["result"]})
	feedback, err := s.repo.CreateRecord(ctx, feedbackTable, RecordInput{
		Key: feedbackKey,
		Data: map[string]any{
			"FeedbackCode": feedbackKey,
			"Name":         "Feedback " + key,
			"Status":       "closed",
			"Payload":      feedbackPayload,
			"provenance":   feedbackPayload["provenance"],
		},
	})
	if err != nil {
		return nil, err
	}
	project, err = s.repo.UpdateRecord(ctx, projectTable, key, RecordInput{Data: map[string]any{"FeedbackStatus": "closed"}})
	if err != nil {
		return nil, err
	}
	return attachPreconditions(&ActionResult{TableCode: "MPRJ", Key: key, Action: "close-feedback", Status: "closed", Record: project, GeneratedRecords: []Record{*feedback}}, check), nil
}

func (s *Service) postGoodsReceiptPO(ctx context.Context, key string) (*ActionResult, error) {
	return s.postStockFulfillment(ctx, "MPDN", key)
}

func (s *Service) postDelivery(ctx context.Context, key string) (*ActionResult, error) {
	return s.postStockFulfillment(ctx, "MDLN", key)
}

func (s *Service) postInvoice(ctx context.Context, key string) (*ActionResult, error) {
	return s.postFinancialInvoice(ctx, "MINV", key)
}

func (s *Service) allocateIncomingPayment(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	return s.allocatePayment(ctx, "MRCT", key, input)
}

func (s *Service) postInventoryDocument(ctx context.Context, tableCode string, key string, direction float64) (*ActionResult, error) {
	return s.postStandaloneInventory(ctx, tableCode, key, direction)
}

func (s *Service) requireDocumentField(ctx context.Context, tableCode string, key string, field string, expected string, message string) error {
	table, err := s.table(tableCode)
	if err != nil {
		return err
	}
	record, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return err
	}
	if isClosedDocument(record) {
		return fmt.Errorf("%w: document is already closed", ErrValidation)
	}
	if !documentFieldEquals(record, field, expected) {
		return fmt.Errorf("%w: %s", ErrValidation, message)
	}
	return nil
}

func passedPrecondition(key string, message string) ActionPrecondition {
	return ActionPrecondition{Key: key, Status: "passed", Message: message}
}

func (s *Service) requireRecordField(ctx context.Context, tableCode string, key string, field string, expected string, message string) (*Record, ActionPrecondition, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, ActionPrecondition{}, err
	}
	record, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, ActionPrecondition{}, err
	}
	check := passedPrecondition(tableCode+"."+field, message)
	if !documentFieldEquals(record, field, expected) {
		check.Status = "failed"
		return nil, check, fmt.Errorf("%w: %s", ErrValidation, message)
	}
	return record, check, nil
}

func (s *Service) requireProjectCostRefreshed(ctx context.Context, key string) (*Record, ActionPrecondition, error) {
	table, err := s.table("MPRJ")
	if err != nil {
		return nil, ActionPrecondition{}, err
	}
	project, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, ActionPrecondition{}, err
	}
	check := passedPrecondition("MPRJ.LastCostCode", "project cost must be refreshed before feedback closes")
	if stringValue(project.Data, "LastCostCode", "") == "" {
		check.Status = "failed"
		return nil, check, fmt.Errorf("%w: %s", ErrValidation, check.Message)
	}
	return project, check, nil
}

func attachPreconditions(result *ActionResult, checks ...ActionPrecondition) *ActionResult {
	if result == nil {
		return result
	}
	result.PreconditionsChecked = append(result.PreconditionsChecked, checks...)
	return result
}

func documentFieldEquals(record *Record, field string, expected string) bool {
	if record == nil || record.Data == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(record.Data[field])), expected)
}

func isClosedDocument(record *Record) bool {
	return documentFieldEquals(record, "DocStatus", "C")
}

func isPostedDocument(record *Record) bool {
	return documentFieldEquals(record, "Posted", "Y")
}

type actionProvenanceInput struct {
	TableCode          string
	Key                string
	Action             string
	ExecutionID        uuid.UUID
	IdempotencyKey     string
	ActorType          string
	ToolExecutionID    *uuid.UUID
	AssistantSessionID *uuid.UUID
}

func actionProvenance(input actionProvenanceInput) map[string]any {
	return map[string]any{
		"source_table_code":     input.TableCode,
		"source_key":            input.Key,
		"source_action":         input.Action,
		"action_execution_id":   input.ExecutionID.String(),
		"idempotency_key":       input.IdempotencyKey,
		"created_by_actor_type": input.ActorType,
		"tool_execution_id":     uuidString(input.ToolExecutionID),
		"assistant_session_id":  uuidString(input.AssistantSessionID),
	}
}

func withProvenance(payload map[string]any, provenance map[string]any) map[string]any {
	next := copyData(payload)
	next["provenance"] = provenance
	return next
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func withActionProvenance(ctx context.Context, sourceTableCode string, sourceKey string, sourceAction string, payload map[string]any) map[string]any {
	meta := actionExecutionFromContext(ctx)
	return withProvenance(payload, actionProvenance(actionProvenanceInput{
		TableCode:          sourceTableCode,
		Key:                sourceKey,
		Action:             sourceAction,
		ExecutionID:        meta.ExecutionID,
		IdempotencyKey:     meta.IdempotencyKey,
		ActorType:          meta.ActorType,
		ToolExecutionID:    meta.ToolExecutionID,
		AssistantSessionID: meta.AssistantSessionID,
	}))
}

func (s *Service) createDocument(ctx context.Context, sourceTableCode string, sourceKey string, sourceAction string, tableCode string, key string, payload map[string]any) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	payload = withActionProvenance(ctx, sourceTableCode, sourceKey, sourceAction, payload)
	data := map[string]any{table.PrimaryKey: key, "Payload": payload}
	if _, ok := table.Field("DocNum"); ok {
		data["DocNum"] = key
	}
	if _, ok := table.Field("DocStatus"); ok {
		data["DocStatus"] = "O"
	}
	if _, ok := table.Field("BtfStatus"); ok {
		data["BtfStatus"] = "O"
	}
	data["provenance"] = payload["provenance"]
	for k, v := range payload {
		data[k] = v
	}
	return s.repo.CreateRecord(ctx, table, RecordInput{Key: key, Data: data})
}

func (s *Service) createDocumentLine(ctx context.Context, parentCode string, parentKey string, childCode string, lineNum string, payload map[string]any) (*Record, error) {
	parent, child, err := s.child(parentCode, childCode)
	if err != nil {
		return nil, err
	}
	data := copyData(payload)
	data[child.ParentKey] = parentKey
	data["LineNum"] = lineNum
	if _, ok := data["LineStatus"]; !ok {
		data["LineStatus"] = "O"
	}
	return s.repo.CreateChildRecord(ctx, parent, child, parentKey, RecordInput{Key: lineNum, Data: data})
}

func (s *Service) listChildPayloads(ctx context.Context, parentCode string, parentKey string, childCode string) ([]map[string]any, error) {
	parent, child, err := s.child(parentCode, childCode)
	if err != nil {
		return nil, err
	}
	var lines []Record
	if reader, ok := s.repo.(allChildReader); ok {
		lines, err = reader.ListBusinessLines(ctx, parent, child, parentKey)
	} else {
		lines, err = s.repo.ListChildRecords(ctx, parent, child, parentKey, 500)
	}
	if err != nil {
		return nil, err
	}
	payloads := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		payload := copyData(line.Data)
		if nested, ok := line.Data["Payload"].(map[string]any); ok {
			for k, v := range nested {
				payload[k] = v
			}
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

func (s *Service) runTrialBalance(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	balance, err := s.TrialBalance(ctx, TrialBalanceInput{
		Currency:    stringValue(input.Data, "Currency", "CNY"),
		PeriodStart: stringValue(input.Data, "PeriodStart", ""), PeriodEnd: stringValue(input.Data, "PeriodEnd", ""),
	})
	if err != nil {
		return nil, err
	}
	if balance.TotalDebit != balance.TotalCredit {
		return nil, fmt.Errorf("%w: ledger is not balanced", ErrValidation)
	}
	data := map[string]any{"ReportCode": key, "Currency": balance.Currency, "PeriodStart": input.Data["PeriodStart"],
		"PeriodEnd": input.Data["PeriodEnd"], "TotalDebit": balance.TotalDebit, "TotalCredit": balance.TotalCredit,
		"JournalCount": balance.JournalCount, "Rows": balance.Rows, "LastRunStatus": "balanced"}
	table, _ := s.table("MGLR")
	report, err := s.repo.GetRecord(ctx, table, key)
	if err == ErrNotFound {
		report, err = s.repo.CreateRecord(ctx, table, RecordInput{Key: key, Data: data})
	} else if err == nil {
		report, err = s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: data})
	}
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MGLR", Key: key, Action: "run", Status: "balanced", Record: report, Effects: map[string]any{"journal_count": balance.JournalCount}}, nil
}

func actionStatus(record *Record) string {
	if record == nil {
		return "accepted"
	}
	for _, key := range []string{"Status", "DocStatus", "WddStatus", "BtfStatus"} {
		if value := stringValue(record.Data, key, ""); value != "" {
			return value
		}
	}
	return "updated"
}

func stringValue(values map[string]any, key string, fallback string) string {
	if values == nil {
		return fallback
	}
	value, ok := values[key]
	if !ok || value == nil {
		return fallback
	}
	text := fmt.Sprint(value)
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func numericValue(values map[string]any, key string) float64 {
	value, present := values[key]
	if !present {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case float32:
		return float64(typed)
	case json.Number:
		value, err := typed.Float64()
		if err != nil {
			return math.NaN()
		}
		return value
	case string:
		value, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return math.NaN()
		}
		return value
	default:
		return math.NaN()
	}
}
