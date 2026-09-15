package ontology

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
)

type fakeBusiness struct {
	denied   map[string]bool
	query    erp.RecordQuery
	table    string
	input    erp.ActionInput
	executed int
}

func (f *fakeBusiness) Catalog(context.Context) erp.Catalog { return erp.DefaultCatalog() }
func (f *fakeBusiness) Actions(context.Context) []erp.ActionDefinition {
	return erp.DefaultActionRegistry().List()
}
func (f *fakeBusiness) CheckAccess(_ context.Context, table, _ string) error {
	if f.denied[table] {
		return erp.ErrForbidden
	}
	return nil
}
func (f *fakeBusiness) GetRecord(_ context.Context, table, key string) (*erp.Record, error) {
	return &erp.Record{TableCode: table, Key: key, Data: map[string]any{"CardCode": "vendor", "DocCur": "CNY", "DocTotal": 100, "FulfillmentEntry": "receipt"}}, nil
}
func (f *fakeBusiness) QueryRecords(_ context.Context, table string, query erp.RecordQuery) (*erp.RecordPage, error) {
	f.table, f.query = table, query
	return &erp.RecordPage{Records: []erp.Record{{Key: "invoice", Data: map[string]any{"DocTotal": 100}}}, NextCursor: "invoice"}, nil
}
func (f *fakeBusiness) ListChildRecords(context.Context, string, string, string, int) ([]erp.Record, error) {
	return nil, nil
}
func (f *fakeBusiness) ListActionExecutions(context.Context, string, string, int) ([]erp.ActionExecution, error) {
	return []erp.ActionExecution{}, nil
}
func (f *fakeBusiness) RunAction(_ context.Context, table, key, action string, input erp.ActionInput) (*erp.ActionResult, error) {
	f.input, f.executed = input, f.executed+1
	return &erp.ActionResult{TableCode: table, Key: key, Action: action, Status: "posted"}, nil
}

func TestSemanticQueryUsesAuthoritativeFields(t *testing.T) {
	business := &fakeBusiness{}
	service := NewService(business)
	page, err := service.Query(context.Background(), "payable_invoice", QueryInput{Filters: map[string]any{"currency": "CNY", "posted": "Y"}, Cursor: "previous", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if business.table != "MPCH" || business.query.Filters["DocCur"] != "CNY" || business.query.Filters["Posted"] != "Y" || business.query.After != "previous" || business.query.Limit != 25 {
		t.Fatalf("unexpected query: %s %#v", business.table, business.query)
	}
	if page.NextCursor != "invoice" || page.Objects[0].Properties["total"] != 100 {
		t.Fatalf("unexpected page: %#v", page)
	}
	if _, err := service.Query(context.Background(), "payable_invoice", QueryInput{Filters: map[string]any{"DocCur": "USD"}}); !errors.Is(err, erp.ErrValidation) {
		t.Fatalf("raw field filter accepted: %v", err)
	}
}

func TestOntologyDoesNotExposeForbiddenTypesOrLinks(t *testing.T) {
	business := &fakeBusiness{denied: map[string]bool{"MCRD": true, "MJDT": true, "MPDN": true}}
	service := NewService(business)
	types, err := service.Types(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, objectType := range types {
		if business.denied[objectType.TableCode] {
			t.Fatalf("forbidden type exposed: %s", objectType.Key)
		}
	}
	links, err := service.Links(context.Background(), "purchase_order", "order")
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Links) != 0 {
		t.Fatalf("forbidden link exposed: %#v", links)
	}
	if _, err := service.Get(context.Background(), "account-does-not-exist", "id"); !errors.Is(err, erp.ErrNotFound) {
		t.Fatalf("unknown type: %v", err)
	}
	if _, err := service.Execute(context.Background(), "goods_receipt", "id", "post", erp.ActionInput{}); !errors.Is(err, erp.ErrForbidden) || business.executed != 0 {
		t.Fatal("forbidden object executed")
	}
}

func TestOntologyHTTPRejectsMalformedInputAndIgnoresSpoofedActor(t *testing.T) {
	business := &fakeBusiness{}
	router := chi.NewRouter()
	NewHandler(NewService(business)).RegisterRoutes(router)
	for _, body := range []string{`{"unknown":true}`, `{} {}`, `{"data":`, `{"data":"bad"}`} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/ontology/objects/payable_invoice/id/actions/post", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/ontology/objects/payable_invoice/id/actions/post", strings.NewReader(`{"actor_type":"human","source":"trusted","actor_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","tool_execution_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}`))
	request.Header.Set("Idempotency-Key", "client-retry")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("execute: %d %s", response.Code, response.Body)
	}
	if business.input.ActorID != nil || business.input.ToolExecutionID != nil || business.input.ActorType != "" || business.input.Source != "ontology_api" || business.input.IdempotencyKey != "client-retry" {
		t.Fatalf("untrusted attribution was accepted: %#v", business.input)
	}
}
