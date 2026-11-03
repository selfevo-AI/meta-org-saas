package erp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Exported SMT read surfaces consumed by the smtmes domain (workstation,
// line board, traceability, and reporting).

// SMTMachineContext exposes the station context of a machine.
func (s *Service) SMTMachineContext(ctx context.Context, machineCode string) (map[string]any, error) {
	return s.smtMachineContext(ctx, machineCode)
}

// SMTExportRecords returns flat record payloads (with "key") for bridge pushes.
func (s *Service) SMTExportRecords(ctx context.Context, tableCode string, filters map[string]any, limit int) ([]map[string]any, error) {
	records, err := s.smtQueryAll(ctx, tableCode, filters, limit)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(records))
	for index := range records {
		payload := copyData(records[index].Data)
		payload["key"] = records[index].Key
		rows = append(rows, payload)
	}
	return rows, nil
}

// SMTExportChildren returns child row payloads for bridge pushes.
func (s *Service) SMTExportChildren(ctx context.Context, parentCode, parentKey, childCode string) ([]map[string]any, error) {
	return s.listChildPayloads(ctx, parentCode, parentKey, childCode)
}

// SMTSettings returns the merged settings (defaults overlaid with stored MSET values).
func (s *Service) SMTSettings(ctx context.Context) (map[string]string, error) {
	values := SMTSettingDefaults()
	records, err := s.smtQueryAll(ctx, "MSET", map[string]any{}, 500)
	if err != nil {
		return values, err
	}
	for index := range records {
		record := &records[index]
		if raw, present := record.Data["Value"]; present && raw != nil {
			values[record.Key] = fmt.Sprint(raw)
		}
	}
	return values, nil
}

