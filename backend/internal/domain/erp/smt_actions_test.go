package erp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// In-memory QueryRecords for the business fake repository so SMT actions can
// run without a database.
func (r *businessFakeRepository) QueryRecords(_ context.Context, table TableDefinition, input RecordQuery) (*RecordPage, error) {
	page := &RecordPage{Records: []Record{}}
	for _, record := range r.records[table.Code] {
		if !fakeRecordMatches(record.Data, input.Filters) {
			continue
		}
		if input.Search != "" {
			encoded, _ := json.Marshal(record.Data)
			if !strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(input.Search)) {
				continue
			}
		}
		if input.ChildCode != "" {
			children := r.children[childBucket(table.Code, record.Key, input.ChildCode)]
			matched := false
			for _, child := range children {
				if fakeRecordMatches(child.Data, input.ChildFilters) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		page.Records = append(page.Records, record)
	}
	page.Total = int64(len(page.Records))
	if input.Limit > 0 && len(page.Records) > input.Limit {
		page.Records = page.Records[:input.Limit]
	}
	return page, nil
}

func fakeRecordMatches(data map[string]any, filters map[string]any) bool {
	for key, expected := range filters {
		value, present := data[key]
		if !present {
			return false
		}
		if fmt.Sprint(value) != fmt.Sprint(expected) {
			return false
		}
	}
	return true
}

// seedSMTBase seeds one line/machine/slot, one product with BOM and feeder
// setup, one running work order and one in-stock reel.
func seedSMTBase(repo *businessFakeRepository) {
	repo.seed("MITM", "C0402-104K", map[string]any{"ItemCode": "C0402-104K", "ItemName": "100nF 0402", "InvntryUom": "pcs"})
	repo.seed("MITM", "C0402-103K", map[string]any{"ItemCode": "C0402-103K", "ItemName": "10nF 0402", "InvntryUom": "pcs"})
	repo.seed("MPRD", "P1", map[string]any{"ProductCode": "P1", "Name": "控制板", "Status": "active"})
	repo.seedChild("MPRD", "P1", "PRD1", map[string]any{"PartNo": "C0402-104K", "Qty": 2, "RefDesignators": "C1,C2", "AltPartNos": "C0402-103K"})
	repo.seed("MLIN", "L1", map[string]any{"LineCode": "L1", "Name": "一线", "Status": "active"})
	repo.seed("MMAC", "M01", map[string]any{"MachineCode": "M01", "Name": "贴片机 1", "line_code": "L1", "current_work_order": "WO-1", "Status": "active"})
	repo.seedChild("MMAC", "M01", "MAC1", map[string]any{"SlotNo": "F01", "Barcode": "M01-F01", "Locked": false})
	repo.seedChild("MMAC", "M01", "MAC1", map[string]any{"SlotNo": "F02", "Barcode": "M01-F02", "Locked": false})
	repo.seed("MFSB", "P1-M01", map[string]any{"SetupCode": "P1-M01", "product_code": "P1", "machine_code": "M01", "active": true, "Status": "active"})
	repo.seedChild("MFSB", "P1-M01", "FSB1", map[string]any{"SlotNo": "F01", "PartNo": "C0402-104K", "AltPartNos": "C0402-103K"})
	repo.seedChild("MFSB", "P1-M01", "FSB1", map[string]any{"SlotNo": "F02", "PartNo": "C0402-103K", "AltPartNos": ""})
	repo.seed("MSWO", "WO-1", map[string]any{"WorkOrderCode": "WO-1", "Name": "工单一", "product_code": "P1", "line_code": "L1", "plan_qty": 100, "Status": "RUNNING"})
	repo.seed("MREL", "R0001", map[string]any{"ReelCode": "R0001", "part_no": "C0402-104K", "qty": 4000, "initial_qty": 4000, "status": "IN_STOCK", "owner_type": "SELF"})
	repo.seed("MREL", "R0002", map[string]any{"ReelCode": "R0002", "part_no": "C0402-103K", "qty": 4000, "initial_qty": 4000, "status": "IN_STOCK", "owner_type": "SELF"})
	// Disable gates that need FAI / sign-off data for these unit tests.
	repo.seed("MSET", "fai_required", map[string]any{"SettingKey": "fai_required", "Value": "false"})
	repo.seed("MSET", "pass_require_machine_ready", map[string]any{"SettingKey": "pass_require_machine_ready", "Value": "false"})
}

func runSMT(t *testing.T, repo *businessFakeRepository, tableCode, key, action string, data map[string]any) *ActionResult {
	t.Helper()
	service := NewService(repo, DefaultCatalog())
	result, err := service.RunAction(context.Background(), tableCode, key, action, ActionInput{Data: data})
	if err != nil {
		t.Fatalf("%s:%s returned error: %v", tableCode, action, err)
	}
	if result == nil {
		t.Fatalf("%s:%s returned nil result", tableCode, action)
	}
	return result
}

func TestSMTLoadSuccessMarksReelLoadedAndRecordsScan(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	result := runSMT(t, repo, "MLOD", "LD-1", "load", map[string]any{
		"machine_code": "M01", "slot_code": "F01", "reel_code": "R0001",
	})
	if result.Status != "OK" {
		t.Fatalf("load status = %s, effects %#v", result.Status, result.Effects)
	}
	reel := repo.records["MREL"]["R0001"]
	if stringValue(reel.Data, "status", "") != "LOADED" || stringValue(reel.Data, "current_slot", "") != "M01-F01" {
		t.Fatalf("reel not marked loaded: %#v", reel.Data)
	}
	loading := repo.records["MLOD"]["LD-1"]
	if stringValue(loading.Data, "status", "") != "LOADED" || stringValue(loading.Data, "position", "") != "MAIN" {
		t.Fatalf("loading record unexpected: %#v", loading.Data)
	}
	if len(repo.records["MVER"]) != 1 {
		t.Fatalf("verification scan rows = %d, want 1", len(repo.records["MVER"]))
	}
	for _, scan := range repo.records["MVER"] {
		if stringValue(scan.Data, "result", "") != "OK" {
			t.Fatalf("scan result = %v, want OK", scan.Data["result"])
		}
	}
}

func TestSMTLoadRejectsPartMismatch(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	// F02 requires C0402-103K; use a reel of the other part with no alternate match on F01.
	repo.seed("MREL", "R0003", map[string]any{"ReelCode": "R0003", "part_no": "X-UNKNOWN", "qty": 100, "status": "IN_STOCK", "owner_type": "SELF"})
	result := runSMT(t, repo, "MLOD", "LD-2", "load", map[string]any{
		"machine_code": "M01", "slot_code": "F01", "reel_code": "R0003",
	})
	if result.Status != "ERROR" {
		t.Fatalf("load status = %s, want ERROR", result.Status)
	}
	if got := stringValue(result.Effects, "error_type", ""); got != "PART_MISMATCH" {
		t.Fatalf("error_type = %s, want PART_MISMATCH", got)
	}
	if _, ok := repo.records["MLOD"]["LD-2"]; ok {
		t.Fatal("rejected load must not create a loading record")
	}
	foundError := false
	for _, scan := range repo.records["MVER"] {
		if stringValue(scan.Data, "result", "") == "ERROR" && stringValue(scan.Data, "error_type", "") == "PART_MISMATCH" {
			foundError = true
		}
	}
	if !foundError {
		t.Fatal("expected an ERROR verification scan row")
	}
}

func TestSMTLoadAlternatePartIsAccepted(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	result := runSMT(t, repo, "MLOD", "LD-3", "load", map[string]any{
		"machine_code": "M01", "slot_code": "F01", "reel_code": "R0002",
	})
	if result.Status != "OK" {
		t.Fatalf("alternate load status = %s, effects %#v", result.Status, result.Effects)
	}
	if !isTruthy(result.Effects["is_alternate"]) {
		t.Fatalf("expected is_alternate, effects %#v", result.Effects)
	}
}

func TestSMTLoadConfirmsWhenSlotOccupied(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	repo.seed("MLOD", "LD-OLD", map[string]any{
		"LoadCode": "LD-OLD", "machine_code": "M01", "slot_no": "F01", "reel_code": "R0001",
		"status": "LOADED", "position": "MAIN", "work_order": "WO-1", "part_no": "C0402-104K", "qty_at_load": 4000,
	})
	repo.seed("MREL", "R0004", map[string]any{"ReelCode": "R0004", "part_no": "C0402-104K", "qty": 100, "status": "IN_STOCK", "owner_type": "SELF"})
	result := runSMT(t, repo, "MLOD", "LD-4", "load", map[string]any{
		"machine_code": "M01", "slot_code": "F01", "reel_code": "R0004",
	})
	if result.Status != "CONFIRM" {
		t.Fatalf("load status = %s, want CONFIRM", result.Status)
	}
	// With splice=true the reel is attached as the spliced reel.
	result = runSMT(t, repo, "MLOD", "LD-5", "load", map[string]any{
		"machine_code": "M01", "slot_code": "F01", "reel_code": "R0004", "splice": true,
	})
	if result.Status != "OK" {
		t.Fatalf("splice status = %s, effects %#v", result.Status, result.Effects)
	}
	if stringValue(repo.records["MLOD"]["LD-5"].Data, "position", "") != "SPLICED" {
		t.Fatal("spliced loading must have position SPLICED")
	}
}

func TestSMTUnloadReturnsReelAndClearsVerification(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	// Verification with empty required slots must fail and leave the machine unverified.
	result := runSMT(t, repo, "MMAC", "M01", "verify", map[string]any{})
	if result.Status != "failed" || stringValue(repo.records["MMAC"]["M01"].Data, "verified_at", "") != "" {
		t.Fatalf("verify with empty slots: status = %s, machine %#v", result.Status, repo.records["MMAC"]["M01"].Data)
	}
	// The feeder setup requires both F01 and F02; load both before verifying.
	runSMT(t, repo, "MLOD", "LD-6", "load", map[string]any{
		"machine_code": "M01", "slot_code": "F01", "reel_code": "R0001",
	})
	runSMT(t, repo, "MLOD", "LD-7", "load", map[string]any{
		"machine_code": "M01", "slot_code": "F02", "reel_code": "R0002",
	})
	runSMT(t, repo, "MMAC", "M01", "verify", map[string]any{})
	if stringValue(repo.records["MMAC"]["M01"].Data, "verified_at", "") == "" {
		t.Fatal("machine should be verified after verify with loaded setup")
	}
	result = runSMT(t, repo, "MLOD", "LD-6", "unload", map[string]any{
		"reason": "RETURN", "qty_at_unload": 1500, "old_reel_action": "RETURN",
	})
	if result.Status != "unloaded" {
		t.Fatalf("unload status = %s", result.Status)
	}
	reel := repo.records["MREL"]["R0001"]
	if stringValue(reel.Data, "status", "") != "IN_STOCK" || numericValue(reel.Data, "qty") != 1500 {
		t.Fatalf("reel after unload: %#v", reel.Data)
	}
	if stringValue(repo.records["MMAC"]["M01"].Data, "verified_at", "") != "" {
		t.Fatal("unloading must clear machine verification")
	}
}

func TestSMTUnitPassCreatesUnitAndRejectsRouteSkip(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	repo.seed("MOPR", "SMT-1", map[string]any{"OpCode": "SMT-1", "Name": "贴片", "OpType": "SMT"})
	repo.seed("MOPR", "AOI-1", map[string]any{"OpCode": "AOI-1", "Name": "AOI", "OpType": "INSPECT"})
	repo.seed("MROU", "R1", map[string]any{"RouteCode": "R1", "product_code": "P1", "active": true, "Status": "active"})
	repo.seedChild("MROU", "R1", "ROU1", map[string]any{"Seq": 10, "OpCode": "SMT-1"})
	repo.seedChild("MROU", "R1", "ROU1", map[string]any{"Seq": 20, "OpCode": "AOI-1"})
	repo.records["MSWO"]["WO-1"].Data["route_code"] = "R1"

	result := runSMT(t, repo, "MUNT", "SN0001", "pass", map[string]any{"work_order": "WO-1", "operation_code": "SMT-1", "machine_code": "M01"})
	if result.Status != "OK" {
		t.Fatalf("first pass status = %s, effects %#v", result.Status, result.Effects)
	}
	unit := repo.records["MUNT"]["SN0001"]
	if numericValue(unit.Data, "last_seq") != 10 || stringValue(unit.Data, "status", "") != "IN_PROCESS" {
		t.Fatalf("unit after first pass: %#v", unit.Data)
	}
	// Jumping to the last step must be rejected as a route skip.
	result = runSMT(t, repo, "MUNT", "SN0002", "pass", map[string]any{"work_order": "WO-1", "operation_code": "AOI-1"})
	if result.Status != "REJECT" {
		t.Fatalf("skip pass status = %s, want REJECT", result.Status)
	}
	if got := stringValue(result.Effects, "reject_type", ""); got != "ROUTE_SKIP" && got != "SN_NOT_STARTED" {
		t.Fatalf("reject_type = %s", got)
	}
	// Completing the last step finishes the unit and reports progress.
	result = runSMT(t, repo, "MUNT", "SN0001", "pass", map[string]any{"work_order": "WO-1", "operation_code": "AOI-1"})
	if result.Status != "OK" || !isTruthy(result.Effects["finished"]) {
		t.Fatalf("finish pass: %#v", result.Effects)
	}
	unit = repo.records["MUNT"]["SN0001"]
	if stringValue(unit.Data, "status", "") != "FINISHED" {
		t.Fatalf("unit status = %v, want FINISHED", unit.Data["status"])
	}
	if numericValue(repo.records["MSWO"]["WO-1"].Data, "done_qty") != 1 {
		t.Fatalf("work order done_qty = %v, want 1", repo.records["MSWO"]["WO-1"].Data["done_qty"])
	}
}

func TestSMTUnitFailRequiresDefectCodeAndCreatesDefect(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	repo.seed("MOPR", "SMT-1", map[string]any{"OpCode": "SMT-1", "Name": "贴片", "OpType": "SMT"})
	repo.seed("MROU", "R1", map[string]any{"RouteCode": "R1", "product_code": "P1", "active": true, "Status": "active"})
	repo.seedChild("MROU", "R1", "ROU1", map[string]any{"Seq": 10, "OpCode": "SMT-1"})
	repo.records["MSWO"]["WO-1"].Data["route_code"] = "R1"
	repo.seed("MDFC", "SD02", map[string]any{"DefectCode": "SD02", "Name": "少锡", "Status": "active"})

	result := runSMT(t, repo, "MUNT", "SN0100", "fail", map[string]any{"work_order": "WO-1", "operation_code": "SMT-1", "defects": []any{}})
	if result.Status != "REJECT" || stringValue(result.Effects, "reject_type", "") != "DEFECT_REQUIRED" {
		t.Fatalf("fail without defects: %#v", result.Effects)
	}
	result = runSMT(t, repo, "MUNT", "SN0100", "fail", map[string]any{
		"work_order": "WO-1", "operation_code": "SMT-1",
		"defects": []any{map[string]any{"code": "SD02", "location": "C1"}},
	})
	if result.Status != "FAIL" {
		t.Fatalf("fail status = %s, effects %#v", result.Status, result.Effects)
	}
	unit := repo.records["MUNT"]["SN0100"]
	if stringValue(unit.Data, "status", "") != "NG" {
		t.Fatalf("unit status = %v, want NG", unit.Data["status"])
	}
	foundDefect := false
	for _, defect := range repo.records["MUDF"] {
		if stringValue(defect.Data, "defect_code", "") == "SD02" && stringValue(defect.Data, "part_no", "") == "C0402-104K" {
			foundDefect = true
		}
	}
	if !foundDefect {
		t.Fatalf("expected defect row with part resolved from designator: %#v", repo.records["MUDF"])
	}
	// Repair the defect and confirm the unit returns to in-process.
	var defectKey string
	for key := range repo.records["MUDF"] {
		defectKey = key
	}
	result = runSMT(t, repo, "MUNT", "SN0100", "repair", map[string]any{"defect_key": defectKey, "action": "REPAIRED"})
	if result.Status != "repaired" {
		t.Fatalf("repair status = %s, effects %#v", result.Status, result.Effects)
	}
	if stringValue(repo.records["MUNT"]["SN0100"].Data, "status", "") != "IN_PROCESS" {
		t.Fatalf("unit status after repair = %v", repo.records["MUNT"]["SN0100"].Data["status"])
	}
}

func TestSMTAndonRaiseStartsAndClosesDowntime(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	result := runSMT(t, repo, "MAND", "AD-1", "raise", map[string]any{
		"machine_code": "M01", "call_type": "EQUIPMENT", "description": "贴片机报警",
	})
	if result.Status != "open" {
		t.Fatalf("andon status = %s, effects %#v", result.Status, result.Effects)
	}
	openDowntimes := 0
	var downtimeKey string
	for key, record := range repo.records["MDWN"] {
		if stringValue(record.Data, "ended_at", "") == "" {
			openDowntimes++
			downtimeKey = key
		}
	}
	if openDowntimes != 1 {
		t.Fatalf("open downtimes = %d, want 1 (%#v)", openDowntimes, repo.records["MDWN"])
	}
	runSMT(t, repo, "MAND", "AD-1", "ack", map[string]any{})
	runSMT(t, repo, "MAND", "AD-1", "close", map[string]any{"cause": "传感器故障", "solution": "更换传感器"})
	record := repo.records["MDWN"][downtimeKey]
	if stringValue(record.Data, "ended_at", "") == "" {
		t.Fatal("closing the andon must end the linked downtime")
	}
	if stringValue(repo.records["MAND"]["AD-1"].Data, "status", "") != "CLOSED" {
		t.Fatal("andon call must be closed")
	}
}

func TestSMTMachineBindStartsChangeoverDowntime(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	repo.seed("MSWO", "WO-2", map[string]any{"WorkOrderCode": "WO-2", "product_code": "P1", "line_code": "L1", "plan_qty": 50, "Status": "PENDING"})
	result := runSMT(t, repo, "MMAC", "M01", "bind", map[string]any{"work_order": "WO-2"})
	if result.Status != "bound" {
		t.Fatalf("bind status = %s, effects %#v", result.Status, result.Effects)
	}
	if stringValue(repo.records["MMAC"]["M01"].Data, "current_work_order", "") != "WO-2" {
		t.Fatal("machine must bind the new work order")
	}
	found := false
	for _, record := range repo.records["MDWN"] {
		if stringValue(record.Data, "source", "") == "CHANGEOVER" {
			found = true
		}
	}
	if !found {
		t.Fatal("changeover must start a downtime record")
	}
}

func TestSMTReelReceiveGeneratesReels(t *testing.T) {
	repo := newBusinessFakeRepository()
	seedSMTBase(repo)
	result := runSMT(t, repo, "MREL", "RCV-1", "receive", map[string]any{
		"part_no": "C0402-104K", "qty": 3000, "reel_count": 3, "lot_no": "LOT-A",
	})
	if result.Status != "received" {
		t.Fatalf("receive status = %s, effects %#v", result.Status, result.Effects)
	}
	created := 0
	for key, record := range repo.records["MREL"] {
		if key == "RCV-1" {
			continue
		}
		if stringValue(record.Data, "lot_no", "") == "LOT-A" {
			created++
		}
	}
	if created != 3 {
		t.Fatalf("created reels = %d, want 3", created)
	}
}
