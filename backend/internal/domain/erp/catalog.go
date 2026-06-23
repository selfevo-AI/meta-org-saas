package erp

func DefaultCatalog() Catalog {
	tables := []TableDefinition{
		table("MACT", "G/L Accounts", "finance", "AcctCode", []FieldDefinition{
			field("AcctCode", "nVarChar", "15", "Account Code", true),
			field("Finanse", "VarChar", "1", "Cash Account", false),
			field("Budget", "VarChar", "1", "Budget", false),
			field("Frozen", "VarChar", "1", "Account on Hold [Y/N]", false),
			field("Postable", "VarChar", "1", "Account [Active/Title]", false),
			field("ActType", "VarChar", "1", "Account Type", false),
			field("DataSource", "VarChar", "1", "Data Source", false),
			field("ValidFor", "VarChar", "1", "Active", false),
			field("FrozenFor", "VarChar", "1", "Inactive", false),
			field("FatherNum", "nVarChar", "15", "Parent Account", false),
			field("ActCurr", "nVarChar", "3", "Currency", false),
			field("FormatCode", "nVarChar", "20", "Format Code", false),
			field("ActId", "nVarChar", "15", "Account Identifier", false),
		}, child("AACT", "G/L Account - History", "AcctCode")),
		table("MBTD", "Journal Vouchers List", "finance", "BatchNum", []FieldDefinition{
			field("BatchNum", "Int", "11", "Batch Number", true),
			field("Status", "VarChar", "1", "Open/Closed Document", false),
		}),
		table("MBTF", "Journal Voucher Entry", "finance", "BatchNum", journalFields("BatchNum"), child("BTF1", "Journal Voucher - Rows", "BatchNum")),
		table("MJDT", "Journal Entry", "finance", "TransId", journalFields("TransId"), child("JDT1", "Journal Entry - Rows", "TransId")),
		table("MPRC", "Cost Center", "finance", "PrcCode", []FieldDefinition{
			field("PrcCode", "nVarChar", "8", "Cost Center Code", true),
			field("Locked", "VarChar", "1", "Locked", false),
			field("DataSource", "VarChar", "1", "Data Source", false),
			field("Active", "VarChar", "1", "Active", false),
		}, child("APRC", "Cost Center", "PrcCode")),
		table("MCRD", "Business Partners", "partner", "CardCode", append([]FieldDefinition{
			field("CardCode", "nVarChar", "15", "Business Partner Code", true),
			field("CardName", "nVarChar", "100", "Business Partner Name", false),
			field("CardType", "VarChar", "1", "BP Type", false),
			field("GroupCode", "Int", "11", "Group Code", false),
			field("Currency", "nVarChar", "3", "Currency", false),
			field("Phone1", "nVarChar", "20", "Telephone 1", false),
			field("E_Mail", "nVarChar", "100", "E-Mail", false),
			field("ValidFor", "VarChar", "1", "Active", false),
			field("FrozenFor", "VarChar", "1", "Inactive", false),
		}, flagFields("QryGroup", 64)...), child("CRD1", "Business Partners - Addresses", "CardCode")),
		table("MITM", "Items", "product", "ItemCode", []FieldDefinition{
			field("ItemCode", "nVarChar", "50", "Item Code", true),
			field("ItemName", "nVarChar", "100", "Item Name", false),
			field("FrgnName", "nVarChar", "100", "Foreign Name", false),
			field("ItmsGrpCod", "Int", "11", "Item Group", false),
			field("InvntItem", "VarChar", "1", "Inventory Item", false),
			field("SellItem", "VarChar", "1", "Sales Item", false),
			field("PrchseItem", "VarChar", "1", "Purchase Item", false),
			field("BuyUnitMsr", "nVarChar", "100", "Purchasing UoM", false),
			field("SalUnitMsr", "nVarChar", "100", "Sales UoM", false),
			field("InvntryUom", "nVarChar", "100", "Inventory UoM", false),
			field("validFor", "VarChar", "1", "Active", false),
			field("frozenFor", "VarChar", "1", "Inactive", false),
		}, child("ITM1", "Items - Prices", "ItemCode")),
		table("MITW", "Items - Warehouse", "product", "ItemCode", []FieldDefinition{
			field("ItemCode", "nVarChar", "50", "Item Code", true),
			field("WhsCode", "nVarChar", "8", "Warehouse Code", false),
			field("WasCounted", "VarChar", "1", "Counted Yes/No", false),
			field("Locked", "VarChar", "1", "Locked", false),
			field("DftBinEnfd", "VarChar", "1", "Default Bin Enforced [Y/N]", false),
			field("Freezed", "VarChar", "1", "Item Frozen in Warehouse", false),
			field("IndEscala", "VarChar", "1", "Indicator for Relevant Scale", false),
		}, child("ITW1", "Item Count Alert", "ItemCode")),
		table("MPRJ", "Project Codes", "product", "PrjCode", []FieldDefinition{
			field("PrjCode", "nVarChar", "20", "Project Code", true),
			field("Locked", "VarChar", "1", "Locked", false),
			field("DataSource", "VarChar", "1", "Data Source", false),
			field("Active", "VarChar", "1", "Active", false),
		}, child("APRJ", "Project Codes", "PrjCode")),
		documentTable("MDLN", "Delivery", "sale", "DLN1"),
		documentTable("MDPS", "Deposit", "sale", "DPS1"),
		documentTable("MINV", "A/R Invoice", "sale", "INV1"),
		documentTable("MQUT", "Sales Quotation", "sale", "QUT1"),
		documentTable("MRCT", "Incoming Payments", "sale", "RCT1"),
		documentTable("MRDN", "Returns", "sale", "RDN1"),
		documentTable("MRDR", "Sales Order", "sale", "RDR1"),
		documentTable("MRIN", "A/R Credit Memo", "sale", "RIN1"),
		documentTable("MPCH", "A/P Invoice", "purchase", "PCH1"),
		documentTable("MPDN", "Goods Receipt PO", "purchase", "PDN1"),
		documentTable("MPOR", "Purchase Order", "purchase", "POR1"),
		documentTable("MRPC", "A/P Credit Memo", "purchase", "RPC1"),
		documentTable("MRPD", "Goods Return", "purchase", "RPD1"),
		documentTable("MIGE", "Goods Issue", "warehouse", "IGE1"),
		documentTable("MIGN", "Goods Receipt", "warehouse", "IGN1"),
		table("MWHS", "Warehouses", "warehouse", "WhsCode", []FieldDefinition{
			field("WhsCode", "nVarChar", "8", "Warehouse Code", true),
			field("WhsName", "nVarChar", "100", "Warehouse Name", false),
			field("Locked", "VarChar", "1", "Locked", false),
			field("DataSource", "VarChar", "1", "Data Source", false),
			field("Inactive", "VarChar", "1", "Inactive", false),
			field("DropShip", "VarChar", "1", "Drop Ship", false),
			field("Nettable", "VarChar", "1", "Nettable", false),
		}, child("AWHS", "Warehouses - History", "WhsCode")),
		table("MUSR", "Users", "user", "USERID", []FieldDefinition{
			field("USERID", "Int", "11", "User ID", true),
			field("USER_CODE", "nVarChar", "25", "User Code", false),
			field("USER_NAME", "nVarChar", "155", "User Name", false),
			field("PASSWORD", "nVarChar", "254", "Password", false),
			field("E_Mail", "nVarChar", "100", "E-Mail", false),
			field("GROUPS", "Int", "6", "User Group", false),
			field("SUPERUSER", "VarChar", "1", "Superuser", false),
			field("Locked", "VarChar", "1", "User Locked", false),
			field("validFor", "VarChar", "1", "Active", false),
			field("frozenFor", "VarChar", "1", "Inactive", false),
		}, child("AUSR", "Users - History", "USERID")),
		extensionTable("MREQ", "Requirements", "project", "ReqCode", "REQ1", "Requirement Rows"),
		extensionTable("MCST", "Cost Records", "finance", "CostCode", "CST1", "Cost Rows"),
		extensionTable("MFDB", "Feedback Records", "project", "FeedbackCode", "FDB1", "Feedback Rows"),
		extensionTable("MORG", "Organizations", "platform", "OrgCode", "AORG", "Organizations - History"),
		extensionTable("MDEP", "Departments", "platform", "DeptCode", "ADEP", "Departments - History"),
		extensionTable("MPOS", "Positions", "platform", "PosCode", "APOS", "Positions - History"),
		extensionTable("MROL", "Roles", "platform", "RoleCode", "AROL", "Role Permissions"),
		extensionTable("MSAS", "SaaS Modules", "platform", "ModuleCode", "ASAS", "SaaS Module Details"),
		extensionTable("MWFL", "Workflows", "platform", "WorkflowCode", "WFL1", "Workflow Rows"),
		extensionTable("MGOV", "Governance", "platform", "GovCode", "GOV1", "Governance Rows"),
		extensionTable("MAIG", "AI Gateway", "platform", "AIGCode", "AIG1", "AI Gateway Rows"),
		extensionTable("MAST", "Assistant", "platform", "AssistantCode", "AST1", "Assistant Rows"),
		extensionTable("MTOL", "Tool Runtime", "platform", "ToolCode", "TOL1", "Tool Runtime Rows"),
		extensionTable("MOBS", "Observability", "platform", "ObsCode", "OBS1", "Observability Rows"),
		extensionTable("MRTM", "Runtime Configuration", "platform", "RuntimeCode", "RTM1", "Runtime Configuration Rows"),
	}
	catalog := buildCatalog(tables)
	catalog.Modules = defaultModules()
	return catalog
}

