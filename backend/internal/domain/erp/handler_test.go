package erp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestHandlerReturnsCatalog(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(&fakeRepository{}, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/erp/catalog", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /erp/catalog status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"code":"MINV"`) {
		t.Fatalf("GET /erp/catalog body = %s, want MINV table", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"modules"`) || !strings.Contains(resp.Body.String(), `"name":"Purchase Order"`) {
		t.Fatalf("GET /erp/catalog body = %s, want module hierarchy with Purchase Order", resp.Body.String())
	}
}

func TestHandlerReturnsActionCatalog(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(&fakeRepository{}, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/erp/actions", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /erp/actions status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"table_code":"MPOR"`) || !strings.Contains(resp.Body.String(), `"action":"approve"`) {
		t.Fatalf("GET /erp/actions body = %s, want MPOR approve action", resp.Body.String())
	}
}

func TestHandlerRunsERPAction(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(&fakeRepository{}, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/erp/MPOR/1001/actions/submit", strings.NewReader(`{"data":{"actor":"u1"}}`))
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("POST /erp/MPOR/1001/actions/submit status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"table_code":"MPOR"`) || !strings.Contains(resp.Body.String(), `"action":"submit"`) {
		t.Fatalf("POST /erp/MPOR/1001/actions/submit body = %s, want action result", resp.Body.String())
	}
}

func TestHandlerReturnsActionExecutionTimeline(t *testing.T) {
	repo := &fakeRepository{}
	repo.ensureExecutionLedger()
	executionID := uuid.New()
	repo.executions[executionID] = ActionExecution{
		ID:             executionID,
		TableCode:      "MREQ",
		RecordKey:      "REQ-1001",
		Action:         "convert-to-project",
		Status:         ActionExecutionCompleted,
		IdempotencyKey: "erp:MREQ:REQ-1001:convert-to-project:PRJ-1001",
		Payload:        map[string]any{"status": "converted"},
	}
	repo.generatedRecords[executionID] = []ActionGeneratedRecord{
		{ActionID: executionID, LineNum: 1, GeneratedTableCode: "MPRJ", GeneratedKey: "PRJ-1001", RelationType: "created"},
	}
	router := chi.NewRouter()
	NewHandler(NewService(repo, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/erp/MREQ/REQ-1001/action-executions?limit=5", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /erp/MREQ/REQ-1001/action-executions status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"action_executions"`) || !strings.Contains(body, `"generated_table_code":"MPRJ"`) {
		t.Fatalf("GET /erp/MREQ/REQ-1001/action-executions body = %s, want timeline with generated MPRJ", body)
	}
}

func TestHandlerCreatesRecordByTableCode(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(&fakeRepository{}, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/erp/MCRD", strings.NewReader(`{"key":"C0001","data":{"CardCode":"C0001","CardType":"C","CardName":"Acme"}}`))
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /erp/MCRD status = %d, want 201: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"table_code":"MCRD"`) {
		t.Fatalf("POST /erp/MCRD body = %s, want MCRD record", resp.Body.String())
	}
}

func TestHandlerUpdatesChildRecord(t *testing.T) {
	repo := &fakeRepository{}
	router := chi.NewRouter()
	NewHandler(NewService(repo, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPatch, "/erp/MPOR/1001/POR1/1", strings.NewReader(`{"data":{"Payload":{"ItemCode":"I-1001","Quantity":3}}}`))
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH /erp/MPOR/1001/POR1/1 status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if repo.updatedChild.Key != "1" || repo.updatedChild.ParentKey != "1001" || repo.updatedChild.TableCode != "POR1" {
		t.Fatalf("updated child = %#v, want POR1 line 1 under MPOR/1001", repo.updatedChild)
	}
	if !strings.Contains(resp.Body.String(), `"table_code":"POR1"`) {
		t.Fatalf("PATCH child body = %s, want POR1 record", resp.Body.String())
	}
}

func TestHandlerDeletesChildRecord(t *testing.T) {
	repo := &fakeRepository{}
	router := chi.NewRouter()
	NewHandler(NewService(repo, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodDelete, "/erp/MPOR/1001/POR1/1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("DELETE /erp/MPOR/1001/POR1/1 status = %d, want 204: %s", resp.Code, resp.Body.String())
	}
	if repo.deletedChildTable != "POR1" || repo.deletedChildParentKey != "1001" || repo.deletedChildKey != "1" {
		t.Fatalf("deleted child = %s/%s/%s, want POR1/1001/1", repo.deletedChildTable, repo.deletedChildParentKey, repo.deletedChildKey)
	}
}

func TestHandlerRejectsUnknownTable(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(&fakeRepository{}, DefaultCatalog())).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/erp/NOPE", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /erp/NOPE status = %d, want 400: %s", resp.Code, resp.Body.String())
	}
}
