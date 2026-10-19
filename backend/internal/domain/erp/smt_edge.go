package erp

import (
	"context"
	"fmt"
	"strings"
)

// SMT edge event ingestion (bridge from SMT-MES reference instances).
//
// ApplyEdgeEvent mirrors shop-floor execution reported by a connected SMT-MES
// instance into the platform code tables. The edge system is authoritative for
// the events it reports (it enforced operator permissions and foolproofing
// locally), so mirror writes use deterministic keys and skip the interactive
// validation chain. Duplicate deliveries are rejected by the caller through the
// smt_edge_events unique constraint and by the action ledger idempotency key.

// EdgeEvent is one SMT-MES integration event (see docs/INTEGRATION.md in the
// SMT-MES repository).
type EdgeEvent struct {
	ID   int64          `json:"id"`
	Type string         `json:"type"`
	Time string         `json:"time"`
	Ref  string         `json:"ref"`
	Data map[string]any `json:"data"`
}

// ApplyEdgeEvent maps one edge event into the platform tables and returns the
// status string ("processed" or "skipped").
func (s *Service) ApplyEdgeEvent(ctx context.Context, instanceKey string, event EdgeEvent) (string, error) {
	prefix := "EDGE-" + shortInstance(instanceKey)
	data := event.Data
	if data == nil {
		data = map[string]any{}
	}
	now := smtNowISO()
	eventKey := fmt.Sprintf("%s-%d", prefix, event.ID)
	actor := "smt_edge:" + shortInstance(instanceKey)

	switch event.Type {
	case "work_order.created", "work_order.updated", "work_order.started", "work_order.progress", "work_order.finished", "work_order.closed":
		woNo := stringValue(data, "wo_no", event.Ref)
		if woNo == "" {
			return "skipped", nil
		}
		update := map[string]any{
			"product_code": stringValue(data, "product_code", ""),
			"line_code":    stringValue(data, "line_code", ""),
			"plan_qty":     numericValue(data, "plan_qty"),
			"done_qty":     numericValue(data, "done_qty"),
			"customer_code": stringValue(data, "customer_code", ""),
			"Status":       strings.ToUpper(stringValue(data, "status", "RUNNING")),
			"status":       strings.ToUpper(stringValue(data, "status", "RUNNING")),
			"edge_synced_at": now,
		}
		if started := stringValue(data, "started_at", ""); started != "" {
			update["started_at"] = started
		}
		if finished := stringValue(data, "finished_at", ""); finished != "" {
			update["finished_at"] = finished
		}
		if _, err := s.smtUpsertRecord(ctx, "MSWO", woNo, update); err != nil {
			return "", err
		}
		return "processed", nil

	case "machine.bound":
		machineCode := stringValue(data, "machine_code", event.Ref)
		if machineCode == "" {
			return "skipped", nil
		}
		if _, err := s.smtUpsertRecord(ctx, "MMAC", machineCode, map[string]any{
			"current_work_order": stringValue(data, "wo_no", ""),
			"verified_at":        "", "verified_by": "", "signed_off_at": "", "signed_off_by": "",
			"changeover_at": now, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "machine.verified":
		machineCode := stringValue(data, "machine_code", event.Ref)
		if machineCode == "" {
			return "skipped", nil
		}
		verifiedAt := now
		if ok, _ := data["ok"].(bool); !ok {
			verifiedAt = ""
		}
		if _, err := s.smtUpsertRecord(ctx, "MMAC", machineCode, map[string]any{
			"verified_at": verifiedAt, "verified_by": actor, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "machine.verify_failed":
		machineCode := stringValue(data, "machine_code", event.Ref)
		if machineCode == "" {
			return "skipped", nil
		}
		if _, err := s.smtUpsertRecord(ctx, "MMAC", machineCode, map[string]any{
			"verified_at": "", "verified_by": "", "signed_off_at": "", "signed_off_by": "",
			"last_verify_at": now, "last_verify_issues": data["issues"], "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "machine.signed_off":
		machineCode := stringValue(data, "machine_code", event.Ref)
		if machineCode == "" {
			return "skipped", nil
		}
		if _, err := s.smtUpsertRecord(ctx, "MMAC", machineCode, map[string]any{
			"signed_off_at": now, "signed_off_by": stringValue(data, "signer", actor), "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "loading.ok":
		return s.edgeLoadingOK(ctx, prefix, eventKey, data, actor, now)
	case "loading.error":
		return s.edgeLoadingError(ctx, prefix, eventKey, data)
	case "loading.unloaded":
		return s.edgeLoadingUnloaded(ctx, data, now)

	case "material.low":
		machineCode := stringValue(data, "machine_code", "")
		woNo := stringValue(data, "wo_no", "")
		for _, alert := range mapSliceAny(data["alerts"]) {
			slotNo := stringValue(alert, "slot_no", "")
			if machineCode == "" || slotNo == "" {
				continue
			}
			existing, err := s.smtQueryAll(ctx, "MLWA", map[string]any{
				"machine_code": machineCode, "slot_no": slotNo, "status": "OPEN",
			}, 2)
			if err != nil {
				return "", err
			}
			payload := map[string]any{
				"work_order": woNo, "machine_code": machineCode, "slot_no": slotNo,
				"part_no": stringValue(alert, "part_no", ""), "reel_code": stringValue(alert, "reel_uid", ""),
				"qty_est": numericValue(alert, "qty_est"), "boards_left": numericValue(alert, "boards_left"),
				"status": "OPEN", "Status": "OPEN", "raised_at": now, "edge_synced_at": now,
			}
			if len(existing) > 0 {
				if _, err := s.smtUpsertRecord(ctx, "MLWA", existing[0].Key, payload); err != nil {
					return "", err
				}
				continue
			}
			if _, err := s.smtUpsertRecord(ctx, "MLWA", eventKey+"-"+slotNo, payload); err != nil {
				return "", err
			}
		}
		return "processed", nil

	case "reel.received":
		for _, reel := range mapSliceAny(data["reels"]) {
			uid := stringValue(reel, "uid", "")
			if uid == "" {
				continue
			}
			if _, err := s.smtUpsertRecord(ctx, "MREL", uid, map[string]any{
				"part_no":     stringValue(data, "part_no", stringValue(reel, "part_no", "")),
				"qty":         numericValue(reel, "qty"),
				"initial_qty": numericValue(reel, "qty"),
				"lot_no":      stringValue(reel, "lot_no", ""),
				"supplier":    stringValue(reel, "supplier", ""),
				"location":    stringValue(reel, "location", ""),
				"status":      "IN_STOCK", "Status": "active",
				"owner_type":  "SELF",
				"received_at": now, "edge_synced_at": now,
			}); err != nil {
				return "", err
			}
		}
		return "processed", nil

	case "reel.issued", "reel.returned", "reel.empty", "reel.scrapped", "reel.lost", "reel.returned_to_customer", "reel.moved":
		uid := stringValue(data, "uid", event.Ref)
		if uid == "" {
			return "skipped", nil
		}
		update := map[string]any{
			"part_no": stringValue(data, "part_no", ""),
			"qty":     numericValue(data, "qty"),
			"status":  strings.ToUpper(stringValue(data, "status", edgeReelStatusForEvent(event.Type))),
			"edge_synced_at": now,
		}
		if issuedWO := stringValue(data, "issued_wo_no", ""); issuedWO != "" {
			update["issued_wo"] = issuedWO
		}
		if location := stringValue(data, "location", ""); location != "" {
			update["location"] = location
		}
		if position := stringValue(data, "position_barcode", ""); position != "" {
			update["position"] = position
		}
		if event.Type == "reel.moved" {
			update["status"] = "IN_STOCK"
		}
		if _, err := s.smtUpsertRecord(ctx, "MREL", uid, update); err != nil {
			return "", err
		}
		return "processed", nil

	case "requisition.created", "requisition.closed":
		reqNo := stringValue(data, "req_no", event.Ref)
		if reqNo == "" {
			return "skipped", nil
		}
		status := "OPEN"
		if event.Type == "requisition.closed" {
			status = strings.ToUpper(stringValue(data, "status", "CLOSED"))
			if status != "CLOSED" && status != "CANCELLED" {
				status = "CLOSED"
			}
		}
		if _, err := s.smtUpsertRecord(ctx, "MRQN", reqNo, map[string]any{
			"work_order": stringValue(data, "wo_no", ""),
			"batch_qty":  numericValue(data, "batch_qty"),
			"unit_basis": strings.ToUpper(stringValue(data, "unit_basis", "QTY")),
			"Status":     status, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		if event.Type == "requisition.created" {
			for _, item := range mapSliceAny(data["items"]) {
				partNo := stringValue(item, "part_no", "")
				if partNo == "" {
					continue
				}
				if err := s.smtAppendChild(ctx, "MRQN", reqNo, "RQN1", map[string]any{
					"PartNo": partNo, "RequiredQty": numericValue(item, "required_qty"),
					"IssuedQty": numericValue(item, "issued_qty"), "LineStatus": "O",
				}); err != nil {
					return "", err
				}
			}
		}
		return "processed", nil

	case "unit.passed", "unit.failed", "unit.rejected", "unit.finished", "unit.repaired", "unit.scrapped":
		return s.edgeUnitEvent(ctx, event.Type, data, now, actor)

	case "panel.bound":
		panelCode := stringValue(data, "panel_code", event.Ref)
		if panelCode == "" {
			return "skipped", nil
		}
		if _, err := s.smtUpsertRecord(ctx, "MPNL", panelCode, map[string]any{
			"work_order":  stringValue(data, "wo_no", ""),
			"board_count": numericValue(data, "board_count"),
			"status":      "ACTIVE", "Status": "active", "created_at": now, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		for index, board := range mapSliceAny(data["boards"]) {
			sn := stringValue(board, "sn", "")
			if sn == "" {
				continue
			}
			pos := int(numericValue(board, "pos"))
			if pos <= 0 {
				pos = index + 1
			}
			if err := s.smtAppendChild(ctx, "MPNL", panelCode, "PNL1", map[string]any{"pos": pos, "sn": sn}); err != nil {
				return "", err
			}
			if unit, _ := s.smtGetRecord(ctx, "MUNT", sn); unit != nil {
				if _, err := s.smtUpsertRecord(ctx, "MUNT", sn, map[string]any{"panel_code": panelCode}); err != nil {
					return "", err
				}
			}
		}
		return "processed", nil

	case "panel.split":
		panelCode := stringValue(data, "panel_code", event.Ref)
		if panelCode == "" {
			return "skipped", nil
		}
		if _, err := s.smtUpsertRecord(ctx, "MPNL", panelCode, map[string]any{
			"status": "SPLIT", "Status": "split", "split_at": now, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "downtime.started":
		machineCode := stringValue(data, "machine_code", event.Ref)
		lineCode := stringValue(data, "line_code", "")
		if machineCode == "" && lineCode == "" {
			return "skipped", nil
		}
		startedAt := stringValue(data, "started_at", now)
		key := fmt.Sprintf("DT-%s-%s", firstNonEmptyString(machineCode, lineCode), strings.NewReplacer(":", "", "-", "", "T", "", "Z", "").Replace(startedAt))
		if _, err := s.smtUpsertRecord(ctx, "MDWN", key, map[string]any{
			"machine_code": machineCode, "line_code": lineCode,
			"work_order": stringValue(data, "wo_no", ""),
			"reason_type": strings.ToUpper(stringValue(data, "reason_type", "OTHER")),
			"reason":      stringValue(data, "reason", ""),
			"planned":     isTruthy(data["planned"]),
			"source":      strings.ToUpper(stringValue(data, "source", "MANUAL")),
			"started_at":  startedAt, "started_by": actor,
			"ended_at": "", "remark": "", "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "downtime.ended":
		machineCode := stringValue(data, "machine_code", event.Ref)
		endedAt := stringValue(data, "ended_at", now)
		if machineCode == "" {
			return "skipped", nil
		}
		open, err := s.smtQueryAll(ctx, "MDWN", map[string]any{"machine_code": machineCode}, 50)
		if err != nil {
			return "", err
		}
		updated := false
		for index := range open {
			record := &open[index]
			if stringValue(record.Data, "ended_at", "") != "" {
				continue
			}
			if _, err := s.smtUpsertRecord(ctx, "MDWN", record.Key, map[string]any{"ended_at": endedAt, "ended_by": actor}); err != nil {
				return "", err
			}
			updated = true
		}
		if !updated {
			return "skipped", nil
		}
		return "processed", nil

	case "inspection.recorded":
		inspNo := stringValue(data, "insp_no", event.Ref)
		if inspNo == "" {
			return "skipped", nil
		}
		if _, err := s.smtUpsertRecord(ctx, "MINS", inspNo, map[string]any{
			"insp_type": strings.ToUpper(stringValue(data, "insp_type", "FAI")),
			"work_order": stringValue(data, "wo_no", ""),
			"product_code": stringValue(data, "product_code", ""),
			"sample_sn":    stringValue(data, "sample_sn", ""),
			"result":       strings.ToUpper(stringValue(data, "result", "PASS")),
			"items":        data["items"],
			"inspector":    stringValue(data, "inspector", actor),
			"created_at":   now, "Status": "closed", "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "andon.raised", "andon.acked", "andon.closed":
		callNo := stringValue(data, "call_no", event.Ref)
		if callNo == "" {
			return "skipped", nil
		}
		switch event.Type {
		case "andon.raised":
			if _, err := s.smtAndonRaise(ctx, callNo, ActionInput{Data: map[string]any{
				"line_code": stringValue(data, "line_code", ""), "machine_code": stringValue(data, "machine_code", ""),
				"work_order": stringValue(data, "wo_no", ""),
				"call_type":  strings.ToUpper(stringValue(data, "call_type", "OTHER")),
				"description": stringValue(data, "description", ""),
				"source":     strings.ToUpper(stringValue(data, "source", "MANUAL")),
				"ref":        stringValue(data, "ref", ""),
			}}); err != nil {
				return "", err
			}
		case "andon.acked":
			// Best-effort: out-of-order retries may deliver ack before raise.
			_, _ = s.smtAndonTransition(ctx, callNo, "ack", ActionInput{})
		case "andon.closed":
			_, _ = s.smtAndonTransition(ctx, callNo, "close", ActionInput{Data: map[string]any{
				"cause": stringValue(data, "cause", ""), "solution": stringValue(data, "solution", ""),
			}})
		}
		if callNo != "" {
			if _, err := s.smtUpsertRecord(ctx, "MAND", callNo, map[string]any{"edge_synced_at": now}); err != nil {
				return "", err
			}
		}
		return "processed", nil

	case "ping":
		return "skipped", nil

	default:
		return "skipped", nil
	}
}

func shortInstance(instanceKey string) string {
	instanceKey = strings.ReplaceAll(instanceKey, "-", "")
	if len(instanceKey) > 8 {
		return instanceKey[:8]
	}
	return instanceKey
}

func edgeReelStatusForEvent(eventType string) string {
	switch eventType {
	case "reel.issued":
		return "ISSUED"
	case "reel.returned":
		return "IN_STOCK"
	case "reel.empty":
		return "EMPTY"
	case "reel.scrapped":
		return "SCRAPPED"
	case "reel.lost":
		return "LOST"
	case "reel.returned_to_customer":
		return "RETURNED"
	}
	return "IN_STOCK"
}

func (s *Service) edgeLoadingOK(ctx context.Context, prefix, eventKey string, data map[string]any, actor, now string) (string, error) {
	machineCode := stringValue(data, "machine_code", "")
	slotNo := stringValue(data, "slot_no", "")
	reelUID := stringValue(data, "reel_uid", "")
	if machineCode == "" || slotNo == "" || reelUID == "" {
		return "skipped", nil
	}
	// Mirror the reel state first.
	if reel, _ := s.smtGetRecord(ctx, "MREL", reelUID); reel != nil {
		if _, err := s.smtUpsertRecord(ctx, "MREL", reelUID, map[string]any{
			"status": "LOADED", "current_slot": machineCode + "-" + slotNo,
			"qty": numericValue(data, "qty"), "loaded_at": now, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
	} else {
		if _, err := s.smtUpsertRecord(ctx, "MREL", reelUID, map[string]any{
			"part_no": stringValue(data, "part_no", ""), "qty": numericValue(data, "qty"),
			"lot_no": stringValue(data, "lot_no", ""), "status": "LOADED",
			"current_slot": machineCode + "-" + slotNo, "received_at": now, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
	}
	// Existing active loading for the same slot + reel is idempotent.
	active, err := s.smtQueryAll(ctx, "MLOD", map[string]any{"machine_code": machineCode, "slot_no": slotNo, "status": "LOADED"}, 5)
	if err != nil {
		return "", err
	}
	for index := range active {
		if stringValue(active[index].Data, "reel_code", "") == reelUID {
			return "processed", nil
		}
	}
	position := "MAIN"
	if isTruthy(data["spliced"]) {
		position = "SPLICED"
	}
	loadCode := eventKey
	if _, err := s.smtUpsertRecord(ctx, "MLOD", loadCode, map[string]any{
		"work_order": stringValue(data, "wo_no", ""), "machine_code": machineCode,
		"slot_no": slotNo, "slot_code": stringValue(data, "slot_barcode", stringValue(data, "slot_code", "")),
		"reel_code": reelUID, "part_no": stringValue(data, "part_no", ""),
		"required_part_no": stringValue(data, "required_part_no", ""),
		"is_alternate":     isTruthy(data["is_alternate"]),
		"position":         position, "status": "LOADED",
		"qty_at_load": numericValue(data, "qty"), "loaded_by": actor, "loaded_at": now,
		"Status": "active", "source": "EDGE", "edge_event_id": eventKey,
	}); err != nil {
		return "", err
	}
	if err := s.smtAppendChild(ctx, "MLOD", loadCode, "LOD1", map[string]any{
		"action": "LOAD", "reel_code": reelUID, "part_no": stringValue(data, "part_no", ""),
		"operator": actor, "created_at": now,
	}); err != nil {
		return "", err
	}
	if err := s.smtResolveLowAlerts(ctx, machineCode, slotNo, "LOAD"); err != nil {
		return "", err
	}
	// Record the scan for verification statistics.
	if _, err := s.smtUpsertRecord(ctx, "MVER", prefix+"-SS-"+eventKey, map[string]any{
		"work_order": stringValue(data, "wo_no", ""), "machine_code": machineCode,
		"slot_no": slotNo, "reel_code": reelUID,
		"required_part_no": stringValue(data, "required_part_no", ""), "actual_part_no": stringValue(data, "part_no", ""),
		"result": "OK", "error_type": "", "message": "", "operator": actor,
		"created_at": now, "source": "EDGE", "Status": "closed",
	}); err != nil {
		return "", err
	}
	return "processed", nil
}

func (s *Service) edgeLoadingError(ctx context.Context, prefix, eventKey string, data map[string]any) (string, error) {
	if _, err := s.smtUpsertRecord(ctx, "MVER", prefix+"-SS-"+eventKey, map[string]any{
		"work_order": stringValue(data, "wo_no", ""), "machine_code": stringValue(data, "machine_code", ""),
		"slot_code": stringValue(data, "slot_code", ""), "slot_no": stringValue(data, "slot_no", ""),
		"reel_code": stringValue(data, "reel_code", ""),
		"required_part_no": stringValue(data, "required_part_no", ""), "actual_part_no": stringValue(data, "actual_part_no", ""),
		"result": "ERROR", "error_type": stringValue(data, "error_type", ""),
		"message": stringValue(data, "message", ""), "voice": stringValue(data, "error_type_cn", ""),
		"operator": stringValue(data, "operator", ""), "created_at": smtNowISO(),
		"source": "EDGE", "Status": "closed",
	}); err != nil {
		return "", err
	}
	return "processed", nil
}

func (s *Service) edgeLoadingUnloaded(ctx context.Context, data map[string]any, now string) (string, error) {
	machineCode := stringValue(data, "machine_code", "")
	slotNo := stringValue(data, "slot_no", "")
	reelUID := stringValue(data, "reel_uid", "")
	if machineCode == "" || slotNo == "" {
		return "skipped", nil
	}
	active, err := s.smtQueryAll(ctx, "MLOD", map[string]any{"machine_code": machineCode, "slot_no": slotNo, "status": "LOADED"}, 5)
	if err != nil {
		return "", err
	}
	remaining := numericValue(data, "remaining_qty")
	for index := range active {
		loading := &active[index]
		if reelUID != "" && stringValue(loading.Data, "reel_code", "") != reelUID {
			continue
		}
		if _, err := s.smtUnloadRecord(ctx, loading, map[string]any{
			"reason":        strings.ToUpper(stringValue(data, "reason", "OTHER")),
			"qty_at_unload": remaining,
		}, ActionInput{}); err != nil {
			return "", err
		}
		return "processed", nil
	}
	return "skipped", nil
}

func (s *Service) edgeUnitEvent(ctx context.Context, eventType string, data map[string]any, now, actor string) (string, error) {
	sn := stringValue(data, "sn", "")
	if sn == "" {
		return "skipped", nil
	}
	workOrder := stringValue(data, "wo_no", "")
	operationCode := stringValue(data, "operation_code", "")
	seq := int(numericValue(data, "seq"))
	unit, err := s.smtGetRecord(ctx, "MUNT", sn)
	if err != nil {
		return "", err
	}
	switch eventType {
	case "unit.passed", "unit.finished":
		status := "IN_PROCESS"
		update := map[string]any{
			"work_order": workOrder, "product_code": stringValue(data, "product_code", ""),
			"status": status, "last_seq": seq, "updated_at": now, "edge_synced_at": now,
		}
		if isTruthy(data["finished"]) || eventType == "unit.finished" {
			update["status"] = "FINISHED"
			update["finished_at"] = now
		}
		if unit == nil {
			update["created_at"] = now
		}
		if _, err := s.smtUpsertRecord(ctx, "MUNT", sn, update); err != nil {
			return "", err
		}
		if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
			"seq": seq, "operation_code": operationCode, "result": "PASS",
			"work_order": workOrder, "source": strings.ToUpper(stringValue(data, "source", "MANUAL")),
			"operator": actor, "created_at": now,
		}); err != nil {
			return "", err
		}
		if isTruthy(data["finished"]) || eventType == "unit.finished" {
			if order, _ := s.smtGetRecord(ctx, "MSWO", workOrder); order != nil {
				count, err := s.smtCountUnits(ctx, workOrder, "FINISHED")
				if err != nil {
					return "", err
				}
				updated, err := s.smtUpsertRecord(ctx, "MSWO", workOrder, map[string]any{"done_qty": count, "progress_at": now})
				if err != nil {
					return "", err
				}
				if _, err := s.smtSyncLowMaterialAlerts(ctx, updated, ActionInput{}); err != nil {
					return "", err
				}
			}
		}
		return "processed", nil

	case "unit.failed":
		failCount := 1
		if unit != nil {
			failCount = int(float64FromAny(unit.Data["fail_count"])) + 1
		}
		if _, err := s.smtUpsertRecord(ctx, "MUNT", sn, map[string]any{
			"work_order": workOrder, "product_code": stringValue(data, "product_code", ""),
			"status": "NG", "ng_step": seq, "ng_operation": operationCode,
			"fail_count": failCount, "updated_at": now, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
			"seq": seq, "operation_code": operationCode, "result": "FAIL",
			"work_order": workOrder, "source": "MANUAL", "operator": actor, "created_at": now,
		}); err != nil {
			return "", err
		}
		for _, defect := range mapSliceAny(data["defects"]) {
			defectKey := smtGenerateKey("DF")
			if _, err := s.smtUpsertRecord(ctx, "MUDF", defectKey, map[string]any{
				"sn": sn, "work_order": workOrder, "step_seq": seq, "operation_code": operationCode,
				"defect_code": stringValue(defect, "code", ""), "defect_name": stringValue(defect, "name", ""),
				"location": stringValue(defect, "location", ""), "part_no": stringValue(defect, "part_no", ""),
				"status": "OPEN", "Status": "OPEN", "created_by": actor, "created_at": now, "edge_synced_at": now,
			}); err != nil {
				return "", err
			}
		}
		return "processed", nil

	case "unit.rejected":
		if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
			"seq": seq, "operation_code": operationCode, "result": "REJECT",
			"reject_type": stringValue(data, "reject_type", ""), "message": stringValue(data, "message", ""),
			"work_order": workOrder, "source": "MANUAL", "operator": actor, "created_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "unit.repaired":
		openPage, err := s.QueryRecords(ctx, "MUDF", RecordQuery{Filters: map[string]any{"sn": sn, "status": "OPEN"}, Limit: 1})
		if err != nil {
			return "", err
		}
		update := map[string]any{"repaired_at": now, "edge_synced_at": now}
		if int(openPage.Total) == 0 {
			update["status"] = "IN_PROCESS"
			update["ng_step"] = 0
		}
		if _, err := s.smtUpsertRecord(ctx, "MUNT", sn, update); err != nil {
			return "", err
		}
		if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
			"seq": seq, "operation_code": operationCode, "result": "REPAIR",
			"action": stringValue(data, "action", ""), "work_order": workOrder,
			"operator": actor, "created_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil

	case "unit.scrapped":
		if _, err := s.smtUpsertRecord(ctx, "MUNT", sn, map[string]any{
			"status": "SCRAPPED", "scrapped_at": now, "edge_synced_at": now,
		}); err != nil {
			return "", err
		}
		if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
			"seq": seq, "operation_code": operationCode, "result": "SCRAP",
			"work_order": workOrder, "operator": actor, "created_at": now,
		}); err != nil {
			return "", err
		}
		return "processed", nil
	}
	return "skipped", nil
}
