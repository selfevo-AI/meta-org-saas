package erp

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// SMT MES shop-floor actions (module "smt_mes").
//
// These implement the platform-side SMT MES closed loop: reel inventory,
// feeder verification loading, SN pass-station foolproofing, quality/repair,
// andon and downtime. Payload conventions mirror the SMT-MES reference system
// (see docs/SMT-MES-SOLUTION.md); every shop-floor table row carries its
// business fields inside the JSONB payload, while the master row keeps the
// lifecycle value under "Status" where the platform status dictionary applies.
//
// RunAction wraps every action in a tenant transaction guarded by the ERP
// ledger advisory lock, so multi-row mutations here are serialized per tenant
// database and stay consistent with the action execution ledger.

// ---------------------------------------------------------------------------
// Settings (MSET code table with SMT-MES compatible defaults).
// ---------------------------------------------------------------------------

var smtSettingDefaults = map[string]string{
	"reel_uid_prefix":            "R",
	"vendor_label_separator":     "|",
	"vendor_label_fields":        "part_no,lot_no,qty,supplier,date_code",
	"vendor_label_min_fields":    "1",
	"allow_unregistered_reel":    "true",
	"require_issued_before_load": "false",
	"lock_on_mismatch":           "false",
	"low_qty_threshold":          "500",
	"low_pcs_threshold":          "50",
	"customer_material_policy":   "same_customer_or_self",
	"require_issued_to_wo":       "false",
	"requisition_loss_rate":      "2",
	"sn_regex":                   "",
	"pass_require_machine_ready": "true",
	"pass_limit_plan_qty":        "false",
	"pass_auto_progress":         "true",
	"fai_required":               "true",
	"fai_require_verified":       "true",
	"signoff_required":           "false",
	"signoff_role":               "any",
	"andon_fail_threshold":       "3",
	"andon_auto_material":        "false",
	"downtime_from_andon":        "true",
	"downtime_from_changeover":   "true",
	"shift_definitions":          "白班=08:00-20:00;夜班=20:00-08:00",
	"collector_interval_sec":     "10",
	"collector_stable_sec":       "3",
	"voice_enabled":              "true",
	"voice_on_ok":                "true",
	"scanner_interval_ms":        "50",
	"company_name":               "SMT 车间执行",
	"fai_check_items":            "上料核对/签核已通过;锡膏印刷无少锡/偏移/连锡;元件无缺件/错件/多件;极性与方向正确;无立碑/偏移/翘脚;焊点外观良好;AOI 首件检测通过",
}

// SMTSettingDefaults exposes the SMT settings defaults (key -> default value).
func SMTSettingDefaults() map[string]string {
	out := make(map[string]string, len(smtSettingDefaults))
	for key, value := range smtSettingDefaults {
		out[key] = value
	}
	return out
}

func (s *Service) smtSetting(ctx context.Context, key string) string {
	table, err := s.table("MSET")
	if err != nil {
		return smtSettingDefaults[key]
	}
	record, err := s.repo.GetRecord(ctx, table, key)
	if err != nil || record == nil {
		return smtSettingDefaults[key]
	}
	if raw, present := record.Data["Value"]; present && raw != nil {
		return fmt.Sprint(raw)
	}
	return smtSettingDefaults[key]
}

