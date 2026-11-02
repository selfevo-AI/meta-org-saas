package erp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SMT pass-station execution (MUNT:pass / fail / repair / scrap).
//
// The rejection chain mirrors the SMT-MES reference system: work order ->
// routing step -> machine -> SN format -> unit state -> plan limit -> route
// skip/repeat -> FAI -> machine readiness -> defect codes -> panel expansion.
// Rejections are business outcomes (result=REJECT) and are persisted as unit
// event rows for foolproofing statistics; they do not roll back the action.

var smtRejectTypes = map[string]smtLoadError{
	"WO_NOT_FOUND":          {"工单不存在", "工单不存在"},
	"WO_NOT_RUNNING":        {"工单未开工", "工单未开工"},
	"NO_ROUTING":            {"无工艺路线", "无工艺路线"},
	"STEP_NOT_FOUND":        {"工序不在路线中", "工序不在路线中"},
	"MACHINE_NOT_FOUND":     {"机台不存在", "机台不存在"},
	"SN_INVALID":            {"SN 格式不符", "板号格式错误"},
	"SN_WRONG_WO":           {"SN 属于其它工单", "该板属于其它工单"},
	"SN_NOT_STARTED":        {"SN 未投入", "板未投入"},
	"PLAN_QTY_EXCEEDED":     {"投入超计划数", "投入超过计划数"},
	"SN_SCRAPPED":           {"SN 已报废", "板已报废"},
	"SN_FINISHED":           {"SN 已完工", "板已完工"},
	"SN_NG_HOLD":            {"不良待维修", "板不良待维修"},
	"ROUTE_SKIP":            {"跳站", "跳站"},
	"ROUTE_REPEAT":          {"重复过站", "重复过站"},
	"FAI_NOT_PASSED":        {"首件未确认", "首件未确认"},
	"MACHINE_NOT_READY":     {"机台未核对", "机台未核对"},
	"DEFECT_REQUIRED":       {"未选不良代码", "请选择不良代码"},
	"DEFECT_CODE_INVALID":   {"不良代码无效", "不良代码无效"},
	"PANEL_WRONG_WO":        {"拼板属于其它工单", "拼板属于其它工单"},
	"PANEL_SPLIT":           {"拼板已分板", "拼板已分板，请扫单板"},
	"DEFECT_BOARD_REQUIRED": {"未指定不良板位", "请指定不良板位"},
}

func smtRejectError(typeCode string) smtLoadError {
	if item, ok := smtRejectTypes[typeCode]; ok {
		return item
	}
	return smtLoadError{Message: typeCode, Voice: typeCode}
}

// smtRoutingStep describes one routing step for foolproofing.
type smtRoutingStep struct {
	Seq           int
	OperationCode string
	OperationName string
	OpType        string
}

// smtLoadRouting returns the ordered routing steps for a work order.
func (s *Service) smtLoadRouting(ctx context.Context, order *Record) ([]smtRoutingStep, error) {
	routeCode := stringValue(order.Data, "route_code", "")
	if routeCode == "" {
		return nil, nil
	}
	routing, err := s.smtGetRecord(ctx, "MROU", routeCode)
	if err != nil || routing == nil {
		return nil, err
	}
	rows, err := s.listChildPayloads(ctx, "MROU", routeCode, "ROU1")
	if err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	steps := make([]smtRoutingStep, 0, len(rows))
	for _, row := range rows {
		seq := int(numericValue(row, "Seq"))
		if seq <= 0 {
			continue
		}
		step := smtRoutingStep{Seq: seq, OperationCode: stringValue(row, "OpCode", "")}
		if operation, _ := s.smtGetRecord(ctx, "MOPR", step.OperationCode); operation != nil {
			step.OperationName = stringValue(operation.Data, "Name", "")
			step.OpType = strings.ToUpper(stringValue(operation.Data, "OpType", ""))
		}
		steps = append(steps, step)
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].Seq < steps[j].Seq })
	return steps, nil
}

// smtUnitEventReject persists a REJECT unit event row.
func (s *Service) smtUnitEventReject(ctx context.Context, sn, workOrder, operationCode, opType string, seq int, machineCode, typeCode, message string, input ActionInput) error {
	item := smtRejectError(typeCode)
	if message == "" {
		message = item.Message
	}
	return s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
		"seq": seq, "operation_code": operationCode, "op_type": opType,
		"result": "REJECT", "reject_type": typeCode, "message": message, "voice": item.Voice,
		"work_order": workOrder, "machine_code": machineCode,
		"source":   strings.ToUpper(stringValue(input.Data, "source", "MANUAL")),
		"operator": smtActor(ctx, input), "created_at": smtNowISO(),
	})
}

func (s *Service) smtReject(ctx context.Context, sn, workOrder, operationCode, opType string, seq int, machineCode, typeCode, message string, input ActionInput) (*ActionResult, error) {
	if err := s.smtUnitEventReject(ctx, sn, workOrder, operationCode, opType, seq, machineCode, typeCode, message, input); err != nil {
		return nil, err
	}
	item := smtRejectError(typeCode)
	if message == "" {
		message = item.Message
	}
	return &ActionResult{
		TableCode: "MUNT", Key: sn, Action: "pass", Status: "REJECT",
		Effects: map[string]any{
			"result": "REJECT", "reject_type": typeCode,
			"reject_type_cn": item.Message, "message": message, "voice": item.Voice,
			"sn": sn, "work_order": workOrder, "operation_code": operationCode,
		},
	}, nil
}