// SMTSetSettings stores settings values (unknown keys are rejected).
func (s *Service) SMTSetSettings(ctx context.Context, values map[string]string) error {
	known := SMTSettingDefaults()
	for key, value := range values {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("%w: unknown SMT setting %s", ErrValidation, key)
		}
		if len(value) > 2000 {
			return fmt.Errorf("%w: setting %s value is too long", ErrValidation, key)
		}
		if _, err := s.smtUpsertRecord(ctx, "MSET", key, map[string]any{
			"Value":       value,
			"Description": "",
			"updated_at":  smtNowISO(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// SMTWorkOrderProgress returns per-step progress and quality for a work order.
func (s *Service) SMTWorkOrderProgress(ctx context.Context, workOrder string) (map[string]any, error) {
	order, err := s.smtGetRecord(ctx, "MSWO", workOrder)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("%w: work order %s not found", ErrValidation, workOrder)
	}
	steps, err := s.smtLoadRouting(ctx, order)
	if err != nil {
		return nil, err
	}
	units, err := s.smtQueryAll(ctx, "MUNT", map[string]any{"work_order": workOrder}, 1000)
	if err != nil {
		return nil, err
	}
	type stepStats struct {
		FirstPass map[string]bool
		Judged    map[string]bool
		Passed    int
		Failed    int
		Rejected  int
		NgHold    int
	}
	stats := map[int]*stepStats{}
	stepSeq := map[string]int{}
	for _, step := range steps {
		stepSeq[step.OperationCode] = step.Seq
		stats[step.Seq] = &stepStats{FirstPass: map[string]bool{}, Judged: map[string]bool{}}
	}
	statusCounts := map[string]int{}
	for index := range units {
		unit := &units[index]
		status := smtUnitStatus(unit)
		statusCounts[status]++
		events, err := s.listChildPayloads(ctx, "MUNT", unit.Key, "UNT1")
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			seq := int(numericValue(event, "seq"))
			entry := stats[seq]
			if entry == nil {
				continue
			}
			switch strings.ToUpper(stringValue(event, "result", "")) {
			case "PASS":
				if !entry.Judged[unit.Key] {
					entry.FirstPass[unit.Key] = true
					entry.Judged[unit.Key] = true
					entry.Passed++
				}
			case "FAIL":
				if !entry.Judged[unit.Key] {
					entry.Judged[unit.Key] = true
				}
				entry.Failed++
			case "REJECT":
				entry.Rejected++
			}
		}
		if status == "NG" {
			ngSeq := int(numericValue(unit.Data, "ng_step"))
			if entry := stats[ngSeq]; entry != nil {
				entry.NgHold++
			}
		}
	}
	rows := make([]map[string]any, 0, len(steps))
	fpy := 1.0
	for _, step := range steps {
		entry := stats[step.Seq]
		judged := len(entry.Judged)
		stepFpy := 1.0
		if judged > 0 {
			stepFpy = float64(len(entry.FirstPass)) / float64(judged)
		}
		fpy *= stepFpy
		rows = append(rows, map[string]any{
			"seq": step.Seq, "operation_code": step.OperationCode, "operation_name": step.OperationName,
			"op_type": step.OpType, "judged": judged, "first_pass": len(entry.FirstPass),
			"failed": entry.Failed, "rejected": entry.Rejected, "ng_hold": entry.NgHold,
			"fpy": round4(stepFpy),
		})
	}
	return map[string]any{
		"work_order":   order.Key,
		"product_code": stringValue(order.Data, "product_code", ""),
		"line_code":    stringValue(order.Data, "line_code", ""),
		"status":       smtStatusOf(order),
		"plan_qty":     numericValue(order.Data, "plan_qty"),
		"done_qty":     numericValue(order.Data, "done_qty"),
		"units":        statusCounts,
		"steps":        rows,
		"rty":          round4(fpy),
	}, nil
}

func round4(value float64) float64 {
	return float64(int(value*10000+0.5)) / 10000
}

// SMTBoard returns the line board snapshot.
func (s *Service) SMTBoard(ctx context.Context, lineCode string) (map[string]any, error) {
	machinesFilter := map[string]any{}
	if lineCode != "" {
		machinesFilter["line_code"] = lineCode
	}
	machines, err := s.smtQueryAll(ctx, "MMAC", machinesFilter, 200)
	if err != nil {
		return nil, err
	}
	machineCards := make([]map[string]any, 0, len(machines))
	readyCount := 0
	for index := range machines {
		machine := &machines[index]
		contextData, err := s.smtMachineContext(ctx, machine.Key)
		if err != nil {
			return nil, err
		}
		card := map[string]any{
			"machine_code":     machine.Key,
			"name":             stringValue(machine.Data, "Name", ""),
			"line_code":        stringValue(machine.Data, "line_code", ""),
			"work_order":       stringValue(machine.Data, "current_work_order", ""),
			"ready":            contextData["ready"],
			"awaiting_signoff": contextData["awaiting_signoff"],
			"verified_at":      contextData["verified_at"],
			"signed_off_at":    contextData["signed_off_at"],
			"summary":          contextData["summary"],
		}
		if isTruthy(contextData["ready"]) {
			readyCount++
		}
		machineCards = append(machineCards, card)
	}
	workOrdersFilter := map[string]any{"Status": "RUNNING"}
	if lineCode != "" {
		workOrdersFilter["line_code"] = lineCode
	}
	orders, err := s.smtQueryAll(ctx, "MSWO", workOrdersFilter, 100)
	if err != nil {
		return nil, err
	}
	orderCards := make([]map[string]any, 0, len(orders))
	for index := range orders {
		order := &orders[index]
		plan := numericValue(order.Data, "plan_qty")
		done := numericValue(order.Data, "done_qty")
		achievement := 0.0
		if plan > 0 {
			achievement = done / plan
		}
		orderCards = append(orderCards, map[string]any{
			"work_order": order.Key, "product_code": stringValue(order.Data, "product_code", ""),
			"line_code": stringValue(order.Data, "line_code", ""),
			"plan_qty":  plan, "done_qty": done, "achievement": round4(achievement),
			"customer_code": stringValue(order.Data, "customer_code", ""),
		})
	}
	andons, err := s.smtQueryAll(ctx, "MAND", map[string]any{}, 100)
	if err != nil {
		return nil, err
	}
	openAndons := make([]map[string]any, 0)
	for index := range andons {
		call := &andons[index]
		status := smtStatusOf(call)
		if status == "CLOSED" {
			continue
		}
		if lineCode != "" && stringValue(call.Data, "line_code", "") != lineCode {
			continue
		}
		openAndons = append(openAndons, map[string]any{
			"call_no": call.Key, "call_type": stringValue(call.Data, "call_type", ""),
			"machine_code": stringValue(call.Data, "machine_code", ""),
			"status":       status, "called_at": stringValue(call.Data, "called_at", ""),
			"description": stringValue(call.Data, "description", ""),
		})
	}
	return map[string]any{
		"line_code":      lineCode,
		"machines":       machineCards,
		"ready_machines": readyCount,
		"machine_count":  len(machineCards),
		"work_orders":    orderCards,
		"open_andons":    openAndons,
		"generated_at":   smtNowISO(),
	}, nil
}

// SMTTraceSN returns the full trace of one serial number.
func (s *Service) SMTTraceSN(ctx context.Context, sn string) (map[string]any, error) {
	unit, err := s.smtGetRecord(ctx, "MUNT", sn)
	if err != nil {
		return nil, err
	}
	if unit == nil {
		return nil, ErrNotFound
	}
	events, err := s.listChildPayloads(ctx, "MUNT", sn, "UNT1")
	if err != nil {
		return nil, err
	}
	sort.SliceStable(events, func(i, j int) bool {
		return numericValue(events[i], "seq") < numericValue(events[j], "seq")
	})
	defects, err := s.smtQueryAll(ctx, "MUDF", map[string]any{"sn": sn}, 100)
	if err != nil {
		return nil, err
	}
	defectRows := make([]map[string]any, 0, len(defects))
	for index := range defects {
		defectRows = append(defectRows, copyData(defects[index].Data))
	}
	// Materials: loadings active on the machine at each SMT pass time.
	materials := make([]map[string]any, 0)
	workOrder := stringValue(unit.Data, "work_order", "")
	for _, event := range events {
		if !strings.EqualFold(stringValue(event, "op_type", ""), "SMT") {
			continue
		}
		if strings.ToUpper(stringValue(event, "result", "")) != "PASS" {
			continue
		}
		passAt := stringValue(event, "created_at", "")
		machineCode := stringValue(event, "machine_code", "")
		if passAt == "" {
			continue
		}
		filter := map[string]any{"work_order": workOrder}
		if machineCode != "" {
			filter["machine_code"] = machineCode
		}
		loadings, err := s.smtQueryAll(ctx, "MLOD", filter, 500)
		if err != nil {
			return nil, err
		}
		for index := range loadings {
			loading := &loadings[index]
			loadedAt := stringValue(loading.Data, "loaded_at", "")
			unloadedAt := stringValue(loading.Data, "unloaded_at", "")
			if loadedAt <= passAt && (unloadedAt == "" || unloadedAt >= passAt) {
				materials = append(materials, map[string]any{
					"machine_code": stringValue(loading.Data, "machine_code", ""),
					"slot_no":      stringValue(loading.Data, "slot_no", ""),
					"reel_code":    stringValue(loading.Data, "reel_code", ""),
					"part_no":      stringValue(loading.Data, "part_no", ""),
					"lot_no":       stringValue(loading.Data, "lot_no", ""),
					"position":     stringValue(loading.Data, "position", "MAIN"),
					"passed_at":    passAt, "operation_code": stringValue(event, "operation_code", ""),
				})
			}
		}
	}
	panelCode := stringValue(unit.Data, "panel_code", "")
	var panel any
	if panelCode != "" {
		if record, _ := s.smtGetRecord(ctx, "MPNL", panelCode); record != nil {
			panel = copyData(record.Data)
			panel.(map[string]any)["panel_code"] = panelCode
		}
	}
	return map[string]any{
		"sn": sn, "unit": copyData(unit.Data),
		"events": events, "defects": defectRows, "materials": materials, "panel": panel,
	}, nil
}

// SMTReelUnits returns the units that may have consumed a reel.
func (s *Service) SMTReelUnits(ctx context.Context, reelCode string) (map[string]any, error) {
	reel, err := s.smtGetRecord(ctx, "MREL", reelCode)
	if err != nil {
		return nil, err
	}
	if reel == nil {
		return nil, ErrNotFound
	}
	loadings, err := s.smtQueryAll(ctx, "MLOD", map[string]any{"reel_code": reelCode}, 200)
	if err != nil {
		return nil, err
	}
	seen := map[string]map[string]any{}
	for index := range loadings {
		loading := &loadings[index]
		loadedAt := stringValue(loading.Data, "loaded_at", "")
		unloadedAt := stringValue(loading.Data, "unloaded_at", "")
		machineCode := stringValue(loading.Data, "machine_code", "")
		workOrder := stringValue(loading.Data, "work_order", "")
		units, err := s.smtQueryAll(ctx, "MUNT", map[string]any{"work_order": workOrder}, 2000)
		if err != nil {
			return nil, err
		}
		for unitIndex := range units {
			unit := &units[unitIndex]
			events, err := s.listChildPayloads(ctx, "MUNT", unit.Key, "UNT1")
			if err != nil {
				return nil, err
			}
			for _, event := range events {
				if !strings.EqualFold(stringValue(event, "op_type", ""), "SMT") ||
					strings.ToUpper(stringValue(event, "result", "")) != "PASS" {
					continue
				}
				if machineCode != "" && stringValue(event, "machine_code", "") != machineCode {
					continue
				}
				passAt := stringValue(event, "created_at", "")
				if passAt >= loadedAt && (unloadedAt == "" || passAt <= unloadedAt) {
					seen[unit.Key] = map[string]any{
						"sn": unit.Key, "work_order": workOrder, "status": smtUnitStatus(unit),
						"passed_at": passAt, "machine_code": stringValue(event, "machine_code", ""),
						"operation_code": stringValue(event, "operation_code", ""),
					}
				}
			}
		}
	}
	rows := make([]map[string]any, 0, len(seen))
	for _, row := range seen {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return stringValue(rows[i], "passed_at", "") < stringValue(rows[j], "passed_at", "")
	})
	return map[string]any{"reel_code": reelCode, "reel": copyData(reel.Data), "units": rows}, nil
}

// ---------------------------------------------------------------------------
// Reporting.
// ---------------------------------------------------------------------------

// SMTProductionReport aggregates in/out, per-step FPY/RTY, defect pareto and
// reject pareto for a date range.
func (s *Service) SMTProductionReport(ctx context.Context, from, to, lineCode string) (map[string]any, error) {
	start, end := smtParseRange(from, to)
	units, err := s.smtQueryAll(ctx, "MUNT", map[string]any{}, 3000)
	if err != nil {
		return nil, err
	}
	daily := map[string]map[string]int{}
	stepStats := map[string]map[string]map[string]bool{}
	rejectCounts := map[string]int{}
	defectCounts := map[string]int{}
	failCounts := map[string]int{}
	unitCount := 0
	for index := range units {
		unit := &units[index]
		if !smtInRange(stringValue(unit.Data, "created_at", ""), start, end) {
			// still consider units that finished in range even if started earlier
			if !smtInRange(stringValue(unit.Data, "finished_at", ""), start, end) {
				continue
			}
		}
		unitCount++
		day := firstNonEmptyString(truncDay(stringValue(unit.Data, "created_at", "")), truncDay(stringValue(unit.Data, "finished_at", "")))
		if daily[day] == nil {
			daily[day] = map[string]int{"in": 0, "out": 0}
		}
		if smtInRange(stringValue(unit.Data, "created_at", ""), start, end) {
			daily[day]["in"]++
		}
		if smtInRange(stringValue(unit.Data, "finished_at", ""), start, end) {
			daily[day]["out"]++
		}
		events, err := s.listChildPayloads(ctx, "MUNT", unit.Key, "UNT1")
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			operation := stringValue(event, "operation_code", "")
			if stepStats[operation] == nil {
				stepStats[operation] = map[string]map[string]bool{"judged": {}, "first_pass": {}}
			}
			res := strings.ToUpper(stringValue(event, "result", ""))
			switch res {
			case "PASS":
				if !stepStats[operation]["judged"][unit.Key] {
					stepStats[operation]["judged"][unit.Key] = true
					stepStats[operation]["first_pass"][unit.Key] = true
				}
			case "FAIL":
				stepStats[operation]["judged"][unit.Key] = true
				failCounts[operation]++
			case "REJECT":
				rejectCounts[stringValue(event, "reject_type", "")]++
			}
		}
	}
	// Defect pareto within range.
	defects, err := s.smtQueryAll(ctx, "MUDF", map[string]any{}, 3000)
	if err != nil {
		return nil, err
	}
	for index := range defects {
		defect := &defects[index]
		if !smtInRange(stringValue(defect.Data, "created_at", ""), start, end) {
			continue
		}
		defectCounts[stringValue(defect.Data, "defect_code", "")]++
	}
	stepRows := make([]map[string]any, 0, len(stepStats))
	rty := 1.0
	operations := make([]string, 0, len(stepStats))
	for operation := range stepStats {
		operations = append(operations, operation)
	}
	sort.Strings(operations)
	for _, operation := range operations {
		judged := len(stepStats[operation]["judged"])
		firstPass := len(stepStats[operation]["first_pass"])
		fpy := 1.0
		if judged > 0 {
			fpy = float64(firstPass) / float64(judged)
		}
		rty *= fpy
		stepRows = append(stepRows, map[string]any{
			"operation_code": operation, "judged": judged, "first_pass": firstPass,
			"failed": failCounts[operation], "fpy": round4(fpy),
		})
	}
	dailyRows := make([]map[string]any, 0, len(daily))
	days := make([]string, 0, len(daily))
	for day := range daily {
		days = append(days, day)
	}
	sort.Strings(days)
	for _, day := range days {
		dailyRows = append(dailyRows, map[string]any{"day": day, "in": daily[day]["in"], "out": daily[day]["out"]})
	}
	return map[string]any{
		"from": start.Format("2006-01-02"), "to": end.Format("2006-01-02"),
		"line_code": lineCode, "unit_count": unitCount,
		"daily": dailyRows, "steps": stepRows, "rty": round4(rty),
		"defects": paretoRows(defectCounts), "rejects": paretoRows(rejectCounts),
	}, nil
}