func (s *Service) smtBoolSetting(ctx context.Context, key string) bool {
	switch strings.ToLower(strings.TrimSpace(s.smtSetting(ctx, key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func (s *Service) smtIntSetting(ctx context.Context, key string, fallback int) int {
	value := strings.TrimSpace(s.smtSetting(ctx, key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

// ---------------------------------------------------------------------------
// Shared helpers.
// ---------------------------------------------------------------------------

func smtNowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func smtActor(ctx context.Context, input ActionInput) string {
	if input.ActorID != nil {
		return input.ActorID.String()
	}
	if meta := actionExecutionFromContext(ctx); meta.ActorType != "" {
		return meta.ActorType
	}
	return "system"
}

// smtGenerateKey builds a shop-floor document key such as "LD-261017143005-4821".
func smtGenerateKey(prefix string) string {
	return fmt.Sprintf("%s-%s-%04d", prefix, time.Now().Format("060102150405"), rand.Intn(10000))
}

// smtLoadError carries the Chinese message and voice line for a scan rejection.
type smtLoadError struct {
	Message string
	Voice   string
}

var smtLoadErrors = map[string]smtLoadError{
	"WO_NOT_FOUND":          {"工单不存在", "工单不存在"},
	"WO_NOT_RUNNING":        {"工单未开工", "工单未开工"},
	"MACHINE_NOT_FOUND":     {"机台不存在", "机台不存在"},
	"SLOT_NOT_FOUND":        {"站位码无效", "站位码无效"},
	"SLOT_WRONG_MACHINE":    {"站位不属于本机台", "站位不属于本机台"},
	"NO_STATION_TABLE":      {"无料站表", "没有料站表"},
	"SLOT_NOT_IN_TABLE":     {"站位不在料站表", "站位不在料站表"},
	"INVALID_REEL_CODE":     {"料盘码无法识别", "料盘码无法识别"},
	"REEL_NOT_FOUND":        {"料盘未登记", "料盘未登记"},
	"MATERIAL_NOT_FOUND":    {"料号不存在", "料号不存在"},
	"REEL_SCRAPPED":         {"料盘已报废", "料盘已报废"},
	"REEL_EMPTY":            {"料盘已用完", "料盘已用完"},
	"REEL_LOADED_ELSEWHERE": {"料盘已在其它站位", "料盘已在其它站位"},
	"REEL_NOT_ISSUED":       {"料盘未发料", "料盘未发料"},
	"PART_MISMATCH":         {"料号错误", "料号错误"},
	"SLOT_SPLICE_FULL":      {"站位已有接料盘", "站位已有接料盘，不能再接"},
	"SLOT_HAS_SPLICE":       {"站位有接料盘时不能换料", "请先处理接料盘"},
	"SLOT_LOCKED":           {"站位已锁定", "站位已锁定"},
	"REEL_UNAVAILABLE":      {"料盘不可用", "料盘不可用"},
	"CUSTOMER_MISMATCH":     {"客供料客户不符", "客供料客户不符"},
	"REEL_ISSUED_OTHER_WO":  {"料盘发给了其它工单", "料盘发给了其它工单"},
	"VERIFY_FAILED":         {"整机核对未通过", "核对未通过"},
	"SIGNOFF_FAILED":        {"签核未通过", "签核未通过"},
}

func smtLoadErrorFor(typeCode string) smtLoadError {
	if item, ok := smtLoadErrors[typeCode]; ok {
		return item
	}
	return smtLoadError{Message: typeCode, Voice: typeCode}
}

// smtGetRecord returns nil (no error) when the record does not exist.
func (s *Service) smtGetRecord(ctx context.Context, tableCode, key string) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

func (s *Service) smtUpsertRecord(ctx context.Context, tableCode, key string, data map[string]any) (*Record, error) {
	table, err := s.table(tableCode)
	if err != nil {
		return nil, err
	}
	_, err = s.repo.GetRecord(ctx, table, key)
	if err == ErrNotFound {
		payload := copyData(data)
		payload[table.PrimaryKey] = key
		return s.repo.CreateRecord(ctx, table, RecordInput{Key: key, Data: payload})
	}
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: data})
}

// smtAppendChild appends a child row with the next line number.
func (s *Service) smtAppendChild(ctx context.Context, parentCode, parentKey, childCode string, payload map[string]any) error {
	lines, err := s.listChildPayloads(ctx, parentCode, parentKey, childCode)
	if err != nil {
		return err
	}
	_, err = s.createDocumentLine(ctx, parentCode, parentKey, childCode, fmt.Sprint(len(lines)+1), payload)
	return err
}

func smtGenerated(tableCode, key, relation string, payload map[string]any) Record {
	return Record{TableCode: tableCode, Key: key, Data: payload, ParentTableCode: "", ParentKey: ""}
}

// smtQueryAll pages through a filtered query and returns up to `limit` records.
func (s *Service) smtQueryAll(ctx context.Context, tableCode string, filters map[string]any, limit int) ([]Record, error) {
	page, err := s.QueryRecords(ctx, tableCode, RecordQuery{Filters: filters, Limit: 200})
	if err != nil {
		return nil, err
	}
	records := append([]Record{}, page.Records...)
	for page.NextCursor != "" && len(records) < limit {
		page, err = s.QueryRecords(ctx, tableCode, RecordQuery{Filters: filters, After: page.NextCursor, Limit: 200})
		if err != nil {
			return nil, err
		}
		records = append(records, page.Records...)
	}
	if len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

// smtStatusOf reads the domain lifecycle status of an SMT record. The
// lowercase "status" field carries the SMT-MES domain state (IN_STOCK,
// LOADED, OPEN, ...); "Status" is only used by records that follow the ERP
// lifecycle dictionary (work orders, requisitions).
func smtStatusOf(record *Record) string {
	if record == nil {
		return ""
	}
	if raw, present := record.Data["status"]; present && raw != nil {
		text := strings.TrimSpace(fmt.Sprint(raw))
		if text != "" && text != "<nil>" {
			return strings.ToUpper(text)
		}
	}
	return strings.ToUpper(stringValue(record.Data, "Status", ""))
}

// ---------------------------------------------------------------------------
// Dispatch.
// ---------------------------------------------------------------------------

// smtBusinessActions lists every governed shop-floor action. Each invocation is
// a distinct scan/operator event (a unit passes several stations, a machine is
// re-verified after every load), so without an explicit idempotency key from
// the caller these actions must never collide with an earlier execution.
var smtBusinessActions = map[string]bool{
	"MREL:receive": true, "MREL:issue": true, "MREL:return": true, "MREL:empty": true, "MREL:scrap": true,
	"MREL:lost": true, "MREL:return-to-customer": true, "MREL:adjust": true,
	"MLOD:load": true, "MLOD:unload": true,
	"MMAC:bind": true, "MMAC:verify": true, "MMAC:sign-off": true, "MMAC:lock-slot": true, "MMAC:unlock-slot": true,
	"MSWO:start": true, "MSWO:progress": true, "MSWO:finish": true, "MSWO:close": true,
	"MRQN:issue": true, "MRQN:close": true, "MRQN:cancel": true,
	"MUNT:pass": true, "MUNT:fail": true, "MUNT:repair": true, "MUNT:scrap": true,
	"MPNL:bind": true, "MPNL:split": true,
	"MINS:record": true,
	"MAND:raise":  true, "MAND:ack": true, "MAND:close": true,
	"MDWN:start": true, "MDWN:end": true,
}

func isSMTBusinessAction(tableCode, action string) bool {
	return smtBusinessActions[tableCode+":"+action]
}

func (s *Service) runSMTBusinessAction(ctx context.Context, tableCode string, key string, action string, input ActionInput) (*ActionResult, error) {
	switch tableCode + ":" + action {
	case "MREL:receive":
		return s.smtReelReceive(ctx, key, input)
	case "MREL:issue":
		return s.smtReelTransition(ctx, key, action, input)
	case "MREL:return", "MREL:empty", "MREL:scrap", "MREL:lost", "MREL:return-to-customer":
		return s.smtReelTransition(ctx, key, action, input)
	case "MREL:adjust":
		return s.smtReelAdjust(ctx, key, input)
	case "MLOD:load":
		return s.smtLoad(ctx, key, input)
	case "MLOD:unload":
		return s.smtUnload(ctx, key, input)
	case "MMAC:bind":
		return s.smtMachineBind(ctx, key, input)
	case "MMAC:verify":
		return s.smtMachineVerify(ctx, key, input)
	case "MMAC:sign-off":
		return s.smtMachineSignOff(ctx, key, input)
	case "MMAC:lock-slot", "MMAC:unlock-slot":
		return s.smtMachineSlotLock(ctx, key, action, input)
	case "MSWO:start", "MSWO:finish", "MSWO:close":
		return s.smtWorkOrderTransition(ctx, key, action, input)
	case "MSWO:progress":
		return s.smtWorkOrderProgress(ctx, key, input)
	case "MRQN:issue":
		return s.smtRequisitionIssue(ctx, key, input)
	case "MRQN:close", "MRQN:cancel":
		return s.smtRequisitionTransition(ctx, key, action, input)
	case "MUNT:pass":
		return s.smtUnitPass(ctx, key, input)
	case "MUNT:fail":
		return s.smtUnitFail(ctx, key, input)
	case "MUNT:repair":
		return s.smtUnitRepair(ctx, key, input)
	case "MUNT:scrap":
		return s.smtUnitScrap(ctx, key, input)
	case "MPNL:bind":
		return s.smtPanelBind(ctx, key, input)
	case "MPNL:split":
		return s.smtPanelSplit(ctx, key, input)
	case "MINS:record":
		return s.smtInspectionRecord(ctx, key, input)
	case "MAND:raise":
		return s.smtAndonRaise(ctx, key, input)
	case "MAND:ack", "MAND:close":
		return s.smtAndonTransition(ctx, key, action, input)
	case "MDWN:start":
		return s.smtDowntimeStart(ctx, key, input)
	case "MDWN:end":
		return s.smtDowntimeEnd(ctx, key, input)
	default:
		return nil, fmt.Errorf("%w: unknown SMT action %s for %s", ErrValidation, action, tableCode)
	}
}

// ---------------------------------------------------------------------------
// Reel inventory (MREL).
// ---------------------------------------------------------------------------

func (s *Service) smtReelReceive(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	partNo := strings.TrimSpace(stringValue(input.Data, "part_no", ""))
	if partNo == "" {
		return nil, fmt.Errorf("%w: part_no is required", ErrValidation)
	}
	if _, err := s.smtGetRecord(ctx, "MITM", partNo); err != nil {
		return nil, err
	}
	item, _ := s.smtGetRecord(ctx, "MITM", partNo)
	if item == nil {
		return nil, fmt.Errorf("%w: material %s not found", ErrValidation, partNo)
	}
	reelCount := numericValue(input.Data, "reel_count")
	if reelCount <= 0 {
		reelCount = 1
	}
	if reelCount > 200 {
		return nil, fmt.Errorf("%w: reel_count exceeds the batch limit", ErrValidation)
	}
	qtyPerReel := numericValue(input.Data, "qty")
	now := smtNowISO()
	actor := smtActor(ctx, input)
	prefix := s.smtSetting(ctx, "reel_uid_prefix")
	if prefix == "" {
		prefix = "R"
	}
	generated := make([]Record, 0, int(reelCount))
	reelCodes := make([]string, 0, int(reelCount))
	for index := 0; index < int(reelCount); index++ {
		reelCode := generateReelCode(prefix, ctx, s)
		payload := map[string]any{
			"part_no":          partNo,
			"name":             stringValue(item.Data, "ItemName", ""),
			"lot_no":           stringValue(input.Data, "lot_no", ""),
			"qty":              qtyPerReel,
			"initial_qty":      qtyPerReel,
			"supplier":         stringValue(input.Data, "supplier", ""),
			"date_code":        stringValue(input.Data, "date_code", ""),
			"vendor_barcode":   stringValue(input.Data, "vendor_barcode", ""),
			"location":         strings.TrimSpace(stringValue(input.Data, "location", "")),
			"status":           "IN_STOCK",
			"owner_type":       strings.ToUpper(stringValue(input.Data, "owner_type", "SELF")),
			"customer_code":    strings.TrimSpace(stringValue(input.Data, "customer_code", "")),
			"customer_part_no": strings.TrimSpace(stringValue(input.Data, "customer_part_no", "")),
			"remark":           stringValue(input.Data, "remark", ""),
			"received_at":      now,
			"Status":           "active",
		}
		if payload["owner_type"] != "CUSTOMER" {
			payload["owner_type"] = "SELF"
			payload["customer_code"] = ""
			payload["customer_part_no"] = ""
		} else if payload["customer_code"] == "" {
			return nil, fmt.Errorf("%w: customer_code is required for customer-supplied reels", ErrValidation)
		}
		if _, err := s.smtUpsertRecord(ctx, "MREL", reelCode, payload); err != nil {
			return nil, err
		}
		if err := s.smtAppendChild(ctx, "MREL", reelCode, "REL1", map[string]any{
			"type": "RECEIVE", "qty_change": qtyPerReel, "qty_after": qtyPerReel,
			"ref": key, "operator": actor, "remark": "", "created_at": now,
		}); err != nil {
			return nil, err
		}
		reelCodes = append(reelCodes, reelCode)
		generated = append(generated, smtGenerated("MREL", reelCode, "created", map[string]any{"part_no": partNo, "qty": qtyPerReel}))
	}
	batch, err := s.smtUpsertRecord(ctx, "MREL", key, map[string]any{
		"part_no": partNo, "Name": fmt.Sprintf("收料 %s × %d", partNo, len(reelCodes)),
		"Status": "active", "reel_codes": reelCodes, "received_at": now,
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MREL", Key: key, Action: "receive", Status: "received", Record: batch,
		GeneratedRecords: generated,
		Effects:          map[string]any{"created": len(reelCodes), "reel_codes": reelCodes},
	}, nil
}

func generateReelCode(prefix string, ctx context.Context, s *Service) string {
	for attempt := 0; attempt < 5; attempt++ {
		code := fmt.Sprintf("%s%s%04d", prefix, time.Now().Format("060102"), rand.Intn(10000))
		existing, err := s.smtGetRecord(ctx, "MREL", code)
		if err != nil {
			continue
		}
		if existing == nil {
			return code
		}
	}
	return fmt.Sprintf("%s%s%06d", prefix, time.Now().Format("060102"), time.Now().UnixNano()%1000000)
}

func (s *Service) smtReelTransition(ctx context.Context, key string, action string, input ActionInput) (*ActionResult, error) {
	reel, err := s.smtGetRecord(ctx, "MREL", key)
	if err != nil {
		return nil, err
	}
	if reel == nil {
		return nil, fmt.Errorf("%w: reel %s not found", ErrValidation, key)
	}
	status := smtStatusOf(reel)
	now := smtNowISO()
	actor := smtActor(ctx, input)
	update := map[string]any{}
	transaction := ""

	switch action {
	case "issue":
		if status != "IN_STOCK" {
			return nil, fmt.Errorf("%w: reel %s is %s; only in-stock reels can be issued", ErrValidation, key, status)
		}
		workOrder := strings.TrimSpace(stringValue(input.Data, "work_order", ""))
		if workOrder != "" {
			wo, err := s.smtGetRecord(ctx, "MSWO", workOrder)
			if err != nil {
				return nil, err
			}
			if wo == nil {
				return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, workOrder)
			}
		}
		update["status"] = "ISSUED"
		update["issued_wo"] = workOrder
		update["issued_at"] = now
		transaction = "ISSUE"
	case "return":
		if status != "ISSUED" && status != "LOADED" {
			return nil, fmt.Errorf("%w: reel %s is %s; only issued or loaded reels can be returned", ErrValidation, key, status)
		}
		qty := numericValue(input.Data, "qty")
		if qty > 0 {
			update["qty"] = qty
		}
		update["status"] = "IN_STOCK"
		update["current_slot"] = ""
		update["returned_at"] = now
		transaction = "RETURN"
	case "empty":
		if status == "EMPTY" {
			return nil, fmt.Errorf("%w: reel %s is already empty", ErrValidation, key)
		}
		update["status"] = "EMPTY"
		update["qty"] = 0
		update["current_slot"] = ""
		update["emptied_at"] = now
		transaction = "EMPTY"
	case "scrap":
		if status == "SCRAPPED" {
			return nil, fmt.Errorf("%w: reel %s is already scrapped", ErrValidation, key)
		}
		update["status"] = "SCRAPPED"
		update["scrapped_at"] = now
		update["scrap_reason"] = stringValue(input.Data, "reason", "")
		transaction = "SCRAP"
	case "lost":
		if status == "LOST" {
			return nil, fmt.Errorf("%w: reel %s is already marked lost", ErrValidation, key)
		}
		update["status"] = "LOST"
		update["lost_at"] = now
		update["lost_reason"] = stringValue(input.Data, "reason", "")
		transaction = "LOST"
	case "return-to-customer":
		if strings.ToUpper(stringValue(reel.Data, "owner_type", "SELF")) != "CUSTOMER" {
			return nil, fmt.Errorf("%w: reel %s is not customer supplied", ErrValidation, key)
		}
		update["status"] = "RETURNED"
		update["returned_to_customer_at"] = now
		transaction = "RETURN_CUSTOMER"
	default:
		return nil, fmt.Errorf("%w: unsupported reel action %s", ErrValidation, action)
	}

	if action == "return" || action == "empty" || action == "scrap" || action == "lost" || action == "return-to-customer" {
		if loading, err := s.smtActiveLoadingForReel(ctx, key); err != nil {
			return nil, err
		} else if loading != nil {
			return nil, fmt.Errorf("%w: reel %s is loaded on slot %s; unload it first", ErrValidation, key, stringValue(loading.Data, "slot_no", ""))
		}
	}

	record, err := s.smtUpsertRecord(ctx, "MREL", key, update)
	if err != nil {
		return nil, err
	}
	qtyAfter := numericValue(record.Data, "qty")
	if err := s.smtAppendChild(ctx, "MREL", key, "REL1", map[string]any{
		"type": transaction, "qty_change": 0, "qty_after": qtyAfter,
		"ref": stringValue(input.Data, "ref", ""), "operator": actor,
		"remark": stringValue(input.Data, "remark", ""), "created_at": now,
	}); err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MREL", Key: key, Action: action, Status: strings.ToLower(action), Record: record,
		Effects: map[string]any{"status": smtStatusOf(record)},
	}, nil
}

func (s *Service) smtReelAdjust(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	reel, err := s.smtGetRecord(ctx, "MREL", key)
	if err != nil {
		return nil, err
	}
	if reel == nil {
		return nil, fmt.Errorf("%w: reel %s not found", ErrValidation, key)
	}
	if smtStatusOf(reel) == "LOADED" {
		return nil, fmt.Errorf("%w: reel %s is loaded; adjust after unloading", ErrValidation, key)
	}
	qty := numericValue(input.Data, "qty")
	if qty < 0 {
		return nil, fmt.Errorf("%w: qty must not be negative", ErrValidation)
	}
	before := numericValue(reel.Data, "qty")
	now := smtNowISO()
	record, err := s.smtUpsertRecord(ctx, "MREL", key, map[string]any{"qty": qty, "adjusted_at": now})
	if err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MREL", key, "REL1", map[string]any{
		"type": "ADJUST", "qty_change": qty - before, "qty_after": qty,
		"ref": "", "operator": smtActor(ctx, input),
		"remark": stringValue(input.Data, "reason", ""), "created_at": now,
	}); err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MREL", Key: key, Action: "adjust", Status: "adjusted", Record: record,
		Effects: map[string]any{"qty_before": before, "qty_after": qty},
	}, nil
}

func (s *Service) smtActiveLoadingForReel(ctx context.Context, reelCode string) (*Record, error) {
	records, err := s.smtQueryAll(ctx, "MLOD", map[string]any{"reel_code": reelCode, "status": "LOADED"}, 5)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

// ---------------------------------------------------------------------------
// Work orders (MSWO).
// ---------------------------------------------------------------------------

var smtWorkOrderTransitions = map[string][2]string{
	"start":  {"RUNNING", "started"},
	"finish": {"FINISHED", "finished"},
	"close":  {"CLOSED", "closed"},
}

func (s *Service) smtWorkOrderTransition(ctx context.Context, key string, action string, input ActionInput) (*ActionResult, error) {
	order, err := s.smtGetRecord(ctx, "MSWO", key)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, key)
	}
	current := smtStatusOf(order)
	target := smtWorkOrderTransitions[action]
	switch action {
	case "start":
		if current == "RUNNING" {
			return nil, fmt.Errorf("%w: work order %s is already running", ErrValidation, key)
		}
		if current == "CLOSED" {
			return nil, fmt.Errorf("%w: work order %s is closed", ErrValidation, key)
		}
	case "finish":
		if current != "RUNNING" {
			return nil, fmt.Errorf("%w: work order %s is %s; only running work orders can finish", ErrValidation, key, current)
		}
	case "close":
		if current == "CLOSED" {
			return nil, fmt.Errorf("%w: work order %s is already closed", ErrValidation, key)
		}
	}
	now := smtNowISO()
	update := map[string]any{"Status": target[0]}
	switch action {
	case "start":
		update["started_at"] = now
		update["completed_units"] = 0
	case "finish":
		update["finished_at"] = now
	case "close":
		update["closed_at"] = now
	}
	record, err := s.smtUpsertRecord(ctx, "MSWO", key, update)
	if err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MSWO", Key: key, Action: action, Status: target[1], Record: record,
		Effects: map[string]any{"status": target[0]},
	}, nil
}