// smtUnitPass validates and applies a pass-station scan.
func (s *Service) smtUnitPass(ctx context.Context, sn string, input ActionInput) (*ActionResult, error) {
	sn = strings.TrimSpace(sn)
	if sn == "" {
		return nil, fmt.Errorf("%w: sn is required", ErrValidation)
	}
	operationCode := strings.TrimSpace(stringValue(input.Data, "operation_code", ""))
	machineCode := strings.TrimSpace(stringValue(input.Data, "machine_code", ""))
	dryRun := isTruthy(input.Data["dry_run"])

	// Panel codes expand to a whole-panel pass (all-or-nothing).
	if panel, err := s.smtGetRecord(ctx, "MPNL", sn); err != nil {
		return nil, err
	} else if panel != nil {
		return s.smtPassPanel(ctx, panel, operationCode, machineCode, dryRun, input)
	}

	order, unit, step, steps, rejection, err := s.smtValidateUnitStep(ctx, sn, operationCode, machineCode, input)
	if err != nil {
		return nil, err
	}
	if rejection != nil {
		if dryRun {
			// Device check mode: report without persisting a rejection event.
			return rejection, nil
		}
		return s.smtReject(ctx, sn, rejectionWorkOrder(rejection), operationCode, "", rejectionSeq(rejection), machineCode,
			stringValue(rejection.Effects, "reject_type", ""), stringValue(rejection.Effects, "message", ""), input)
	}

	if dryRun {
		return &ActionResult{
			TableCode: "MUNT", Key: sn, Action: "pass", Status: "OK",
			Effects: map[string]any{"allowed": true, "sn": sn, "seq": step.Seq, "operation_code": step.OperationCode},
		}, nil
	}
	result, err := s.smtApplyUnitPass(ctx, order, unit, step, steps, sn, machineCode, input)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type smtRejection struct {
	ErrorType string
	Message   string
}

func (r *smtRejection) Error() string { return r.ErrorType }

func rejectionWorkOrder(result *ActionResult) string {
	if result != nil && result.Effects != nil {
		return stringValue(result.Effects, "work_order", "")
	}
	return ""
}

func rejectionSeq(result *ActionResult) int {
	if result != nil && result.Effects != nil {
		return int(numericValue(result.Effects, "seq"))
	}
	return 0
}

// smtValidateUnitStep runs the pass-station foolproofing chain and returns the
// resolved work order, unit, routing step and step list. A non-nil rejection
// result means the scan must be rejected with the embedded type.
func (s *Service) smtValidateUnitStep(ctx context.Context, sn, operationCode, machineCode string, input ActionInput) (*Record, *Record, smtRoutingStep, []smtRoutingStep, *ActionResult, error) {
	reject := func(workOrder string, seq int, typeCode, message string) (*Record, *Record, smtRoutingStep, []smtRoutingStep, *ActionResult, error) {
		return nil, nil, smtRoutingStep{}, nil, &ActionResult{
			TableCode: "MUNT", Key: sn, Action: "pass", Status: "REJECT",
			Effects: map[string]any{
				"result": "REJECT", "reject_type": typeCode,
				"reject_type_cn": smtRejectError(typeCode).Message,
				"message":        message, "voice": smtRejectError(typeCode).Voice,
				"sn": sn, "work_order": workOrder, "seq": seq, "operation_code": operationCode,
			},
		}, nil
	}

	unit, err := s.smtGetRecord(ctx, "MUNT", sn)
	if err != nil {
		return nil, nil, smtRoutingStep{}, nil, nil, err
	}
	if unit == nil && !s.smtValidSN(ctx, sn) {
		return reject("", 0, "SN_INVALID", "")
	}
	workOrderCode := strings.TrimSpace(stringValue(input.Data, "work_order", ""))
	if workOrderCode == "" && unit != nil {
		workOrderCode = stringValue(unit.Data, "work_order", "")
	}
	if workOrderCode == "" {
		return reject("", 0, "WO_NOT_FOUND", "未投入的新板需要指定工单")
	}
	order, err := s.smtGetRecord(ctx, "MSWO", workOrderCode)
	if err != nil {
		return nil, nil, smtRoutingStep{}, nil, nil, err
	}
	if order == nil {
		return reject(workOrderCode, 0, "WO_NOT_FOUND", "")
	}
	if smtStatusOf(order) != "RUNNING" {
		return reject(workOrderCode, 0, "WO_NOT_RUNNING", "")
	}
	if operationCode == "" {
		return nil, nil, smtRoutingStep{}, nil, nil, fmt.Errorf("%w: operation_code is required", ErrValidation)
	}
	steps, err := s.smtLoadRouting(ctx, order)
	if err != nil {
		return nil, nil, smtRoutingStep{}, nil, nil, err
	}
	if len(steps) == 0 {
		return reject(workOrderCode, 0, "NO_ROUTING", "")
	}
	var step *smtRoutingStep
	for index := range steps {
		if steps[index].OperationCode == operationCode {
			step = &steps[index]
			break
		}
	}
	if step == nil {
		return reject(workOrderCode, 0, "STEP_NOT_FOUND", "")
	}
	if machineCode != "" {
		machine, err := s.smtGetRecord(ctx, "MMAC", machineCode)
		if err != nil {
			return nil, nil, smtRoutingStep{}, nil, nil, err
		}
		if machine == nil {
			return reject(workOrderCode, step.Seq, "MACHINE_NOT_FOUND", "")
		}
	}
	if unit != nil {
		if workOrder := stringValue(unit.Data, "work_order", ""); workOrder != "" && workOrder != workOrderCode {
			return reject(workOrderCode, step.Seq, "SN_WRONG_WO", fmt.Sprintf("该板属于工单 %s", workOrder))
		}
		switch smtUnitStatus(unit) {
		case "SCRAPPED":
			return reject(workOrderCode, step.Seq, "SN_SCRAPPED", "")
		case "FINISHED":
			return reject(workOrderCode, step.Seq, "SN_FINISHED", "")
		case "NG":
			return reject(workOrderCode, step.Seq, "SN_NG_HOLD", "")
		}
	}
	lastSeq := 0
	if unit != nil {
		lastSeq = int(numericValue(unit.Data, "last_seq"))
	}
	firstSeq := steps[0].Seq
	if unit == nil {
		if step.Seq != firstSeq {
			return reject(workOrderCode, step.Seq, "SN_NOT_STARTED", "")
		}
		if s.smtBoolSetting(ctx, "pass_limit_plan_qty") {
			inProcess, err := s.smtCountUnits(ctx, workOrderCode, "IN_PROCESS")
			if err != nil {
				return nil, nil, smtRoutingStep{}, nil, nil, err
			}
			ng, err := s.smtCountUnits(ctx, workOrderCode, "NG")
			if err != nil {
				return nil, nil, smtRoutingStep{}, nil, nil, err
			}
			finished, err := s.smtCountUnits(ctx, workOrderCode, "FINISHED")
			if err != nil {
				return nil, nil, smtRoutingStep{}, nil, nil, err
			}
			planQty := numericValue(order.Data, "plan_qty")
			if planQty > 0 && float64(inProcess+ng+finished) >= planQty {
				return reject(workOrderCode, step.Seq, "PLAN_QTY_EXCEEDED", "")
			}
		}
	} else {
		expected := firstSeq
		for _, candidate := range steps {
			if candidate.Seq > lastSeq {
				expected = candidate.Seq
				break
			}
		}
		if step.Seq < expected {
			return reject(workOrderCode, step.Seq, "ROUTE_REPEAT", "")
		}
		if step.Seq > expected {
			return reject(workOrderCode, step.Seq, "ROUTE_SKIP", "")
		}
	}
	// First-article inspection gate.
	if s.smtBoolSetting(ctx, "fai_required") {
		if err := s.smtRequireFAI(ctx, order, machineCode); err != nil {
			if rejection, ok := err.(*smtRejection); ok {
				return reject(workOrderCode, step.Seq, rejection.ErrorType, rejection.Message)
			}
			return nil, nil, smtRoutingStep{}, nil, nil, err
		}
	}
	// SMT placement steps require a verified (and signed-off) machine.
	if step.OpType == "SMT" && s.smtBoolSetting(ctx, "pass_require_machine_ready") {
		if err := s.smtRequireMachineReady(ctx, order, machineCode); err != nil {
			if rejection, ok := err.(*smtRejection); ok {
				return reject(workOrderCode, step.Seq, rejection.ErrorType, rejection.Message)
			}
			return nil, nil, smtRoutingStep{}, nil, nil, err
		}
	}
	if unit == nil {
		unit = &Record{TableCode: "MUNT", Key: sn, Data: map[string]any{"work_order": workOrderCode}}
	}
	return order, unit, *step, steps, nil, nil
}

func (s *Service) smtCountUnits(ctx context.Context, workOrder, status string) (int, error) {
	page, err := s.QueryRecords(ctx, "MUNT", RecordQuery{Filters: map[string]any{"work_order": workOrder, "status": status}, Limit: 1})
	if err != nil {
		return 0, err
	}
	return int(page.Total), nil
}

func smtUnitStatus(unit *Record) string {
	if unit == nil {
		return ""
	}
	status := stringValue(unit.Data, "status", "")
	if status == "" {
		status = stringValue(unit.Data, "Status", "IN_PROCESS")
	}
	return strings.ToUpper(status)
}

// smtRequireFAI checks that a passed first-article inspection exists for the
// work order and is still valid for the machine (changeover invalidates FAI).
func (s *Service) smtRequireFAI(ctx context.Context, order *Record, machineCode string) error {
	inspections, err := s.smtQueryAll(ctx, "MINS", map[string]any{"work_order": order.Key, "insp_type": "FAI", "result": "PASS"}, 20)
	if err != nil {
		return err
	}
	if len(inspections) == 0 {
		return &smtRejection{ErrorType: "FAI_NOT_PASSED", Message: "首件检验未通过或未做，不能过站"}
	}
	if machineCode == "" {
		return nil
	}
	machine, err := s.smtGetRecord(ctx, "MMAC", machineCode)
	if err != nil || machine == nil {
		return err
	}
	changeoverAt := stringValue(machine.Data, "changeover_at", "")
	if changeoverAt == "" {
		return nil
	}
	for index := range inspections {
		if stringValue(inspections[index].Data, "created_at", "") > changeoverAt {
			return nil
		}
	}
	return &smtRejection{ErrorType: "FAI_NOT_PASSED", Message: fmt.Sprintf("机台 %s 换线后首件失效，需重做首件", machineCode)}
}

// smtRequireMachineReady enforces the feeder verification gate for SMT steps.
func (s *Service) smtRequireMachineReady(ctx context.Context, order *Record, machineCode string) error {
	if machineCode == "" {
		return &smtRejection{ErrorType: "MACHINE_NOT_READY", Message: "贴片工序过站必须指定机台"}
	}
	machine, err := s.smtGetRecord(ctx, "MMAC", machineCode)
	if err != nil {
		return err
	}
	if machine == nil {
		return &smtRejection{ErrorType: "MACHINE_NOT_FOUND", Message: ""}
	}
	if bound := stringValue(machine.Data, "current_work_order", ""); bound != order.Key {
		return &smtRejection{ErrorType: "MACHINE_NOT_READY", Message: fmt.Sprintf("机台 %s 未绑定本工单（当前：%s）", machineCode, bound)}
	}
	if stringValue(machine.Data, "verified_at", "") == "" {
		return &smtRejection{ErrorType: "MACHINE_NOT_READY", Message: fmt.Sprintf("机台 %s 未完成整机核对", machineCode)}
	}
	if s.smtBoolSetting(ctx, "signoff_required") && stringValue(machine.Data, "signed_off_at", "") == "" {
		return &smtRejection{ErrorType: "MACHINE_NOT_READY", Message: fmt.Sprintf("机台 %s 等待第二人签核", machineCode)}
	}
	return nil
}

// smtApplyUnitPass writes the pass: unit upsert, event row, completion and
// auto progress.
func (s *Service) smtApplyUnitPass(ctx context.Context, order, unit *Record, step smtRoutingStep, steps []smtRoutingStep, sn, machineCode string, input ActionInput) (*ActionResult, error) {
	now := smtNowISO()
	actor := smtActor(ctx, input)
	source := strings.ToUpper(stringValue(input.Data, "source", "MANUAL"))
	if source == "" {
		source = "MANUAL"
	}
	lastStep := steps[len(steps)-1].Seq
	finished := step.Seq == lastStep
	update := map[string]any{
		"work_order":   order.Key,
		"product_code": stringValue(order.Data, "product_code", ""),
		"route_code":   stringValue(order.Data, "route_code", ""),
		"status":       "FINISHED",
		"last_seq":     step.Seq,
		"updated_at":   now,
	}
	if finished {
		update["finished_at"] = now
	} else {
		update["status"] = "IN_PROCESS"
	}
	if smtUnitStatus(unit) == "" || unit.CreatedAt.IsZero() {
		update["created_at"] = now
		update["created_by"] = actor
	}
	unitRecord, err := s.smtUpsertRecord(ctx, "MUNT", sn, update)
	if err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
		"seq": step.Seq, "operation_code": step.OperationCode, "op_type": step.OpType,
		"result": "PASS", "reject_type": "", "message": "",
		"work_order": order.Key, "machine_code": machineCode,
		"source": source, "operator": actor, "created_at": now,
	}); err != nil {
		return nil, err
	}
	// A pass resets the consecutive-fail streak for this operation.
	_ = s.smtResetOpFailStreak(ctx, order.Key, step.OperationCode)
	effects := map[string]any{
		"result": "PASS", "sn": sn, "seq": step.Seq,
		"operation_code": step.OperationCode, "operation_name": step.OperationName,
		"finished": finished, "message": fmt.Sprintf("%s 通过（%s）", sn, step.OperationName),
		"voice": "过站成功",
	}
	if finished {
		effects["message"] = fmt.Sprintf("%s 完工", sn)
		effects["voice"] = "完工"
	}
	if finished && s.smtBoolSetting(ctx, "pass_auto_progress") {
		count, err := s.smtCountUnits(ctx, order.Key, "FINISHED")
		if err != nil {
			return nil, err
		}
		updated, err := s.smtUpsertRecord(ctx, "MSWO", order.Key, map[string]any{"done_qty": count, "progress_at": now})
		if err != nil {
			return nil, err
		}
		alerts, err := s.smtSyncLowMaterialAlerts(ctx, updated, input)
		if err != nil {
			return nil, err
		}
		if len(alerts) > 0 {
			effects["low_alerts"] = alerts
		}
		effects["done_qty"] = count
	}
	return &ActionResult{
		TableCode: "MUNT", Key: sn, Action: "pass", Status: "OK", Record: unitRecord,
		Effects: effects,
	}, nil
}

