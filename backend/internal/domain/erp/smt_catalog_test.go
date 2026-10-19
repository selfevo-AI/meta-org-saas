package erp

import (
	"strings"
	"testing"
)

var smtMESTableCodes = []string{
	"MLIN", "LIN1", "MMAC", "MAC1", "MFSB", "FSB1", "MOPR", "MROU", "ROU1",
	"MPRD", "PRD1", "MCPM", "MREL", "REL1", "MSWO", "SWO1", "MRQN", "RQN1",
	"MLOD", "LOD1", "MVER", "MLWA", "MUNT", "UNT1", "MPNL", "PNL1",
	"MDFC", "MUDF", "MINS", "INS1", "MAND", "AND1", "MDWN", "MSET",
}

func TestSMTMESCatalogRegistersAllShopFloorTables(t *testing.T) {
	catalog := DefaultCatalog()
	for _, code := range smtMESTableCodes {
		table, ok := catalog.Table(code)
		if !ok {
			t.Fatalf("catalog missing SMT MES table %s", code)
		}
		if ModuleForTable(table) != "smt_mes" {
			t.Fatalf("table %s module = %s, want smt_mes", code, ModuleForTable(table))
		}
	}
	// The SMT module must be exposed with its documents bound to the tables.
	foundModule := false
	for _, module := range catalog.Modules {
		if module.Key != "smt_mes" {
			continue
		}
		foundModule = true
		if len(module.Submodules) == 0 {
			t.Fatal("smt_mes module has no submodules")
		}
	}
	if !foundModule {
		t.Fatal("catalog missing smt_mes module definition")
	}
}

func TestSMTMESActionsAreRegisteredForExistingTables(t *testing.T) {
	catalog := DefaultCatalog()
	registry := DefaultActionRegistry()
	smtActionTables := []string{"MREL", "MLOD", "MMAC", "MSWO", "MRQN", "MUNT", "MPNL", "MINS", "MAND", "MDWN"}
	seen := map[string]bool{}
	for _, action := range registry.List() {
		if !containsString(smtActionTables, action.TableCode) {
			continue
		}
		seen[action.TableCode+":"+action.Action] = true
		if _, ok := catalog.Table(action.TableCode); !ok {
			t.Fatalf("action %s refers to missing table %s", action.Action, action.TableCode)
		}
	}
	for _, expected := range []string{
		"MREL:receive", "MREL:issue", "MLOD:load", "MLOD:unload",
		"MMAC:bind", "MMAC:verify", "MMAC:sign-off", "MMAC:lock-slot", "MMAC:unlock-slot",
		"MSWO:start", "MSWO:progress", "MSWO:finish", "MSWO:close",
		"MRQN:issue", "MRQN:close", "MRQN:cancel",
		"MUNT:pass", "MUNT:fail", "MUNT:repair", "MUNT:scrap",
		"MPNL:bind", "MPNL:split", "MINS:record",
		"MAND:raise", "MAND:ack", "MAND:close", "MDWN:start", "MDWN:end",
	} {
		if !seen[expected] {
			t.Fatalf("registry missing SMT MES action %s", expected)
		}
	}
}

func TestSMTMESMutationPolicyCoversShopFloorTables(t *testing.T) {
	catalog := DefaultCatalog()
	registry := DefaultActionRegistry()

	// Action-only tables stay writable exclusively through registered actions.
	for code := range smtActionOnlyTables {
		table, ok := catalog.Table(code)
		if !ok {
			t.Fatalf("action-only table %s missing from catalog", code)
		}
		if !smtActionOnlyTable(table) {
			t.Fatalf("action-only table %s is not covered by policy", code)
		}
	}

	// Every executor-allowed action must be registered, and governance actions
	// must stay reviewer-gated (not listed as executor actions).
	for key := range smtExecutorActions {
		tableCode, action, _ := strings.Cut(key, ":")
		if _, ok := registry.Lookup(tableCode, action); !ok {
			t.Fatalf("executor action %s is not registered", key)
		}
	}
	for _, governance := range []string{
		"MMAC:sign-off", "MMAC:unlock-slot", "MREL:scrap", "MREL:lost", "MREL:adjust",
		"MREL:return-to-customer", "MUNT:scrap", "MSWO:close", "MRQN:cancel",
	} {
		if smtExecutorActions[governance] {
			t.Fatalf("governance action %s must require reviewer authority", governance)
		}
		tableCode, action, _ := strings.Cut(governance, ":")
		if _, ok := registry.Lookup(tableCode, action); !ok {
			t.Fatalf("governance action %s is not registered", governance)
		}
	}
}