// SMTAndonReport aggregates andon calls and their response/close durations.
func (s *Service) SMTAndonReport(ctx context.Context, from, to string) (map[string]any, error) {
	start, end := smtParseRange(from, to)
	calls, err := s.smtQueryAll(ctx, "MAND", map[string]any{}, 3000)
	if err != nil {
		return nil, err
	}
	type row struct {
		CallNo   string `json:"call_no"`
		CallType string `json:"call_type"`
		Machine  string `json:"machine_code"`
		Status   string `json:"status"`
		CalledAt string `json:"called_at"`
		AckAt    string `json:"ack_at"`
		ClosedAt string `json:"closed_at"`
	}
	rows := []row{}
	typeCounts := map[string]int{}
	totalResponse := 0.0
	totalClose := 0.0
	responded := 0
	closed := 0
	for index := range calls {
		call := &calls[index]
		calledAt := stringValue(call.Data, "called_at", "")
		if !smtInRange(calledAt, start, end) {
			continue
		}
		item := row{
			CallNo: call.Key, CallType: stringValue(call.Data, "call_type", ""),
			Machine: stringValue(call.Data, "machine_code", ""), Status: smtStatusOf(call),
			CalledAt: calledAt, AckAt: stringValue(call.Data, "ack_at", ""),
			ClosedAt: stringValue(call.Data, "closed_at", ""),
		}
		rows = append(rows, item)
		typeCounts[item.CallType]++
		if minutes, ok := smtMinutesBetween(item.CalledAt, item.AckAt); ok {
			totalResponse += minutes
			responded++
		}
		if minutes, ok := smtMinutesBetween(item.CalledAt, item.ClosedAt); ok {
			totalClose += minutes
			closed++
		}
	}
	avgResponse, avgClose := 0.0, 0.0
	if responded > 0 {
		avgResponse = totalResponse / float64(responded)
	}
	if closed > 0 {
		avgClose = totalClose / float64(closed)
	}
	return map[string]any{
		"from": start.Format("2006-01-02"), "to": end.Format("2006-01-02"),
		"total": len(rows), "by_type": typeCounts,
		"avg_response_min": round4(avgResponse), "avg_close_min": round4(avgClose),
		"calls": rows,
	}, nil
}