// smtUnitFail judges a unit as defective at the current step.
func (s *Service) smtUnitFail(ctx context.Context, sn string, input ActionInput) (*ActionResult, error) {
	sn = strings.TrimSpace(sn)
	if sn == "" {
		return nil, fmt.Errorf("%w: sn is required", ErrValidation)
	}
	operationCode := strings.TrimSpace(stringValue(input.Data, "operation_code", ""))
	machineCode := strings.TrimSpace(stringValue(input.Data, "machine_code", ""))
	defects := mapSliceAny(input.Data["defects"])
	if len(defects) == 0 {
		if rejection, err := s.smtFailReject(ctx, sn, operationCode, machineCode, "DEFECT_REQUIRED", "", input); err != nil {
			return nil, err
		} else {
			return rejection, nil
		}
	}
	order, unit, step, _, rejection, err := s.smtValidateUnitStep(ctx, sn, operationCode, machineCode, input)
	if err != nil {
		return nil, err
	}
	if rejection != nil {
		return s.smtReject(ctx, sn, rejectionWorkOrder(rejection), operationCode, "", rejectionSeq(rejection), machineCode,
			stringValue(rejection.Effects, "reject_type", ""), stringValue(rejection.Effects, "message", ""), input)
	}
	// Validate defect codes against the defect code master.
	for index := range defects {
		code := stringValue(defects[index], "code", "")
		if code == "" {
			return s.smtFailReject(ctx, sn, operationCode, machineCode, "DEFECT_REQUIRED", "", input)
		}
		defect, err := s.smtGetRecord(ctx, "MDFC", code)
		if err != nil {
			return nil, err
		}
		if defect == nil || strings.EqualFold(stringValue(defect.Data, "Status", "active"), "inactive") {
			return s.smtFailReject(ctx, sn, operationCode, machineCode, "DEFECT_CODE_INVALID", "", input)
		}
		defects[index]["name"] = stringValue(defect.Data, "Name", "")
	}
	now := smtNowISO()
	actor := smtActor(ctx, input)
	productCode := stringValue(order.Data, "product_code", "")
	failCount := 1
	if unit != nil {
		failCount = int(numericValue(unit.Data, "fail_count")) + 1
	}
	update := map[string]any{
		"work_order": order.Key, "product_code": productCode,
		"route_code": stringValue(order.Data, "route_code", ""),
		"status":     "NG", "ng_step": step.Seq, "ng_operation": step.OperationCode,
		"fail_count": failCount, "updated_at": now,
	}
	if unit == nil || unit.CreatedAt.IsZero() {
		update["created_at"] = now
		update["created_by"] = actor
	}
	unitRecord, err := s.smtUpsertRecord(ctx, "MUNT", sn, update)
	if err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
		"seq": step.Seq, "operation_code": step.OperationCode, "op_type": step.OpType,
		"result": "FAIL", "reject_type": "", "message": "",
		"work_order": order.Key, "machine_code": machineCode,
		"source":   strings.ToUpper(stringValue(input.Data, "source", "MANUAL")),
		"operator": actor, "created_at": now,
	}); err != nil {
		return nil, err
	}
	// Defect rows with reference-designator resolved part numbers.
	defectSummaries := make([]map[string]any, 0, len(defects))
	for _, defect := range defects {
		location := stringValue(defect, "location", "")
		partNo := stringValue(defect, "part_no", "")
		if partNo == "" && location != "" {
			partNo = s.smtPartForDesignators(ctx, productCode, location)
		}
		defectKey := smtGenerateKey("DF")
		if _, err := s.smtUpsertRecord(ctx, "MUDF", defectKey, map[string]any{
			"sn": sn, "work_order": order.Key,
			"step_seq": step.Seq, "operation_code": step.OperationCode,
			"defect_code": stringValue(defect, "code", ""), "defect_name": stringValue(defect, "name", ""),
			"location": location, "part_no": partNo,
			"status": "OPEN", "Status": "OPEN",
			"repair_action": "", "root_cause": "", "remark": "",
			"created_by": actor, "created_at": now,
		}); err != nil {
			return nil, err
		}
		defectSummaries = append(defectSummaries, map[string]any{
			"defect_key": defectKey, "code": stringValue(defect, "code", ""),
			"name": stringValue(defect, "name", ""), "location": location, "part_no": partNo,
		})
	}
	// Consecutive-fail streak for auto quality andon.
	streak, err := s.smtBumpOpFailStreak(ctx, order.Key, step.OperationCode, 1)
	if err != nil {
		return nil, err
	}
	effects := map[string]any{
		"result": "FAIL", "sn": sn, "seq": step.Seq,
		"operation_code": step.OperationCode, "defects": defectSummaries,
		"fail_count": failCount, "fail_streak": streak,
		"message": fmt.Sprintf("%s 判不良", sn), "voice": "判不良",
	}
	threshold := s.smtIntSetting(ctx, "andon_fail_threshold", 3)
	if threshold > 0 && streak >= threshold {
		call, err := s.smtAndonRaise(ctx, smtGenerateKey("AD"), ActionInput{Data: map[string]any{
			"line_code":    stringValue(order.Data, "line_code", ""),
			"machine_code": machineCode,
			"work_order":   order.Key,
			"call_type":    "QUALITY", "source": "AUTO",
			"ref":         "step:" + operationCode,
			"description": fmt.Sprintf("工序 %s 连续 %d 块不良", step.OperationCode, streak),
		}})
		if err != nil {
			return nil, err
		}
		effects["andon"] = call.Effects
	}
	return &ActionResult{
		TableCode: "MUNT", Key: sn, Action: "fail", Status: "FAIL", Record: unitRecord,
		Effects: effects,
	}, nil
}

