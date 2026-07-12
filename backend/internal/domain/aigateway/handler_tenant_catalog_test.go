package aigateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestTenantCatalogReturnsOnlyActiveSanitizedEntries(t *testing.T) {
	activeProviderID := uuid.New()
	disabledProviderID := uuid.New()
	repo := newFakeGatewayRepo()
	repo.catalogProviders = []ModelProvider{
		{ID: activeProviderID, Name: "Tenant Available", ProviderType: ProviderOpenAI, Status: "active", BaseURL: "https://internal.example", MaskedAPIKey: "sk-secret", LastTestError: "private failure", Metadata: map[string]any{"internal": true}},
		{ID: disabledProviderID, Name: "Disabled Internal", ProviderType: ProviderOpenAI, Status: "disabled"},
	}
	repo.catalogModels = []Model{
		{ID: uuid.New(), ProviderID: activeProviderID, ModelKey: "active-model", DisplayName: "Active Model", Status: "active", Metadata: map[string]any{"route": "internal"}},
	}
	router := chi.NewRouter()
	NewHandler(NewService(repo, nil)).RegisterTenantRoutes(router)

	providerRecorder := httptest.NewRecorder()
	router.ServeHTTP(providerRecorder, httptest.NewRequest(http.MethodGet, "/model-providers", nil))
	if providerRecorder.Code != http.StatusOK {
		t.Fatalf("provider status = %d, body = %s", providerRecorder.Code, providerRecorder.Body.String())
	}
	providerBody := providerRecorder.Body.String()
	for _, forbidden := range []string{"base_url", "masked_api_key", "last_test_error", "metadata", "Disabled Internal", "private failure"} {
		if strings.Contains(providerBody, forbidden) {
			t.Fatalf("tenant provider body exposes %q: %s", forbidden, providerBody)
		}
	}

	modelRecorder := httptest.NewRecorder()
	router.ServeHTTP(modelRecorder, httptest.NewRequest(http.MethodGet, "/models", nil))
	if modelRecorder.Code != http.StatusOK {
		t.Fatalf("model status = %d, body = %s", modelRecorder.Code, modelRecorder.Body.String())
	}
	modelBody := modelRecorder.Body.String()
	if !strings.Contains(modelBody, `"model_key":"active-model"`) {
		t.Fatalf("tenant model body = %s, want active model", modelBody)
	}
	for _, forbidden := range []string{"metadata", "created_at", "updated_at", "internal"} {
		if strings.Contains(modelBody, forbidden) {
			t.Fatalf("tenant model body exposes %q: %s", forbidden, modelBody)
		}
	}
}