func (s *Service) smtWorkOrderProgress(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	order, err := s.smtGetRecord(ctx, "MSWO", key)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, key)
	}
	if smtStatusOf(order) != "RUNNING" {
		return nil, fmt.Errorf("%w: work order %s is not running", ErrValidation, key)
	}
	doneQty := numericValue(input.Data, "done_qty")
	if doneQty < 0 {
		return nil, fmt.Errorf("%w: done_qty must not be negative", ErrValidation)
	}
	planQty := numericValue(order.Data, "plan_qty")
	if planQty > 0 && doneQty > planQty {
		doneQty = planQty
	}
	record, err := s.smtUpsertRecord(ctx, "MSWO", key, map[string]any{"done_qty": doneQty, "progress_at": smtNowISO()})
	if err != nil {
		return nil, err
	}
	alerts, err := s.smtSyncLowMaterialAlerts(ctx, record, input)
	if err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MSWO", Key: key, Action: "progress", Status: "reported", Record: record,
		Effects: map[string]any{"done_qty": doneQty, "low_alerts": alerts},
	}, nil
}

// ---------------------------------------------------------------------------
// Requisitions (MRQN).
// ---------------------------------------------------------------------------

func (s *Service) smtRequisitionIssue(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	requisition, err := s.smtGetRecord(ctx, "MRQN", key)
	if err != nil {
		return nil, err
	}
	if requisition == nil {
		return nil, fmt.Errorf("%w: requisition %s not found", ErrValidation, key)
	}
	if smtStatusOf(requisition) != "OPEN" {
		return nil, fmt.Errorf("%w: requisition %s is not open", ErrValidation, key)
	}
	reelCode := strings.TrimSpace(stringValue(input.Data, "reel_code", ""))
	reel, err := s.smtGetRecord(ctx, "MREL", reelCode)
	if err != nil {
		return nil, err
	}
	if reel == nil {
		return nil, fmt.Errorf("%w: reel %s not found", ErrValidation, reelCode)
	}
	reelStatus := smtStatusOf(reel)
	if reelStatus != "IN_STOCK" && reelStatus != "ISSUED" {
		return nil, fmt.Errorf("%w: reel %s is %s and cannot be issued", ErrValidation, reelCode, reelStatus)
	}
	partNo := strings.TrimSpace(stringValue(input.Data, "part_no", stringValue(reel.Data, "part_no", "")))
	lines, err := s.listChildPayloads(ctx, "MRQN", key, "RQN1")
	if err != nil {
		return nil, err
	}
	lineIndex := -1
	var line map[string]any
	for index, item := range lines {
		if stringValue(item, "PartNo", "") == partNo {
			lineIndex = index
			line = item
			break
		}
	}
	if line == nil {
		return nil, fmt.Errorf("%w: item %s is not on requisition %s", ErrValidation, partNo, key)
	}
	if err := s.smtRequireReelMatchesProductPart(ctx, stringValue(requisition.Data, "work_order", ""), partNo, stringValue(reel.Data, "part_no", "")); err != nil {
		return nil, err
	}
	now := smtNowISO()
	actor := smtActor(ctx, input)
	order, err := s.smtGetRecord(ctx, "MSWO", stringValue(requisition.Data, "work_order", ""))
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, stringValue(requisition.Data, "work_order", ""))
	}
	// Issue the reel to the work order.
	if _, err := s.smtUpsertRecord(ctx, "MREL", reelCode, map[string]any{
		"status": "ISSUED", "issued_wo": order.Key, "issued_at": now, "requisition": key,
	}); err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MREL", reelCode, "REL1", map[string]any{
		"type": "ISSUE", "qty_change": 0, "qty_after": numericValue(reel.Data, "qty"),
		"ref": key, "operator": actor, "remark": "", "created_at": now,
	}); err != nil {
		return nil, err
	}
	// Update the requisition line progress (issued quantity and reels).
	reels, _ := line["Reels"].([]any)
	reels = append(reels, reelCode)
	issuedQty := numericValue(line, "IssuedQty") + numericValue(reel.Data, "qty")
	lineStatus := "O"
	if required := numericValue(line, "RequiredQty"); required > 0 && issuedQty >= required {
		lineStatus = "C"
	}
	updatedLine := copyData(line)
	updatedLine["IssuedQty"] = issuedQty
	updatedLine["Reels"] = reels
	updatedLine["LineStatus"] = lineStatus
	parent, child, err := s.child("MRQN", "RQN1")
	if err != nil {
		return nil, err
	}
	lineKey := fmt.Sprint(lineIndex + 1)
	if _, err := s.repo.UpdateChildRecord(ctx, parent, child, key, lineKey, RecordInput{Data: updatedLine}); err != nil {
		return nil, err
	}
	// Refresh the work order material summary.
	if err := s.smtRefreshWorkOrderMaterials(ctx, order.Key); err != nil {
		return nil, err
	}
	updated, err := s.smtGetRecord(ctx, "MRQN", key)
	if err != nil {
		return nil, err
	}
	over := false
	if required := numericValue(line, "RequiredQty"); required > 0 && issuedQty > required {
		over = true
	}
	return &ActionResult{
		TableCode: "MRQN", Key: key, Action: "issue", Status: "issued", Record: updated,
		GeneratedRecords: []Record{
			smtGenerated("MREL", reelCode, "issued", map[string]any{"part_no": partNo, "qty": numericValue(reel.Data, "qty"), "work_order": order.Key}),
		},
		Effects: map[string]any{"part_no": partNo, "reel_code": reelCode, "issued_qty": issuedQty, "over": over},
	}, nil
}