// smtFailReject persists a REJECT event while attempting an explicit FAIL scan.
func (s *Service) smtFailReject(ctx context.Context, sn, operationCode, machineCode, typeCode, message string, input ActionInput) (*ActionResult, error) {
	unit, err := s.smtGetRecord(ctx, "MUNT", sn)
	if err != nil {
		return nil, err
	}
	workOrder := stringValue(input.Data, "work_order", "")
	seq := 0
	if unit != nil {
		if workOrder == "" {
			workOrder = stringValue(unit.Data, "work_order", "")
		}
		seq = int(numericValue(unit.Data, "last_seq"))
	}
	if err := s.smtUnitEventReject(ctx, sn, workOrder, operationCode, "", seq, machineCode, typeCode, message, input); err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MUNT", Key: sn, Action: "fail", Status: "REJECT",
		Effects: map[string]any{
			"result": "REJECT", "reject_type": typeCode,
			"reject_type_cn": smtRejectError(typeCode).Message,
			"message":        message, "voice": smtRejectError(typeCode).Voice,
			"sn": sn, "work_order": workOrder,
		},
	}, nil
}

// smtPartForDesignators resolves a reference designator (C12 / R1-R4) to a part.
func (s *Service) smtPartForDesignators(ctx context.Context, productCode, designators string) string {
	if productCode == "" || designators == "" {
		return ""
	}
	lines, err := s.listChildPayloads(ctx, "MPRD", productCode, "PRD1")
	if err != nil {
		return ""
	}
	wanted := expandDesignators(designators)
	for _, line := range lines {
		raw := strings.ReplaceAll(stringValue(line, "RefDesignators", ""), "，", ",")
		raw = strings.ReplaceAll(raw, ";", ",")
		for _, token := range strings.Split(raw, ",") {
			token = strings.ToUpper(strings.TrimSpace(token))
			if token == "" {
				continue
			}
			for _, candidate := range wanted {
				if token == candidate {
					return stringValue(line, "PartNo", "")
				}
			}
		}
	}
	return ""
}

