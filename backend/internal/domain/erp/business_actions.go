package erp

import (
	"context"
	"fmt"
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
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"DocStatus": "S"})
	case "MPOR:approve":
		if err := s.requireDocumentField(ctx, tableCode, key, "DocStatus", "S", "purchase order must be submitted before approval"); err != nil {
			return nil, err
		}
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
	case "MPDN:post":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postGoodsReceiptPO(ctx, key)
		})
	case "MRDR:confirm":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Confirmed": "Y"})
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
			return tx.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"BtfStatus": "P"})
		})
	case "MGLR:run":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.runTrialBalance(ctx, key, input)
		})
	case "MPUB:publish":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "published"})
	case "MRPS:close":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.closePOSSale(ctx, key)
		})
	case "MDRQ:submit":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"DocStatus": "S"})
	case "MDRQ:approve":
		if err := s.requireDocumentField(ctx, tableCode, key, "DocStatus", "S", "distribution request must be submitted before approval"); err != nil {
			return nil, err
		}
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
	case "MDRQ:auto-allocate":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.autoAllocateDistributionRequest(ctx, key)
		})
	case "MDSP:ship":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.shipDistribution(ctx, key)
		})
	case "MDRC:receive":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.receiveDistribution(ctx, key)
		})
	case "MDIF:resolve":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.resolveDistributionDifference(ctx, key)
		})
	case "MSTP:replenish":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.replenishFromStockPolicy(ctx, key)
		})
	case "MCNT:submit":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"DocStatus": "S"})
	case "MCNT:approve":
		if err := s.requireDocumentField(ctx, tableCode, key, "DocStatus", "S", "store count must be submitted before approval"); err != nil {
			return nil, err
		}
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
	case "MCNT:post-adjustment":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.postStoreCountAdjustment(ctx, key)
		})
	case "MSPR:submit":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"DocStatus": "S"})
	case "MSPR:approve":
		if err := s.requireDocumentField(ctx, tableCode, key, "DocStatus", "S", "special purchase request must be submitted before approval"); err != nil {
			return nil, err
		}
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"WddStatus": "A"})
	case "MSPR:convert-to-purchase-order":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.convertSpecialPurchaseRequest(ctx, key)
		})
	case "MBOM:approve":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.approveBOM(ctx, key, input)
		})
	case "MBOM:make-work-order":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.makeWorkOrderFromBOM(ctx, key, input)
		})
	case "MWOR:release":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.releaseWorkOrder(ctx, key)
		})
	case "MWOR:issue-material":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.issueWorkOrderMaterial(ctx, key)
		})
	case "MWOR:complete":
		return s.runInTx(ctx, func(tx *Service) (*ActionResult, error) {
			return tx.completeWorkOrder(ctx, key, input)
		})
	case "MWOR:close":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "closed"})
	case "MWOR:stop":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "stopped"})
	case "MWOR:reopen":
		return s.mergeStatusAction(ctx, tableCode, key, action, map[string]any{"Status": "released"})
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
	receiptTable, _ := s.table("MPDN")
	receipt, err := s.repo.GetRecord(ctx, receiptTable, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(receipt) {
		return &ActionResult{TableCode: "MPDN", Key: key, Action: "post", Status: "posted", Record: receipt}, nil
	}
	if !documentFieldEquals(receipt, "WddStatus", "A") {
		return nil, fmt.Errorf("%w: goods receipt PO must be approved before posting", ErrValidation)
	}
	lines, err := s.listChildPayloads(ctx, "MPDN", key, "PDN1")
	if err != nil {
		return nil, err
	}
	total := sumLineAmount(lines)
	goodsReceipt, err := s.createDocument(ctx, "MPDN", key, "post", "MIGN", "IGN-"+key, map[string]any{"CardCode": receipt.Data["CardCode"], "DocTotal": total, "BaseEntry": key})
	if err != nil {
		return nil, err
	}
	payable, err := s.createDocument(ctx, "MPDN", key, "post", "MPCH", "AP-"+key, map[string]any{"CardCode": receipt.Data["CardCode"], "DocTotal": total, "BaseEntry": key, "PaidToDate": 0})
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
	if isPostedDocument(delivery) {
		return &ActionResult{TableCode: "MDLN", Key: key, Action: "post", Status: "posted", Record: delivery}, nil
	}
	if !documentFieldEquals(delivery, "WddStatus", "A") {
		return nil, fmt.Errorf("%w: delivery must be approved before posting", ErrValidation)
	}
	lines, err := s.listChildPayloads(ctx, "MDLN", key, "DLN1")
	if err != nil {
		return nil, err
	}
	total := sumLineAmount(lines)
	goodsIssue, err := s.createDocument(ctx, "MDLN", key, "post", "MIGE", "IGE-"+key, map[string]any{"CardCode": delivery.Data["CardCode"], "DocTotal": total, "BaseEntry": key})
	if err != nil {
		return nil, err
	}
	invoice, err := s.createDocument(ctx, "MDLN", key, "post", "MINV", "INV-"+key, map[string]any{"CardCode": delivery.Data["CardCode"], "DocTotal": total, "PaidToDate": 0, "BaseEntry": key})
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
	current, err := s.repo.GetRecord(ctx, invoiceTable, key)
	if err != nil {
		return nil, err
	}
	amount := positiveNumeric(current.Data, "DocTotal", 0)
	journal, err := s.createDocument(ctx, "MINV", key, "post", "MJDT", "JE-"+key, map[string]any{"BaseEntry": key, "BtfStatus": "P", "TransType": "ar_invoice", "Memo": "A/R invoice " + key})
	if err != nil {
		return nil, err
	}
	if amount > 0 {
		if err := s.createBalancedJournalLines(ctx, journal.Key, "Accounts Receivable", "Sales Revenue", amount, "Invoice "+key); err != nil {
			return nil, err
		}
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
	journal, err := s.createDocument(ctx, "MRCT", key, "allocate", "MJDT", "JE-"+key, map[string]any{"BaseEntry": key, "BtfStatus": "P", "TransType": "incoming_payment", "Memo": "Incoming payment " + key})
	if err != nil {
		return nil, err
	}
	if err := s.createBalancedJournalLines(ctx, journal.Key, "Cash", "Accounts Receivable", amount, "Payment "+key); err != nil {
		return nil, err
	}
	payment, err := s.repo.UpdateRecord(ctx, paymentTable, key, RecordInput{Data: map[string]any{"OpenBal": currentOpenBal - amount}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MRCT", Key: key, Action: "allocate", Status: "allocated", Record: payment, GeneratedRecords: []Record{*journal}, Effects: map[string]any{"allocated_amount": amount, "target_table": targetTableCode, "target_key": targetKey}}, nil
}

func (s *Service) closePOSSale(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MRPS")
	sale, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(sale) {
		return &ActionResult{TableCode: "MRPS", Key: key, Action: "close", Status: "posted", Record: sale}, nil
	}
	lines, err := s.listChildPayloads(ctx, "MRPS", key, "RPS1")
	if err != nil {
		return nil, err
	}
	total := sumLineAmount(lines)
	issue, err := s.createDocument(ctx, "MRPS", key, "close", "MIGE", "IGE-"+key, map[string]any{"CardCode": sale.Data["CardCode"], "DocTotal": total, "BaseEntry": key})
	if err != nil {
		return nil, err
	}
	invoice, err := s.createDocument(ctx, "MRPS", key, "close", "MINV", "INV-"+key, map[string]any{"CardCode": sale.Data["CardCode"], "DocTotal": total, "PaidToDate": total, "BaseEntry": key, "DocStatus": "C"})
	if err != nil {
		return nil, err
	}
	payment, err := s.createDocument(ctx, "MRPS", key, "close", "MRCT", "RCT-"+key, map[string]any{"CardCode": sale.Data["CardCode"], "DocTotal": total, "OpenBal": 0, "BaseEntry": "INV-" + key})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), numericValue(line, "Quantity"), -1); err != nil {
			return nil, err
		}
		lineNum := fmt.Sprint(i + 1)
		if _, err := s.createDocumentLine(ctx, "MIGE", issue.Key, "IGE1", lineNum, line); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, "MINV", invoice.Key, "INV1", lineNum, line); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, "MRCT", payment.Key, "RCT1", lineNum, line); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y", "PaidToDate": total}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MRPS", Key: key, Action: "close", Status: "closed", Record: updated, GeneratedRecords: []Record{*issue, *invoice, *payment}}, nil
}

