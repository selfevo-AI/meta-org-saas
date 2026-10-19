package erp

import (
	"context"
	"fmt"
	"strings"
)

// SMT feeder-verification loading chain (MLOD:load / MLOD:unload).
//
// The validation order mirrors the SMT-MES reference system:
// work order state -> slot/machine -> slot lock -> feeder setup table ->
// reel resolution -> reel state -> customer policy -> issued-to-work-order ->
// part/alternate match -> slot occupancy (confirm / splice / replace).

// smtScanResult builds the persisted scan outcome (MVER row) plus the action
// result. Scan rejections are business outcomes, not errors: the transaction
// commits so the verification log survives for traceability and reporting.
func (s *Service) smtScanResult(ctx context.Context, tableCode, key, action string, scan map[string]any, input ActionInput, extra map[string]any) (*ActionResult, error) {
	scanCode := smtGenerateKey("SS")
	payload := map[string]any{
		"work_order": stringValue(scan, "work_order", ""),
		"machine_code": stringValue(scan, "machine_code", ""),
		"slot_code":    stringValue(scan, "slot_code", ""),
		"slot_no":      stringValue(scan, "slot_no", ""),
		"reel_code":    stringValue(scan, "reel_code", ""),
		"required_part_no": stringValue(scan, "required_part_no", ""),
		"actual_part_no":   stringValue(scan, "actual_part_no", ""),
		"result":       stringValue(scan, "result", "OK"),
		"error_type":   stringValue(scan, "error_type", ""),
		"message":      stringValue(scan, "message", ""),
		"voice":        stringValue(scan, "voice", ""),
		"operator":     smtActor(ctx, input),
		"created_at":   smtNowISO(),
		"Status":       "closed",
	}
	if _, err := s.smtUpsertRecord(ctx, "MVER", scanCode, payload); err != nil {
		return nil, err
	}
	effects := map[string]any{
		"result":       payload["result"],
		"error_type":   payload["error_type"],
		"message":      payload["message"],
		"voice":        payload["voice"],
		"scan_code":    scanCode,
		"slot_no":      payload["slot_no"],
		"reel_code":    payload["reel_code"],
	}
	for name, value := range extra {
		effects[name] = value
	}
	return &ActionResult{
		TableCode: tableCode, Key: key, Action: action,
		Status:           stringValue(scan, "result", "OK"),
		GeneratedRecords: []Record{smtGenerated("MVER", scanCode, "created", map[string]any{"result": payload["result"], "error_type": payload["error_type"]})},
		Effects:          effects,
	}, nil
}

// smtLoadReject records a scan rejection with the standard message table.
func (s *Service) smtLoadReject(ctx context.Context, key string, input ActionInput, machineCode, workOrder, slotCode, slotNo, reelCode, requiredPart, actualPart, errorType, extraMessage string) (*ActionResult, error) {
	item := smtLoadErrorFor(errorType)
	message := item.Message
	if extraMessage != "" {
		message = extraMessage
	}
	scan := map[string]any{
		"work_order": workOrder, "machine_code": machineCode,
		"slot_code": slotCode, "slot_no": slotNo, "reel_code": reelCode,
		"required_part_no": requiredPart, "actual_part_no": actualPart,
		"result": "ERROR", "error_type": errorType,
		"message": message, "voice": item.Voice,
	}
	return s.smtScanResult(ctx, "MLOD", key, "load", scan, input, nil)
}