// expandDesignators expands "C12, R1-R4" into upper-case designator tokens.
func expandDesignators(value string) []string {
	value = strings.ReplaceAll(value, "，", ",")
	value = strings.ReplaceAll(value, ";", ",")
	var out []string
	for _, token := range strings.Split(value, ",") {
		token = strings.ToUpper(strings.TrimSpace(token))
		if token == "" {
			continue
		}
		if strings.Contains(token, "-") {
			parts := strings.SplitN(token, "-", 2)
			prefix := strings.TrimRight(parts[0], "0123456789")
			startText := strings.TrimPrefix(parts[0], prefix)
			endText := strings.TrimPrefix(parts[1], prefix)
			start, errStart := strconv.Atoi(strings.TrimSpace(startText))
			end, errEnd := strconv.Atoi(strings.TrimSpace(endText))
			if prefix != "" && errStart == nil && errEnd == nil && end >= start && end-start <= 200 {
				for i := start; i <= end; i++ {
					out = append(out, fmt.Sprintf("%s%d", prefix, i))
				}
				continue
			}
		}
		out = append(out, token)
	}
	return out
}

// smtBumpOpFailStreak maintains the consecutive fail streak per work order +
// operation inside the SWO1 child rows (serialized by the ERP ledger lock).
func (s *Service) smtBumpOpFailStreak(ctx context.Context, workOrder, operationCode string, delta int) (int, error) {
	streakKey := "__streak__:" + operationCode
	lines, err := s.listChildPayloads(ctx, "MSWO", workOrder, "SWO1")
	if err != nil {
		return 0, err
	}
	parent, child, err := s.child("MSWO", "SWO1")
	if err != nil {
		return 0, err
	}
	for index, line := range lines {
		if stringValue(line, "PartNo", "") != streakKey {
			continue
		}
		streak := int(numericValue(line, "Streak")) + delta
		if streak < 0 {
			streak = 0
		}
		data := copyData(line)
		data["Streak"] = streak
		if _, err := s.repo.UpdateChildRecord(ctx, parent, child, workOrder, fmt.Sprint(index+1), RecordInput{Data: data}); err != nil {
			return 0, err
		}
		return streak, nil
	}
	streak := delta
	if streak < 0 {
		streak = 0
	}
	if err := s.smtAppendChild(ctx, "MSWO", workOrder, "SWO1", map[string]any{
		"PartNo": streakKey, "Usage": 0, "RequiredQty": 0, "IssuedQty": 0, "Gap": 0,
		"Streak": streak, "LineStatus": "O",
	}); err != nil {
		return 0, err
	}
	return streak, nil
}

