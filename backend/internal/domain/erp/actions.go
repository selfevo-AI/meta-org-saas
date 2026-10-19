package erp

import (
	"errors"
	"sort"
)

var errUnsupportedERPAction = errors.New("unsupported ERP action")

type ActionRegistry struct {
	actions map[string]ActionDefinition
}

func DefaultActionRegistry() ActionRegistry {
	defs := []ActionDefinition{
		{TableCode: "MREQ", Action: "analyze", Label: "Analyze requirement", Description: "Analyze requirement and prepare approval"},
		{TableCode: "MREQ", Action: "approve", Label: "Approve requirement", Description: "Approve requirement for conversion"},
		{TableCode: "MREQ", Action: "convert-to-project", Label: "Convert requirement to project", Description: "Create or link project from approved requirement", NextTables: []string{"MPRJ"}},
		{TableCode: "MPRJ", Action: "refresh-cost", Label: "Refresh project cost", Description: "Aggregate ERP cost effects into project cost records", NextTables: []string{"MCST"}},
		{TableCode: "MPRJ", Action: "close-feedback", Label: "Close feedback", Description: "Close project feedback loop", NextTables: []string{"MFDB"}},
		{TableCode: "MPOR", Action: "submit", Label: "Submit purchase order", Description: "Submit purchase order for approval"},
		{TableCode: "MPOR", Action: "approve", Label: "Approve purchase order", Description: "Approve purchase order"},
		{TableCode: "MPOR", Action: "receive", Label: "Create goods receipt", Description: "Create goods receipt from an approved purchase order", NextTables: []string{"MPDN"}},
		{TableCode: "MPDN", Action: "approve", Label: "Approve goods receipt", Description: "Approve goods receipt for posting"},
		{TableCode: "MPDN", Action: "post", Label: "Post goods receipt PO", Description: "Post goods receipt and inventory receipt", NextTables: []string{"MIGN", "MPCH"}},
		{TableCode: "MRDR", Action: "confirm", Label: "Confirm sales order", Description: "Confirm sales order"},
		{TableCode: "MRDR", Action: "approve", Label: "Approve sales order", Description: "Approve sales order"},
		{TableCode: "MRDR", Action: "deliver", Label: "Create delivery", Description: "Create delivery from an approved sales order", NextTables: []string{"MDLN"}},
		{TableCode: "MDLN", Action: "approve", Label: "Approve delivery", Description: "Approve delivery for posting"},
		{TableCode: "MDLN", Action: "post", Label: "Post delivery", Description: "Post delivery and inventory issue", NextTables: []string{"MIGE", "MINV"}},
		{TableCode: "MINV", Action: "post", Label: "Post A/R invoice", Description: "Post receivable invoice", NextTables: []string{"MJDT"}},
		{TableCode: "MPCH", Action: "post", Label: "Post A/P invoice", Description: "Post payable invoice", NextTables: []string{"MJDT"}},
		{TableCode: "MRCT", Action: "allocate", Label: "Allocate incoming payment", Description: "Allocate payment to receivable invoice"},
		{TableCode: "MVPM", Action: "allocate", Label: "Allocate outgoing payment", Description: "Allocate payment to payable invoice", NextTables: []string{"MJDT"}},
		{TableCode: "MIGN", Action: "post", Label: "Post goods receipt", Description: "Increase inventory balance"},
		{TableCode: "MIGE", Action: "post", Label: "Post goods issue", Description: "Decrease inventory balance"},
		{TableCode: "MJDT", Action: "post", Label: "Post journal entry", Description: "Post journal entry"},
		{TableCode: "MGLR", Action: "run", Label: "Run trial balance", Description: "Calculate posted journal entry debit and credit totals"},
		// SMT MES shop-floor actions (module "smt_mes").
		{TableCode: "MREL", Action: "receive", Label: "Receive reels", Description: "Receive reel stock: generate reel identities and stock-in transactions", NextTables: []string{"REL1"}},
		{TableCode: "MREL", Action: "issue", Label: "Issue reel", Description: "Issue a reel to a work order or requisition", NextTables: []string{"REL1"}},
		{TableCode: "MREL", Action: "return", Label: "Return reel", Description: "Return a reel to stock", NextTables: []string{"REL1"}},
		{TableCode: "MREL", Action: "empty", Label: "Reel empty", Description: "Mark a reel as consumed", NextTables: []string{"REL1"}},
		{TableCode: "MREL", Action: "scrap", Label: "Scrap reel", Description: "Scrap a reel", NextTables: []string{"REL1"}},
		{TableCode: "MREL", Action: "lost", Label: "Mark reel lost", Description: "Mark a reel as lost during stock count", NextTables: []string{"REL1"}},
		{TableCode: "MREL", Action: "return-to-customer", Label: "Return reel to customer", Description: "Return customer-supplied reel to the customer", NextTables: []string{"REL1"}},
		{TableCode: "MREL", Action: "adjust", Label: "Adjust reel quantity", Description: "Adjust reel quantity with a stock transaction", NextTables: []string{"REL1"}},
		{TableCode: "MLOD", Action: "load", Label: "Load reel", Description: "Feeder verification load: validate slot, reel, part and work order, then load the reel onto the slot", NextTables: []string{"MVER", "MREL"}},
		{TableCode: "MLOD", Action: "unload", Label: "Unload reel", Description: "Unload the loaded reel from the slot and clear machine verification", NextTables: []string{"MREL"}},
		{TableCode: "MMAC", Action: "bind", Label: "Bind work order", Description: "Bind a work order to the machine (changeover) and capture the changeover checklist", NextTables: []string{"MLOD"}},
		{TableCode: "MMAC", Action: "verify", Label: "Verify machine", Description: "Verify the whole machine feeder setup against the feeder setup table"},
		{TableCode: "MMAC", Action: "sign-off", Label: "Sign off machine", Description: "Second-person sign-off after a successful machine verification"},
		{TableCode: "MMAC", Action: "lock-slot", Label: "Lock slot", Description: "Lock a feeder slot (reject further loading until unlocked)"},
		{TableCode: "MMAC", Action: "unlock-slot", Label: "Unlock slot", Description: "Unlock a feeder slot"},
		{TableCode: "MSWO", Action: "start", Label: "Start work order", Description: "Start production for the work order (PENDING/FINISHED to RUNNING)"},
		{TableCode: "MSWO", Action: "progress", Label: "Report progress", Description: "Report produced quantity and estimate low-material alerts", NextTables: []string{"MLWA"}},
		{TableCode: "MSWO", Action: "finish", Label: "Finish work order", Description: "Finish production for the work order (RUNNING to FINISHED)"},
		{TableCode: "MSWO", Action: "close", Label: "Close work order", Description: "Close the work order"},
		{TableCode: "MRQN", Action: "issue", Label: "Issue reel to requisition", Description: "Scan-issue a reel against the requisition: validate part, alternative, quantity and ownership", NextTables: []string{"MREL"}},
		{TableCode: "MRQN", Action: "close", Label: "Close requisition", Description: "Close the requisition"},
		{TableCode: "MRQN", Action: "cancel", Label: "Cancel requisition", Description: "Cancel the requisition"},
		{TableCode: "MUNT", Action: "pass", Label: "Pass unit", Description: "Pass a unit through a routing step with foolproofing (skip, repeat, FAI, machine readiness)", NextTables: []string{"UNT1"}},
		{TableCode: "MUNT", Action: "fail", Label: "Fail unit", Description: "Judge a unit as defective with defect code and reference designator", NextTables: []string{"MUDF", "UNT1"}},
		{TableCode: "MUNT", Action: "repair", Label: "Complete repair", Description: "Complete repair of a defective unit (repaired, misjudge or scrap)", NextTables: []string{"MUDF", "UNT1"}},
		{TableCode: "MUNT", Action: "scrap", Label: "Scrap unit", Description: "Scrap a unit", NextTables: []string{"UNT1"}},
		{TableCode: "MPNL", Action: "bind", Label: "Bind panel", Description: "Bind a panel code to its board serial numbers (all or nothing pass)", NextTables: []string{"PNL1"}},
		{TableCode: "MPNL", Action: "split", Label: "Split panel", Description: "Split the panel so boards pass individually"},
		{TableCode: "MINS", Action: "record", Label: "Record inspection", Description: "Record a first-article (FAI) or in-process (IPQC) inspection with machine snapshots", NextTables: []string{"INS1"}},
		{TableCode: "MAND", Action: "raise", Label: "Raise andon call", Description: "Raise an andon abnormality call"},
		{TableCode: "MAND", Action: "ack", Label: "Acknowledge andon", Description: "Acknowledge an andon call"},
		{TableCode: "MAND", Action: "close", Label: "Close andon", Description: "Close an andon call with cause and solution", NextTables: []string{"MDWN"}},
		{TableCode: "MDWN", Action: "start", Label: "Start downtime", Description: "Start a downtime record for a machine or line"},
		{TableCode: "MDWN", Action: "end", Label: "End downtime", Description: "End an open downtime record"},
	}
	registry := ActionRegistry{actions: map[string]ActionDefinition{}}
	for _, def := range defs {
		registry.actions[actionKey(def.TableCode, def.Action)] = def
	}
	return registry
}

func (r ActionRegistry) Lookup(tableCode string, action string) (ActionDefinition, bool) {
	def, ok := r.actions[actionKey(tableCode, action)]
	return def, ok
}

func (r ActionRegistry) List() []ActionDefinition {
	result := make([]ActionDefinition, 0, len(r.actions))
	for _, def := range r.actions {
		result = append(result, def)
	}
	sort.Slice(result, func(i, j int) bool {
		return actionKey(result[i].TableCode, result[i].Action) < actionKey(result[j].TableCode, result[j].Action)
	})
	return result
}

func actionKey(tableCode string, action string) string {
	return tableCode + ":" + action
}