func (s *Service) smtRequireReelMatchesProductPart(ctx context.Context, workOrder, linePartNo, reelPartNo string) error {
	if linePartNo == reelPartNo {
		return nil
	}
	alternates, err := s.smtProductPartAlternates(ctx, workOrder, linePartNo)
	if err != nil {
		return err
	}
	for _, alt := range alternates {
		if alt == reelPartNo {
			return nil
		}
	}
	return fmt.Errorf("%w: reel part %s does not match requisition item %s", ErrValidation, reelPartNo, linePartNo)
}

func (s *Service) smtProductPartAlternates(ctx context.Context, workOrder, partNo string) ([]string, error) {
	order, err := s.smtGetRecord(ctx, "MSWO", workOrder)
	if err != nil || order == nil {
		return nil, err
	}
	productCode := stringValue(order.Data, "product_code", "")
	if productCode == "" {
		return nil, nil
	}
	return s.smtBOMPartAlternates(ctx, productCode, partNo)
}

func (s *Service) smtBOMPartAlternates(ctx context.Context, productCode, partNo string) ([]string, error) {
	lines, err := s.listChildPayloads(ctx, "MPRD", productCode, "PRD1")
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		if stringValue(line, "PartNo", "") != partNo {
			continue
		}
		raw := stringValue(line, "AltPartNos", "")
		raw = strings.ReplaceAll(raw, ",", ";")
		var alternates []string
		for _, value := range strings.Split(raw, ";") {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				alternates = append(alternates, trimmed)
			}
		}
		return alternates, nil
	}
	return nil, nil
}

func (s *Service) smtRequisitionTransition(ctx context.Context, key string, action string, input ActionInput) (*ActionResult, error) {
	requisition, err := s.smtGetRecord(ctx, "MRQN", key)
	if err != nil {
		return nil, err
	}
	if requisition == nil {
		return nil, fmt.Errorf("%w: requisition %s not found", ErrValidation, key)
	}
	current := smtStatusOf(requisition)
	if current != "OPEN" {
		return nil, fmt.Errorf("%w: requisition %s is %s", ErrValidation, key, current)
	}
	target := "CLOSED"
	status := "closed"
	if action == "cancel" {
		target = "CANCELLED"
		status = "cancelled"
	}
	now := smtNowISO()
	record, err := s.smtUpsertRecord(ctx, "MRQN", key, map[string]any{"Status": target, "closed_at": now})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MRQN", Key: key, Action: action, Status: status, Record: record}, nil
}

// smtRefreshWorkOrderMaterials recomputes the SWO1 material summary rows from
// the BOM demand against issued reels.
func (s *Service) smtRefreshWorkOrderMaterials(ctx context.Context, workOrder string) error {
	order, err := s.smtGetRecord(ctx, "MSWO", workOrder)
	if err != nil || order == nil {
		return err
	}
	productCode := stringValue(order.Data, "product_code", "")
	if productCode == "" {
		return nil
	}
	bomLines, err := s.listChildPayloads(ctx, "MPRD", productCode, "PRD1")
	if err != nil {
		return err
	}
	planQty := numericValue(order.Data, "plan_qty")
	lossRate := numericValue(order.Data, "loss_rate")
	requisitions, err := s.smtQueryAll(ctx, "MRQN", map[string]any{"work_order": workOrder}, 500)
	if err != nil {
		return err
	}
	issuedByPart := map[string]float64{}
	issuedReels := map[string]float64{}
	for _, requisition := range requisitions {
		if smtStatusOf(&requisition) == "CANCELLED" {
			continue
		}
		lines, err := s.listChildPayloads(ctx, "MRQN", requisition.Key, "RQN1")
		if err != nil {
			return err
		}
		for _, line := range lines {
			partNo := stringValue(line, "PartNo", "")
			issuedByPart[partNo] += numericValue(line, "IssuedQty")
			if reels, ok := line["Reels"].([]any); ok {
				issuedReels[partNo] += float64(len(reels))
			}
		}
	}
	existing, err := s.listChildPayloads(ctx, "MSWO", workOrder, "SWO1")
	if err != nil {
		return err
	}
	existingByPart := map[string]map[string]any{}
	for _, line := range existing {
		existingByPart[stringValue(line, "PartNo", "")] = line
	}
	parent, child, err := s.child("MSWO", "SWO1")
	if err != nil {
		return err
	}
	for index, bom := range bomLines {
		partNo := stringValue(bom, "PartNo", "")
		usage := numericValue(bom, "Qty")
		required := usage * planQty * (1 + lossRate/100)
		lineKey := fmt.Sprint(index + 1)
		data := map[string]any{
			"PartNo": partNo, "Usage": usage, "RequiredQty": required,
			"IssuedQty": issuedByPart[partNo], "Reels": issuedReels[partNo],
			"Gap": required - issuedByPart[partNo], "LineStatus": "O",
		}
		if _, ok := existingByPart[partNo]; ok {
			if _, err := s.repo.UpdateChildRecord(ctx, parent, child, workOrder, lineKey, RecordInput{Data: data}); err != nil {
				return err
			}
			continue
		}
		if err := s.smtAppendChild(ctx, "MSWO", workOrder, "SWO1", data); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Machine actions (MMAC): bind, verify, sign-off, slot lock.
// ---------------------------------------------------------------------------

// smtActiveLoadings returns the currently loaded MLOD records of a machine.
func (s *Service) smtActiveLoadings(ctx context.Context, machineCode string) ([]Record, error) {
	return s.smtQueryAll(ctx, "MLOD", map[string]any{"machine_code": machineCode, "status": "LOADED"}, 500)
}

// smtActiveFeederSetup finds the active feeder setup for a product + machine.
func (s *Service) smtActiveFeederSetup(ctx context.Context, productCode, machineCode string) (*Record, []map[string]any, error) {
	setups, err := s.smtQueryAll(ctx, "MFSB", map[string]any{"machine_code": machineCode, "product_code": productCode}, 50)
	if err != nil {
		return nil, nil, err
	}
	for index := range setups {
		setup := setups[index]
		if active, ok := setup.Data["active"].(bool); ok && !active {
			continue
		}
		if value := strings.ToLower(stringValue(setup.Data, "active", "true")); value == "false" || value == "n" || value == "0" {
			continue
		}
		rows, err := s.listChildPayloads(ctx, "MFSB", setup.Key, "FSB1")
		if err != nil {
			return nil, nil, err
		}
		return &setup, rows, nil
	}
	return nil, nil, nil
}

// smtSlotRows returns the slot configuration rows of a machine.
func (s *Service) smtSlotRows(ctx context.Context, machineCode string) ([]map[string]any, error) {
	machine, err := s.smtGetRecord(ctx, "MMAC", machineCode)
	if err != nil || machine == nil {
		return nil, err
	}
	return s.listChildPayloads(ctx, "MMAC", machineCode, "MAC1")
}

// smtFindSlot resolves a scanned slot code ("F01" or "M01-F01") into the slot row.
func (s *Service) smtFindSlot(ctx context.Context, machineCode, slotCode string) (map[string]any, error) {
	slotCode = strings.TrimSpace(slotCode)
	if slotCode == "" {
		return nil, nil
	}
	normalized := slotCode
	if index := strings.LastIndex(slotCode, "-"); index >= 0 {
		normalized = slotCode[index+1:]
	}
	rows, err := s.smtSlotRows(ctx, machineCode)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if strings.EqualFold(stringValue(row, "SlotNo", ""), normalized) || strings.EqualFold(stringValue(row, "Barcode", ""), slotCode) {
			return row, nil
		}
	}
	return nil, nil
}

func (s *Service) smtMachineBind(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	machine, err := s.smtGetRecord(ctx, "MMAC", key)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return nil, fmt.Errorf("%w: machine %s not found", ErrValidation, key)
	}
	workOrder := strings.TrimSpace(stringValue(input.Data, "work_order", ""))
	order, err := s.smtGetRecord(ctx, "MSWO", workOrder)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, workOrder)
	}
	if smtStatusOf(order) == "CLOSED" {
		return nil, fmt.Errorf("%w: work order %s is closed", ErrValidation, workOrder)
	}
	previous := stringValue(machine.Data, "current_work_order", "")
	now := smtNowISO()
	checklist, err := s.smtChangeoverChecklist(ctx, key, stringValue(order.Data, "product_code", ""))
	if err != nil {
		return nil, err
	}
	record, err := s.smtUpsertRecord(ctx, "MMAC", key, map[string]any{
		"current_work_order":   workOrder,
		"verified_at":          "",
		"verified_by":          "",
		"signed_off_at":        "",
		"signed_off_by":        "",
		"changeover_at":        now,
		"changeover_from":      previous,
		"changeover_checklist": checklist,
	})
	if err != nil {
		return nil, err
	}
	// Changeover downtime: start when enabled and the machine actually changes order.
	if previous != workOrder && s.smtBoolSetting(ctx, "downtime_from_changeover") {
		if err := s.smtStartDowntime(ctx, map[string]any{
			"machine_code": key, "line_code": stringValue(machine.Data, "line_code", ""),
			"work_order": workOrder, "reason_type": "CHANGEOVER",
			"reason": fmt.Sprintf("换线：%s → %s", previous, workOrder),
			"source": "CHANGEOVER", "changeover_at": now,
		}, input); err != nil {
			return nil, err
		}
	}
	return &ActionResult{
		TableCode: "MMAC", Key: key, Action: "bind", Status: "bound", Record: record,
		Effects: map[string]any{"work_order": workOrder, "previous_work_order": previous, "checklist": checklist},
	}, nil
}