func (s *Service) autoAllocateDistributionRequest(ctx context.Context, key string) (*ActionResult, error) {
	if err := s.requireDocumentField(ctx, "MDRQ", key, "WddStatus", "A", "distribution request must be approved before allocation"); err != nil {
		return nil, err
	}
	table, _ := s.table("MDRQ")
	request, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	lines, err := s.listChildPayloads(ctx, "MDRQ", key, "DRQ1")
	if err != nil {
		return nil, err
	}
	shipmentKey := "DSP-" + key
	shipment, err := s.createDocument(ctx, "MDRQ", key, "auto-allocate", "MDSP", shipmentKey, map[string]any{"BaseEntry": key, "FromWhsCode": request.Data["FromWhsCode"], "ToWhsCode": request.Data["ToWhsCode"], "DocTotal": sumLineAmount(lines)})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		next := copyData(line)
		if stringValue(next, "FromWhsCode", "") == "" {
			next["FromWhsCode"] = request.Data["FromWhsCode"]
		}
		if stringValue(next, "ToWhsCode", "") == "" {
			next["ToWhsCode"] = request.Data["ToWhsCode"]
		}
		if _, err := s.createDocumentLine(ctx, "MDSP", shipment.Key, "DSP1", fmt.Sprint(i+1), next); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"AllocatedShipmentKey": shipmentKey, "DocStatus": "A"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MDRQ", Key: key, Action: "auto-allocate", Status: "allocated", Record: updated, GeneratedRecords: []Record{*shipment}}, nil
}