func (s *Service) smtLoad(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	machineCode := strings.TrimSpace(stringValue(input.Data, "machine_code", ""))
	if machineCode == "" {
		return nil, fmt.Errorf("%w: machine_code is required", ErrValidation)
	}
	slotCode := strings.TrimSpace(stringValue(input.Data, "slot_code", stringValue(input.Data, "slot_no", "")))
	reelCode := strings.TrimSpace(stringValue(input.Data, "reel_code", ""))
	if slotCode == "" || reelCode == "" {
		return nil, fmt.Errorf("%w: slot_code and reel_code are required", ErrValidation)
	}

	machine, err := s.smtGetRecord(ctx, "MMAC", machineCode)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return s.smtLoadReject(ctx, key, input, machineCode, "", slotCode, "", reelCode, "", "", "MACHINE_NOT_FOUND", "")
	}
	workOrderCode := stringValue(machine.Data, "current_work_order", "")
	if workOrderCode == "" {
		return s.smtLoadReject(ctx, key, input, machineCode, "", slotCode, "", reelCode, "", "", "WO_NOT_RUNNING", "机台未绑定工单")
	}
	order, err := s.smtGetRecord(ctx, "MSWO", workOrderCode)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, "", reelCode, "", "", "WO_NOT_FOUND", "")
	}
	if smtStatusOf(order) != "RUNNING" {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, "", reelCode, "", "", "WO_NOT_RUNNING", "")
	}

	// Slot resolution and lock state.
	slotRow, err := s.smtFindSlot(ctx, machineCode, slotCode)
	if err != nil {
		return nil, err
	}
	if slotRow == nil {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, "", reelCode, "", "", "SLOT_NOT_FOUND", "")
	}
	slotNo := stringValue(slotRow, "SlotNo", "")
	if boolPayload(slotRow, "Locked") {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, "", "", "SLOT_LOCKED", "")
	}

	// Feeder setup table for the machine + product.
	productCode := stringValue(order.Data, "product_code", "")
	setup, setupRows, err := s.smtActiveFeederSetup(ctx, productCode, machineCode)
	if err != nil {
		return nil, err
	}
	if setup == nil || len(setupRows) == 0 {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, "", "", "NO_STATION_TABLE", "")
	}
	var requiredRow map[string]any
	for _, row := range setupRows {
		if strings.EqualFold(stringValue(row, "SlotNo", ""), slotNo) {
			requiredRow = row
			break
		}
	}
	if requiredRow == nil {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, "", "", "SLOT_NOT_IN_TABLE", "")
	}
	requiredPart := stringValue(requiredRow, "PartNo", "")

	// Reel resolution: system UID, vendor barcode, customer label, external label.
	reel, err := s.smtResolveReel(ctx, machineCode, order, reelCode)
	if err != nil {
		return nil, err
	}
	if reel == nil {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, "", "REEL_NOT_FOUND", "")
	}
	actualPart := stringValue(reel.Data, "part_no", "")

	// Reel state checks.
	switch smtStatusOf(reel) {
	case "SCRAPPED":
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "REEL_SCRAPPED", "")
	case "EMPTY":
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "REEL_EMPTY", "")
	case "LOST", "RETURNED":
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "REEL_UNAVAILABLE", "")
	case "LOADED":
		currentSlot := stringValue(reel.Data, "current_slot", "")
		if currentSlot != "" && !strings.EqualFold(currentSlot, machineCode+"-"+slotNo) {
			return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "REEL_LOADED_ELSEWHERE", fmt.Sprintf("料盘已在 %s", currentSlot))
		}
	case "IN_STOCK":
		if s.smtBoolSetting(ctx, "require_issued_before_load") {
			return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "REEL_NOT_ISSUED", "")
		}
	}

	// Customer-supplied material policy.
	if err := s.smtCheckCustomerPolicy(ctx, order, reel); err != nil {
		if rejection, ok := err.(*smtPolicyRejection); ok {
			return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, rejection.ErrorType, rejection.Message)
		}
		return nil, err
	}

	// Reels issued to another work order.
	if s.smtBoolSetting(ctx, "require_issued_to_wo") {
		issuedWO := stringValue(reel.Data, "issued_wo", "")
		if issuedWO != "" && issuedWO != workOrderCode {
			return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "REEL_ISSUED_OTHER_WO", fmt.Sprintf("料盘发给了 %s", issuedWO))
		}
	}

	// Part match with BOM alternates.
	isAlternate := false
	if actualPart != requiredPart {
		alternates, err := s.smtBOMPartAlternates(ctx, productCode, requiredPart)
		if err != nil {
			return nil, err
		}
		matched := false
		for _, alt := range alternates {
			if alt == actualPart {
				matched = true
				break
			}
		}
		if matched {
			isAlternate = true
		} else {
			if s.smtBoolSetting(ctx, "lock_on_mismatch") {
				_ = s.smtUpdateSlot(ctx, machineCode, slotNo, func(row map[string]any) map[string]any {
					row["Locked"] = true
					row["LockReason"] = "料号错误自动锁定"
					row["LockedBy"] = "system"
					row["LockedAt"] = smtNowISO()
					return row
				})
			}
			message := fmt.Sprintf("料号错误！站位 %s 应上 %s，扫描料盘 %s 的料号是 %s", slotNo, requiredPart, reelKey(reel), actualPart)
			return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "PART_MISMATCH", message)
		}
	}

	// Slot occupancy: confirm / splice / replace.
	loadings, err := s.smtActiveLoadings(ctx, machineCode)
	if err != nil {
		return nil, err
	}
	var mainLoading, splicedLoading *Record
	for index := range loadings {
		loading := &loadings[index]
		if !strings.EqualFold(stringValue(loading.Data, "slot_no", ""), slotNo) {
			continue
		}
		if strings.EqualFold(stringValue(loading.Data, "position", "MAIN"), "SPLICED") {
			splicedLoading = loading
		} else {
			mainLoading = loading
		}
	}
	spliceRequested := isTruthy(input.Data["splice"])
	replaceRequested := isTruthy(input.Data["replace"])
	position := "MAIN"
	if mainLoading != nil {
		switch {
		case spliceRequested:
			if splicedLoading != nil {
				return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "SLOT_SPLICE_FULL", "")
			}
			position = "SPLICED"
		case replaceRequested:
			if splicedLoading != nil {
				return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "SLOT_HAS_SPLICE", "")
			}
		default:
			// Position occupied: ask the operator to choose splice or replace.
			return &ActionResult{
				TableCode: "MLOD", Key: key, Action: "load", Status: "CONFIRM",
				Effects: map[string]any{
					"confirm":        true,
					"slot_no":        slotNo,
					"required_part_no": requiredPart,
					"actual_part_no": actualPart,
					"current_reel":   stringValue(mainLoading.Data, "reel_code", ""),
					"current_part_no": stringValue(mainLoading.Data, "part_no", ""),
					"message":        fmt.Sprintf("站位 %s 已有料盘 %s", slotNo, stringValue(mainLoading.Data, "reel_code", "")),
					"voice":          fmt.Sprintf("%s 已有料盘", slotNo),
				},
			}
		}
	} else if splicedLoading != nil && !spliceRequested {
		return s.smtLoadReject(ctx, key, input, machineCode, workOrderCode, slotCode, slotNo, reelCode, requiredPart, actualPart, "SLOT_HAS_SPLICE", "")
	}

	// Replace: unload the current main reel first (returns it to stock or empties it).
	if replaceRequested && mainLoading != nil {
		if _, err := s.smtUnloadRecord(ctx, mainLoading, map[string]any{
			"reason":          "REPLACE",
			"qty_at_unload":   input.Data["old_reel_remaining_qty"],
			"old_reel_action": input.Data["old_reel_action"],
		}, input); err != nil {
			return nil, err
		}
	}

	// Commit the loading record.
	loadCode := key
	existing, err := s.smtGetRecord(ctx, "MLOD", loadCode)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: loading record %s already exists", ErrConflict, loadCode)
	}
	now := smtNowISO()
	doneQty := numericValue(order.Data, "done_qty")
	loadingPayload := map[string]any{
		"work_order": workOrderCode, "machine_code": machineCode,
		"slot_no": slotNo, "slot_code": stringValue(slotRow, "Barcode", machineCode+"-"+slotNo),
		"reel_code": reelKey(reel), "part_no": actualPart,
		"required_part_no": requiredPart, "is_alternate": isAlternate,
		"position": position, "status": "LOADED",
		"qty_at_load": numericValue(reel.Data, "qty"), "done_qty_at_load": doneQty,
		"loaded_by": smtActor(ctx, input), "loaded_at": now,
		"unloaded_at": "", "qty_at_unload": nil, "unload_reason": "",
		"Status": "active",
	}
	if _, err := s.smtUpsertRecord(ctx, "MLOD", loadCode, loadingPayload); err != nil {
		return nil, err
	}
	actionName := "LOAD"
	if position == "SPLICED" {
		actionName = "SPLICE"
	} else if replaceRequested {
		actionName = "REPLACE"
	}
	if err := s.smtAppendChild(ctx, "MLOD", loadCode, "LOD1", map[string]any{
		"action": actionName, "reel_code": reelKey(reel), "part_no": actualPart,
		"operator": smtActor(ctx, input), "created_at": now,
	}); err != nil {
		return nil, err
	}
	reelUpdate := map[string]any{
		"status": "LOADED",
		"current_slot": machineCode + "-" + slotNo,
		"loaded_at": now,
	}
	if stringValue(reel.Data, "issued_wo", "") == "" {
		reelUpdate["issued_wo"] = workOrderCode
	}
	if _, err := s.smtUpsertRecord(ctx, "MREL", reelKey(reel), reelUpdate); err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MREL", reelKey(reel), "REL1", map[string]any{
		"type": "LOAD", "qty_change": 0, "qty_after": numericValue(reel.Data, "qty"),
		"ref": loadCode, "operator": smtActor(ctx, input), "remark": "", "created_at": now,
	}); err != nil {
		return nil, err
	}
	// Loading resolves any low-material alert on this slot.
	if err := s.smtResolveLowAlerts(ctx, machineCode, slotNo, map[string]string{"LOAD": "LOAD", "SPLICE": "SPLICE", "REPLACE": "REPLACE"}[actionName]); err != nil {
		return nil, err
	}

	voice := fmt.Sprintf("%s 上料正确", slotNo)
	message := fmt.Sprintf("%s 上料正确：%s", slotNo, actualPart)
	if isAlternate {
		voice = fmt.Sprintf("%s 替代料上料", slotNo)
		message = fmt.Sprintf("%s 使用替代料 %s（应上 %s）", slotNo, actualPart, requiredPart)
	}
	scan := map[string]any{
		"work_order": workOrderCode, "machine_code": machineCode,
		"slot_code": stringValue(slotRow, "Barcode", ""), "slot_no": slotNo,
		"reel_code": reelKey(reel), "required_part_no": requiredPart, "actual_part_no": actualPart,
		"result": "OK", "error_type": "", "message": message, "voice": voice,
	}
	return s.smtScanResult(ctx, "MLOD", loadCode, "load", scan, input, map[string]any{
		"load_code":    loadCode,
		"position":     position,
		"is_alternate": isAlternate,
		"required_part_no": requiredPart,
		"actual_part_no":   actualPart,
	})
}