func (s *Service) smtChangeoverChecklist(ctx context.Context, machineCode, productCode string) (map[string]any, error) {
	setup, rows, err := s.smtActiveFeederSetup(ctx, productCode, machineCode)
	if err != nil {
		return nil, err
	}
	if setup == nil {
		return map[string]any{"has_setup": false, "keep": 0, "swap": 0, "missing": 0, "extra": 0}, nil
	}
	loadings, err := s.smtActiveLoadings(ctx, machineCode)
	if err != nil {
		return nil, err
	}
	requiredByPart := map[string]bool{}
	for _, row := range rows {
		requiredByPart[stringValue(row, "PartNo", "")] = true
	}
	loadedParts := map[string]bool{}
	keep, swap := 0, 0
	for _, loading := range loadings {
		partNo := stringValue(loading.Data, "part_no", "")
		loadedParts[partNo] = true
		if requiredByPart[partNo] {
			keep++
		} else {
			swap++
		}
	}
	missing := 0
	for partNo := range requiredByPart {
		if !loadedParts[partNo] {
			missing++
		}
	}
	extra := 0
	for partNo := range loadedParts {
		if !requiredByPart[partNo] {
			extra++
		}
	}
	return map[string]any{
		"has_setup": true, "setup": setup.Key,
		"keep": keep, "swap": swap, "missing": missing, "extra": extra,
	}, nil
}

// smtMachineContext computes the station context of a machine: per-slot state
// (OK/ALT/MISMATCH/EXTRA/EMPTY/NA) with main and spliced reels plus estimates.
func (s *Service) smtMachineContext(ctx context.Context, machineCode string) (map[string]any, error) {
	machine, err := s.smtGetRecord(ctx, "MMAC", machineCode)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return nil, fmt.Errorf("%w: machine %s not found", ErrValidation, machineCode)
	}
	workOrderCode := stringValue(machine.Data, "current_work_order", "")
	var productCode string
	doneQty := 0.0
	if workOrderCode != "" {
		if order, err := s.smtGetRecord(ctx, "MSWO", workOrderCode); err != nil {
			return nil, err
		} else if order != nil {
			productCode = stringValue(order.Data, "product_code", "")
			doneQty = numericValue(order.Data, "done_qty")
		}
	}
	slotRows, err := s.smtSlotRows(ctx, machineCode)
	if err != nil {
		return nil, err
	}
	loadings, err := s.smtActiveLoadings(ctx, machineCode)
	if err != nil {
		return nil, err
	}
	var setupRows []map[string]any
	if productCode != "" {
		_, setupRows, err = s.smtActiveFeederSetup(ctx, productCode, machineCode)
		if err != nil {
			return nil, err
		}
	}
	requiredBySlot := map[string]map[string]any{}
	for _, row := range setupRows {
		requiredBySlot[strings.ToUpper(stringValue(row, "SlotNo", ""))] = row
	}
	loadingsBySlot := map[string]map[string]*Record{}
	for index := range loadings {
		loading := &loadings[index]
		slotNo := strings.ToUpper(stringValue(loading.Data, "slot_no", ""))
		if loadingsBySlot[slotNo] == nil {
			loadingsBySlot[slotNo] = map[string]*Record{}
		}
		position := strings.ToUpper(stringValue(loading.Data, "position", "MAIN"))
		loadingsBySlot[slotNo][position] = loading
	}
	lowQty := float64(s.smtIntSetting(ctx, "low_qty_threshold", 500))
	bomUsage := map[string]float64{}
	if productCode != "" {
		bomUsage, err = s.smtBOMUsage(ctx, productCode)
		if err != nil {
			return nil, err
		}
	}
	slotStates := make([]map[string]any, 0, len(slotRows))
	summary := map[string]int{"ok": 0, "alt": 0, "mismatch": 0, "extra": 0, "empty": 0, "na": 0}
	for _, row := range slotRows {
		slotNo := strings.ToUpper(stringValue(row, "SlotNo", ""))
		state := "NA"
		entry := map[string]any{
			"slot_no": stringValue(row, "SlotNo", ""),
			"barcode": stringValue(row, "Barcode", ""),
			"locked":  boolPayload(row, "Locked"),
		}
		required := requiredBySlot[slotNo]
		loadingsForSlot := loadingsBySlot[slotNo]
		if required == nil {
			if len(loadingsForSlot) > 0 {
				state = "EXTRA"
				entry["main"] = smtLoadingPayload(loadingsForSlot["MAIN"], bomUsage, doneQty, lowQty)
				entry["spliced"] = smtLoadingPayload(loadingsForSlot["SPLICED"], bomUsage, doneQty, lowQty)
			}
		} else {
			entry["required_part_no"] = stringValue(required, "PartNo", "")
			if len(loadingsForSlot) == 0 {
				state = "EMPTY"
			} else {
				main := loadingsForSlot["MAIN"]
				state = "MISMATCH"
				if main != nil {
					partNo := stringValue(main.Data, "part_no", "")
					requiredPart := stringValue(required, "PartNo", "")
					if partNo == requiredPart {
						state = "OK"
					} else if isAlternateOf(required, partNo) {
						state = "ALT"
					}
				}
				entry["main"] = smtLoadingPayload(main, bomUsage, doneQty, lowQty)
				entry["spliced"] = smtLoadingPayload(loadingsForSlot["SPLICED"], bomUsage, doneQty, lowQty)
			}
		}
		entry["state"] = state
		summary[strings.ToLower(state)]++
		slotStates = append(slotStates, entry)
	}
	verifiedAt := stringValue(machine.Data, "verified_at", "")
	signedOffAt := stringValue(machine.Data, "signed_off_at", "")
	signoffRequired := s.smtBoolSetting(ctx, "signoff_required")
	ready := verifiedAt != "" && (!signoffRequired || signedOffAt != "")
	return map[string]any{
		"machine_code":     machineCode,
		"line_code":        stringValue(machine.Data, "line_code", ""),
		"work_order":       workOrderCode,
		"product_code":     productCode,
		"verified_at":      verifiedAt,
		"verified_by":      stringValue(machine.Data, "verified_by", ""),
		"signed_off_at":    signedOffAt,
		"signed_off_by":    stringValue(machine.Data, "signed_off_by", ""),
		"signoff_required": signoffRequired,
		"ready":            ready,
		"awaiting_signoff": verifiedAt != "" && signoffRequired && signedOffAt == "",
		"slots":            slotStates,
		"summary":          summary,
	}, nil
}