func (s *Service) shipDistribution(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MDSP")
	shipment, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(shipment) {
		return &ActionResult{TableCode: "MDSP", Key: key, Action: "ship", Status: "posted", Record: shipment}, nil
	}
	lines, err := s.listChildPayloads(ctx, "MDSP", key, "DSP1")
	if err != nil {
		return nil, err
	}
	issue, err := s.createDocument(ctx, "MDSP", key, "ship", "MIGE", "IGE-"+key, map[string]any{"BaseEntry": key, "DocTotal": sumLineAmount(lines)})
	if err != nil {
		return nil, err
	}
	receipt, err := s.createDocument(ctx, "MDSP", key, "ship", "MDRC", "DRC-"+key, map[string]any{"BaseEntry": key, "FromWhsCode": shipment.Data["FromWhsCode"], "ToWhsCode": shipment.Data["ToWhsCode"]})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		fromWhs := firstNonEmptyString(stringValue(line, "FromWhsCode", ""), stringValue(line, "WhsCode", ""), stringValue(shipment.Data, "FromWhsCode", ""))
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), fromWhs, numericValue(line, "Quantity"), -1); err != nil {
			return nil, err
		}
		lineNum := fmt.Sprint(i + 1)
		if _, err := s.createDocumentLine(ctx, "MIGE", issue.Key, "IGE1", lineNum, line); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, "MDRC", receipt.Key, "DRC1", lineNum, line); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y", "ReceiptKey": receipt.Key}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MDSP", Key: key, Action: "ship", Status: "shipped", Record: updated, GeneratedRecords: []Record{*issue, *receipt}}, nil
}