func reelKey(reel *Record) string {
	if reel == nil {
		return ""
	}
	return reel.Key
}

func boolSettingFromValue(values map[string]any, key string) bool {
	value, present := values[key]
	if !present {
		return false
	}
	return isTruthy(value)
}

type smtPolicyRejection struct {
	ErrorType string
	Message   string
}

func (e *smtPolicyRejection) Error() string { return e.ErrorType + ": " + e.Message }

// smtCheckCustomerPolicy applies the customer_material_policy setting.
func (s *Service) smtCheckCustomerPolicy(ctx context.Context, order *Record, reel *Record) error {
	policy := s.smtSetting(ctx, "customer_material_policy")
	if policy == "any" {
		return nil
	}
	owner := strings.ToUpper(stringValue(reel.Data, "owner_type", "SELF"))
	workOrderCustomer := stringValue(order.Data, "customer_code", "")
	reelCustomer := stringValue(reel.Data, "customer_code", "")
	switch policy {
	case "customer_only":
		if workOrderCustomer != "" {
			if owner != "CUSTOMER" || reelCustomer != workOrderCustomer {
				return &smtPolicyRejection{ErrorType: "CUSTOMER_MISMATCH", Message: "客户工单必须使用该客户客供料"}
			}
		}
	default: // same_customer_or_self
		if owner == "CUSTOMER" && workOrderCustomer != "" && reelCustomer != workOrderCustomer {
			return &smtPolicyRejection{ErrorType: "CUSTOMER_MISMATCH", Message: fmt.Sprintf("客供料属于客户 %s，本工单客户为 %s", reelCustomer, workOrderCustomer)}
		}
	}
	return nil
}

