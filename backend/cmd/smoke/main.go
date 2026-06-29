package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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
	erp := runERPSmoke(c, stamp, orgID, userID)

	fmt.Printf(
		"smoke ok: org=%s user=%s partner=%s item=%s order=%s invoice=%s\n",
		orgID,
		userID,
		erp.PartnerKey,
		erp.ItemKey,
		erp.SalesOrderKey,
		erp.InvoiceKey,
	)
}

type erpSmokeResult struct {
	PartnerKey    string
	ItemKey       string
	WarehouseKey  string
	ProjectKey    string
	PurchaseKey   string
	SalesOrderKey string
	InvoiceKey    string
}

func runERPSmoke(c *client, stamp string, orgID string, userID string) erpSmokeResult {
	catalog := c.get("/erp/catalog")
	must(len(asList(catalog["tables"])) > 0, "missing ERP catalog")

	partnerKey := "C-" + stamp
	supplierKey := "S-" + stamp
	itemKey := "I-" + stamp
	warehouseKey := "W-" + stamp
	requirementKey := "REQ-" + stamp
	projectKey := "PRJ-" + stamp
	purchaseOrderKey := stamp + "PO"
	goodsReceiptKey := stamp + "GR"
	salesOrderKey := stamp + "01"
	deliveryKey := stamp + "DL"
	paymentKey := stamp + "PY"

	c.post("/erp/MCRD", responseMap{
		"key": partnerKey,
		"data": responseMap{
			"CardCode": partnerKey,
			"CardName": "Smoke Customer " + stamp,
			"CardType": "C",
			"Payload":  responseMap{"organization_id": orgID, "created_by": userID},
		},
	})
	c.post("/erp/MCRD", responseMap{
		"key": supplierKey,
		"data": responseMap{
			"CardCode": supplierKey,
			"CardName": "Smoke Supplier " + stamp,
			"CardType": "S",
		},
	})
	c.post("/erp/MITM", responseMap{
		"key": itemKey,
		"data": responseMap{
			"ItemCode":  itemKey,
			"ItemName":  "Smoke Item " + stamp,
			"InvntItem": "Y",
			"SellItem":  "Y",
		},
	})
	c.post("/erp/MWHS", responseMap{
		"key": warehouseKey,
		"data": responseMap{
			"WhsCode": warehouseKey,
			"WhsName": "Smoke Warehouse " + stamp,
		},
	})
	c.post("/erp/MREQ", responseMap{
		"key": requirementKey,
		"data": responseMap{
			"ReqCode": requirementKey,
			"Name":    "Smoke Requirement " + stamp,
			"Status":  "draft",
		},
	})
	c.post("/erp/MREQ/"+requirementKey+"/actions/analyze", responseMap{})
	approvedRequirement := c.post("/erp/MREQ/"+requirementKey+"/actions/approve", responseMap{"data": responseMap{"approver": userID}, "idempotency_key": "smoke-approve-" + requirementKey})
	requireActionContract(approvedRequirement, "requirement approve")
	converted := c.post("/erp/MREQ/"+requirementKey+"/actions/convert-to-project", responseMap{"data": responseMap{"PrjCode": projectKey}, "idempotency_key": "smoke-convert-" + requirementKey})
	requireActionContract(converted, "requirement convert")
	must(len(asList(converted["generated_records"])) > 0, "requirement conversion did not generate project")
	requireGeneratedProvenance(converted, "MPRJ")

	c.post("/erp/MPOR", responseMap{
		"key": purchaseOrderKey,
		"data": responseMap{
			"DocEntry":  purchaseOrderKey,
			"DocNum":    purchaseOrderKey,
			"CardCode":  supplierKey,
			"DocStatus": "O",
			"WddStatus": "W",
			"Project":   projectKey,
		},
	})
	c.post("/erp/MPOR/"+purchaseOrderKey+"/POR1", responseMap{
		"key": "1",
		"data": responseMap{
			"LineNum":    1,
			"LineStatus": "O",
			"Payload": responseMap{
				"ItemCode": itemKey,
				"Quantity": 5,
				"WhsCode":  warehouseKey,
				"Price":    20,
			},
		},
	})
	c.post("/erp/MPOR/"+purchaseOrderKey+"/actions/submit", responseMap{})
	c.post("/erp/MPOR/"+purchaseOrderKey+"/actions/approve", responseMap{})
	c.post("/erp/MPDN", responseMap{
		"key": goodsReceiptKey,
		"data": responseMap{
			"DocEntry":  goodsReceiptKey,
			"DocNum":    goodsReceiptKey,
			"CardCode":  supplierKey,
			"DocStatus": "O",
			"Project":   projectKey,
		},
	})
	c.post("/erp/MPDN/"+goodsReceiptKey+"/PDN1", responseMap{
		"key": "1",
		"data": responseMap{
			"LineNum":    1,
			"LineStatus": "O",
			"Payload": responseMap{
				"ItemCode": itemKey,
				"Quantity": 5,
				"WhsCode":  warehouseKey,
				"Price":    20,
			},
		},
	})
	receiptPost := c.post("/erp/MPDN/"+goodsReceiptKey+"/actions/post", responseMap{"idempotency_key": "smoke-receipt-" + goodsReceiptKey})
	requireActionContract(receiptPost, "goods receipt post")
	must(hasGeneratedRecord(receiptPost, "MIGN"), "goods receipt did not generate MIGN")
	must(hasGeneratedRecord(receiptPost, "MPCH"), "goods receipt did not generate MPCH")
	requireGeneratedProvenance(receiptPost, "MIGN")
	balanceKey := url.PathEscape(itemKey + "|" + warehouseKey)
	must(numberField(c.get("/erp/MITW/" + balanceKey)["data"].(map[string]any), "OnHand") == 5, "inventory balance after receipt is not 5")

	c.post("/erp/MRDR", responseMap{
		"key": salesOrderKey,
		"data": responseMap{
			"DocEntry":  salesOrderKey,
			"DocNum":    salesOrderKey,
			"CardCode":  partnerKey,
			"CardName":  "Smoke Customer " + stamp,
			"DocStatus": "O",
			"WddStatus": "W",
			"Confirmed": "N",
			"DocTotal":  100,
			"Project":   projectKey,
		},
	})
	c.post("/erp/MRDR/"+salesOrderKey+"/RDR1", responseMap{
		"key": "1",
		"data": responseMap{
			"LineNum":    1,
			"LineStatus": "O",
			"Payload": responseMap{
				"ItemCode": itemKey,
				"Quantity": 2,
				"WhsCode":  warehouseKey,
				"Price":    25,
			},
		},
	})
	c.post("/erp/MRDR/"+salesOrderKey+"/actions/confirm", responseMap{})
	c.post("/erp/MRDR/"+salesOrderKey+"/actions/approve", responseMap{})
	c.post("/erp/MDLN", responseMap{
		"key": deliveryKey,
		"data": responseMap{
			"DocEntry":  deliveryKey,
			"DocNum":    deliveryKey,
			"CardCode":  partnerKey,
			"DocStatus": "O",
			"Project":   projectKey,
		},
	})
	c.post("/erp/MDLN/"+deliveryKey+"/DLN1", responseMap{
		"key": "1",
		"data": responseMap{
			"LineNum":    1,
			"LineStatus": "O",
			"Payload": responseMap{
				"BaseType":  "MRDR",
				"BaseEntry": salesOrderKey,
				"ItemCode":  itemKey,
				"Quantity":  2,
				"WhsCode":   warehouseKey,
				"Price":     25,
			},
		},
	})
	deliveryPost := c.post("/erp/MDLN/"+deliveryKey+"/actions/post", responseMap{"idempotency_key": "smoke-delivery-" + deliveryKey})
	requireActionContract(deliveryPost, "delivery post")
	must(hasGeneratedRecord(deliveryPost, "MIGE"), "delivery did not generate MIGE")
	must(hasGeneratedRecord(deliveryPost, "MINV"), "delivery did not generate MINV")
	requireGeneratedProvenance(deliveryPost, "MIGE")
	requireGeneratedProvenance(deliveryPost, "MINV")
	invoiceKey := generatedRecordKey(deliveryPost, "MINV")
	must(invoiceKey != "", "missing generated invoice key")
	must(numberField(c.get("/erp/MITW/" + balanceKey)["data"].(map[string]any), "OnHand") == 3, "inventory balance after delivery is not 3")
	c.post("/erp/MINV/"+invoiceKey+"/actions/post", responseMap{})
	c.post("/erp/MRCT", responseMap{
		"key": paymentKey,
		"data": responseMap{
			"DocEntry": paymentKey,
			"DocNum":   paymentKey,
			"CardCode": partnerKey,
			"DocTotal": 50,
			"OpenBal":  50,
		},
	})
	allocation := c.post("/erp/MRCT/"+paymentKey+"/actions/allocate", responseMap{
		"data": responseMap{
			"TargetTable": "MINV",
			"TargetKey":   invoiceKey,
			"Amount":      50,
		},
	})
	must(numberField(allocation["effects"].(map[string]any), "allocated_amount") == 50, "payment allocation failed")
	costRefresh := c.post("/erp/MPRJ/"+projectKey+"/actions/refresh-cost", responseMap{"idempotency_key": "smoke-cost-" + projectKey})
	requireActionContract(costRefresh, "project cost refresh")
	feedback := c.post("/erp/MPRJ/"+projectKey+"/actions/close-feedback", responseMap{"data": responseMap{"result": "accepted"}, "idempotency_key": "smoke-feedback-" + projectKey})
	requireActionContract(feedback, "project feedback close")
	must(hasGeneratedRecord(feedback, "MFDB"), "project feedback close did not generate MFDB")
	must(stringField(c.get("/erp/MCRD/"+partnerKey), "key") == partnerKey, "missing ERP partner")
	must(len(asList(c.get("/erp/MINV/" + invoiceKey + "/INV1")["records"])) > 0, "missing ERP invoice rows")

	return erpSmokeResult{
		PartnerKey:    partnerKey,
		ItemKey:       itemKey,
		WarehouseKey:  warehouseKey,
		ProjectKey:    projectKey,
		PurchaseKey:   purchaseOrderKey,
		SalesOrderKey: salesOrderKey,
		InvoiceKey:    invoiceKey,
	}
}

