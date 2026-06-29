package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
)

func TestRegisterRoutesMountsOpenAICompatibleGatewayAtRootV1(t *testing.T) {
	router := chi.NewMux()
	RegisterRoutes(router, &Dependencies{
		AIGatewayHandler: aigateway.NewHandler(nil),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer ak-test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("/v1/models was not mounted")
	}
}