// smtResolveReel resolves a scanned reel code: system UID, vendor barcode,
// customer part number translation, or (when allowed) an external label that
// is registered on the fly.
func (s *Service) smtResolveReel(ctx context.Context, machineCode string, order *Record, scanned string) (*Record, error) {
	scanned = strings.TrimSpace(scanned)
	if scanned == "" {
		return nil, nil
	}
	reel, err := s.smtGetRecord(ctx, "MREL", scanned)
	if err != nil {
		return nil, err
	}
	if reel != nil {
		return reel, nil
	}
	byBarcode, err := s.smtQueryAll(ctx, "MREL", map[string]any{"vendor_barcode": scanned}, 2)
	if err != nil {
		return nil, err
	}
	if len(byBarcode) > 0 {
		return &byBarcode[0], nil
	}
	// Customer part number -> own part number translation.
	translated := ""
	workOrderCustomer := stringValue(order.Data, "customer_code", "")
	maps, err := s.smtQueryAll(ctx, "MCPM", map[string]any{"customer_part_no": scanned}, 5)
	if err != nil {
		return nil, err
	}
	for index := range maps {
		entry := &maps[index]
		if workOrderCustomer != "" && stringValue(entry.Data, "customer_code", "") != workOrderCustomer {
			continue
		}
		translated = stringValue(entry.Data, "part_no", "")
		break
	}
	if translated == "" {
		// External label parsing (part_no|lot|qty|supplier|date_code).
		translated, err = s.smtParseVendorLabel(ctx, scanned)
		if err != nil {
			return nil, err
		}
	}
	if translated == "" || !s.smtBoolSetting(ctx, "allow_unregistered_reel") {
		return nil, nil
	}
	// Auto-register the external label as an in-stock reel.
	prefix := s.smtSetting(ctx, "reel_uid_prefix")
	if prefix == "" {
		prefix = "R"
	}
	reelCode := generateReelCode(prefix, ctx, s)
	now := smtNowISO()
	ownerType := "SELF"
	customerCode := ""
	if workOrderCustomer != "" {
		ownerType = "CUSTOMER"
		customerCode = workOrderCustomer
	}
	if _, err := s.smtUpsertRecord(ctx, "MREL", reelCode, map[string]any{
		"part_no": translated, "vendor_barcode": scanned,
		"qty": 0, "initial_qty": 0, "status": "IN_STOCK",
		"owner_type": ownerType, "customer_code": customerCode,
		"received_at": now, "Status": "active", "auto_registered": true,
	}); err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MREL", reelCode, "REL1", map[string]any{
		"type": "RECEIVE", "qty_change": 0, "qty_after": 0,
		"ref": "auto-register", "operator": "system", "remark": scanned, "created_at": now,
	}); err != nil {
		return nil, err
	}
	return s.smtGetRecord(ctx, "MREL", reelCode)
}