func runSupplyChainSmoke(c *client, stamp string, orgID string, deptID string) erpSmokeResult {
	return runERPSmoke(c, stamp, orgID, "")
}

func runCostingSmoke(c *client, stamp string, orgID string, deptID string, projectID string, userID string) {
	_ = runERPSmoke(c, stamp, orgID, userID)
}

func runFinanceSmoke(c *client, stamp string, orgID string, deptID string, projectID string, requirementID string, deliverableID string) {
	_ = runERPSmoke(c, stamp, orgID, "")
}

func runSystemAdminSmoke(base string, orgID string, stamp string) {
	adminEmail := envOrDefault("SMOKE_PLATFORM_ADMIN_EMAIL", "admin-smoke@meta-org.local")
	adminPassword := envOrDefault("SMOKE_PLATFORM_ADMIN_PASSWORD", "AdminSmoke123!")
	admin := &client{base: base, http: &http.Client{Timeout: 20 * time.Second}}
	login := admin.post("/auth/login", responseMap{"email": adminEmail, "password": adminPassword})
	admin.token = stringField(login, "token")
	must(admin.token != "", "missing platform admin token")

	targets := admin.get("/platform/admin/industry-solution-targets")
	must(asList(targets["items"]) != nil, "industry solution targets did not return a list")
	masters := admin.get("/platform/admin/modules/data_catalog/masters")
	must(len(asList(masters["items"])) > 0, "missing data catalog platform masters")
	exported := admin.get("/platform/admin/organizations/" + orgID + "/industry-solution-manifest/export")
	must(stringField(exported, "format_version") != "", "missing exported industry solution manifest")
	change := admin.post("/platform/admin/organizations/"+orgID+"/industry-solution-change-requests", responseMap{
		"request_type":      "smoke_solution_validation",
		"reason":            "smoke industry solution validation " + stamp,
		"solution_manifest": exported,
	})
	changeID := stringField(change, "id")
	must(changeID != "", "missing industry solution change request id")
	change = admin.post("/platform/admin/industry-solution-change-requests/"+changeID+"/approve", responseMap{"reason": "smoke approval"})
	must(stringField(change, "status") == "approved", "industry solution change was not approved")
	job := admin.post("/platform/admin/industry-solution-change-requests/"+changeID+"/apply", responseMap{})
	must(stringField(job, "status") == "applied", "industry solution change was not applied")
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
		(strings.HasPrefix(path, "/erp/") && !strings.Contains(path, "/actions/")) ||
		path == "/organizations" ||
		path == "/workflows/templates" ||
		path == "/costing/currencies" ||
		path == "/costing/exchange-rates" ||
		path == "/costing/rate-cards" ||
		path == "/costing/ledger-entries" ||
		path == "/costing/budgets" ||
		(strings.HasPrefix(path, "/platform/admin/organizations/") && strings.HasSuffix(path, "/industry-solution-change-requests")) ||
		(strings.HasPrefix(path, "/organizations/") && strings.HasSuffix(path, "/departments"))
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

func requireActionContract(result responseMap, label string) {
	must(fmt.Sprint(result["execution_id"]) != "" && fmt.Sprint(result["execution_id"]) != "<nil>", label+" missing execution_id")
	must(fmt.Sprint(result["idempotency_key"]) != "" && fmt.Sprint(result["idempotency_key"]) != "<nil>", label+" missing idempotency_key")
	preconditions := asList(result["preconditions_checked"])
	must(len(preconditions) > 0, label+" missing preconditions_checked")
}

func requireGeneratedProvenance(result responseMap, tableCode string) {
	for _, item := range asList(result["generated_records"]) {
		record, ok := asMap(item)
		if !ok {
			continue
		}
		if fmt.Sprint(record["table_code"]) != tableCode {
			continue
		}
		data, _ := asMap(record["data"])
		provenance, _ := asMap(data["provenance"])
		must(fmt.Sprint(provenance["source_table_code"]) != "", "missing provenance source_table_code for "+tableCode)
		return
	}
	must(false, "missing generated table "+tableCode)
}

func hasGeneratedRecord(result responseMap, tableCode string) bool {
	return generatedRecordKey(result, tableCode) != ""
}

func generatedRecordKey(result responseMap, tableCode string) string {
	for _, raw := range asList(result["generated_records"]) {
		record, ok := asMap(raw)
		if !ok {
			continue
		}
		if stringField(record, "table_code") == tableCode {
			return stringField(record, "key")
		}
	}
	return ""
}

func asMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case responseMap:
		return map[string]any(typed), true
	default:
		return nil, false
	}
}

func must(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