func (s *Service) receiveDistribution(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MDRC")
	receiptDoc, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(receiptDoc) {
		return &ActionResult{TableCode: "MDRC", Key: key, Action: "receive", Status: "posted", Record: receiptDoc}, nil
	}
	lines, err := s.listChildPayloads(ctx, "MDRC", key, "DRC1")
	if err != nil {
		return nil, err
	}
	receipt, err := s.createDocument(ctx, "MDRC", key, "receive", "MIGN", "IGN-"+key, map[string]any{"BaseEntry": key, "DocTotal": sumLineAmount(lines)})
	if err != nil {
		return nil, err
	}
	generated := []Record{*receipt}
	for i, line := range lines {
		toWhs := firstNonEmptyString(stringValue(line, "ToWhsCode", ""), stringValue(line, "WhsCode", ""), stringValue(receiptDoc.Data, "ToWhsCode", ""))
		receivedQty := numericValue(line, "ReceivedQuantity")
		if receivedQty == 0 {
			receivedQty = numericValue(line, "Quantity")
		}
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), toWhs, receivedQty, 1); err != nil {
			return nil, err
		}
		lineNum := fmt.Sprint(i + 1)
		if _, err := s.createDocumentLine(ctx, "MIGN", receipt.Key, "IGN1", lineNum, line); err != nil {
			return nil, err
		}
		if diff := receivedQty - numericValue(line, "Quantity"); diff != 0 {
			difference, err := s.createDocument(ctx, "MDRC", key, "receive", "MDIF", "DIF-"+key, map[string]any{"BaseEntry": key, "DifferenceQuantity": diff})
			if err != nil {
				return nil, err
			}
			if _, err := s.createDocumentLine(ctx, "MDIF", difference.Key, "DIF1", lineNum, withDifferenceQuantity(line, diff)); err != nil {
				return nil, err
			}
			generated = append(generated, *difference)
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MDRC", Key: key, Action: "receive", Status: "received", Record: updated, GeneratedRecords: generated}, nil
}

func (s *Service) resolveDistributionDifference(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MDIF")
	lines, err := s.listChildPayloads(ctx, "MDIF", key, "DIF1")
	if err != nil {
		return nil, err
	}
	generated := []Record{}
	for i, line := range lines {
		diff := numericValue(line, "DifferenceQuantity")
		if diff == 0 {
			continue
		}
		tableCode := "MIGN"
		childCode := "IGN1"
		direction := 1.0
		docKey := "IGN-" + key
		if diff < 0 {
			tableCode = "MIGE"
			childCode = "IGE1"
			direction = -1
			docKey = "IGE-" + key
			diff = -diff
		}
		doc, err := s.createDocument(ctx, "MDIF", key, "resolve", tableCode, docKey, map[string]any{"BaseEntry": key})
		if err != nil {
			return nil, err
		}
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), diff, direction); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, tableCode, doc.Key, childCode, fmt.Sprint(i+1), line); err != nil {
			return nil, err
		}
		generated = append(generated, *doc)
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MDIF", Key: key, Action: "resolve", Status: "resolved", Record: updated, GeneratedRecords: generated}, nil
}

func (s *Service) replenishFromStockPolicy(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MSTP")
	policy, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	lines, err := s.listChildPayloads(ctx, "MSTP", key, "STP1")
	if err != nil {
		return nil, err
	}
	request, err := s.createDocument(ctx, "MSTP", key, "replenish", "MDRQ", "DRQ-"+key, map[string]any{"BaseEntry": key, "FromWhsCode": policy.Data["FromWhsCode"], "ToWhsCode": policy.Data["ToWhsCode"], "DocStatus": "O"})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if _, err := s.createDocumentLine(ctx, "MDRQ", request.Key, "DRQ1", fmt.Sprint(i+1), line); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"Status": "replenished", "LastRequestKey": request.Key}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MSTP", Key: key, Action: "replenish", Status: "replenished", Record: updated, GeneratedRecords: []Record{*request}}, nil
}