func (s *Service) smtResetOpFailStreak(ctx context.Context, workOrder, operationCode string) error {
	if workOrder == "" || operationCode == "" {
		return nil
	}
	_, err := s.smtBumpOpFailStreak(ctx, workOrder, operationCode, -1<<30)
	if err != nil {
		// Resetting is best-effort; a missing streak row is fine.
		return nil
	}
	return nil
}

// smtUnitRepair completes repair of a defective unit.
func (s *Service) smtUnitRepair(ctx context.Context, sn string, input ActionInput) (*ActionResult, error) {
	unit, err := s.smtGetRecord(ctx, "MUNT", sn)
	if err != nil {
		return nil, err
	}
	if unit == nil {
		return nil, fmt.Errorf("%w: unit %s not found", ErrValidation, sn)
	}
	if smtUnitStatus(unit) != "NG" {
		return nil, fmt.Errorf("%w: unit %s is not waiting for repair", ErrValidation, sn)
	}
	defectKey := strings.TrimSpace(stringValue(input.Data, "defect_key", ""))
	action := strings.ToUpper(stringValue(input.Data, "action", "REPAIRED"))
	switch action {
	case "REPAIRED", "MISJUDGE", "SCRAPPED":
	default:
		return nil, fmt.Errorf("%w: repair action must be REPAIRED, MISJUDGE or SCRAPPED", ErrValidation)
	}
	now := smtNowISO()
	actor := smtActor(ctx, input)
	if defectKey != "" {
		defect, err := s.smtGetRecord(ctx, "MUDF", defectKey)
		if err != nil {
			return nil, err
		}
		if defect == nil {
			return nil, fmt.Errorf("%w: defect record %s not found", ErrValidation, defectKey)
		}
		if _, err := s.smtUpsertRecord(ctx, "MUDF", defectKey, map[string]any{
			"status": action, "Status": strings.ToLower(action),
			"repair_action": stringValue(input.Data, "repair_action", ""),
			"root_cause":    stringValue(input.Data, "root_cause", ""),
			"remark":        stringValue(input.Data, "remark", ""),
			"repaired_by":   actor, "repaired_at": now,
		}); err != nil {
			return nil, err
		}
	}
	if action == "SCRAPPED" {
		return s.smtUnitScrap(ctx, sn, ActionInput{Data: map[string]any{"reason": "维修判定报废"}, ActorID: input.ActorID})
	}
	// Remaining open defects keep the unit in NG hold.
	openPage, err := s.QueryRecords(ctx, "MUDF", RecordQuery{Filters: map[string]any{"sn": sn, "status": "OPEN"}, Limit: 1})
	if err != nil {
		return nil, err
	}
	remaining := int(openPage.Total)
	update := map[string]any{"repaired_at": now, "repair_action": stringValue(input.Data, "repair_action", "")}
	if remaining == 0 {
		update["status"] = "IN_PROCESS"
		update["ng_step"] = 0
	}
	unitRecord, err := s.smtUpsertRecord(ctx, "MUNT", sn, update)
	if err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
		"seq": int(numericValue(unit.Data, "last_seq")), "operation_code": stringValue(unit.Data, "ng_operation", ""),
		"result": "REPAIR", "message": stringValue(input.Data, "remark", ""),
		"work_order": stringValue(unit.Data, "work_order", ""),
		"defect_key": defectKey, "action": action,
		"source": "MANUAL", "operator": actor, "created_at": now,
	}); err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MUNT", Key: sn, Action: "repair", Status: strings.ToLower(action), Record: unitRecord,
		Effects: map[string]any{
			"action": action, "remaining": remaining,
			"message": fmt.Sprintf("%s 维修%s", sn, map[string]string{"REPAIRED": "完成", "MISJUDGE": "误判"}[action]),
			"voice":   "维修完成",
		},
	}, nil
}

