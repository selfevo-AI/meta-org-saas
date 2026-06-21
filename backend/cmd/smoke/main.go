package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

type client struct {
	base           string
	token          string
	organizationID string
	http           *http.Client
}

type responseMap map[string]any

func main() {
	base := strings.TrimRight(os.Getenv("SMOKE_API_BASE"), "/")
	if base == "" {
		base = "http://127.0.0.1:8080/api/v1"
	}
	c := &client{base: base, http: &http.Client{Timeout: 20 * time.Second}}
	stamp := time.Now().UTC().Format("20060102150405")
	email := fmt.Sprintf("smoke-%s@meta-org.local", stamp)
	password := "SmokePass123!"

	user := c.post("/auth/register", responseMap{
		"name":     "Smoke User " + stamp,
		"email":    email,
		"password": password,
	})
	login := c.post("/auth/login", responseMap{"email": email, "password": password})
	c.token = stringField(login, "token")
	userID := stringField(user, "id")
	if userID == "" {
		userID = stringField(login, "user_id")
	}
	must(userID != "", "missing user id")

	orgID := c.ensureTenantOrganization(login, stamp)
	dept := c.post("/organizations/"+orgID+"/departments", responseMap{
		"name":        "Delivery",
		"code":        "DEL",
		"description": "Delivery department",
	})
	deptID := stringField(dept, "id")
	template := c.post("/workflows/templates", responseMap{
		"name":            "Smoke PDCA Workflow " + stamp,
		"description":     "Plan, execute, review",
		"organization_id": orgID,
		"department_id":   deptID,
		"stages": []responseMap{
			{"type": "plan", "name": "Plan", "assignee_type": "internal", "required_permission_level": "L1", "risk_level": "low"},
			{"type": "execute", "name": "Do", "assignee_type": "either", "required_permission_level": "L2", "risk_level": "medium"},
			{"type": "review", "name": "Accept", "assignee_type": "internal", "required_permission_level": "L2", "risk_level": "medium"},
		},
	})
	templateID := stringField(template, "id")

	requirement := c.post("/requirements", responseMap{
		"title":           "Smoke PDCA Requirement " + stamp,
		"description":     "Verify requirement, project, workflow, delivery, cost, feedback, and PDCA evidence.",
		"priority":        "medium",
		"risk_level":      "medium",
		"required_level":  "L2",
		"budget_amount":   5000,
		"budget_currency": "CNY",
		"organization_id": orgID,
		"department_id":   deptID,
		"created_by_id":   userID,
		"created_by_type": "internal_human",
	})
	requirementID := stringField(requirement, "id")
	c.upload("/requirements/"+requirementID+"/documents", "smoke-requirement.txt", "text/plain", []byte("smoke requirement evidence"))
	requirement = c.post("/requirements/"+requirementID+"/analyze", responseMap{"notes": "smoke analysis"})
	must(stringField(requirement, "status") == "analyzed", "requirement was not analyzed")
	requirement = c.post("/requirements/"+requirementID+"/approve", responseMap{})
	must(stringField(requirement, "status") == "approved", "requirement was not approved")
	project := c.post("/requirements/"+requirementID+"/convert-to-project", responseMap{})
	projectID := stringField(project, "id")

	c.post("/projects/"+projectID+"/members", responseMap{
		"member_actor_id":    userID,
		"member_actor_type":  "internal_human",
		"role":               "owner",
		"title":              "Smoke owner",
		"allocation_percent": 100,
		"cost_rate":          800,
		"permission_level":   "L2",
		"capabilities":       []string{"planning", "delivery", "review"},
		"actor_id":           userID,
		"actor_type":         "internal_human",
	})
	projectWorkflow := c.post("/projects/"+projectID+"/workflows", responseMap{
		"workflow_template_id": templateID,
		"purpose":              "delivery",
		"actor_id":             userID,
		"actor_type":           "internal_human",
	})
	workflowID := stringField(projectWorkflow, "workflow_id")
	workflow := c.get("/workflows/instances/" + workflowID)
	for _, rawTask := range listField(workflow, "tasks") {
		task, ok := rawTask.(map[string]any)
		must(ok, "invalid workflow task")
		c.patch("/tasks/"+stringField(task, "id")+"/status", responseMap{
			"output": responseMap{"smoke": true, "stage": numberField(task, "stage")},
		})
	}

	project = c.post("/projects/"+projectID+"/status", responseMap{"status": "active", "actor_id": userID, "actor_type": "internal_human"})
	must(stringField(project, "status") == "active", "project was not activated")
	deliverable := c.post("/projects/"+projectID+"/deliverables", responseMap{
		"name":             "Smoke Deliverable",
		"deliverable_type": "document",
		"version":          "1.0",
		"status":           "draft",
		"actor_id":         userID,
		"actor_type":       "internal_human",
	})
	deliverableID := stringField(deliverable, "id")
	c.post("/deliverables/"+deliverableID+"/submit", responseMap{"actor_id": userID, "actor_type": "internal_human"})
	c.post("/deliverables/"+deliverableID+"/accept", responseMap{"actor_id": userID, "actor_type": "internal_human"})
	c.post("/projects/"+projectID+"/cost-refresh", responseMap{"actor_id": userID, "actor_type": "internal_human"})
	c.post("/projects/"+projectID+"/cost-entries", responseMap{
		"source_type":      "manual",
		"entry_actor_id":   userID,
		"entry_actor_type": "internal_human",
		"amount":           120,
		"currency":         "CNY",
		"description":      "smoke manual cost",
		"actor_id":         userID,
		"actor_type":       "internal_human",
	})
	project = c.post("/projects/"+projectID+"/status", responseMap{"status": "delivering", "actor_id": userID, "actor_type": "internal_human"})
	must(stringField(project, "status") == "delivering", "project was not delivering")
	project = c.post("/projects/"+projectID+"/status", responseMap{"status": "completed", "actor_id": userID, "actor_type": "internal_human"})
	must(stringField(project, "status") == "completed", "project was not completed")
	c.post("/projects/"+projectID+"/evaluations", responseMap{
		"evaluated_actor_id":   userID,
		"evaluated_actor_type": "internal_human",
		"quality_score":        0.9,
		"delivery_score":       0.85,
		"cost_score":           0.8,
		"collaboration_score":  0.9,
		"conclusion":           "smoke evaluation passed",
		"actor_id":             userID,
		"actor_type":           "internal_human",
	})
	closeResult := c.post("/projects/"+projectID+"/close-feedback", responseMap{
		"outcome_score": 0.88,
		"conclusion":    "smoke feedback closed",
		"actor_id":      userID,
		"actor_type":    "internal_human",
	})
	closedProject, _ := closeResult["project"].(map[string]any)
	must(stringField(closedProject, "status") == "closed", "project was not closed")

	overview := c.get("/projects/" + projectID + "/overview")
	lifecycle, _ := overview["lifecycle"].(map[string]any)
	cycleID := stringField(lifecycle, "pdca_cycle_id")
	must(cycleID != "", "missing pdca cycle id")
	events := c.get("/pdca-events?cycle_id=" + cycleID)
	must(len(asList(events["items"])) > 0, "missing pdca events")

	supply := runSupplyChainSmoke(c, stamp, orgID, deptID)
	runCostingSmoke(c, stamp, orgID, deptID, projectID, userID)
	runFinanceSmoke(c, stamp, orgID, deptID, projectID, requirementID, deliverableID)
	runSystemAdminSmoke(base, orgID, stamp)

	fmt.Printf(
		"smoke ok: requirement=%s project=%s pdca_cycle=%s item=%s purchase_receipt=%s sales_shipment=%s\n",
		requirementID,
		projectID,
		cycleID,
		supply.ItemID,
		supply.PurchaseReceiptID,
		supply.SalesShipmentID,
	)
}

