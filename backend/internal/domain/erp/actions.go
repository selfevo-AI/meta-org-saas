package erp

import "errors"

var errUnsupportedERPAction = errors.New("unsupported ERP action")

type ActionRegistry struct {
	actions map[string]ActionDefinition
}

func DefaultActionRegistry() ActionRegistry {
	defs := []ActionDefinition{
		{TableCode: "MREQ", Action: "analyze", Label: "Analyze requirement", Description: "Analyze requirement and prepare approval"},
		{TableCode: "MREQ", Action: "approve", Label: "Approve requirement", Description: "Approve requirement for conversion"},
		{TableCode: "MREQ", Action: "convert-to-project", Label: "Convert requirement to project", Description: "Create or link project from approved requirement", NextTables: []string{"MPRJ"}},
		{TableCode: "MPRJ", Action: "add-member", Label: "Add project member", Description: "Append project member row"},
		{TableCode: "MPRJ", Action: "add-deliverable", Label: "Add deliverable", Description: "Create delivery document for project", NextTables: []string{"MDLN"}},
		{TableCode: "MPRJ", Action: "refresh-cost", Label: "Refresh project cost", Description: "Aggregate ERP cost effects into project cost records", NextTables: []string{"MCST"}},
		{TableCode: "MPRJ", Action: "close-feedback", Label: "Close feedback", Description: "Close project feedback loop", NextTables: []string{"MFDB"}},
		{TableCode: "MPOR", Action: "submit", Label: "Submit purchase order", Description: "Submit purchase order for approval"},
		{TableCode: "MPOR", Action: "approve", Label: "Approve purchase order", Description: "Approve purchase order"},
		{TableCode: "MPDN", Action: "post", Label: "Post goods receipt PO", Description: "Post goods receipt and inventory receipt", NextTables: []string{"MIGN", "MPCH"}},
		{TableCode: "MRDR", Action: "confirm", Label: "Confirm sales order", Description: "Confirm sales order"},
		{TableCode: "MRDR", Action: "approve", Label: "Approve sales order", Description: "Approve sales order"},
		{TableCode: "MDLN", Action: "post", Label: "Post delivery", Description: "Post delivery and inventory issue", NextTables: []string{"MIGE", "MINV"}},
		{TableCode: "MINV", Action: "post", Label: "Post A/R invoice", Description: "Post receivable invoice", NextTables: []string{"MJDT"}},
		{TableCode: "MRCT", Action: "allocate", Label: "Allocate incoming payment", Description: "Allocate payment to receivable invoice"},
		{TableCode: "MIGN", Action: "post", Label: "Post goods receipt", Description: "Increase inventory balance"},
		{TableCode: "MIGE", Action: "post", Label: "Post goods issue", Description: "Decrease inventory balance"},
		{TableCode: "MJDT", Action: "post", Label: "Post journal entry", Description: "Post journal entry"},
		{TableCode: "MGLR", Action: "run", Label: "Run trial balance", Description: "Calculate posted journal entry debit and credit totals"},
		{TableCode: "MRPS", Action: "close", Label: "Close POS sale", Description: "Close POS sale and generate cash, invoice, and inventory issue", NextTables: []string{"MIGE", "MINV", "MRCT"}},
		{TableCode: "MDRQ", Action: "submit", Label: "Submit distribution request", Description: "Submit store distribution request for approval"},
		{TableCode: "MDRQ", Action: "approve", Label: "Approve distribution request", Description: "Approve store distribution request"},
		{TableCode: "MDRQ", Action: "auto-allocate", Label: "Auto allocate distribution request", Description: "Create HQ distribution shipment from approved request", NextTables: []string{"MDSP"}},
		{TableCode: "MDSP", Action: "ship", Label: "Ship distribution", Description: "Post HQ distribution shipment and decrease source inventory", NextTables: []string{"MIGE"}},
		{TableCode: "MDRC", Action: "receive", Label: "Receive distribution", Description: "Post store distribution receipt and increase destination inventory", NextTables: []string{"MIGN", "MDIF"}},
		{TableCode: "MDIF", Action: "resolve", Label: "Resolve distribution difference", Description: "Resolve distribution difference with inventory adjustment", NextTables: []string{"MIGN", "MIGE"}},
		{TableCode: "MSTP", Action: "replenish", Label: "Generate replenishment request", Description: "Generate distribution request from stock policy", NextTables: []string{"MDRQ"}},
		{TableCode: "MCNT", Action: "submit", Label: "Submit store count", Description: "Submit store inventory count for HQ approval"},
		{TableCode: "MCNT", Action: "approve", Label: "Approve store count", Description: "Approve store inventory count"},
		{TableCode: "MCNT", Action: "post-adjustment", Label: "Post count adjustment", Description: "Post store count differences to inventory", NextTables: []string{"MIGN", "MIGE"}},
		{TableCode: "MSPR", Action: "submit", Label: "Submit special purchase request", Description: "Submit branch special purchase request"},
		{TableCode: "MSPR", Action: "approve", Label: "Approve special purchase request", Description: "Approve branch special purchase request"},
		{TableCode: "MSPR", Action: "convert-to-purchase-order", Label: "Convert to purchase order", Description: "Create purchase order from approved branch special purchase request", NextTables: []string{"MPOR"}},
		{TableCode: "MPUB", Action: "publish", Label: "Publish items", Description: "Publish new items and promotion visibility to stores"},
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
	return result
}

func actionKey(tableCode string, action string) string {
	return tableCode + ":" + action
}