func buildCatalog(tables []TableDefinition) Catalog {
	byCode := map[string]TableDefinition{}
	for i := range tables {
		if _, ok := findField(tables[i].Fields, "Payload"); !ok {
			tables[i].Fields = append(tables[i].Fields, field("Payload", "JSONB", "", "Structured extension payload", false))
		}
		tables[i].fieldLookup = map[string]FieldDefinition{}
		for _, field := range tables[i].Fields {
			tables[i].fieldLookup[field.Name] = field
		}
		tables[i].childLookup = map[string]ChildTableDefinition{}
		for j := range tables[i].Children {
			tables[i].Children[j].fieldLookup = map[string]FieldDefinition{}
			for _, field := range tables[i].Children[j].Fields {
				tables[i].Children[j].fieldLookup[field.Name] = field
			}
			tables[i].childLookup[tables[i].Children[j].Code] = tables[i].Children[j]
		}
		byCode[tables[i].Code] = tables[i]
	}
	return Catalog{Tables: tables, byCode: byCode}
}

func findField(fields []FieldDefinition, name string) (FieldDefinition, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field, true
		}
	}
	return FieldDefinition{}, false
}

func defaultModules() []ModuleDefinition {
	return []ModuleDefinition{
		moduleDef("project", "Project",
			submoduleDef("requirements", "Requirements", docDef("requirement", "Requirement", "MREQ", []string{"REQ1"}, []string{"analyze", "approve", "convert-to-project"})),
			submoduleDef("projects", "Projects", docDef("project", "Project", "MPRJ", []string{"APRJ"}, []string{"add-member", "add-deliverable", "refresh-cost", "close-feedback"})),
			submoduleDef("deliverables", "Deliverables", docDef("delivery", "Delivery", "MDLN", []string{"DLN1"}, []string{"post"})),
			submoduleDef("cost", "Cost", docDef("cost_record", "Cost Record", "MCST", []string{"CST1"}, nil)),
			submoduleDef("feedback", "Feedback", docDef("feedback", "Feedback", "MFDB", []string{"FDB1"}, nil)),
		),
		moduleDef("master_data", "Master Data",
			submoduleDef("business_partners", "Business Partners", docDef("business_partner", "Business Partner", "MCRD", []string{"CRD1"}, nil)),
			submoduleDef("items", "Items", docDef("item", "Item", "MITM", []string{"ITM1"}, nil)),
			submoduleDef("warehouses", "Warehouses", docDef("warehouse", "Warehouse", "MWHS", []string{"AWHS"}, nil)),
		),
		moduleDef("purchasing", "Purchasing",
			submoduleDef("purchase_orders", "Purchase Orders", docDef("purchase_order", "Purchase Order", "MPOR", []string{"POR1"}, []string{"submit", "approve"})),
			submoduleDef("goods_receipt_po", "Goods Receipt PO", docDef("goods_receipt_po", "Goods Receipt PO", "MPDN", []string{"PDN1"}, []string{"post"})),
			submoduleDef("ap_invoices", "A/P Invoices", docDef("ap_invoice", "A/P Invoice", "MPCH", []string{"PCH1"}, nil)),
			submoduleDef("goods_returns", "Goods Returns", docDef("goods_return", "Goods Return", "MRPD", []string{"RPD1"}, nil)),
			submoduleDef("ap_credit_memos", "A/P Credit Memos", docDef("ap_credit_memo", "A/P Credit Memo", "MRPC", []string{"RPC1"}, nil)),
		),
		moduleDef("sales", "Sales",
			submoduleDef("quotations", "Quotations", docDef("sales_quotation", "Sales Quotation", "MQUT", []string{"QUT1"}, nil)),
			submoduleDef("sales_orders", "Sales Orders", docDef("sales_order", "Sales Order", "MRDR", []string{"RDR1"}, []string{"confirm", "approve"})),
			submoduleDef("deliveries", "Deliveries", docDef("delivery", "Delivery", "MDLN", []string{"DLN1"}, []string{"post"})),
			submoduleDef("ar_invoices", "A/R Invoices", docDef("ar_invoice", "A/R Invoice", "MINV", []string{"INV1"}, []string{"post"})),
			submoduleDef("returns", "Returns", docDef("return", "Return", "MRDN", []string{"RDN1"}, nil)),
			submoduleDef("incoming_payments", "Incoming Payments", docDef("incoming_payment", "Incoming Payment", "MRCT", []string{"RCT1"}, []string{"allocate"})),
		),
		moduleDef("inventory", "Inventory",
			submoduleDef("warehouse_balances", "Warehouse Balances", docDef("item_warehouse", "Item Warehouse", "MITW", []string{"ITW1"}, nil)),
			submoduleDef("goods_receipts", "Goods Receipts", docDef("goods_receipt", "Goods Receipt", "MIGN", []string{"IGN1"}, []string{"post"})),
			submoduleDef("goods_issues", "Goods Issues", docDef("goods_issue", "Goods Issue", "MIGE", []string{"IGE1"}, []string{"post"})),
			submoduleDef("inventory_adjustments", "Inventory Adjustments", docDef("inventory_adjustment", "Inventory Adjustment", "MIGE", []string{"IGE1"}, []string{"post"})),
		),
		moduleDef("finance", "Finance",
			submoduleDef("chart_of_accounts", "Chart of Accounts", docDef("gl_account", "G/L Account", "MACT", []string{"AACT"}, nil)),
			submoduleDef("cost_centers", "Cost Centers", docDef("cost_center", "Cost Center", "MPRC", []string{"APRC"}, nil)),
			submoduleDef("journal_entries", "Journal Entries", docDef("journal_entry", "Journal Entry", "MJDT", []string{"JDT1"}, []string{"post"})),
			submoduleDef("journal_vouchers", "Journal Vouchers", docDef("journal_voucher", "Journal Voucher", "MBTF", []string{"BTF1"}, nil)),
		),
		moduleDef("platform", "Platform",
			submoduleDef("users_permissions", "Users and Permissions", docDef("user", "User", "MUSR", []string{"AUSR"}, nil)),
			submoduleDef("saas_industry", "SaaS and Industry Solution Management", docDef("saas_module", "SaaS Module", "MSAS", []string{"ASAS"}, nil)),
		),
	}
}