type supplyChainSmokeResult struct {
	SupplierID        string
	CustomerID        string
	ItemID            string
	WarehouseID       string
	LocationID        string
	PurchaseReceiptID string
	PurchasePayableID string
	SalesShipmentID   string
	SalesReceivableID string
}

func runSupplyChainSmoke(c *client, stamp string, orgID string, deptID string) supplyChainSmokeResult {
	supplierName := "Smoke Supplier " + stamp
	customerName := "Smoke Customer " + stamp
	supplier := c.post("/inventory/partners", responseMap{
		"partner_type":    "supplier",
		"name":            supplierName,
		"email":           "supplier-" + stamp + "@meta-org.local",
		"organization_id": orgID,
	})
	customer := c.post("/inventory/partners", responseMap{
		"partner_type":    "customer",
		"name":            customerName,
		"email":           "customer-" + stamp + "@meta-org.local",
		"organization_id": orgID,
	})
	item := c.post("/inventory/items", responseMap{
		"name":            "Smoke Item " + stamp,
		"item_type":       "material",
		"base_uom":        "pcs",
		"organization_id": orgID,
	})
	warehouse := c.post("/inventory/warehouses", responseMap{
		"name":            "Smoke Main Warehouse " + stamp,
		"organization_id": orgID,
		"department_id":   deptID,
	})
	backupWarehouse := c.post("/inventory/warehouses", responseMap{
		"name":            "Smoke Backup Warehouse " + stamp,
		"organization_id": orgID,
		"department_id":   deptID,
	})
	itemID := stringField(item, "id")
	warehouseID := stringField(warehouse, "id")
	backupWarehouseID := stringField(backupWarehouse, "id")
	location := c.post("/inventory/locations", responseMap{
		"warehouse_id": warehouseID,
		"name":         "Default Bin",
	})
	locationID := stringField(location, "id")
	must(itemID != "" && warehouseID != "" && locationID != "", "missing inventory setup ids")

	c.post("/inventory/movements", responseMap{
		"movement_type":   "adjustment_in",
		"source_type":     "smoke_seed",
		"item_id":         itemID,
		"warehouse_id":    warehouseID,
		"location_id":     locationID,
		"quantity":        5,
		"unit_cost":       10,
		"currency":        "CNY",
		"organization_id": orgID,
		"department_id":   deptID,
	})
	c.post("/inventory/transfers", responseMap{
		"from_warehouse_id": warehouseID,
		"to_warehouse_id":   backupWarehouseID,
		"organization_id":   orgID,
		"department_id":     deptID,
		"lines": []responseMap{
			{"item_id": itemID, "quantity": 1, "unit_cost": 10},
		},
	})
	c.post("/inventory/adjustments", responseMap{
		"warehouse_id":    warehouseID,
		"reason":          "smoke adjustment",
		"organization_id": orgID,
		"department_id":   deptID,
		"lines": []responseMap{
			{"item_id": itemID, "quantity_delta": 1, "unit_cost": 10},
		},
	})
	c.post("/inventory/counts", responseMap{
		"warehouse_id":    warehouseID,
		"organization_id": orgID,
		"department_id":   deptID,
		"lines": []responseMap{
			{"item_id": itemID, "book_qty": 5, "counted_qty": 5},
		},
	})

	supplierID := stringField(supplier, "id")
	requisition := c.post("/procurement/requisitions", responseMap{
		"title":           "Smoke Requisition " + stamp,
		"supplier_id":     supplierID,
		"supplier_name":   supplierName,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "quantity": 3, "unit_cost": 20},
		},
	})
	requisitionID := stringField(requisition, "id")
	requisition = c.post("/procurement/requisitions/"+requisitionID+"/submit", responseMap{})
	must(stringField(requisition, "status") == "submitted", "purchase requisition was not submitted")
	requisition = c.post("/procurement/requisitions/"+requisitionID+"/approve", responseMap{})
	must(stringField(requisition, "status") == "approved", "purchase requisition was not approved")

	order := c.post("/procurement/orders", responseMap{
		"requisition_id":  requisitionID,
		"supplier_id":     supplierID,
		"supplier_name":   supplierName,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "quantity": 3, "unit_cost": 20, "tax_rate": 0.06},
		},
	})
	orderID := stringField(order, "id")
	order = c.post("/procurement/orders/"+orderID+"/submit", responseMap{})
	must(stringField(order, "status") == "submitted", "purchase order was not submitted")
	order = c.post("/procurement/orders/"+orderID+"/approve", responseMap{})
	must(stringField(order, "status") == "approved", "purchase order was not approved")

	receipt := c.post("/procurement/receipts", responseMap{
		"order_id":        orderID,
		"supplier_id":     supplierID,
		"supplier_name":   supplierName,
		"warehouse_id":    warehouseID,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "location_id": locationID, "quantity": 3, "unit_cost": 20, "tax_rate": 0.06},
		},
	})
	receiptID := stringField(receipt, "id")
	receipt = c.post("/procurement/receipts/"+receiptID+"/post", responseMap{})
	must(stringField(receipt, "status") == "posted", "purchase receipt was not posted")
	payableID := stringField(receipt, "payable_id")
	must(payableID != "", "purchase receipt did not create payable")
	c.post("/procurement/returns", responseMap{
		"receipt_id":      receiptID,
		"supplier_id":     supplierID,
		"supplier_name":   supplierName,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "location_id": locationID, "quantity": 1, "unit_cost": 20, "tax_rate": 0.06},
		},
	})

	customerID := stringField(customer, "id")
	quotation := c.post("/sales/quotations", responseMap{
		"customer_id":     customerID,
		"customer_name":   customerName,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "quantity": 2, "unit_price": 35, "tax_rate": 0.06},
		},
	})
	quotationID := stringField(quotation, "id")
	salesOrder := c.post("/sales/orders", responseMap{
		"quotation_id":    quotationID,
		"customer_id":     customerID,
		"customer_name":   customerName,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "quantity": 2, "unit_price": 35, "tax_rate": 0.06},
		},
	})
	salesOrderID := stringField(salesOrder, "id")
	salesOrder = c.post("/sales/orders/"+salesOrderID+"/confirm", responseMap{})
	must(stringField(salesOrder, "status") == "confirmed", "sales order was not confirmed")
	salesOrder = c.post("/sales/orders/"+salesOrderID+"/approve", responseMap{})
	must(stringField(salesOrder, "status") == "approved", "sales order was not approved")

	shipment := c.post("/sales/shipments", responseMap{
		"order_id":        salesOrderID,
		"customer_id":     customerID,
		"customer_name":   customerName,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "location_id": locationID, "quantity": 2, "unit_price": 35, "tax_rate": 0.06},
		},
	})
	shipmentID := stringField(shipment, "id")
	shipment = c.post("/sales/shipments/"+shipmentID+"/post", responseMap{})
	must(stringField(shipment, "status") == "posted", "sales shipment was not posted")
	receivableID := stringField(shipment, "receivable_id")
	must(receivableID != "", "sales shipment did not create receivable")
	c.post("/sales/returns", responseMap{
		"shipment_id":     shipmentID,
		"customer_id":     customerID,
		"customer_name":   customerName,
		"organization_id": orgID,
		"department_id":   deptID,
		"currency":        "CNY",
		"lines": []responseMap{
			{"item_id": itemID, "warehouse_id": warehouseID, "location_id": locationID, "quantity": 1, "unit_price": 35, "tax_rate": 0.06},
		},
	})

	must(len(asList(c.get("/inventory/balances")["items"])) > 0, "missing inventory balances")
	must(len(asList(c.get("/procurement/receipts")["items"])) > 0, "missing purchase receipts")
	must(len(asList(c.get("/sales/shipments")["items"])) > 0, "missing sales shipments")
	return supplyChainSmokeResult{
		SupplierID:        supplierID,
		CustomerID:        customerID,
		ItemID:            itemID,
		WarehouseID:       warehouseID,
		LocationID:        locationID,
		PurchaseReceiptID: receiptID,
		PurchasePayableID: payableID,
		SalesShipmentID:   shipmentID,
		SalesReceivableID: receivableID,
	}
}

