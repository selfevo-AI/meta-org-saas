package aigateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestCreateAccessTokenRouteReturnsOneTimePlainToken(t *testing.T) {
	repo := newFakeGatewayRepo()
	router := chi.NewRouter()
	NewHandler(NewService(repo, nil)).RegisterRoutes(router)
	orgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/ai-gateway/access-tokens", strings.NewReader(`{
		"organization_id":"`+orgID.String()+`",
		"name":"ERP integration",
		"allowed_models":["gpt-*"]
	}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"plain_token":"ak-meta-`) {
		t.Fatalf("response body = %s, want one-time plain token", rec.Body.String())
	}
	if repo.lastTokenStore.OrganizationID != orgID {
		t.Fatalf("stored org = %s, want %s", repo.lastTokenStore.OrganizationID, orgID)
	}
}

func TestAdjustGatewayBalanceRouteRecordsManualTopUp(t *testing.T) {
	repo := newFakeGatewayRepo()
	router := chi.NewRouter()
	NewHandler(NewService(repo, nil)).RegisterRoutes(router)
	orgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/ai-gateway/balance-adjustments", strings.NewReader(`{
		"organization_id":"`+orgID.String()+`",
		"amount":250,
		"currency":"CNY",
		"reason":"finance_top_up"
	}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"balance_amount":250`) {
		t.Fatalf("response body = %s, want adjusted balance", rec.Body.String())
	}
	if repo.lastAdjustment.OrganizationID != orgID || repo.lastAdjustment.Amount != 250 {
		t.Fatalf("adjustment = %#v, want org and amount", repo.lastAdjustment)
	}
}