func (s *Service) postStoreCountAdjustment(ctx context.Context, key string) (*ActionResult, error) {
	if err := s.requireDocumentField(ctx, "MCNT", key, "WddStatus", "A", "store count must be approved before adjustment"); err != nil {
		return nil, err
	}
	table, _ := s.table("MCNT")
	lines, err := s.listChildPayloads(ctx, "MCNT", key, "CNT1")
	if err != nil {
		return nil, err
	}
	generated := []Record{}
	for i, line := range lines {
		diff := numericValue(line, "CountedQuantity") - numericValue(line, "BookQuantity")
		if diff == 0 {
			continue
		}
		tableCode := "MIGN"
		childCode := "IGN1"
		direction := 1.0
		docKey := "IGN-" + key
		if diff < 0 {
			tableCode = "MIGE"
			childCode = "IGE1"
			direction = -1
			docKey = "IGE-" + key
			diff = -diff
		}
		doc, err := s.createDocument(ctx, "MCNT", key, "post-adjustment", tableCode, docKey, map[string]any{"BaseEntry": key})
		if err != nil {
			return nil, err
		}
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), diff, direction); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, tableCode, doc.Key, childCode, fmt.Sprint(i+1), withDifferenceQuantity(line, diff)); err != nil {
			return nil, err
		}
		generated = append(generated, *doc)
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MCNT", Key: key, Action: "post-adjustment", Status: "posted", Record: updated, GeneratedRecords: generated}, nil
}

func (s *Service) convertSpecialPurchaseRequest(ctx context.Context, key string) (*ActionResult, error) {
	if err := s.requireDocumentField(ctx, "MSPR", key, "WddStatus", "A", "special purchase request must be approved before conversion"); err != nil {
		return nil, err
	}
	table, _ := s.table("MSPR")
	request, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	lines, err := s.listChildPayloads(ctx, "MSPR", key, "SPR1")
	if err != nil {
		return nil, err
	}
	order, err := s.createDocument(ctx, "MSPR", key, "convert-to-purchase-order", "MPOR", "PO-"+key, map[string]any{"BaseEntry": key, "CardCode": request.Data["CardCode"], "DocTotal": sumLineAmount(lines)})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if _, err := s.createDocumentLine(ctx, "MPOR", order.Key, "POR1", fmt.Sprint(i+1), line); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "ConvertedOrderKey": order.Key}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MSPR", Key: key, Action: "convert-to-purchase-order", Status: "converted", Record: updated, GeneratedRecords: []Record{*order}}, nil
}

func (s *Service) approveBOM(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	lines, err := s.listChildPayloads(ctx, "MBOM", key, "BOM1")
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: BOM requires at least one component before approval", ErrValidation)
	}
	for _, line := range lines {
		if stringValue(line, "ItemCode", "") == "" || numericValue(line, "Quantity") <= 0 {
			return nil, fmt.Errorf("%w: BOM component ItemCode and positive Quantity are required", ErrValidation)
		}
	}
	data := map[string]any{"Status": "approved"}
	if approver := stringValue(input.Data, "approver", ""); approver != "" {
		data["ApprovedBy"] = approver
	}
	result, err := s.mergeStatusAction(ctx, "MBOM", key, "approve", data)
	if err != nil {
		return nil, err
	}
	result.Status = "approved"
	return result, nil
}

