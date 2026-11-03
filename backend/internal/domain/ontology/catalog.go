package ontology

import "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"

func property(key, field, kind, zh, en string) Property {
	return Property{Key: key, SourceField: field, DataType: kind, Label: Label{ZH: zh, EN: en}}
}

func defaultTypes(catalog erp.Catalog, actions []erp.ActionDefinition) []ObjectType {
	specs := []struct{ key, table, zh, en string }{
		{"business_partner", "MCRD", "业务伙伴", "Business Partner"},
		{"item", "MITM", "物料", "Item"},
		{"warehouse", "MWHS", "仓库", "Warehouse"},
		{"stock_balance", "MITW", "库存余额", "Stock Balance"},
		{"purchase_order", "MPOR", "采购订单", "Purchase Order"},
		{"goods_receipt", "MPDN", "采购收货", "Goods Receipt"},
		{"payable_invoice", "MPCH", "应付发票", "Payable Invoice"},
		{"outgoing_payment", "MVPM", "付款单", "Outgoing Payment"},
		{"sales_order", "MRDR", "销售订单", "Sales Order"},
		{"delivery", "MDLN", "销售交货", "Delivery"},
		{"receivable_invoice", "MINV", "应收发票", "Receivable Invoice"},
		{"incoming_payment", "MRCT", "收款单", "Incoming Payment"},
		{"inventory_receipt", "MIGN", "库存入库", "Inventory Receipt"},
		{"inventory_issue", "MIGE", "库存出库", "Inventory Issue"},
		{"account", "MACT", "会计科目", "Account"},
		{"journal_entry", "MJDT", "会计分录", "Journal Entry"},
		{"cost_center", "MPRC", "成本中心", "Cost Center"},
		{"project", "MPRJ", "项目", "Project"},
		{"requirement", "MREQ", "需求", "Requirement"},
		{"smt_line", "MLIN", "SMT 产线", "SMT Line"},
		{"smt_machine", "MMAC", "SMT 机台", "SMT Machine"},
		{"smt_feeder_setup", "MFSB", "料站表", "Feeder Setup"},
		{"smt_operation", "MOPR", "工序", "Operation"},
		{"smt_routing", "MROU", "工艺路线", "Routing"},
		{"smt_product", "MPRD", "SMT 产品", "SMT Product"},
		{"smt_reel", "MREL", "料盘", "Reel"},
		{"smt_work_order", "MSWO", "SMT 工单", "SMT Work Order"},
		{"smt_requisition", "MRQN", "领料单", "Requisition"},
		{"smt_loading_record", "MLOD", "上料记录", "Loading Record"},
		{"smt_verification_scan", "MVER", "防错扫描", "Verification Scan"},
		{"smt_unit", "MUNT", "在制品", "Production Unit"},
		{"smt_panel", "MPNL", "拼板", "Panel"},
		{"smt_unit_defect", "MUDF", "不良维修", "Unit Defect"},
		{"smt_inspection", "MINS", "首件/巡检", "Inspection"},
		{"smt_andon_call", "MAND", "Andon 呼叫", "Andon Call"},
		{"smt_downtime", "MDWN", "停机记录", "Downtime"},
	}
	result := make([]ObjectType, 0, len(specs))
	for _, spec := range specs {
		table, ok := catalog.Table(spec.table)
		if !ok {
			continue
		}
		typ := ObjectType{Key: spec.key, TableCode: spec.table, Label: Label{ZH: spec.zh, EN: spec.en}, Module: erp.ModuleForTable(table), PrimaryKey: table.PrimaryKey, Links: []LinkType{}, Actions: []Action{}}
		typ.Properties = []Property{property("key", table.PrimaryKey, "string", "编号", "Key")}
		if table.PrimaryKey == "DocEntry" {
			typ.Importable = true
			typ.Properties = append(typ.Properties,
				property("external_number", "NumAtCard", "string", "外部单号", "External Reference"),
				property("note", "Comments", "string", "备注", "Notes"),
				property("partner", "CardCode", "string", "业务伙伴", "Business Partner"),
				property("date", "DocDate", "date", "单据日期", "Document Date"),
				property("due_date", "DocDueDate", "date", "到期日期", "Due Date"),
				property("currency", "DocCur", "string", "币种", "Currency"),
				property("total", "DocTotal", "decimal", "含税金额", "Total"),
				property("tax", "VatSum", "decimal", "税额", "Tax"),
				property("status", "DocStatus", "string", "状态", "Status"),
				property("approval_status", "WddStatus", "string", "审批状态", "Approval Status"),
				property("posted", "Posted", "string", "已过账", "Posted"),
			)
			typ.Links = append(typ.Links, directLink("partner", "business_partner", "CardCode", "业务伙伴", "Business Partner"))
			typ.Links = append(typ.Links, LinkType{Key: "source", Label: Label{ZH: "来源单据", EN: "Source Document"}, TargetType: "*", Cardinality: "one", field: "BaseEntry", baseTable: true})
			typ.Links = append(typ.Links, LinkType{Key: "journals", Label: Label{ZH: "会计分录", EN: "Journal Entries"}, TargetType: "journal_entry", Cardinality: "many", field: "BaseEntry", inverse: true, baseTable: true})
			if len(table.Children) > 0 && spec.table != "MVPM" && spec.table != "MRCT" {
				typ.Links = append(typ.Links,
					LinkType{Key: "items", Label: Label{ZH: "物料", EN: "Items"}, TargetType: "item", Cardinality: "many", field: "ItemCode", child: table.Children[0].Code},
					LinkType{Key: "warehouses", Label: Label{ZH: "仓库", EN: "Warehouses"}, TargetType: "warehouse", Cardinality: "many", field: "WhsCode", child: table.Children[0].Code},
				)
			}
		}
		switch spec.table {
		case "MPOR":
			typ.Links = append(typ.Links, directLink("fulfillment", "goods_receipt", "FulfillmentEntry", "收货单", "Receipt"))
		case "MRDR":
			typ.Links = append(typ.Links, directLink("fulfillment", "delivery", "FulfillmentEntry", "交货单", "Delivery"))
		case "MPDN":
			typ.Links = append(typ.Links, directLink("invoice", "payable_invoice", "InvoiceEntry", "应付发票", "Payable Invoice"))
		case "MDLN":
			typ.Links = append(typ.Links, directLink("invoice", "receivable_invoice", "InvoiceEntry", "应收发票", "Receivable Invoice"))
		case "MINV", "MPCH":
			typ.Properties = append(typ.Properties, property("paid", "PaidToDate", "decimal", "已结算", "Settled"))
			target, child := "incoming_payment", "RCT1"
			if spec.table == "MPCH" {
				target, child = "outgoing_payment", "VPM1"
			}
			typ.Links = append(typ.Links, LinkType{Key: "payments", Label: Label{ZH: "收付款", EN: "Payments"}, TargetType: target, Cardinality: "many", field: "TargetKey", child: child, inverse: true})
		case "MRCT", "MVPM":
			typ.Properties = append(typ.Properties, property("allocated", "AllocatedAmount", "decimal", "已核销", "Allocated"), property("open_balance", "OpenBal", "decimal", "未核销金额", "Open Balance"))
			target, child := "receivable_invoice", "RCT1"
			if spec.table == "MVPM" {
				target, child = "payable_invoice", "VPM1"
			}
			typ.Links = append(typ.Links, LinkType{Key: "invoices", Label: Label{ZH: "核销发票", EN: "Allocated Invoices"}, TargetType: target, Cardinality: "many", field: "TargetKey", child: child})
		case "MITW":
			typ.Properties = append(typ.Properties, property("item", "BaseItemCode", "string", "物料", "Item"), property("warehouse", "WhsCode", "string", "仓库", "Warehouse"), property("quantity", "OnHand", "decimal", "现存量", "On Hand"), property("value", "InventoryValue", "decimal", "存货价值", "Inventory Value"), property("average_cost", "AvgPrice", "decimal", "平均成本", "Average Cost"), property("currency", "Currency", "string", "币种", "Currency"))
			typ.Links = append(typ.Links, directLink("item", "item", "BaseItemCode", "物料", "Item"), directLink("warehouse", "warehouse", "WhsCode", "仓库", "Warehouse"))
		case "MCRD":
			typ.Properties = append(typ.Properties, property("name", "CardName", "string", "名称", "Name"), property("partner_type", "CardType", "string", "伙伴类型", "Partner Type"))
		case "MITM":
			typ.Properties = append(typ.Properties, property("name", "ItemName", "string", "名称", "Name"), property("unit", "InvntryUom", "string", "计量单位", "Unit"))
			typ.Links = append(typ.Links, LinkType{Key: "stock", Label: Label{ZH: "仓库库存", EN: "Warehouse Stock"}, TargetType: "stock_balance", Cardinality: "many", field: "BaseItemCode", inverse: true})
		case "MWHS":
			typ.Properties = append(typ.Properties, property("name", "WhsName", "string", "名称", "Name"))
		case "MJDT":
			typ.Properties = append(typ.Properties, property("name", "Memo", "string", "摘要", "Memo"), property("date", "RefDate", "date", "记账日期", "Posting Date"), property("currency", "Currency", "string", "币种", "Currency"), property("status", "BtfStatus", "string", "状态", "Status"))
			typ.Links = append(typ.Links, LinkType{Key: "source", Label: Label{ZH: "来源单据", EN: "Source Document"}, TargetType: "*", Cardinality: "one", field: "BaseEntry", baseTable: true}, LinkType{Key: "accounts", Label: Label{ZH: "会计科目", EN: "Accounts"}, TargetType: "account", Cardinality: "many", field: "AccountCode", child: "JDT1"})
		case "MLIN":
			typ.Properties = append(typ.Properties, property("name", "Name", "string", "名称", "Name"), property("status", "Status", "string", "状态", "Status"))
		case "MMAC":
			typ.Properties = append(typ.Properties,
				property("name", "Name", "string", "名称", "Name"),
				property("line", "line_code", "string", "产线", "Line"),
				property("current_work_order", "current_work_order", "string", "当前工单", "Current Work Order"),
				property("verified_at", "verified_at", "string", "核对时间", "Verified At"),
				property("signed_off_at", "signed_off_at", "string", "签核时间", "Signed Off At"))
			typ.Links = append(typ.Links,
				directLink("line", "smt_line", "line_code", "产线", "Line"),
				directLink("work_order", "smt_work_order", "current_work_order", "当前工单", "Current Work Order"))
		case "MFSB":
			typ.Properties = append(typ.Properties,
				property("product", "product_code", "string", "产品", "Product"),
				property("machine", "machine_code", "string", "机台", "Machine"),
				property("program", "program_name", "string", "程序", "Program"),
				property("version", "version", "string", "版本", "Version"),
				property("active", "active", "boolean", "启用", "Active"))
			typ.Links = append(typ.Links,
				directLink("product", "smt_product", "product_code", "产品", "Product"),
				directLink("machine", "smt_machine", "machine_code", "机台", "Machine"))
		case "MOPR":
			typ.Properties = append(typ.Properties, property("name", "Name", "string", "名称", "Name"), property("op_type", "OpType", "string", "工序类型", "Operation Type"), property("status", "Status", "string", "状态", "Status"))
		case "MROU":
			typ.Properties = append(typ.Properties,
				property("product", "product_code", "string", "产品", "Product"),
				property("version", "version", "string", "版本", "Version"),
				property("active", "active", "boolean", "启用", "Active"))
			typ.Links = append(typ.Links, directLink("product", "smt_product", "product_code", "产品", "Product"))
		case "MPRD":
			typ.Properties = append(typ.Properties,
				property("name", "Name", "string", "名称", "Name"),
				property("version", "version", "string", "版本", "Version"),
				property("status", "Status", "string", "状态", "Status"))
			typ.Links = append(typ.Links, directLink("customer", "business_partner", "customer_code", "客户", "Customer"))
		case "MREL":
			typ.Properties = append(typ.Properties,
				property("part", "part_no", "string", "料号", "Item"),
				property("qty", "qty", "decimal", "数量", "Quantity"),
				property("lot", "lot_no", "string", "批次", "Lot"),
				property("status", "status", "string", "状态", "Status"),
				property("owner_type", "owner_type", "string", "归属", "Ownership"),
				property("location", "location", "string", "库位", "Location"))
			typ.Links = append(typ.Links,
				directLink("item", "item", "part_no", "料号", "Item"),
				directLink("issued_work_order", "smt_work_order", "issued_wo", "发料工单", "Issued Work Order"),
				directLink("customer", "business_partner", "customer_code", "客户", "Customer"))
		case "MSWO":
			typ.Properties = append(typ.Properties,
				property("product", "product_code", "string", "产品", "Product"),
				property("line", "line_code", "string", "产线", "Line"),
				property("plan_qty", "plan_qty", "decimal", "计划数", "Planned Quantity"),
				property("done_qty", "done_qty", "decimal", "完成数", "Completed Quantity"),
				property("status", "Status", "string", "状态", "Status"))
			typ.Links = append(typ.Links,
				directLink("product", "smt_product", "product_code", "产品", "Product"),
				directLink("line", "smt_line", "line_code", "产线", "Line"),
				directLink("customer", "business_partner", "customer_code", "客户", "Customer"),
				LinkType{Key: "units", Label: Label{ZH: "在制品", EN: "Units"}, TargetType: "smt_unit", Cardinality: "many", field: "work_order", inverse: true})
		case "MRQN":
			typ.Properties = append(typ.Properties,
				property("work_order", "work_order", "string", "工单", "Work Order"),
				property("batch_qty", "batch_qty", "decimal", "本批板数", "Batch Quantity"),
				property("status", "Status", "string", "状态", "Status"))
			typ.Links = append(typ.Links, directLink("work_order", "smt_work_order", "work_order", "工单", "Work Order"))
		case "MLOD":
			typ.Properties = append(typ.Properties,
				property("machine", "machine_code", "string", "机台", "Machine"),
				property("slot", "slot_no", "string", "站位", "Slot"),
				property("reel", "reel_code", "string", "料盘", "Reel"),
				property("part", "part_no", "string", "料号", "Item"),
				property("position", "position", "string", "位置", "Position"),
				property("status", "status", "string", "状态", "Status"))
			typ.Links = append(typ.Links,
				directLink("machine", "smt_machine", "machine_code", "机台", "Machine"),
				directLink("reel", "smt_reel", "reel_code", "料盘", "Reel"),
				directLink("work_order", "smt_work_order", "work_order", "工单", "Work Order"))
		case "MVER":
			typ.Properties = append(typ.Properties,
				property("result", "result", "string", "结果", "Result"),
				property("error_type", "error_type", "string", "错误类型", "Error Type"),
				property("message", "message", "string", "信息", "Message"),
				property("slot", "slot_no", "string", "站位", "Slot"),
				property("reel", "reel_code", "string", "料盘", "Reel"))
			typ.Links = append(typ.Links, directLink("machine", "smt_machine", "machine_code", "机台", "Machine"))
		case "MUNT":
			typ.Properties = append(typ.Properties,
				property("work_order", "work_order", "string", "工单", "Work Order"),
				property("product", "product_code", "string", "产品", "Product"),
				property("status", "status", "string", "状态", "Status"),
				property("last_seq", "last_seq", "integer", "最后通过工序", "Last Passed Step"),
				property("fail_count", "fail_count", "integer", "不良次数", "Fail Count"))
			typ.Links = append(typ.Links,
				directLink("work_order", "smt_work_order", "work_order", "工单", "Work Order"),
				directLink("product", "smt_product", "product_code", "产品", "Product"),
				directLink("panel", "smt_panel", "panel_code", "拼板", "Panel"))
		case "MPNL":
			typ.Properties = append(typ.Properties,
				property("work_order", "work_order", "string", "工单", "Work Order"),
				property("board_count", "board_count", "integer", "板数", "Board Count"),
				property("status", "status", "string", "状态", "Status"))
			typ.Links = append(typ.Links, directLink("work_order", "smt_work_order", "work_order", "工单", "Work Order"))
		case "MUDF":
			typ.Properties = append(typ.Properties,
				property("sn", "sn", "string", "板号", "Serial Number"),
				property("defect_code", "defect_code", "string", "不良代码", "Defect Code"),
				property("location", "location", "string", "位号", "Reference Designator"),
				property("part", "part_no", "string", "料号", "Item"),
				property("status", "status", "string", "状态", "Status"))
			typ.Links = append(typ.Links,
				directLink("unit", "smt_unit", "sn", "板号", "Unit"),
				directLink("item", "item", "part_no", "料号", "Item"))
		case "MINS":
			typ.Properties = append(typ.Properties,
				property("type", "insp_type", "string", "类型", "Type"),
				property("work_order", "work_order", "string", "工单", "Work Order"),
				property("sample_sn", "sample_sn", "string", "样品板号", "Sample SN"),
				property("result", "result", "string", "结果", "Result"))
			typ.Links = append(typ.Links, directLink("work_order", "smt_work_order", "work_order", "工单", "Work Order"))
		case "MAND":
			typ.Properties = append(typ.Properties,
				property("call_type", "call_type", "string", "类型", "Call Type"),
				property("status", "status", "string", "状态", "Status"),
				property("source", "source", "string", "来源", "Source"),
				property("machine", "machine_code", "string", "机台", "Machine"),
				property("cause", "cause", "string", "原因", "Cause"),
				property("solution", "solution", "string", "对策", "Solution"))
			typ.Links = append(typ.Links,
				directLink("machine", "smt_machine", "machine_code", "机台", "Machine"),
				directLink("line", "smt_line", "line_code", "产线", "Line"),
				directLink("work_order", "smt_work_order", "work_order", "工单", "Work Order"))
		case "MDWN":
			typ.Properties = append(typ.Properties,
				property("machine", "machine_code", "string", "机台", "Machine"),
				property("reason_type", "reason_type", "string", "原因类型", "Reason Type"),
				property("planned", "planned", "boolean", "计划停机", "Planned"),
				property("started_at", "started_at", "string", "开始时间", "Started At"),
				property("ended_at", "ended_at", "string", "结束时间", "Ended At"))
			typ.Links = append(typ.Links,
				directLink("machine", "smt_machine", "machine_code", "机台", "Machine"),
				directLink("work_order", "smt_work_order", "work_order", "工单", "Work Order"))
		default:
			typ.Properties = append(typ.Properties, property("name", "Name", "string", "名称", "Name"))
		}
		for _, action := range actions {
			if action.TableCode != table.Code {
				continue
			}
			label := actionLabel(action.TableCode, action.Action)
			def := Action{Key: action.Action, Label: label, RequiresApproval: true}
			if action.Action == "allocate" {
				def.Parameters = []Property{property("TargetKey", "TargetKey", "string", "发票编号", "Invoice Key"), property("Amount", "Amount", "decimal", "核销金额", "Allocation Amount")}
			}
			typ.Actions = append(typ.Actions, def)
		}
		result = append(result, typ)
	}
	return result
}

