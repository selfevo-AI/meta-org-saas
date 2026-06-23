package erp

import (
	"context"
	"fmt"
)

func (s *Service) runBusinessAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	switch tableCode + ":" + action {
	case "MREQ:analyze":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "analyzed"})
	case "MREQ:approve":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "approved", "ApprovedBy": input.Data["approver"]})
	case "MREQ:convert-to-project":
		return s.convertRequirementToProject(ctx, key, input)
	case "MPRJ:refresh-cost":
		return s.refreshProjectCost(ctx, key, input)
	case "MPRJ:close-feedback":
		return s.closeProjectFeedback(ctx, key, input)
	case "MPOR:submit":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"DocStatus": "S"})
	case "MPOR:approve":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
	case "MPDN:post":
		return s.postGoodsReceiptPO(ctx, key)
	case "MRDR:confirm":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Confirmed": "Y"})
	case "MRDR:approve":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
	case "MDLN:post":
		return s.postDelivery(ctx, key)
	case "MINV:post":
		return s.postInvoice(ctx, key)
	case "MRCT:allocate":
		return s.allocateIncomingPayment(ctx, key, input)
	case "MIGN:post":
		return s.postInventoryDocument(ctx, tableCode, key, 1)
	case "MIGE:post":
		return s.postInventoryDocument(ctx, tableCode, key, -1)
	case "MJDT:post":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"BtfStatus": "P"})
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
	project, err := s.repo.CreateRecord(ctx, projectTable, RecordInput{
		Key: projectKey,
		Data: map[string]any{
			"PrjCode": projectKey,
			"Active":  "Y",
			"Payload": map[string]any{
				"Name":            stringValue(req.Data, "Name", projectKey),
				"RequirementCode": key,
				"Status":          "active",
			},
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
	cost, err := s.repo.CreateRecord(ctx, costTable, RecordInput{
		Key: costKey,
		Data: map[string]any{
			"CostCode": costKey,
			"Name":     "Project cost " + key,
			"Status":   "refreshed",
			"Payload":  map[string]any{"ProjectCode": key},
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
	projectTable, _ := s.table("MPRJ")
	feedbackTable, _ := s.table("MFDB")
	feedbackKey := stringValue(input.Data, "FeedbackCode", "FDB-"+key)
	feedback, err := s.repo.CreateRecord(ctx, feedbackTable, RecordInput{
		Key: feedbackKey,
		Data: map[string]any{
			"FeedbackCode": feedbackKey,
			"Name":         "Feedback " + key,
			"Status":       "closed",
			"Payload":      map[string]any{"ProjectCode": key, "Result": input.Data["result"]},
		},
	})
	if err != nil {
		return nil, err
	}
	project, err := s.repo.UpdateRecord(ctx, projectTable, key, RecordInput{Data: map[string]any{"FeedbackStatus": "closed"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MPRJ", Key: key, Action: "close-feedback", Status: "closed", Record: project, GeneratedRecords: []Record{*feedback}}, nil
}

func (s *Service) postGoodsReceiptPO(ctx context.Context, key string) (*ActionResult, error) {
	receiptTable, _ := s.table("MPDN")
	receipt, err := s.repo.GetRecord(ctx, receiptTable, key)
	if err != nil {
		return nil, err
	}
	lines, err := s.listChildPayloads(ctx, "MPDN", key, "PDN1")
	if err != nil {
		return nil, err
	}
	total := sumLineAmount(lines)
	goodsReceipt, err := s.createDocument(ctx, "MIGN", "IGN-"+key, map[string]any{"CardCode": receipt.Data["CardCode"], "DocTotal": total, "BaseEntry": key})
	if err != nil {
		return nil, err
	}
	payable, err := s.createDocument(ctx, "MPCH", "AP-"+key, map[string]any{"CardCode": receipt.Data["CardCode"], "DocTotal": total, "BaseEntry": key, "PaidToDate": 0})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), numericValue(line, "Quantity"), 1); err != nil {
			return nil, err
		}
		lineNum := fmt.Sprint(i + 1)
		if _, err := s.createDocumentLine(ctx, "MIGN", goodsReceipt.Key, "IGN1", lineNum, line); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, "MPCH", payable.Key, "PCH1", lineNum, line); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, receiptTable, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MPDN", Key: key, Action: "post", Status: "posted", Record: updated, GeneratedRecords: []Record{*goodsReceipt, *payable}}, nil
}

func (s *Service) postDelivery(ctx context.Context, key string) (*ActionResult, error) {
	deliveryTable, _ := s.table("MDLN")
	delivery, err := s.repo.GetRecord(ctx, deliveryTable, key)
	if err != nil {
		return nil, err
	}
	lines, err := s.listChildPayloads(ctx, "MDLN", key, "DLN1")
	if err != nil {
		return nil, err
	}
	total := sumLineAmount(lines)
	goodsIssue, err := s.createDocument(ctx, "MIGE", "IGE-"+key, map[string]any{"CardCode": delivery.Data["CardCode"], "DocTotal": total, "BaseEntry": key})
	if err != nil {
		return nil, err
	}
	invoice, err := s.createDocument(ctx, "MINV", "INV-"+key, map[string]any{"CardCode": delivery.Data["CardCode"], "DocTotal": total, "PaidToDate": 0, "BaseEntry": key})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), numericValue(line, "Quantity"), -1); err != nil {
			return nil, err
		}
		lineNum := fmt.Sprint(i + 1)
		if _, err := s.createDocumentLine(ctx, "MIGE", goodsIssue.Key, "IGE1", lineNum, line); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, "MINV", invoice.Key, "INV1", lineNum, line); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, deliveryTable, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MDLN", Key: key, Action: "post", Status: "posted", Record: updated, GeneratedRecords: []Record{*goodsIssue, *invoice}}, nil
}

func (s *Service) postInvoice(ctx context.Context, key string) (*ActionResult, error) {
	invoiceTable, _ := s.table("MINV")
	journal, err := s.createDocument(ctx, "MJDT", "JE-"+key, map[string]any{"BaseEntry": key})
	if err != nil {
		return nil, err
	}
	invoice, err := s.repo.UpdateRecord(ctx, invoiceTable, key, RecordInput{Data: map[string]any{"Posted": "Y"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MINV", Key: key, Action: "post", Status: "posted", Record: invoice, GeneratedRecords: []Record{*journal}}, nil
}

func (s *Service) allocateIncomingPayment(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	paymentTable, _ := s.table("MRCT")
	targetTableCode := stringValue(input.Data, "TargetTable", "MINV")
	targetKey := stringValue(input.Data, "TargetKey", "")
	amount := numericValue(input.Data, "Amount")
	if targetKey == "" || amount <= 0 {
		return nil, fmt.Errorf("%w: TargetKey and positive Amount are required", ErrValidation)
	}
	currentPayment, err := s.repo.GetRecord(ctx, paymentTable, key)
	if err != nil {
		return nil, err
	}
	currentOpenBal := numericValue(currentPayment.Data, "OpenBal")
	if currentOpenBal == 0 {
		currentOpenBal = numericValue(currentPayment.Data, "DocTotal")
	}
	if currentOpenBal > 0 && amount > currentOpenBal {
		return nil, fmt.Errorf("%w: Amount exceeds payment open balance", ErrValidation)
	}
	targetTable, err := s.table(targetTableCode)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.GetRecord(ctx, targetTable, targetKey)
	if err != nil {
		return nil, err
	}
	paid := numericValue(target.Data, "PaidToDate") + amount
	docTotal := numericValue(target.Data, "DocTotal")
	status := target.Data["DocStatus"]
	if docTotal > 0 && paid >= docTotal {
		status = "C"
	}
	if _, err := s.repo.UpdateRecord(ctx, targetTable, targetKey, RecordInput{Data: map[string]any{"PaidToDate": paid, "DocStatus": status}}); err != nil {
		return nil, err
	}
	payment, err := s.repo.UpdateRecord(ctx, paymentTable, key, RecordInput{Data: map[string]any{"OpenBal": currentOpenBal - amount}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MRCT", Key: key, Action: "allocate", Status: "allocated", Record: payment, Effects: map[string]any{"allocated_amount": amount, "target_table": targetTableCode, "target_key": targetKey}}, nil
}

func (s *Service) postInventoryDocument(ctx context.Context, tableCode string, key string, direction float64) (*ActionResult, error) {
	table, _ := s.table(tableCode)
	childCode := "IGN1"
	if tableCode == "MIGE" {
		childCode = "IGE1"
	}
	lines, err := s.listChildPayloads(ctx, tableCode, key, childCode)
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), numericValue(line, "Quantity"), direction); err != nil {
			return nil, err
		}
	}
	record, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: record}, nil
}

func (s *Service) createDocument(ctx context.Context, tableCode string, key string, payload map[string]any) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
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
	data := map[string]any{
		child.ParentKey: parentKey,
		"LineNum":       lineNum,
		"LineStatus":    "O",
		"Payload":       payload,
	}
	return s.repo.CreateChildRecord(ctx, parent, child, parentKey, RecordInput{Key: lineNum, Data: data})
}

func (s *Service) listChildPayloads(ctx context.Context, parentCode string, parentKey string, childCode string) ([]map[string]any, error) {
	parent, child, err := s.child(parentCode, childCode)
	if err != nil {
		return nil, err
	}
	lines, err := s.repo.ListChildRecords(ctx, parent, child, parentKey, 500)
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

func (s *Service) adjustInventory(ctx context.Context, itemCode string, whsCode string, quantity float64, direction float64) error {
	if itemCode == "" || whsCode == "" || quantity <= 0 {
		return fmt.Errorf("%w: ItemCode, WhsCode, and positive Quantity are required", ErrValidation)
	}
	table, _ := s.table("MITW")
	key := itemCode + "|" + whsCode
	current, err := s.repo.GetRecord(ctx, table, key)
	if err != nil && err != ErrNotFound {
		return err
	}
	onHand := 0.0
	if current != nil {
		onHand = numericValue(current.Data, "OnHand")
	}
	next := onHand + quantity*direction
	if next < 0 {
		return fmt.Errorf("%w: insufficient inventory for %s/%s", ErrValidation, itemCode, whsCode)
	}
	data := map[string]any{"ItemCode": key, "WhsCode": whsCode, "OnHand": next, "Payload": map[string]any{"BaseItemCode": itemCode}}
	if current == nil {
		_, err = s.repo.CreateRecord(ctx, table, RecordInput{Key: key, Data: data})
		return err
	}
	_, err = s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: data})
	return err
}

func sumLineAmount(lines []map[string]any) float64 {
	total := 0.0
	for _, line := range lines {
		quantity := numericValue(line, "Quantity")
		price := numericValue(line, "Price")
		total += quantity * price
	}
	return total
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
	if values == nil {
		return 0
	}
	switch typed := values[key].(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case float32:
		return float64(typed)
	default:
		return 0
	}
}