func (s *Service) makeWorkOrderFromBOM(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	bomTable, _ := s.table("MBOM")
	bom, err := s.repo.GetRecord(ctx, bomTable, key)
	if err != nil {
		return nil, err
	}
	if !documentFieldEquals(bom, "Status", "approved") {
		return nil, fmt.Errorf("%w: BOM must be approved before making a work order", ErrValidation)
	}
	lines, err := s.listChildPayloads(ctx, "MBOM", key, "BOM1")
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: BOM requires at least one component", ErrValidation)
	}
	workOrderKey := stringValue(input.Data, "WorkOrderCode", "WO-"+key)
	quantity := positiveNumeric(input.Data, "Quantity", positiveNumeric(bom.Data, "Quantity", 1))
	payload := map[string]any{
		"WorkOrderCode":   workOrderKey,
		"Name":            stringValue(input.Data, "Name", "Work order "+workOrderKey),
		"Status":          "planned",
		"BOMCode":         key,
		"ItemCode":        stringValue(input.Data, "ItemCode", stringValue(bom.Data, "ItemCode", "")),
		"Quantity":        quantity,
		"SourceWhsCode":   stringValue(input.Data, "SourceWhsCode", stringValue(bom.Data, "SourceWhsCode", "")),
		"WipWhsCode":      stringValue(input.Data, "WipWhsCode", stringValue(bom.Data, "WipWhsCode", "")),
		"FinishedWhsCode": stringValue(input.Data, "FinishedWhsCode", stringValue(bom.Data, "FinishedWhsCode", "")),
	}
	workOrder, err := s.createDocument(ctx, "MBOM", key, "make-work-order", "MWOR", workOrderKey, payload)
	if err != nil {
		return nil, err
	}
	if err := s.createWorkOrderRequiredLines(ctx, workOrderKey, key, bom, lines, quantity); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateRecord(ctx, bomTable, key, RecordInput{Data: map[string]any{"LastWorkOrderCode": workOrderKey}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MBOM", Key: key, Action: "make-work-order", Status: "planned", Record: updated, GeneratedRecords: []Record{*workOrder}}, nil
}

func (s *Service) releaseWorkOrder(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MWOR")
	workOrder, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if status := stringValue(workOrder.Data, "Status", ""); status == "completed" || status == "closed" {
		return nil, fmt.Errorf("%w: completed or closed work order cannot be released", ErrValidation)
	}
	lines, err := s.listChildPayloads(ctx, "MWOR", key, "WOR1")
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		bomCode := stringValue(workOrder.Data, "BOMCode", "")
		if bomCode == "" {
			return nil, fmt.Errorf("%w: BOMCode is required to release work order without required items", ErrValidation)
		}
		bomTable, _ := s.table("MBOM")
		bom, err := s.repo.GetRecord(ctx, bomTable, bomCode)
		if err != nil {
			return nil, err
		}
		if !documentFieldEquals(bom, "Status", "approved") {
			return nil, fmt.Errorf("%w: BOM must be approved before work order release", ErrValidation)
		}
		bomLines, err := s.listChildPayloads(ctx, "MBOM", bomCode, "BOM1")
		if err != nil {
			return nil, err
		}
		if err := s.createWorkOrderRequiredLines(ctx, key, bomCode, bom, bomLines, positiveNumeric(workOrder.Data, "Quantity", 1)); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"Status": "released"}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MWOR", Key: key, Action: "release", Status: "released", Record: updated}, nil
}

func (s *Service) issueWorkOrderMaterial(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MWOR")
	workOrder, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if documentFieldEquals(workOrder, "MaterialIssued", "Y") {
		return &ActionResult{TableCode: "MWOR", Key: key, Action: "issue-material", Status: "in_process", Record: workOrder}, nil
	}
	status := stringValue(workOrder.Data, "Status", "")
	if status != "released" && status != "in_process" {
		return nil, fmt.Errorf("%w: work order must be released before material issue", ErrValidation)
	}
	lines, err := s.listChildPayloads(ctx, "MWOR", key, "WOR1")
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: work order requires material lines before issue", ErrValidation)
	}
	issue, err := s.createDocument(ctx, "MWOR", key, "issue-material", "MIGE", "IGE-"+key, map[string]any{"BaseEntry": key, "DocTotal": sumLineAmount(lines)})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		whsCode := firstNonEmptyString(stringValue(line, "WhsCode", ""), stringValue(workOrder.Data, "SourceWhsCode", ""))
		if err := s.adjustInventory(ctx, stringValue(line, "ItemCode", ""), whsCode, numericValue(line, "Quantity"), -1); err != nil {
			return nil, err
		}
		next := copyData(line)
		next["WhsCode"] = whsCode
		if _, err := s.createDocumentLine(ctx, "MIGE", issue.Key, "IGE1", fmt.Sprint(i+1), next); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"Status": "in_process", "MaterialIssued": "Y", "MaterialIssueKey": issue.Key}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MWOR", Key: key, Action: "issue-material", Status: "in_process", Record: updated, GeneratedRecords: []Record{*issue}}, nil
}