func moduleDef(key, name string, submodules ...SubmoduleDefinition) ModuleDefinition {
	return ModuleDefinition{Key: key, Name: name, Submodules: submodules}
}

func submoduleDef(key, name string, documents ...BusinessDocumentType) SubmoduleDefinition {
	return SubmoduleDefinition{Key: key, Name: name, Documents: documents}
}

func docDef(key, name, tableCode string, childCodes []string, actionNames []string) BusinessDocumentType {
	return BusinessDocumentType{Key: key, Name: name, TableCode: tableCode, ChildCodes: childCodes, ActionNames: actionNames}
}

func table(code, name, module, primaryKey string, fields []FieldDefinition, children ...ChildTableDefinition) TableDefinition {
	return TableDefinition{Code: code, Name: name, Module: module, PrimaryKey: primaryKey, Fields: fields, Children: children}
}

func child(code, name, parentKey string) ChildTableDefinition {
	return ChildTableDefinition{
		Code:      code,
		Name:      name,
		ParentKey: parentKey,
		LineKey:   "LineNum",
		Fields: []FieldDefinition{
			field(parentKey, "nVarChar", "64", "Parent Key", true),
			field("LineNum", "Int", "11", "Line Number", true),
			field("LineStatus", "VarChar", "1", "Line Status", false),
			field("Payload", "JSONB", "", "Structured row payload for fields not expanded in tables_structure.md", false),
		},
	}
}