func directLink(key, target, field, zh, en string) LinkType {
	return LinkType{Key: key, Label: Label{ZH: zh, EN: en}, TargetType: target, Cardinality: "one", field: field}
}

func actionLabel(tableCode, action string) Label {
	tableLabels := map[string]Label{
		"MREL:receive":            {ZH: "收料入库", EN: "Receive Reels"},
		"MREL:issue":              {ZH: "发料", EN: "Issue Reel"},
		"MREL:return":             {ZH: "退库", EN: "Return Reel"},
		"MREL:empty":              {ZH: "料盘用完", EN: "Reel Empty"},
		"MREL:scrap":              {ZH: "料盘报废", EN: "Scrap Reel"},
		"MREL:lost":               {ZH: "盘亏", EN: "Mark Lost"},
		"MREL:return-to-customer": {ZH: "客供料退回客户", EN: "Return to Customer"},
		"MREL:adjust":             {ZH: "库存调整", EN: "Adjust Quantity"},
		"MLOD:load":               {ZH: "上料", EN: "Load Reel"},
		"MLOD:unload":             {ZH: "卸料", EN: "Unload Reel"},
		"MMAC:bind":               {ZH: "绑定工单（换线）", EN: "Bind Work Order"},
		"MMAC:verify":             {ZH: "整机核对", EN: "Verify Machine"},
		"MMAC:sign-off":           {ZH: "第二人签核", EN: "Sign Off"},
		"MMAC:lock-slot":          {ZH: "锁定站位", EN: "Lock Slot"},
		"MMAC:unlock-slot":        {ZH: "站位解锁", EN: "Unlock Slot"},
		"MSWO:start":              {ZH: "开工", EN: "Start"},
		"MSWO:progress":           {ZH: "报工", EN: "Report Progress"},
		"MSWO:finish":             {ZH: "完工", EN: "Finish"},
		"MSWO:close":              {ZH: "关闭", EN: "Close"},
		"MRQN:issue":              {ZH: "领料发料（扫料盘）", EN: "Issue Reel"},
		"MRQN:close":              {ZH: "领料单完成", EN: "Close Requisition"},
		"MRQN:cancel":             {ZH: "领料单取消", EN: "Cancel Requisition"},
		"MUNT:pass":               {ZH: "过站（良品）", EN: "Pass"},
		"MUNT:fail":               {ZH: "过站判不良", EN: "Fail"},
		"MUNT:repair":             {ZH: "维修完成", EN: "Complete Repair"},
		"MUNT:scrap":              {ZH: "报废", EN: "Scrap"},
		"MPNL:bind":               {ZH: "拼板绑定", EN: "Bind Panel"},
		"MPNL:split":              {ZH: "分板", EN: "Split Panel"},
		"MINS:record":             {ZH: "首件 / 巡检登记", EN: "Record Inspection"},
		"MAND:raise":              {ZH: "Andon 呼叫", EN: "Raise Andon"},
		"MAND:ack":                {ZH: "Andon 响应", EN: "Acknowledge Andon"},
		"MAND:close":              {ZH: "Andon 关闭", EN: "Close Andon"},
		"MDWN:start":              {ZH: "开始停机", EN: "Start Downtime"},
		"MDWN:end":                {ZH: "结束停机", EN: "End Downtime"},
	}
	if label, ok := tableLabels[tableCode+":"+action]; ok {
		return label
	}
	labels := map[string]Label{
		"submit": {ZH: "提交审批", EN: "Submit"}, "approve": {ZH: "批准", EN: "Approve"},
		"post": {ZH: "过账", EN: "Post"}, "allocate": {ZH: "核销", EN: "Allocate"},
		"receive": {ZH: "生成收货单", EN: "Create Receipt"}, "deliver": {ZH: "生成交货单", EN: "Create Delivery"},
		"confirm": {ZH: "确认", EN: "Confirm"}, "convert-to-project": {ZH: "创建项目", EN: "Create Project"},
		"analyze":      {ZH: "分析", EN: "Analyze"},
		"refresh-cost": {ZH: "刷新项目成本", EN: "Refresh Project Cost"}, "close-feedback": {ZH: "关闭反馈", EN: "Close Feedback"},
	}
	if label, ok := labels[action]; ok {
		return label
	}
	return Label{ZH: action, EN: action}
}