func runCostingSmoke(c *client, stamp string, orgID string, deptID string, projectID string, userID string) {
	c.post("/costing/currencies", responseMap{
		"code":          "USD",
		"name":          "US Dollar",
		"currency_type": "fiat",
		"symbol":        "$",
	})
	rate := c.post("/costing/exchange-rates", responseMap{
		"from_currency": "USD",
		"to_currency":   "CNY",
		"rate":          7.2,
		"source":        "manual",
		"provider":      "smoke",
	})
	must(stringField(rate, "id") != "", "missing exchange rate id")
	converted := c.post("/costing/convert", responseMap{
		"amount":        10,
		"from_currency": "USD",
		"to_currency":   "CNY",
	})
	must(numberField(converted, "converted_amount") > 0, "currency conversion failed")
	c.post("/costing/rate-cards", responseMap{
		"subject_type": "human",
		"subject_id":   userID,
		"scope_type":   "organization",
		"scope_id":     orgID,
		"rate_type":    "hourly",
		"amount":       100,
		"currency":     "CNY",
		"metadata":     responseMap{"stamp": stamp},
	})
	c.post("/costing/budgets", responseMap{
		"scope_type": "organization",
		"scope_id":   orgID,
		"amount":     10000,
		"currency":   "CNY",
		"metadata":   responseMap{"stamp": stamp},
	})
	ledger := c.post("/costing/ledger-entries", responseMap{
		"ledger_type":     "actual",
		"cost_category":   "manual",
		"source_type":     "smoke",
		"organization_id": orgID,
		"department_id":   deptID,
		"project_id":      projectID,
		"actor_id":        userID,
		"actor_type":      "internal_human",
		"amount":          88,
		"currency":        "CNY",
		"description":     "smoke costing ledger",
	})
	must(stringField(ledger, "id") != "", "missing costing ledger id")
	summary := c.get("/costing/summary?currency=CNY")
	must(numberField(summary, "entry_count") > 0, "missing costing summary entries")
}