func boolPayload(values map[string]any, key string) bool {
	value, present := values[key]
	if !present {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		return lower == "true" || lower == "1" || lower == "y" || lower == "yes"
	default:
		return false
	}
}

func isAlternateOf(requiredRow map[string]any, partNo string) bool {
	raw := stringValue(requiredRow, "AltPartNos", "")
	raw = strings.ReplaceAll(raw, ",", ";")
	for _, value := range strings.Split(raw, ";") {
		if strings.TrimSpace(value) == partNo {
			return true
		}
	}
	return false
}

// smtBOMUsage returns per-part single-board usage from the product BOM.
func (s *Service) smtBOMUsage(ctx context.Context, productCode string) (map[string]float64, error) {
	usage := map[string]float64{}
	if productCode == "" {
		return usage, nil
	}
	lines, err := s.listChildPayloads(ctx, "MPRD", productCode, "PRD1")
	if err != nil {
		if err == ErrNotFound {
			return usage, nil
		}
		return nil, err
	}
	for _, line := range lines {
		usage[stringValue(line, "PartNo", "")] += numericValue(line, "Qty")
		if usage[stringValue(line, "PartNo", "")] == 0 {
			usage[stringValue(line, "PartNo", "")] = 1
		}
	}
	return usage, nil
}

// smtLoadingPayload enriches a loading row with usage estimates and low warning.
func smtLoadingPayload(loading *Record, bomUsage map[string]float64, doneQty, lowQty float64) map[string]any {
	if loading == nil {
		return nil
	}
	partNo := stringValue(loading.Data, "part_no", "")
	qtyAtLoad := numericValue(loading.Data, "qty_at_load")
	doneAtLoad := numericValue(loading.Data, "done_qty_at_load")
	usage := bomUsage[partNo]
	if usage <= 0 {
		usage = 1
	}
	consumed := (doneQty - doneAtLoad) * usage
	if consumed < 0 {
		consumed = 0
	}
	qtyEst := qtyAtLoad - consumed
	if qtyEst < 0 {
		qtyEst = 0
	}
	boardsLeft := qtyEst / usage
	return map[string]any{
		"load_code": loading.Key, "reel_code": stringValue(loading.Data, "reel_code", ""),
		"part_no": partNo, "position": stringValue(loading.Data, "position", "MAIN"),
		"qty_at_load": qtyAtLoad, "qty_est": qtyEst, "boards_left": boardsLeft,
		"is_alternate": isTruthy(loading.Data["is_alternate"]),
		"low":          lowQty > 0 && qtyEst > 0 && qtyEst < lowQty,
		"loaded_at":    stringValue(loading.Data, "loaded_at", ""),
	}
}

func isTruthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		return lower == "true" || lower == "1" || lower == "y"
	}
	return false
}

func (s *Service) smtMachineVerify(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	machine, err := s.smtGetRecord(ctx, "MMAC", key)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return nil, fmt.Errorf("%w: machine %s not found", ErrValidation, key)
	}
	contextData, err := s.smtMachineContext(ctx, key)
	if err != nil {
		return nil, err
	}
	issues := make([]map[string]any, 0)
	for _, slot := range mapSliceAny(contextData["slots"]) {
		state := stringValue(slot, "state", "")
		if state != "OK" && state != "ALT" && state != "NA" {
			issues = append(issues, map[string]any{"slot_no": stringValue(slot, "slot_no", ""), "state": state})
		}
	}
	now := smtNowISO()
	actor := smtActor(ctx, input)
	if len(issues) > 0 {
		if _, err := s.smtUpsertRecord(ctx, "MMAC", key, map[string]any{
			"verified_at": "", "verified_by": "", "signed_off_at": "", "signed_off_by": "",
			"last_verify_at": now, "last_verify_issues": issues,
		}); err != nil {
			return nil, err
		}
		return &ActionResult{
			TableCode: "MMAC", Key: key, Action: "verify", Status: "failed",
			Effects: map[string]any{
				"ok": false, "issue_count": len(issues), "issues": issues,
				"message": "整机核对未通过", "voice": "核对未通过",
			},
		}, nil
	}
	signoffRequired := s.smtBoolSetting(ctx, "signoff_required")
	update := map[string]any{
		"verified_at": now, "verified_by": actor,
		"last_verify_at": now, "last_verify_issues": []map[string]any{},
	}
	if !signoffRequired {
		update["signed_off_at"] = ""
		update["signed_off_by"] = ""
	}
	record, err := s.smtUpsertRecord(ctx, "MMAC", key, update)
	if err != nil {
		return nil, err
	}
	// A successful verification ends an open changeover downtime when no sign-off is required.
	if !signoffRequired && s.smtBoolSetting(ctx, "downtime_from_changeover") {
		if err := s.smtEndDowntimeBySource(ctx, "CHANGEOVER", key, input); err != nil {
			return nil, err
		}
	}
	status := "verified"
	message := "整机核对通过"
	if signoffRequired {
		status = "awaiting_signoff"
		message = "核对通过，等待签核"
	}
	return &ActionResult{
		TableCode: "MMAC", Key: key, Action: "verify", Status: status, Record: record,
		Effects: map[string]any{
			"ok": true, "signoff_required": signoffRequired,
			"message": message, "voice": "核对通过",
			"summary": contextData["summary"],
		},
	}, nil
}

func mapSliceAny(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				items = append(items, mapped)
			}
		}
		return items
	}
	return nil
}

func (s *Service) smtMachineSignOff(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	machine, err := s.smtGetRecord(ctx, "MMAC", key)
	if err != nil {
		return nil, err
	}
	if machine == nil {
		return nil, fmt.Errorf("%w: machine %s not found", ErrValidation, key)
	}
	if stringValue(machine.Data, "verified_at", "") == "" {
		return nil, fmt.Errorf("%w: machine %s must be verified before sign-off", ErrValidation, key)
	}
	verifier := stringValue(machine.Data, "verified_by", "")
	actor := smtActor(ctx, input)
	if verifier != "" && verifier == actor {
		return nil, fmt.Errorf("%w: sign-off requires a second person (verifier %s cannot sign off)", ErrValidation, verifier)
	}
	now := smtNowISO()
	record, err := s.smtUpsertRecord(ctx, "MMAC", key, map[string]any{"signed_off_at": now, "signed_off_by": actor})
	if err != nil {
		return nil, err
	}
	if s.smtBoolSetting(ctx, "downtime_from_changeover") {
		if err := s.smtEndDowntimeBySource(ctx, "CHANGEOVER", key, input); err != nil {
			return nil, err
		}
	}
	return &ActionResult{
		TableCode: "MMAC", Key: key, Action: "sign-off", Status: "signed_off", Record: record,
		Effects: map[string]any{"message": "签核通过", "voice": "签核通过", "signer": actor, "verifier": verifier},
	}, nil
}

func (s *Service) smtMachineSlotLock(ctx context.Context, key string, action string, input ActionInput) (*ActionResult, error) {
	slotCode := strings.TrimSpace(stringValue(input.Data, "slot_no", ""))
	if slotCode == "" {
		return nil, fmt.Errorf("%w: slot_no is required", ErrValidation)
	}
	slot, err := s.smtFindSlot(ctx, key, slotCode)
	if err != nil {
		return nil, err
	}
	if slot == nil {
		return nil, fmt.Errorf("%w: slot %s not found on machine %s", ErrValidation, slotCode, key)
	}
	slotNo := stringValue(slot, "SlotNo", "")
	if err := s.smtUpdateSlot(ctx, key, slotNo, func(row map[string]any) map[string]any {
		if action == "lock-slot" {
			row["Locked"] = true
			row["LockReason"] = stringValue(input.Data, "reason", "")
			row["LockedBy"] = smtActor(ctx, input)
			row["LockedAt"] = smtNowISO()
		} else {
			row["Locked"] = false
			row["LockReason"] = ""
			row["LockedBy"] = ""
			row["LockedAt"] = ""
		}
		return row
	}); err != nil {
		return nil, err
	}
	status := "locked"
	if action == "unlock-slot" {
		status = "unlocked"
	}
	return &ActionResult{
		TableCode: "MMAC", Key: key, Action: action, Status: status,
		Effects: map[string]any{"slot_no": slotNo},
	}, nil
}

func (s *Service) smtUpdateSlot(ctx context.Context, machineCode, slotNo string, mutate func(map[string]any) map[string]any) error {
	rows, err := s.smtSlotRows(ctx, machineCode)
	if err != nil {
		return err
	}
	parent, child, err := s.child("MMAC", "MAC1")
	if err != nil {
		return err
	}
	for index, row := range rows {
		if !strings.EqualFold(stringValue(row, "SlotNo", ""), slotNo) {
			continue
		}
		lineKey := fmt.Sprint(index + 1)
		_, err := s.repo.UpdateChildRecord(ctx, parent, child, machineCode, lineKey, RecordInput{Data: mutate(copyData(row))})
		return err
	}
	return fmt.Errorf("%w: slot %s not found on machine %s", ErrValidation, slotNo, machineCode)
}

