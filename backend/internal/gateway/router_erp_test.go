package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
)

func TestRegisterRoutesIncludesERPCatalog(t *testing.T) {
	router := chi.NewMux()
	RegisterRoutes(router, &Dependencies{
		JWTSecret:  "test-secret",
		ErpHandler: erp.NewHandler(erp.NewService(&gatewayERPRepo{}, erp.DefaultCatalog())),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/erp/catalog", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code == http.StatusNotFound {
		t.Fatalf("GET /api/v1/erp/catalog status = 404, want registered route")
	}
}

type gatewayERPRepo struct{}

func (r *gatewayERPRepo) ListRecords(ctx context.Context, table erp.TableDefinition, limit int) ([]erp.Record, error) {
	return []erp.Record{}, nil
}

func (r *gatewayERPRepo) CreateRecord(ctx context.Context, table erp.TableDefinition, input erp.RecordInput) (*erp.Record, error) {
	return &erp.Record{TableCode: table.Code, Key: input.Key, Data: input.Data}, nil
}

func (r *gatewayERPRepo) GetRecord(ctx context.Context, table erp.TableDefinition, key string) (*erp.Record, error) {
	return &erp.Record{TableCode: table.Code, Key: key, Data: map[string]any{}}, nil
}

func (r *gatewayERPRepo) UpdateRecord(ctx context.Context, table erp.TableDefinition, key string, input erp.RecordInput) (*erp.Record, error) {
	return &erp.Record{TableCode: table.Code, Key: key, Data: input.Data}, nil
}

func (r *gatewayERPRepo) DeleteRecord(ctx context.Context, table erp.TableDefinition, key string) error {
	return nil
}

func (r *gatewayERPRepo) ListChildRecords(ctx context.Context, parent erp.TableDefinition, child erp.ChildTableDefinition, parentKey string, limit int) ([]erp.Record, error) {
	return []erp.Record{}, nil
}

func (r *gatewayERPRepo) CreateChildRecord(ctx context.Context, parent erp.TableDefinition, child erp.ChildTableDefinition, parentKey string, input erp.RecordInput) (*erp.Record, error) {
	return &erp.Record{TableCode: child.Code, ParentTableCode: parent.Code, ParentKey: parentKey, Key: input.Key, Data: input.Data}, nil
}