// SMTDowntimeReport computes machine availability from downtime records and
// shift windows (parameter shift_definitions).
func (s *Service) SMTDowntimeReport(ctx context.Context, from, to string) (map[string]any, error) {
	start, end := smtParseRange(from, to)
	machines, err := s.smtQueryAll(ctx, "MMAC", map[string]any{}, 200)
	if err != nil {
		return nil, err
	}
	downtimes, err := s.smtQueryAll(ctx, "MDWN", map[string]any{}, 3000)
	if err != nil {
		return nil, err
	}
	shiftWindows, err := s.smtShiftWindows(ctx, start, end)
	if err != nil {
		return nil, err
	}
	// Planned production time = shift windows inside the range.
	planned := 0.0
	type interval struct{ start, end time.Time }
	shiftIntervals := []interval{}
	for _, window := range shiftWindows {
		planned += window[1].Sub(window[0]).Seconds()
		shiftIntervals = append(shiftIntervals, interval{window[0], window[1]})
	}
	lines := map[string]map[string]any{}
	reasonCounts := map[string]int{}
	mttrValues := []float64{}
	for index := range machines {
		machine := &machines[index]
		down := 0.0
		for dIndex := range downtimes {
			record := &downtimes[dIndex]
			machineCode := stringValue(record.Data, "machine_code", "")
			lineCode := stringValue(record.Data, "line_code", "")
			if machineCode != "" && machineCode != machine.Key {
				continue
			}
			if machineCode == "" && lineCode != "" && lineCode != stringValue(machine.Data, "line_code", "") {
				continue
			}
			dStart, okStart := smtParseTime(stringValue(record.Data, "started_at", ""))
			if !okStart || dStart.After(end) {
				continue
			}
			dEnd, okEnd := smtParseTime(stringValue(record.Data, "ended_at", ""))
			if !okEnd {
				dEnd = end
			}
			if dEnd.Before(start) {
				continue
			}
			if dStart.Before(start) {
				dStart = start
			}
			if dEnd.After(end) {
				dEnd = end
			}
			if dEnd.Before(dStart) {
				continue
			}
			plannedFlag := isTruthy(record.Data["planned"])
			duration := dEnd.Sub(dStart).Seconds()
			if plannedFlag {
				planned -= duration
				continue
			}
			// Clip to shift windows.
			covered := 0.0
			for _, window := range shiftIntervals {
				overlapStart := dStart
				if window.start.After(overlapStart) {
					overlapStart = window.start
				}
				overlapEnd := dEnd
				if window.end.Before(overlapEnd) {
					overlapEnd = window.end
				}
				if overlapEnd.After(overlapStart) {
					covered += overlapEnd.Sub(overlapStart).Seconds()
				}
			}
			down += covered
			reasonCounts[stringValue(record.Data, "reason_type", "")]++
			mttrValues = append(mttrValues, duration/60)
		}
		availability := 1.0
		if planned > 0 {
			availability = (planned - down) / planned
			if availability < 0 {
				availability = 0
			}
		}
		lines[machine.Key] = map[string]any{
			"machine_code": machine.Key, "line_code": stringValue(machine.Data, "line_code", ""),
			"planned_seconds": planned, "downtime_seconds": down,
			"availability": round4(availability),
		}
	}
	rows := make([]map[string]any, 0, len(lines))
	for _, row := range lines {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return stringValue(rows[i], "machine_code", "") < stringValue(rows[j], "machine_code", "")
	})
	mttr := 0.0
	if len(mttrValues) > 0 {
		sum := 0.0
		for _, value := range mttrValues {
			sum += value
		}
		mttr = sum / float64(len(mttrValues))
	}
	return map[string]any{
		"from": start.Format("2006-01-02"), "to": end.Format("2006-01-02"),
		"machines": rows, "reasons": paretoRows(reasonCounts), "mttr_min": round4(mttr),
	}, nil
}