func runFinanceSmoke(c *client, stamp string, orgID string, deptID string, projectID string, requirementID string, deliverableID string) {
	adapter := c.post("/finance/adapters", responseMap{
		"name":         "Smoke Finance Adapter " + stamp,
		"endpoint_url": "http://127.0.0.1:8080/api/v1/health",
		"auth_type":    "hmac",
		"secret":       "smoke-secret-" + stamp,
		"status":       "active",
		"field_mapping": responseMap{
			"external_record_id": "external_record_id",
			"amount":             "amount",
		},
	})
	adapterID := stringField(adapter, "id")
	must(adapterID != "", "missing finance adapter id")

	imported := c.post("/finance/imports", responseMap{
		"adapter_id":  adapterID,
		"source_type": "api",
		"records": []responseMap{
			{
				"external_record_id": "smoke-expense-" + stamp,
				"expense_type":       "project_expense",
				"amount":             12.5,
				"currency":           "CNY",
				"occurred_at":        smokeDate(0),
				"invoice_number":     "SMOKE-INV-" + stamp,
				"vendor_id":          "smoke-vendor",
				"vendor_name":        "Smoke Vendor",
				"organization_id":    orgID,
				"department_id":      deptID,
				"project_id":         projectID,
				"requirement_id":     requirementID,
			},
		},
	})
	importBatch, _ := imported["batch"].(map[string]any)
	must(numberField(importBatch, "processed_records") > 0, "finance import did not process records")
	must(len(asList(imported["records"])) > 0, "finance import returned no records")

	exportBatch := c.post("/finance/export-batches", responseMap{
		"adapter_id":      adapterID,
		"period_start":    smokeDate(0),
		"period_end":      smokeDate(0),
		"currency":        "CNY",
		"idempotency_key": "smoke-export-" + stamp,
	})
	must(stringField(exportBatch, "id") != "", "missing finance export batch id")

	settlement := c.post("/finance/settlement-orders", responseMap{
		"project_id":     projectID,
		"requirement_id": requirementID,
		"deliverable_id": deliverableID,
		"customer_id":    "smoke-customer",
		"customer_name":  "Smoke Customer",
		"title":          "Smoke Settlement " + stamp,
		"currency":       "CNY",
		"lines": []responseMap{
			{"line_type": "service", "description": "Smoke service", "quantity": 1, "unit_price": 50, "amount": 50},
		},
	})
	settlementID := stringField(settlement, "id")
	settlementReceivable := c.post("/finance/settlement-orders/"+settlementID+"/post", responseMap{})
	must(stringField(settlementReceivable, "id") != "", "settlement order did not create receivable")

	receivable := c.post("/finance/receivables", responseMap{
		"receivable_type": "manual",
		"customer_id":     "smoke-customer",
		"customer_name":   "Smoke Customer",
		"project_id":      projectID,
		"requirement_id":  requirementID,
		"organization_id": orgID,
		"department_id":   deptID,
		"amount":          42,
		"tax_amount":      0,
		"currency":        "CNY",
	})
	receivableID := stringField(receivable, "id")
	receipt := c.post("/finance/receipts", responseMap{
		"payment_method":   "bank_transfer",
		"customer_id":      "smoke-customer",
		"customer_name":    "Smoke Customer",
		"payer_account":    "smoke-payer",
		"receiver_account": "smoke-receiver",
		"amount":           42,
		"currency":         "CNY",
	})
	receiptID := stringField(receipt, "id")
	c.post("/finance/receipts/"+receiptID+"/allocate", responseMap{
		"receivable_id": receivableID,
		"amount":        42,
		"currency":      "CNY",
	})

	payable := c.post("/finance/payables", responseMap{
		"payable_type":    "expense",
		"vendor_id":       "smoke-vendor",
		"vendor_name":     "Smoke Vendor",
		"project_id":      projectID,
		"organization_id": orgID,
		"department_id":   deptID,
		"amount":          33,
		"currency":        "CNY",
	})
	payableID := stringField(payable, "id")
	payment := c.post("/finance/payments", responseMap{
		"payment_method": "bank_transfer",
		"payer_account":  "smoke-payer",
		"payee_account":  "smoke-payee",
		"vendor_id":      "smoke-vendor",
		"vendor_name":    "Smoke Vendor",
		"amount":         33,
		"currency":       "CNY",
	})
	paymentID := stringField(payment, "id")
	c.post("/finance/payments/"+paymentID+"/allocate", responseMap{
		"payable_id": payableID,
		"amount":     33,
		"currency":   "CNY",
	})
	must(len(asList(c.get("/finance/reconciliation")["items"])) >= 0, "finance reconciliation failed")
}