func field(name, dataType, size, description string, primaryKey bool) FieldDefinition {
	return FieldDefinition{Name: name, DataType: dataType, Size: size, Description: description, PrimaryKey: primaryKey}
}

func journalFields(primaryKey string) []FieldDefinition {
	return []FieldDefinition{
		field(primaryKey, "Int", "11", "Primary Key", true),
		field("BtfStatus", "VarChar", "1", "Status", false),
		field("TransType", "nVarChar", "20", "Origin", false),
		field("PCAddition", "VarChar", "1", "PC Addition", false),
		field("DataSource", "VarChar", "1", "Data Source", false),
		field("RefndRprt", "VarChar", "1", "Repayment Report", false),
		field("AutoStorno", "VarChar", "1", "Use Auto-Reverse", false),
		field("AutoVAT", "VarChar", "1", "Automatic Tax", false),
		field("Printed", "VarChar", "1", "Printed", false),
		field("Project", "nVarChar", "20", "Project", false),
		field("RefDate", "Date", "", "Reference Date", false),
	}
}

func documentTable(code, name, module, childCode string) TableDefinition {
	return table(code, name, module, "DocEntry", append([]FieldDefinition{
		field("DocEntry", "Int", "11", "Document Entry", true),
		field("DocNum", "Int", "11", "Document Number", false),
		field("DocType", "VarChar", "1", "Document Type", false),
		field("CANCELED", "VarChar", "1", "Canceled", false),
		field("Handwrtten", "VarChar", "1", "Manual Numbering", false),
		field("Printed", "VarChar", "1", "Printed", false),
		field("DocStatus", "VarChar", "1", "Document Status", false),
		field("InvntSttus", "VarChar", "1", "Warehouse Status", false),
		field("Transfered", "VarChar", "1", "Year Transfer", false),
		field("ObjType", "nVarChar", "20", "Object Type", false),
		field("CardCode", "nVarChar", "15", "Business Partner Code", false),
		field("CardName", "nVarChar", "100", "Business Partner Name", false),
		field("DocDate", "Date", "", "Document Date", false),
		field("DocDueDate", "Date", "", "Document Due Date", false),
		field("TaxDate", "Date", "", "Tax Date", false),
		field("DocCur", "nVarChar", "3", "Document Currency", false),
		field("DocRate", "Numeric", "19,6", "Document Rate", false),
		field("DocTotal", "Numeric", "19,6", "Document Total", false),
		field("Comments", "nVarChar", "254", "Comments", false),
		field("DataSource", "VarChar", "1", "Data Source", false),
		field("WddStatus", "VarChar", "1", "Authorization Status", false),
		field("Project", "nVarChar", "20", "Project", false),
	}, documentFlagFields()...), child(childCode, name+" - Rows", "DocEntry"))
}