// smtUnitScrap scraps a unit.
func (s *Service) smtUnitScrap(ctx context.Context, sn string, input ActionInput) (*ActionResult, error) {
	unit, err := s.smtGetRecord(ctx, "MUNT", sn)
	if err != nil {
		return nil, err
	}
	if unit == nil {
		return nil, fmt.Errorf("%w: unit %s not found", ErrValidation, sn)
	}
	if smtUnitStatus(unit) == "SCRAPPED" {
		return nil, fmt.Errorf("%w: unit %s is already scrapped", ErrValidation, sn)
	}
	now := smtNowISO()
	actor := smtActor(ctx, input)
	unitRecord, err := s.smtUpsertRecord(ctx, "MUNT", sn, map[string]any{
		"status": "SCRAPPED", "scrapped_at": now,
		"scrap_reason": stringValue(input.Data, "reason", ""),
	})
	if err != nil {
		return nil, err
	}
	if err := s.smtAppendChild(ctx, "MUNT", sn, "UNT1", map[string]any{
		"seq": int(numericValue(unit.Data, "last_seq")), "operation_code": stringValue(unit.Data, "ng_operation", ""),
		"result": "SCRAP", "message": stringValue(input.Data, "reason", ""),
		"work_order": stringValue(unit.Data, "work_order", ""),
		"source":     "MANUAL", "operator": actor, "created_at": now,
	}); err != nil {
		return nil, err
	}
	return &ActionResult{
		TableCode: "MUNT", Key: sn, Action: "scrap", Status: "scrapped", Record: unitRecord,
		Effects: map[string]any{"message": fmt.Sprintf("%s 已报废", sn), "voice": "已报废"},
	}, nil
}

// ---------------------------------------------------------------------------
// Panel pass (whole panel, all-or-nothing).
// ---------------------------------------------------------------------------