// ---------------------------------------------------------------------------
// Inspection (MINS).
// ---------------------------------------------------------------------------

func (s *Service) smtInspectionRecord(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	inspType := strings.ToUpper(stringValue(input.Data, "insp_type", "FAI"))
	if inspType != "FAI" && inspType != "IPQC" {
		return nil, fmt.Errorf("%w: insp_type must be FAI or IPQC", ErrValidation)
	}
	workOrder := strings.TrimSpace(stringValue(input.Data, "work_order", ""))
	order, err := s.smtGetRecord(ctx, "MSWO", workOrder)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, workOrder)
	}
	if smtStatusOf(order) != "RUNNING" {
		return nil, fmt.Errorf("%w: work order %s is not running", ErrValidation, workOrder)
	}
	result := strings.ToUpper(stringValue(input.Data, "result", "PASS"))
	if result != "PASS" && result != "FAIL" {
		return nil, fmt.Errorf("%w: result must be PASS or FAIL", ErrValidation)
	}
	machinesSnapshot, err := s.smtMachineSnapshots(ctx, workOrder)
	if err != nil {
		return nil, err
	}
	if inspType == "FAI" && s.smtBoolSetting(ctx, "fai_require_verified") {
		for _, snapshot := range machinesSnapshot {
			if !isTruthy(snapshot["verified"]) {
				return nil, fmt.Errorf("%w: machine %s is not verified; complete feeder verification before FAI", ErrValidation, stringValue(snapshot, "machine_code", ""))
			}
		}
	}
	now := smtNowISO()
	actor := smtActor(ctx, input)
	record, err := s.smtUpsertRecord(ctx, "MINS", key, map[string]any{
		"insp_type": inspType, "work_order": workOrder,
		"product_code": stringValue(order.Data, "product_code", ""),
		"sample_sn":    stringValue(input.Data, "sample_sn", ""),
		"result":       result,
		"items":        input.Data["items"],
		"machines":     machinesSnapshot,
		"remark":       stringValue(input.Data, "remark", ""),
		"inspector":    actor, "created_at": now,
		"Status": "closed",
	})
	if err != nil {
		return nil, err
	}
	if items, ok := input.Data["items"].([]any); ok && len(items) > 0 {
		parent, child, err := s.child("MINS", "INS1")
		if err != nil {
			return nil, err
		}
		for index, item := range items {
			mapped, ok := item.(map[string]any)
			if !ok {
				continue
			}
			line := copyData(mapped)
			line["LineNum"] = index + 1
			if _, err := s.repo.CreateChildRecord(ctx, parent, child, key, RecordInput{Key: fmt.Sprint(index + 1), Data: line}); err != nil {
				return nil, err
			}
		}
	}
	return &ActionResult{
		TableCode: "MINS", Key: key, Action: "record", Status: strings.ToLower(result), Record: record,
		Effects: map[string]any{"insp_type": inspType, "result": result},
	}, nil
}

// smtMachineSnapshots captures the verification state of all machines bound to
// a work order (stored on FAI/IPQC records).
func (s *Service) smtMachineSnapshots(ctx context.Context, workOrder string) ([]map[string]any, error) {
	machines, err := s.smtQueryAll(ctx, "MMAC", map[string]any{"current_work_order": workOrder}, 100)
	if err != nil {
		return nil, err
	}
	snapshots := make([]map[string]any, 0, len(machines))
	for index := range machines {
		machine := &machines[index]
		loadings, err := s.smtActiveLoadings(ctx, machine.Key)
		if err != nil {
			return nil, err
		}
		slots := make([]map[string]any, 0, len(loadings))
		for _, loading := range loadings {
			slots = append(slots, map[string]any{
				"slot_no":   stringValue(loading.Data, "slot_no", ""),
				"reel_code": stringValue(loading.Data, "reel_code", ""),
				"part_no":   stringValue(loading.Data, "part_no", ""),
				"position":  stringValue(loading.Data, "position", "MAIN"),
			})
		}
		snapshots = append(snapshots, map[string]any{
			"machine_code": machine.Key,
			"verified":     stringValue(machine.Data, "verified_at", "") != "",
			"verified_at":  stringValue(machine.Data, "verified_at", ""),
			"signed_off":   stringValue(machine.Data, "signed_off_at", "") != "",
			"slots":        slots,
		})
	}
	return snapshots, nil
}

// ---------------------------------------------------------------------------
// Andon (MAND).
// ---------------------------------------------------------------------------

var smtAndonTypes = map[string]string{
	"MATERIAL": "缺料", "EQUIPMENT": "设备", "QUALITY": "质量", "PROCESS": "工艺", "OTHER": "其它",
}

func (s *Service) smtAndonRaise(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	callType := strings.ToUpper(stringValue(input.Data, "call_type", "OTHER"))
	if _, ok := smtAndonTypes[callType]; !ok {
		return nil, fmt.Errorf("%w: unknown andon call type %s", ErrValidation, callType)
	}
	source := strings.ToUpper(stringValue(input.Data, "source", "MANUAL"))
	if source != "MANUAL" && source != "AUTO" {
		source = "MANUAL"
	}
	ref := strings.TrimSpace(stringValue(input.Data, "ref", ""))
	if source == "AUTO" && ref != "" {
		// Deduplicate automatic calls while an equivalent call is still open.
		openCalls, err := s.smtQueryAll(ctx, "MAND", map[string]any{"source": "AUTO", "ref": ref, "status": "OPEN"}, 5)
		if err != nil {
			return nil, err
		}
		if len(openCalls) > 0 {
			return &ActionResult{
				TableCode: "MAND", Key: openCalls[0].Key, Action: "raise", Status: "duplicate", Record: &openCalls[0],
				Effects: map[string]any{"duplicate": true},
			}, nil
		}
	}
	now := smtNowISO()
	actor := smtActor(ctx, input)
	machineCode := strings.TrimSpace(stringValue(input.Data, "machine_code", ""))
	lineCode := strings.TrimSpace(stringValue(input.Data, "line_code", ""))
	if machineCode != "" && lineCode == "" {
		if machine, _ := s.smtGetRecord(ctx, "MMAC", machineCode); machine != nil {
			lineCode = stringValue(machine.Data, "line_code", "")
		}
	}
	record, err := s.smtUpsertRecord(ctx, "MAND", key, map[string]any{
		"line_code": lineCode, "machine_code": machineCode,
		"work_order": strings.TrimSpace(stringValue(input.Data, "work_order", "")),
		"call_type":  callType, "description": stringValue(input.Data, "description", ""),
		"source": source, "ref": ref, "status": "OPEN",
		"called_by": actor, "called_at": now,
		"cause": "", "solution": "",
		"Status": "OPEN",
	})
	if err != nil {
		return nil, err
	}
	_ = s.smtAppendChild(ctx, "MAND", key, "AND1", map[string]any{
		"action": "RAISE", "actor": actor,
		"note": stringValue(input.Data, "description", ""), "created_at": now,
	})
	// Equipment calls and manual material calls start a downtime record.
	if s.smtBoolSetting(ctx, "downtime_from_andon") && (callType == "EQUIPMENT" || (callType == "MATERIAL" && source == "MANUAL")) {
		if err := s.smtStartDowntime(ctx, map[string]any{
			"machine_code": machineCode, "line_code": lineCode,
			"work_order":  stringValue(input.Data, "work_order", ""),
			"reason_type": map[string]string{"EQUIPMENT": "EQUIPMENT", "MATERIAL": "MATERIAL"}[callType],
			"reason":      smtAndonTypes[callType] + "呼叫 " + key,
			"source":      "ANDON", "andon_call": key,
		}, input); err != nil {
			return nil, err
		}
	}
	return &ActionResult{
		TableCode: "MAND", Key: key, Action: "raise", Status: "open", Record: record,
		Effects: map[string]any{"call_no": key, "call_type": callType},
	}, nil
}

func (s *Service) smtAndonTransition(ctx context.Context, key string, action string, input ActionInput) (*ActionResult, error) {
	call, err := s.smtGetRecord(ctx, "MAND", key)
	if err != nil {
		return nil, err
	}
	if call == nil {
		return nil, fmt.Errorf("%w: andon call %s not found", ErrValidation, key)
	}
	status := smtStatusOf(call)
	now := smtNowISO()
	actor := smtActor(ctx, input)
	var record *Record
	switch action {
	case "ack":
		if status != "OPEN" {
			return nil, fmt.Errorf("%w: andon call %s is %s", ErrValidation, key, status)
		}
		record, err = s.smtUpsertRecord(ctx, "MAND", key, map[string]any{"status": "ACK", "Status": "ACK", "ack_by": actor, "ack_at": now})
		if err != nil {
			return nil, err
		}
		_ = s.smtAppendChild(ctx, "MAND", key, "AND1", map[string]any{"action": "ACK", "actor": actor, "note": "", "created_at": now})
	case "close":
		if status == "CLOSED" {
			return nil, fmt.Errorf("%w: andon call %s is already closed", ErrValidation, key)
		}
		record, err = s.smtUpsertRecord(ctx, "MAND", key, map[string]any{
			"status": "CLOSED", "Status": "CLOSED",
			"closed_by": actor, "closed_at": now,
			"cause":    stringValue(input.Data, "cause", ""),
			"solution": stringValue(input.Data, "solution", ""),
		})
		if err != nil {
			return nil, err
		}
		_ = s.smtAppendChild(ctx, "MAND", key, "AND1", map[string]any{
			"action": "CLOSE", "actor": actor, "note": stringValue(input.Data, "solution", ""), "created_at": now,
		})
		// Closing the linked call ends the auto-started downtime.
		if s.smtBoolSetting(ctx, "downtime_from_andon") {
			if err := s.smtEndDowntimeByAndon(ctx, key, input); err != nil {
				return nil, err
			}
		}
	}
	return &ActionResult{TableCode: "MAND", Key: key, Action: action, Status: strings.ToLower(action), Record: record}, nil
}

