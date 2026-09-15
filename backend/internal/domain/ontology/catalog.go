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
		default:
			typ.Properties = append(typ.Properties, property("name", "Name", "string", "名称", "Name"))
		}
		for _, action := range actions {
			if action.TableCode != table.Code {
				continue
			}
			label := actionLabel(action.Action)
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

func actionLabel(action string) Label {
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