// SMTMaterialReport aggregates reel stock-in/out by part for the range.
func (s *Service) SMTMaterialReport(ctx context.Context, from, to string) (map[string]any, error) {
	start, end := smtParseRange(from, to)
	reels, err := s.smtQueryAll(ctx, "MREL", map[string]any{}, 3000)
	if err != nil {
		return nil, err
	}
	type totals struct{ inQty, outQty, consumedQty, inReels, outReels float64 }
	byPart := map[string]*totals{}
	total := &totals{}
	for index := range reels {
		reel := &reels[index]
		partNo := stringValue(reel.Data, "part_no", "")
		if byPart[partNo] == nil {
			byPart[partNo] = &totals{}
		}
		entry := byPart[partNo]
		receivedAt := stringValue(reel.Data, "received_at", "")
		if smtInRange(receivedAt, start, end) {
			qty := numericValue(reel.Data, "initial_qty")
			entry.inQty += qty
			total.inQty += qty
			entry.inReels++
			total.inReels++
		}
		// Consumption from loading slices in range.
	}
	// Consumption: loading records unloaded in range.
	loadings, err := s.smtQueryAll(ctx, "MLOD", map[string]any{}, 3000)
	if err != nil {
		return nil, err
	}
	for index := range loadings {
		loading := &loadings[index]
		unloadedAt := stringValue(loading.Data, "unloaded_at", "")
		if unloadedAt == "" || !smtInRange(unloadedAt, start, end) {
			continue
		}
		partNo := stringValue(loading.Data, "part_no", "")
		if byPart[partNo] == nil {
			byPart[partNo] = &totals{}
		}
		consumed := numericValue(loading.Data, "qty_at_load") - numericValue(loading.Data, "qty_at_unload")
		if consumed < 0 {
			consumed = 0
		}
		byPart[partNo].consumedQty += consumed
		total.consumedQty += consumed
	}
	rows := make([]map[string]any, 0, len(byPart))
	for partNo, entry := range byPart {
		rows = append(rows, map[string]any{
			"part_no": partNo, "in_qty": entry.inQty, "consumed_qty": entry.consumedQty,
			"in_reels": entry.inReels, "out_reels": entry.outReels,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return stringValue(rows[i], "part_no", "") < stringValue(rows[j], "part_no", "")
	})
	return map[string]any{
		"from": start.Format("2006-01-02"), "to": end.Format("2006-01-02"),
		"parts": rows,
		"total": map[string]any{"in_qty": total.inQty, "consumed_qty": total.consumedQty, "in_reels": total.inReels},
	}, nil
}

func paretoRows(counts map[string]int) []map[string]any {
	rows := make([]map[string]any, 0, len(counts))
	for key, count := range counts {
		if key == "" {
			key = "(unknown)"
		}
		rows = append(rows, map[string]any{"key": key, "count": count})
	}
	sort.Slice(rows, func(i, j int) bool {
		left := numericValue(rows[i], "count")
		right := numericValue(rows[j], "count")
		if left == right {
			return stringValue(rows[i], "key", "") < stringValue(rows[j], "key", "")
		}
		return left > right
	})
	return rows
}

func smtParseRange(from, to string) (time.Time, time.Time) {
	end := time.Now()
	if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(to)); err == nil {
		end = parsed.Add(24 * time.Hour)
	}
	start := end.AddDate(0, 0, -7)
	if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(from)); err == nil {
		start = parsed
	}
	return start, end
}

func smtParseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func smtInRange(value string, start, end time.Time) bool {
	parsed, ok := smtParseTime(value)
	if !ok {
		return false
	}
	return !parsed.Before(start) && !parsed.After(end)
}

func truncDay(value string) string {
	parsed, ok := smtParseTime(value)
	if !ok {
		return ""
	}
	return parsed.Format("2006-01-02")
}

func smtMinutesBetween(from, to string) (float64, bool) {
	start, okStart := smtParseTime(from)
	end, okEnd := smtParseTime(to)
	if !okStart || !okEnd || end.Before(start) {
		return 0, false
	}
	return end.Sub(start).Minutes(), true
}

// smtShiftWindows parses the shift_definitions parameter into date windows.
func (s *Service) smtShiftWindows(ctx context.Context, start, end time.Time) ([][2]time.Time, error) {
	definitions := s.smtSetting(ctx, "shift_definitions")
	windows := [][2]time.Time{}
	parseMinutes := func(value string) (int, bool) {
		parts := strings.Split(strings.TrimSpace(value), ":")
		if len(parts) != 2 {
			return 0, false
		}
		hour, errHour := strconv.Atoi(strings.TrimSpace(parts[0]))
		minute, errMinute := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errHour != nil || errMinute != nil || hour < 0 || hour > 24 || minute < 0 || minute > 59 {
			return 0, false
		}
		return hour*60 + minute, true
	}
	type shift struct{ startMin, endMin int }
	shifts := []shift{}
	for _, part := range strings.FieldsFunc(definitions, func(r rune) bool { return r == ';' || r == '；' }) {
		segments := strings.SplitN(part, "=", 2)
		if len(segments) != 2 {
			continue
		}
		rangeParts := strings.SplitN(segments[1], "-", 2)
		if len(rangeParts) != 2 {
			continue
		}
		startMin, okStart := parseMinutes(rangeParts[0])
		endMin, okEnd := parseMinutes(rangeParts[1])
		if !okStart || !okEnd || startMin == endMin {
			continue
		}
		shifts = append(shifts, shift{startMin: startMin, endMin: endMin})
	}
	if len(shifts) == 0 {
		return [][2]time.Time{{start, end}}, nil
	}
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	for !day.After(end) {
		for _, item := range shifts {
			windowStart := day.Add(time.Duration(item.startMin) * time.Minute)
			windowEnd := day.Add(time.Duration(item.endMin) * time.Minute)
			if item.endMin <= item.startMin {
				windowEnd = windowEnd.Add(24 * time.Hour)
			}
			if windowEnd.Before(start) || windowStart.After(end) {
				continue
			}
			clipStart := windowStart
			if clipStart.Before(start) {
				clipStart = start
			}
			clipEnd := windowEnd
			if clipEnd.After(end) {
				clipEnd = end
			}
			windows = append(windows, [2]time.Time{clipStart, clipEnd})
		}
		day = day.AddDate(0, 0, 1)
	}
	return windows, nil
}