func (s *Service) completeWorkOrder(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	table, _ := s.table("MWOR")
	workOrder, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(workOrder) || documentFieldEquals(workOrder, "Status", "completed") {
		return &ActionResult{TableCode: "MWOR", Key: key, Action: "complete", Status: "completed", Record: workOrder}, nil
	}
	if !documentFieldEquals(workOrder, "MaterialIssued", "Y") {
		return nil, fmt.Errorf("%w: work order material must be issued before completion", ErrValidation)
	}
	itemCode := stringValue(workOrder.Data, "ItemCode", "")
	finishedWhs := firstNonEmptyString(stringValue(input.Data, "FinishedWhsCode", ""), stringValue(workOrder.Data, "FinishedWhsCode", ""))
	quantity := positiveNumeric(input.Data, "Quantity", positiveNumeric(workOrder.Data, "Quantity", 1))
	if err := s.adjustInventory(ctx, itemCode, finishedWhs, quantity, 1); err != nil {
		return nil, err
	}
	receipt, err := s.createDocument(ctx, "MWOR", key, "complete", "MIGN", "IGN-"+key, map[string]any{"BaseEntry": key, "ItemCode": itemCode, "WhsCode": finishedWhs, "Quantity": quantity})
	if err != nil {
		return nil, err
	}
	if _, err := s.createDocumentLine(ctx, "MIGN", receipt.Key, "IGN1", "1", map[string]any{"ItemCode": itemCode, "WhsCode": finishedWhs, "Quantity": quantity}); err != nil {
		return nil, err
	}
	journal, err := s.createDocument(ctx, "MWOR", key, "complete", "MJDT", "JE-"+key, map[string]any{"BaseEntry": key, "BtfStatus": "P", "TransType": "production", "Memo": "Production completion " + key})
	if err != nil {
		return nil, err
	}
	lines, err := s.listChildPayloads(ctx, "MWOR", key, "WOR1")
	if err != nil {
		return nil, err
	}
	amount := sumLineAmount(lines)
	if amount <= 0 {
		amount = quantity
	}
	if err := s.createBalancedJournalLines(ctx, journal.Key, "Finished Goods Inventory", "Work In Process", amount, "Production "+key); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"Status": "completed", "Posted": "Y", "ProducedQuantity": quantity, "GoodsReceiptKey": receipt.Key, "JournalEntryKey": journal.Key}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MWOR", Key: key, Action: "complete", Status: "completed", Record: updated, GeneratedRecords: []Record{*receipt, *journal}}, nil
}