// ---------------------------------------------------------------------------
// Downtime (MDWN).
// ---------------------------------------------------------------------------

func (s *Service) smtStartDowntime(ctx context.Context, data map[string]any, input ActionInput) error {
	machineCode := stringValue(data, "machine_code", "")
	lineCode := stringValue(data, "line_code", "")
	if machineCode == "" && lineCode == "" {
		return fmt.Errorf("%w: downtime requires a machine or line", ErrValidation)
	}
	key := smtGenerateKey("DT")
	now := smtNowISO()
	_, err := s.smtUpsertRecord(ctx, "MDWN", key, map[string]any{
		"machine_code": machineCode, "line_code": lineCode,
		"work_order":  stringValue(data, "work_order", ""),
		"reason_type": stringValue(data, "reason_type", "OTHER"),
		"reason":      stringValue(data, "reason", ""),
		"planned":     isTruthy(data["planned"]),
		"source":      stringValue(data, "source", "MANUAL"),
		"andon_call":  stringValue(data, "andon_call", ""),
		"started_at":  now, "started_by": smtActor(ctx, input),
		"ended_at": "", "ended_by": "", "remark": "",
	})
	return err
}

func (s *Service) smtEndDowntimeBySource(ctx context.Context, source string, machineCode string, input ActionInput) error {
	open, err := s.smtQueryAll(ctx, "MDWN", map[string]any{"machine_code": machineCode, "source": source}, 20)
	if err != nil {
		return err
	}
	now := smtNowISO()
	for index := range open {
		record := &open[index]
		if stringValue(record.Data, "ended_at", "") != "" {
			continue
		}
		if _, err := s.smtUpsertRecord(ctx, "MDWN", record.Key, map[string]any{"ended_at": now, "ended_by": smtActor(ctx, input)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) smtEndDowntimeByAndon(ctx context.Context, callNo string, input ActionInput) error {
	open, err := s.smtQueryAll(ctx, "MDWN", map[string]any{"andon_call": callNo}, 20)
	if err != nil {
		return err
	}
	now := smtNowISO()
	for index := range open {
		record := &open[index]
		if stringValue(record.Data, "ended_at", "") != "" {
			continue
		}
		if _, err := s.smtUpsertRecord(ctx, "MDWN", record.Key, map[string]any{"ended_at": now, "ended_by": smtActor(ctx, input)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) smtDowntimeStart(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	machineCode := strings.TrimSpace(stringValue(input.Data, "machine_code", ""))
	lineCode := strings.TrimSpace(stringValue(input.Data, "line_code", ""))
	if machineCode == "" && lineCode == "" {
		return nil, fmt.Errorf("%w: downtime requires a machine or line", ErrValidation)
	}
	if machineCode != "" && lineCode == "" {
		if machine, _ := s.smtGetRecord(ctx, "MMAC", machineCode); machine != nil {
			lineCode = stringValue(machine.Data, "line_code", "")
		}
	}
	now := smtNowISO()
	record, err := s.smtUpsertRecord(ctx, "MDWN", key, map[string]any{
		"machine_code": machineCode, "line_code": lineCode,
		"work_order":  strings.TrimSpace(stringValue(input.Data, "work_order", "")),
		"reason_type": strings.ToUpper(stringValue(input.Data, "reason_type", "OTHER")),
		"reason":      stringValue(input.Data, "reason", ""),
		"planned":     isTruthy(input.Data["planned"]),
		"source":      "MANUAL",
		"started_at":  now, "started_by": smtActor(ctx, input),
		"ended_at": "", "ended_by": "", "remark": "",
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MDWN", Key: key, Action: "start", Status: "started", Record: record}, nil
}

func (s *Service) smtDowntimeEnd(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	record, err := s.smtGetRecord(ctx, "MDWN", key)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("%w: downtime %s not found", ErrValidation, key)
	}
	if stringValue(record.Data, "ended_at", "") != "" {
		return nil, fmt.Errorf("%w: downtime %s already ended", ErrValidation, key)
	}
	updated, err := s.smtUpsertRecord(ctx, "MDWN", key, map[string]any{
		"ended_at": smtNowISO(), "ended_by": smtActor(ctx, input),
		"remark": stringValue(input.Data, "remark", ""),
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MDWN", Key: key, Action: "end", Status: "ended", Record: updated}, nil
}

// ---------------------------------------------------------------------------
// Panels (MPNL).
// ---------------------------------------------------------------------------

func (s *Service) smtPanelBind(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	workOrder := strings.TrimSpace(stringValue(input.Data, "work_order", ""))
	order, err := s.smtGetRecord(ctx, "MSWO", workOrder)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, workOrder)
	}
	existing, err := s.smtGetRecord(ctx, "MPNL", key)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: panel %s already exists", ErrValidation, key)
	}
	var sns []string
	if raw, ok := input.Data["sns"].([]any); ok {
		for _, value := range raw {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				sns = append(sns, text)
			}
		}
	}
	if len(sns) == 0 {
		boardCount := int(numericValue(input.Data, "board_count"))
		if boardCount <= 0 || boardCount > 200 {
			return nil, fmt.Errorf("%w: board_count (1-200) or sns is required", ErrValidation)
		}
		for index := 1; index <= boardCount; index++ {
			sns = append(sns, fmt.Sprintf("%s-%02d", key, index))
		}
	}
	seen := map[string]bool{}
	for _, sn := range sns {
		if seen[sn] {
			return nil, fmt.Errorf("%w: duplicate board %s in panel", ErrValidation, sn)
		}
		seen[sn] = true
		if unit, err := s.smtGetRecord(ctx, "MUNT", sn); err != nil {
			return nil, err
		} else if unit != nil {
			if panel := stringValue(unit.Data, "panel_code", ""); panel != "" && panel != key {
				return nil, fmt.Errorf("%w: board %s already belongs to panel %s", ErrValidation, sn, panel)
			}
			if wo := stringValue(unit.Data, "work_order", ""); wo != "" && wo != workOrder {
				return nil, fmt.Errorf("%w: board %s belongs to work order %s", ErrValidation, sn, wo)
			}
		}
	}
	now := smtNowISO()
	record, err := s.smtUpsertRecord(ctx, "MPNL", key, map[string]any{
		"work_order": workOrder, "board_count": len(sns),
		"status": "ACTIVE", "Status": "active",
		"created_at": now, "created_by": smtActor(ctx, input),
	})
	if err != nil {
		return nil, err
	}
	for index, sn := range sns {
		if err := s.smtAppendChild(ctx, "MPNL", key, "PNL1", map[string]any{"pos": index + 1, "sn": sn}); err != nil {
			return nil, err
		}
		// Attach the board to its unit record when it already exists.
		if unit, _ := s.smtGetRecord(ctx, "MUNT", sn); unit != nil {
			if _, err := s.smtUpsertRecord(ctx, "MUNT", sn, map[string]any{"panel_code": key}); err != nil {
				return nil, err
			}
		}
	}
	return &ActionResult{
		TableCode: "MPNL", Key: key, Action: "bind", Status: "bound", Record: record,
		Effects: map[string]any{"board_count": len(sns), "sns": sns},
	}, nil
}

func (s *Service) smtPanelSplit(ctx context.Context, key string, input ActionInput) (*ActionResult, error) {
	panel, err := s.smtGetRecord(ctx, "MPNL", key)
	if err != nil {
		return nil, err
	}
	if panel == nil {
		return nil, fmt.Errorf("%w: panel %s not found", ErrValidation, key)
	}
	if strings.EqualFold(stringValue(panel.Data, "status", ""), "SPLIT") {
		return nil, fmt.Errorf("%w: panel %s is already split", ErrValidation, key)
	}
	record, err := s.smtUpsertRecord(ctx, "MPNL", key, map[string]any{
		"status": "SPLIT", "Status": "split",
		"split_at": smtNowISO(), "split_by": smtActor(ctx, input),
	})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: "MPNL", Key: key, Action: "split", Status: "split", Record: record}, nil
}

func (s *Service) smtValidSN(ctx context.Context, sn string) bool {
	if strings.TrimSpace(sn) == "" {
		return false
	}
	pattern := strings.TrimSpace(s.smtSetting(ctx, "sn_regex"))
	if pattern == "" {
		return true
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return true
	}
	return compiled.MatchString(sn)
}