// smtParseVendorLabel parses an external reel label using the vendor_label_*
// settings. Returns the part number, or "" when the label does not parse.
func (s *Service) smtParseVendorLabel(ctx context.Context, label string) (string, error) {
	separator := s.smtSetting(ctx, "vendor_label_separator")
	if separator == "" {
		separator = "|"
	}
	if !strings.Contains(label, separator) {
		return "", nil
	}
	fields := strings.Split(label, separator)
	names := strings.Split(s.smtSetting(ctx, "vendor_label_fields"), ",")
	minFields := s.smtIntSetting(ctx, "vendor_label_min_fields", 1)
	values := map[string]string{}
	for index, name := range names {
		name = strings.TrimSpace(name)
		if index >= len(fields) || name == "" || name == "skip" {
			continue
		}
		values[name] = strings.TrimSpace(fields[index])
	}
	if len(values) < minFields {
		return "", nil
	}
	return values["part_no"], nil
}

// smtUnloadRecord unloads a loading record (shared by MLOD:unload and replace).
func (s *Service) smtUnloadRecord(ctx context.Context, loading *Record, data map[string]any, input ActionInput) (*Record, error) {
	now := smtNowISO()
	reason := strings.ToUpper(stringValue(data, "reason", "OTHER"))
	reelAction := strings.ToUpper(stringValue(data, "old_reel_action", ""))
	qtyAtUnload := numericValue(data, "qty_at_unload")
	if data["qty_at_unload"] == nil {
		if reelAction == "EMPTY" {
			qtyAtUnload = 0
		} else {
			qtyAtUnload = numericValue(loading.Data, "qty_at_load")
		}
	}
	updated, err := s.smtUpsertRecord(ctx, "MLOD", loading.Key, map[string]any{
		"status": "UNLOADED", "qty_at_unload": qtyAtUnload,
		"unloaded_by": smtActor(ctx, input), "unloaded_at": now, "unload_reason": reason,
	})
	if err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MLOD", loading.Key, "LOD1", map[string]any{
		"action": "UNLOAD", "reel_code": stringValue(loading.Data, "reel_code", ""),
		"part_no": stringValue(loading.Data, "part_no", ""),
		"qty_at_unload": qtyAtUnload, "reason": reason,
		"operator": smtActor(ctx, input), "created_at": now,
	}); err != nil {
		return nil, err
	}
	// Reel disposition.
	reelCode := stringValue(loading.Data, "reel_code", "")
	reel, err := s.smtGetRecord(ctx, "MREL", reelCode)
	if err != nil {
		return nil, err
	}
	if reel != nil && smtStatusOf(reel) == "LOADED" {
		if reelAction == "" {
			if qtyAtUnload <= 0 {
				reelAction = "EMPTY"
			} else {
				reelAction = "RETURN"
			}
		}
		update := map[string]any{"current_slot": "", "qty": qtyAtUnload}
		transaction := "UNLOAD"
		switch reelAction {
		case "EMPTY":
			update["status"] = "EMPTY"
			update["qty"] = 0
			transaction = "EMPTY"
		case "KEEP":
			update["status"] = "ISSUED"
			transaction = "UNLOAD"
		default:
			update["status"] = "IN_STOCK"
			transaction = "RETURN"
		}
		if _, err := s.smtUpsertRecord(ctx, "MREL", reelCode, update); err != nil {
			return nil, err
		}
		if err := s.smtAppendChild(ctx, "MREL", reelCode, "REL1", map[string]any{
			"type": transaction, "qty_change": qtyAtUnload - numericValue(reel.Data, "qty"), "qty_after": qtyAtUnload,
			"ref": loading.Key, "operator": smtActor(ctx, input), "remark": reason, "created_at": now,
		}); err != nil {
			return nil, err
		}
	}
	// Promote a spliced reel to main when the main reel is unloaded.
	if strings.EqualFold(stringValue(loading.Data, "position", "MAIN"), "MAIN") {
		loadings, err := s.smtActiveLoadings(ctx, stringValue(loading.Data, "machine_code", ""))
		if err != nil {
			return nil, err
		}
		for index := range loadings {
			candidate := &loadings[index]
			if strings.EqualFold(stringValue(candidate.Data, "slot_no", ""), stringValue(loading.Data, "slot_no", "")) &&
				strings.EqualFold(stringValue(candidate.Data, "position", ""), "SPLICED") {
				if _, err := s.smtUpsertRecord(ctx, "MLOD", candidate.Key, map[string]any{"position": "MAIN", "promoted_at": now}); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	// Unloading invalidates the machine verification.
	if _, err := s.smtUpsertRecord(ctx, "MMAC", stringValue(loading.Data, "machine_code", ""), map[string]any{
		"verified_at": "", "verified_by": "", "signed_off_at": "", "signed_off_by": "",
	}); err != nil {
		return nil, err
	}
	if err := s.smtResolveLowAlerts(ctx, stringValue(loading.Data, "machine_code", ""), stringValue(loading.Data, "slot_no", ""), "UNLOAD"); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) smtUnload(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	loading, err := s.smtGetRecord(ctx, "MLOD", key)
	if err != nil {
		return nil, err
	}
	if loading == nil {
		return nil, fmt.Errorf("%w: loading record %s not found", ErrValidation, key)
	}
	if smtStatusOf(loading) != "LOADED" {
		return nil, fmt.Errorf("%w: loading record %s is already unloaded", ErrValidation, key)
	}
	updated, err := s.smtUnloadRecord(ctx, loading, input.Data, input)
	if err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MLOD", Key: key, Action: "unload", Status: "unloaded", Record: updated,
		Effects: map[string]any{
			"slot_no": stringValue(loading.Data, "slot_no", ""),
			"reel_code": stringValue(loading.Data, "reel_code", ""),
			"message": fmt.Sprintf("%s 卸料完成", stringValue(loading.Data, "slot_no", "")),
			"voice":   "卸料完成",
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Low-material alerts (MLWA).
// ---------------------------------------------------------------------------

// smtSyncLowMaterialAlerts estimates remaining reel usage for every machine
// bound to the work order after a progress report and maintains OPEN alerts.
func (s *Service) smtSyncLowMaterialAlerts(ctx context.Context, order *Record, input ActionInput) ([]map[string]any, error) {
	workOrder := order.Key
	productCode := stringValue(order.Data, "product_code", "")
	bomUsage, err := s.smtBOMUsage(ctx, productCode)
	if err != nil {
		return nil, err
	}
	doneQty := numericValue(order.Data, "done_qty")
	lowQty := float64(s.smtIntSetting(ctx, "low_qty_threshold", 500))
	lowPcs := float64(s.smtIntSetting(ctx, "low_pcs_threshold", 50))
	machines, err := s.smtQueryAll(ctx, "MMAC", map[string]any{"current_work_order": workOrder}, 100)
	if err != nil {
		return nil, err
	}
	openAlerts, err := s.smtQueryAll(ctx, "MLWA", map[string]any{"status": "OPEN"}, 500)
	if err != nil {
		return nil, err
	}
	openBySlot := map[string]*Record{}
	for index := range openAlerts {
		alert := &openAlerts[index]
		if stringValue(alert.Data, "work_order", "") != workOrder {
			continue
		}
		openBySlot[stringValue(alert.Data, "machine_code", "")+"|"+stringValue(alert.Data, "slot_no", "")] = alert
	}
	raised := []map[string]any{}
	seen := map[string]bool{}
	now := smtNowISO()
	for machineIndex := range machines {
		machine := &machines[machineIndex]
		loadings, err := s.smtActiveLoadings(ctx, machine.Key)
		if err != nil {
			return nil, err
		}
		for loadingIndex := range loadings {
			loading := &loadings[loadingIndex]
			estimate := smtLoadingPayload(loading, bomUsage, doneQty, lowQty)
			slotNo := stringValue(loading.Data, "slot_no", "")
			key := machine.Key + "|" + slotNo
			isLow := isTruthy(estimate["low"]) || float64FromAny(estimate["boards_left"]) < lowPcs && lowPcs > 0 && float64FromAny(estimate["qty_est"]) > 0
			if !isLow {
				continue
			}
			seen[key] = true
			entry := map[string]any{
				"machine_code": machine.Key, "slot_no": slotNo,
				"part_no": stringValue(loading.Data, "part_no", ""),
				"reel_code": stringValue(loading.Data, "reel_code", ""),
				"qty_est": estimate["qty_est"], "boards_left": estimate["boards_left"],
				"work_order": workOrder,
			}
			if existing := openBySlot[key]; existing != nil {
				if _, err := s.smtUpsertRecord(ctx, "MLWA", existing.Key, map[string]any{
					"qty_est": estimate["qty_est"], "boards_left": estimate["boards_left"], "updated_at": now,
				}); err != nil {
					return nil, err
				}
				entry["alert_code"] = existing.Key
				entry["is_new"] = false
			} else {
				alertCode := smtGenerateKey("LA")
				if _, err := s.smtUpsertRecord(ctx, "MLWA", alertCode, map[string]any{
					"work_order": workOrder, "machine_code": machine.Key, "slot_no": slotNo,
					"part_no": stringValue(loading.Data, "part_no", ""),
					"reel_code": stringValue(loading.Data, "reel_code", ""),
					"qty_est": estimate["qty_est"], "boards_left": estimate["boards_left"],
					"status": "OPEN", "Status": "OPEN",
					"raised_at": now, "resolved_at": "", "resolution": "",
				}); err != nil {
					return nil, err
				}
				entry["alert_code"] = alertCode
				entry["is_new"] = true
				ref := "low:" + machine.Key + "-" + slotNo
				if s.smtBoolSetting(ctx, "andon_auto_material") {
					if _, err := s.smtAndonRaise(ctx, smtGenerateKey("AD"), ActionInput{Data: map[string]any{
						"line_code": stringValue(machine.Data, "line_code", ""),
						"machine_code": machine.Key, "work_order": workOrder,
						"call_type": "MATERIAL", "source": "AUTO", "ref": ref,
						"description": fmt.Sprintf("缺料预警：%s 站位 %s 料号 %s 剩余约 %.0f", machine.Key, slotNo, stringValue(loading.Data, "part_no", ""), float64FromAny(estimate["qty_est"])),
					}}); err != nil {
						return nil, err
					}
				}
			}
			raised = append(raised, entry)
		}
	}
	// Alerts that no longer estimate low (loaded reels fine now) resolve as CLEARED.
	for key, alert := range openBySlot {
		if seen[key] {
			continue
		}
		if _, err := s.smtUpsertRecord(ctx, "MLWA", alert.Key, map[string]any{
			"status": "RESOLVED", "Status": "resolved",
			"resolved_at": now, "resolution": "CLEARED",
		}); err != nil {
			return nil, err
		}
	}
	return raised, nil
}

func float64FromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case nil:
		return 0
	default:
		return numericValue(map[string]any{"v": value}, "v")
	}
}

func (s *Service) smtResolveLowAlerts(ctx context.Context, machineCode, slotNo, resolution string) error {
	alerts, err := s.smtQueryAll(ctx, "MLWA", map[string]any{
		"machine_code": machineCode, "slot_no": slotNo, "status": "OPEN",
	}, 20)
	if err != nil {
		return err
	}
	now := smtNowISO()
	for index := range alerts {
		alert := &alerts[index]
		if _, err := s.smtUpsertRecord(ctx, "MLWA", alert.Key, map[string]any{
			"status": "RESOLVED", "Status": "resolved",
			"resolved_at": now, "resolution": resolution,
		}); err != nil {
			return err
		}
	}
	return nil
}
