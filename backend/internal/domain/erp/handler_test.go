package erp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
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