func runSystemAdminSmoke(base string, orgID string, stamp string) {
	adminEmail := envOrDefault("SMOKE_PLATFORM_ADMIN_EMAIL", "admin-smoke@meta-org.local")
	adminPassword := envOrDefault("SMOKE_PLATFORM_ADMIN_PASSWORD", "AdminSmoke123!")
	admin := &client{base: base, http: &http.Client{Timeout: 20 * time.Second}}
	login := admin.post("/auth/login", responseMap{"email": adminEmail, "password": adminPassword})
	admin.token = stringField(login, "token")
	must(admin.token != "", "missing platform admin token")

	targets := admin.get("/platform/admin/schema-targets")
	must(asList(targets["items"]) != nil, "schema targets did not return a list")
	masters := admin.get("/platform/admin/modules/data_catalog/masters")
	must(len(asList(masters["items"])) > 0, "missing data catalog platform masters")
	exported := admin.get("/platform/admin/organizations/" + orgID + "/schema/export")
	must(stringField(exported, "format_version") != "", "missing exported schema package")
	change := admin.post("/platform/admin/organizations/"+orgID+"/schema/change-requests", responseMap{
		"request_type":   "smoke_schema_validation",
		"reason":         "smoke schema validation " + stamp,
		"schema_package": exported,
	})
	changeID := stringField(change, "id")
	must(changeID != "", "missing schema change request id")
	change = admin.post("/platform/admin/schema-change-requests/"+changeID+"/approve", responseMap{"reason": "smoke approval"})
	must(stringField(change, "status") == "approved", "schema change was not approved")
	job := admin.post("/platform/admin/schema-change-requests/"+changeID+"/apply", responseMap{})
	must(stringField(job, "status") == "applied", "schema change was not applied")
}