func extensionTable(code, name, module, primaryKey, childCode, childName string) TableDefinition {
	return table(code, name, module, primaryKey, []FieldDefinition{
		field(primaryKey, "nVarChar", "64", "Primary Key", true),
		field("Name", "nVarChar", "155", "Name", false),
		field("Status", "VarChar", "20", "Status", false),
		field("Payload", "JSONB", "", "Structured payload for project/platform fields outside tables_structure.md", false),
	}, child(childCode, childName, primaryKey))
}

func documentFlagFields() []FieldDefinition {
	names := []string{
		"PartSupply", "Confirmed", "CreateTran", "SummryType", "UpdInvnt", "UpdCardBal", "InvntDirec",
		"ShowSCN", "CurSource", "FatherType", "Exported", "submitted", "PoPrss", "Rounding",
		"RevisionPo", "PickStatus", "PayBlock", "Reserve", "UseShpdGd", "DocSubType", "DpmStatus",
		"Posted", "BillToOW", "ShipToOW", "RetInvoice", "OpenForLaC", "Excised", "DutyStatus",
		"AutoCrtFlw", "ResidenNum", "DocManClsd", "Ordered", "EDocStatus", "EDocCancel", "GTSRlvnt",
		"ReqType", "OriginType", "IsAlt", "AltBaseTyp", "RelatedTyp", "PoDropPrss", "ExclTaxRep",
		"Revision", "BaseType", "ComTrade",
	}
	fields := make([]FieldDefinition, 0, len(names))
	for _, name := range names {
		fields = append(fields, field(name, "VarChar", "1", name, false))
	}
	return fields
}

func flagFields(prefix string, count int) []FieldDefinition {
	fields := make([]FieldDefinition, 0, count)
	for i := 1; i <= count; i++ {
		fields = append(fields, field(prefix+itoa(i), "VarChar", "1", prefix+itoa(i), false))
	}
	return fields
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
