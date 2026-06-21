package runtime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandlerListOperationsReturnsRuntimeCatalog(t *testing.T) {
	repo := &fakeRepository{
		operations: []OperationDefinition{
			{
				ID:         "organization-list",
				Domain:     "Organization",
				Title:      "operation.organization.list",
				Method:     "GET",
				Path:       "/runtime/entities/organization.organization/records",
				Auth:       true,
				Status:     StatusActive,
				ActionType: ActionCRUDList,
				EntityKey:  "organization.organization",
			},
		},
	}
	router := chi.NewRouter()
	NewHandler(NewService(repo)).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/runtime/operations", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /runtime/operations status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"id":"organization-list"`) {
		t.Fatalf("GET /runtime/operations body = %s, want operation id", resp.Body.String())
	}
}

func TestHandlerExecuteOperationCreatesConfiguredRecord(t *testing.T) {
	repo := &fakeRepository{
		operationByID: map[string]OperationDefinition{
			"organization-create": {
				ID:         "organization-create",
				Domain:     "Organization",
				Title:      "operation.organization.create",
				Method:     "POST",
				Path:       "/runtime/operations/organization-create/execute",
				Auth:       true,
				Status:     StatusActive,
				ActionType: ActionCRUDCreate,
				EntityKey:  "organization.organization",
			},
		},
		entityByKey: map[string]EntityDefinition{
			"organization.organization": {
				EntityKey:    "organization.organization",
				ModuleKey:    "organization",
				StorageTable: "organization_masters",
				EntityType:   "organization",
				Status:       StatusActive,
				Fields:       []FieldDefinition{{FieldKey: "title", DataType: "text", Required: true}},
			},
		},
		createdRecord: &RuntimeRecord{
			MasterKey:  "ORG-20260621-000001",
			EntityKey:  "organization.organization",
			EntityType: "organization",
			Title:      "Acme",
			Status:     StatusActive,
			Data:       map[string]any{"title": "Acme"},
			Metadata:   map[string]any{},
		},
	}
	router := chi.NewRouter()
	NewHandler(NewService(repo)).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/runtime/operations/organization-create/execute", strings.NewReader(`{"body":{"title":"Acme"}}`))
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("POST execute status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"status":"created"`) {
		t.Fatalf("POST execute body = %s, want created status", resp.Body.String())
	}
}