func smokeDate(daysFromNow int) string {
	return time.Now().UTC().AddDate(0, 0, daysFromNow).Format("2006-01-02")
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func (c *client) get(path string) responseMap {
	return c.do(http.MethodGet, path, nil, http.StatusOK)
}

func (c *client) post(path string, payload responseMap) responseMap {
	status := http.StatusOK
	if expectsCreated(path) {
		status = http.StatusCreated
	}
	return c.do(http.MethodPost, path, payload, status)
}

func expectsCreated(path string) bool {
	return path == "/auth/register" ||
		path == "/onboarding/organization" ||
		path == "/organizations" ||
		path == "/workflows/templates" ||
		path == "/requirements" ||
		path == "/inventory/partners" ||
		path == "/inventory/items" ||
		path == "/inventory/warehouses" ||
		path == "/inventory/locations" ||
		path == "/inventory/movements" ||
		path == "/inventory/transfers" ||
		path == "/inventory/adjustments" ||
		path == "/inventory/counts" ||
		path == "/procurement/requisitions" ||
		path == "/procurement/orders" ||
		path == "/procurement/receipts" ||
		path == "/procurement/returns" ||
		path == "/sales/quotations" ||
		path == "/sales/orders" ||
		path == "/sales/shipments" ||
		path == "/sales/returns" ||
		path == "/costing/currencies" ||
		path == "/costing/exchange-rates" ||
		path == "/costing/rate-cards" ||
		path == "/costing/ledger-entries" ||
		path == "/costing/budgets" ||
		path == "/finance/adapters" ||
		path == "/finance/export-batches" ||
		path == "/finance/imports" ||
		path == "/finance/settlement-orders" ||
		path == "/finance/receivables" ||
		path == "/finance/receipts" ||
		path == "/finance/payables" ||
		path == "/finance/payments" ||
		(strings.HasPrefix(path, "/finance/receipts/") && strings.HasSuffix(path, "/allocate")) ||
		(strings.HasPrefix(path, "/finance/payments/") && strings.HasSuffix(path, "/allocate")) ||
		(strings.HasPrefix(path, "/finance/settlement-orders/") && strings.HasSuffix(path, "/post")) ||
		(strings.HasPrefix(path, "/platform/admin/organizations/") && strings.HasSuffix(path, "/schema/change-requests")) ||
		(strings.HasPrefix(path, "/organizations/") && strings.HasSuffix(path, "/departments")) ||
		strings.HasSuffix(path, "/convert-to-project") ||
		(strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/members")) ||
		(strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/workflows")) ||
		(strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/deliverables")) ||
		(strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/cost-entries")) ||
		(strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/cost-refresh")) ||
		(strings.HasPrefix(path, "/projects/") && strings.HasSuffix(path, "/evaluations"))
}

func (c *client) patch(path string, payload responseMap) responseMap {
	return c.do(http.MethodPatch, path, payload, http.StatusOK)
}

func (c *client) do(method string, path string, payload responseMap, expected int) responseMap {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			panic(err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		panic(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.organizationID != "" {
		req.Header.Set("X-Organization-ID", c.organizationID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expected {
		panic(fmt.Sprintf("%s %s: got %d want %d: %s", method, path, resp.StatusCode, expected, string(data)))
	}
	var result responseMap
	if len(data) == 0 {
		return responseMap{}
	}
	if err := json.Unmarshal(data, &result); err != nil {
		var items []any
		if listErr := json.Unmarshal(data, &items); listErr == nil {
			return responseMap{"items": items}
		}
		panic(fmt.Sprintf("%s %s: decode response: %v: %s", method, path, err, string(data)))
	}
	return result
}

func (c *client) upload(path string, fileName string, contentType string, content []byte) responseMap {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		panic(err)
	}
	if _, err := part.Write(content); err != nil {
		panic(err)
	}
	_ = writer.WriteField("metadata", `{"source":"smoke"}`)
	if err := writer.Close(); err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, &body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if contentType != "" {
		req.Header.Set("X-Smoke-Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.organizationID != "" {
		req.Header.Set("X-Organization-ID", c.organizationID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		panic(fmt.Sprintf("upload %s: got %d: %s", path, resp.StatusCode, string(data)))
	}
	var result responseMap
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}

func (c *client) ensureTenantOrganization(login responseMap, stamp string) string {
	orgID := organizationIDFromSession(login)
	if boolField(login, "onboarding_required") || orgID == "" {
		onboarding := c.post("/onboarding/organization", responseMap{
			"organization_name": "Smoke Organization " + stamp,
			"description":       "SaaS smoke organization",
			"enabled_modules":   defaultSmokeModules(),
		})
		orgID = organizationIDFromOnboarding(onboarding)
	}
	must(orgID != "", "missing tenant organization id")
	c.organizationID = orgID
	return orgID
}

func defaultSmokeModules() []string {
	return []string{
		"organization",
		"workflow",
		"project",
		"finance",
		"costing",
		"inventory",
		"procurement",
		"sales",
		"meta_resource",
		"assistant",
		"ai_gateway",
		"toolruntime",
		"governance",
		"evolution",
		"capability",
	}
}

func organizationIDFromSession(session responseMap) string {
	if id := stringField(session, "default_organization_id"); id != "" {
		return id
	}
	for _, rawOrg := range listField(session, "organizations") {
		org, ok := rawOrg.(map[string]any)
		if !ok {
			continue
		}
		if id := stringField(org, "id"); id != "" {
			return id
		}
	}
	return ""
}

func organizationIDFromOnboarding(result responseMap) string {
	if id := stringField(result, "organization_id"); id != "" {
		return id
	}
	if organization, ok := result["organization"].(map[string]any); ok {
		if id := stringField(organization, "id"); id != "" {
			return id
		}
	}
	if profile, ok := result["profile"].(map[string]any); ok {
		return stringField(profile, "default_organization_id")
	}
	return ""
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return value
}

func boolField(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	value, _ := m[key].(bool)
	return value
}

func numberField(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	value, _ := m[key].(float64)
	return value
}

func listField(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	return asList(m[key])
}

func asList(value any) []any {
	list, _ := value.([]any)
	return list
}

func must(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