func (s *Service) smtPassPanel(ctx context.Context, panel *Record, operationCode, machineCode string, dryRun bool, input ActionInput) (*ActionResult, error) {
	if strings.EqualFold(stringValue(panel.Data, "status", ""), "SPLIT") {
		return s.smtReject(ctx, panel.Key, stringValue(panel.Data, "work_order", ""), operationCode, "", 0, machineCode, "PANEL_SPLIT", "", input)
	}
	panelWO := stringValue(panel.Data, "work_order", "")
	requestWO := strings.TrimSpace(stringValue(input.Data, "work_order", ""))
	if requestWO != "" && requestWO != panelWO {
		return s.smtReject(ctx, panel.Key, panelWO, operationCode, "", 0, machineCode, "PANEL_WRONG_WO", "", input)
	}
	boards, err := s.listChildPayloads(ctx, "MPNL", panel.Key, "PNL1")
	if err != nil {
		return nil, err
	}
	// Operator-specified defective board positions for a FAIL scan.
	defectPositions := map[int][]map[string]any{}
	scannedResult := strings.ToUpper(stringValue(input.Data, "result", "PASS"))
	rawDefects := mapSliceAny(input.Data["defects"])
	failRequested := scannedResult == "FAIL"
	if failRequested && len(rawDefects) == 0 {
		return s.smtReject(ctx, panel.Key, panelWO, operationCode, "", 0, machineCode, "DEFECT_REQUIRED", "", input)
	}
	for _, defect := range rawDefects {
		posText := stringValue(defect, "pos", "")
		if posText == "" {
			return s.smtReject(ctx, panel.Key, panelWO, operationCode, "", 0, machineCode, "DEFECT_BOARD_REQUIRED", "", input)
		}
		for _, pos := range expandPositions(posText) {
			defectPositions[pos] = append(defectPositions[pos], defect)
		}
	}
	// Pre-validate every non-skipped board (all-or-nothing).
	type panelBoardResult struct {
		pos      int
		sn       string
		skip     string
		reject   *ActionResult
		defects  []map[string]any
		validate bool
	}
	results := make([]panelBoardResult, 0, len(boards))
	for _, board := range boards {
		pos := int(numericValue(board, "pos"))
		sn := stringValue(board, "sn", "")
		entry := panelBoardResult{pos: pos, sn: sn, defects: defectPositions[pos], validate: true}
		unit, err := s.smtGetRecord(ctx, "MUNT", sn)
		if err != nil {
			return nil, err
		}
		if unit != nil {
			switch smtUnitStatus(unit) {
			case "SCRAPPED":
				entry.skip = "SCRAPPED"
			case "FINISHED":
				entry.skip = "FINISHED"
			}
			if entry.skip == "" {
				lastSeq := int(numericValue(unit.Data, "last_seq"))
				if stepSeq, err := s.smtStepSeqForOperation(ctx, panelWO, operationCode); err != nil {
					return nil, err
				} else if lastSeq >= stepSeq && stepSeq > 0 {
					entry.skip = "REPEAT"
				}
			}
		}
		if entry.skip == "" {
			boardInput := ActionInput{Data: copyData(input.Data), ActorID: input.ActorID}
			boardInput.Data["work_order"] = panelWO
			boardInput.Data["dry_run"] = true
			if len(entry.defects) > 0 {
				boardInput.Data["result"] = "FAIL"
			}
			_, _, _, _, rejection, err := s.smtValidateUnitStep(ctx, sn, operationCode, machineCode, boardInput)
			if err != nil {
				return nil, err
			}
			if rejection != nil {
				entry.reject = rejection
			}
		}
		results = append(results, entry)
	}
	// Any board that cannot pass blocks the whole panel.
	for _, entry := range results {
		if entry.reject != nil {
			rejectType := stringValue(entry.reject.Effects, "reject_type", "")
			message := fmt.Sprintf("板位 %d（%s）：%s", entry.pos, entry.sn, stringValue(entry.reject.Effects, "message", smtRejectError(rejectType).Message))
			return s.smtReject(ctx, panel.Key, panelWO, operationCode, "", 0, machineCode, rejectType, message, input)
		}
	}
	boardSummaries := make([]map[string]any, 0, len(results))
	if dryRun {
		for _, entry := range results {
			status := "PASS"
			if entry.skip != "" {
				status = "SKIP"
			} else if len(entry.defects) > 0 {
				status = "FAIL"
			}
			boardSummaries = append(boardSummaries, map[string]any{"pos": entry.pos, "sn": entry.sn, "result": status, "skip": entry.skip})
		}
		return &ActionResult{
			TableCode: "MUNT", Key: panel.Key, Action: "pass", Status: "OK",
			Effects: map[string]any{"allowed": true, "is_panel": true, "boards": boardSummaries},
		}, nil
	}
	// Apply per board.
	step, err := s.smtStepForOperation(ctx, panelWO, operationCode)
	if err != nil {
		return nil, err
	}
	steps, err := s.smtStepsForWorkOrder(ctx, panelWO)
	if err != nil {
		return nil, err
	}
	order, err := s.smtGetRecord(ctx, "MSWO", panelWO)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, panelWO)
	}
	for _, entry := range results {
		status := "PASS"
		switch {
		case entry.skip != "":
			status = "SKIP"
		case len(entry.defects) > 0:
			status = "FAIL"
			boardInput := ActionInput{Data: copyData(input.Data), ActorID: input.ActorID}
			boardInput.Data["work_order"] = panelWO
			boardInput.Data["defects"] = entry.defects
			if _, err := s.smtUnitFail(ctx, entry.sn, boardInput); err != nil {
				return nil, err
			}
		default:
			unit, err := s.smtGetRecord(ctx, "MUNT", entry.sn)
			if err != nil {
				return nil, err
			}
			boardInput := ActionInput{Data: copyData(input.Data), ActorID: input.ActorID}
			boardInput.Data["work_order"] = panelWO
			if _, err := s.smtApplyUnitPass(ctx, order, unit, step, steps, entry.sn, machineCode, boardInput); err != nil {
				return nil, err
			}
		}
		boardSummaries = append(boardSummaries, map[string]any{"pos": entry.pos, "sn": entry.sn, "result": status, "skip": entry.skip})
	}
	// A DEPANEL step splits the panel after a successful pass.
	if step.OpType == "DEPANEL" && !dryRun {
		if _, err := s.smtPanelSplit(ctx, panel.Key, input); err != nil {
			return nil, err
		}
	}
	return &ActionResult{
		TableCode: "MUNT", Key: panel.Key, Action: "pass", Status: "OK",
		Effects: map[string]any{
			"result": "PASS", "is_panel": true, "panel_code": panel.Key,
			"boards": boardSummaries, "seq": step.Seq,
			"message": fmt.Sprintf("拼板 %s 整板过站", panel.Key), "voice": "整板过站成功",
		},
	}, nil
}

func (s *Service) smtStepsForWorkOrder(ctx context.Context, workOrder string) ([]smtRoutingStep, error) {
	order, err := s.smtGetRecord(ctx, "MSWO", workOrder)
	if err != nil || order == nil {
		return nil, err
	}
	return s.smtLoadRouting(ctx, order)
}

func (s *Service) smtStepForOperation(ctx context.Context, workOrder, operationCode string) (smtRoutingStep, error) {
	steps, err := s.smtStepsForWorkOrder(ctx, workOrder)
	if err != nil {
		return smtRoutingStep{}, err
	}
	for _, step := range steps {
		if step.OperationCode == operationCode {
			return step, nil
		}
	}
	return smtRoutingStep{}, fmt.Errorf("%w: operation %s is not on the routing", ErrValidation, operationCode)
}

func (s *Service) smtStepSeqForOperation(ctx context.Context, workOrder, operationCode string) (int, error) {
	step, err := s.smtStepForOperation(ctx, workOrder, operationCode)
	if err != nil {
		return 0, nil
	}
	return step.Seq, nil
}

// expandPositions expands "1,3" / "1-3" into board positions.
func expandPositions(value string) []int {
	value = strings.ReplaceAll(value, "，", ",")
	value = strings.ReplaceAll(value, ";", ",")
	var out []int
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.Contains(token, "-") {
			parts := strings.SplitN(token, "-", 2)
			start, errStart := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, errEnd := strconv.Atoi(strings.TrimSpace(parts[1]))
			if errStart == nil && errEnd == nil && end >= start && end-start <= 200 {
				for i := start; i <= end; i++ {
					out = append(out, i)
				}
				continue
			}
		}
		if parsed, err := strconv.Atoi(token); err == nil {
			out = append(out, parsed)
		}
	}
	return out
}