func (s *Service) createWorkOrderRequiredLines(ctx context.Context, workOrderKey string, bomCode string, bom *Record, bomLines []map[string]any, productionQuantity float64) error {
	bomQuantity := positiveNumeric(bom.Data, "Quantity", 1)
	for i, line := range bomLines {
		quantity := numericValue(line, "Quantity")
		if quantity <= 0 || stringValue(line, "ItemCode", "") == "" {
			return fmt.Errorf("%w: BOM component ItemCode and positive Quantity are required", ErrValidation)
		}
		next := copyData(line)
		requiredQuantity := quantity * productionQuantity / bomQuantity
		next["Quantity"] = requiredQuantity
		next["BOMCode"] = bomCode
		if stringValue(next, "WhsCode", "") == "" {
			next["WhsCode"] = stringValue(bom.Data, "SourceWhsCode", "")
		}
		if payload, ok := next["Payload"].(map[string]any); ok {
			nested := copyData(payload)
			nested["Quantity"] = requiredQuantity
			nested["BOMCode"] = bomCode
			nested["WhsCode"] = next["WhsCode"]
			next["Payload"] = nested
		}
		if _, err := s.createDocumentLine(ctx, "MWOR", workOrderKey, "WOR1", fmt.Sprint(i+1), next); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) postInventoryDocument(ctx context.Context, tableCode string, key string, direction float64) (*ActionResult, error) {
	table, _ := s.table(tableCode)
	current, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(current) {
		return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: current}, nil
	}
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

func (s *Service) runTrialBalance(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	journalTable, _ := s.table("MJDT")
	journals, err := s.repo.ListRecords(ctx, journalTable, 500)
	if err != nil {
		return nil, err
	}
	totalDebit := 0.0
	totalCredit := 0.0
	journalCount := 0
	for _, journal := range journals {
		if !documentFieldEquals(&journal, "BtfStatus", "P") && !documentFieldEquals(&journal, "Posted", "Y") {
			continue
		}
		lines, err := s.listChildPayloads(ctx, "MJDT", journal.Key, "JDT1")
		if err != nil {
			return nil, err
		}
		journalDebit := 0.0
		journalCredit := 0.0
		for _, line := range lines {
			journalDebit += numericValue(line, "Debit")
			journalCredit += numericValue(line, "Credit")
		}
		if journalDebit != journalCredit {
			return nil, fmt.Errorf("%w: journal entry %s is not balanced", ErrValidation, journal.Key)
		}
		if journalDebit == 0 && journalCredit == 0 {
			continue
		}
		totalDebit += journalDebit
		totalCredit += journalCredit
		journalCount++
	}
	reportCode := stringValue(input.Data, "ReportCode", key)
	if reportCode == "" {
		reportCode = key
	}
	data := map[string]any{
		"ReportCode":    reportCode,
		"Currency":      stringValue(input.Data, "Currency", "CNY"),
		"PeriodStart":   input.Data["PeriodStart"],
		"PeriodEnd":     input.Data["PeriodEnd"],
		"TotalDebit":    totalDebit,
		"TotalCredit":   totalCredit,
		"JournalCount":  journalCount,
		"Payload":       map[string]any{"JournalCount": journalCount},
		"LastRunStatus": "balanced",
	}
	reportTable, _ := s.table("MGLR")
	report, err := s.repo.GetRecord(ctx, reportTable, reportCode)
	if err == ErrNotFound {
		report, err = s.repo.CreateRecord(ctx, reportTable, RecordInput{Key: reportCode, Data: data})
	} else if err == nil {
		report, err = s.repo.UpdateRecord(ctx, reportTable, reportCode, RecordInput{Data: data})
	}
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MGLR", Key: reportCode, Action: "run", Status: "balanced", Record: report, Effects: map[string]any{"journal_count": journalCount}}, nil
}

func (s *Service) createBalancedJournalLines(ctx context.Context, journalKey string, debitAccount string, creditAccount string, amount float64, description string) error {
	if amount <= 0 {
		return fmt.Errorf("%w: positive journal amount is required", ErrValidation)
	}
	if _, err := s.createDocumentLine(ctx, "MJDT", journalKey, "JDT1", "1", map[string]any{
		"AccountCode": debitAccount,
		"AccountName": debitAccount,
		"Debit":       amount,
		"Credit":      0.0,
		"Description": description,
	}); err != nil {
		return err
	}
	if _, err := s.createDocumentLine(ctx, "MJDT", journalKey, "JDT1", "2", map[string]any{
		"AccountCode": creditAccount,
		"AccountName": creditAccount,
		"Debit":       0.0,
		"Credit":      amount,
		"Description": description,
	}); err != nil {
		return err
	}
	return nil
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

func withDifferenceQuantity(line map[string]any, quantity float64) map[string]any {
	next := copyData(line)
	next["DifferenceQuantity"] = quantity
	return next
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

func positiveNumeric(values map[string]any, key string, fallback float64) float64 {
	value := numericValue(values, key)
	if value <= 0 {
		return fallback
	}
	return value
}
