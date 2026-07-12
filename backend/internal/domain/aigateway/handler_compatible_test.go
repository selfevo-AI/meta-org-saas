package aigateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestCompatibleChatCompletionsUsesOrganizationAccessToken(t *testing.T) {
	repo := newFakeGatewayRepo()
	repo.accessToken = AccessTokenContext{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AllowedModels:  []string{"gpt-test"},
		Status:         "active",
	}
	repo.reservation = BalanceReservation{ID: uuid.New(), ReservedAmount: 0.001, Currency: "CNY"}
	svc := NewService(repo, AdapterRegistry{ProviderOpenAI: fakeAdapter{resp: ProviderResponse{
		ProviderRequestID: "req_compatible",
		Content:           "hello",
		Usage:             TokenUsage{InputTokens: 4, OutputTokens: 5},
	}}})
	router := chi.NewRouter()
	NewHandler(svc).RegisterCompatibleRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-test",
		"messages":[{"role":"user","content":"hi"}],
		"max_tokens":16
	}`))
	req.Header.Set("Authorization", "Bearer ak-org")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"req_compatible"`) {
		t.Fatalf("response body = %s, want compatible id", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"content":"hello"`) {
		t.Fatalf("response body = %s, want assistant content", rec.Body.String())
	}
	if !repo.reserved || !repo.settled {
		t.Fatalf("compatible route did not reserve and settle organization balance")
	}
}

func TestCompatibleChatCompletionsRequiresBearerToken(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(newFakeGatewayRepo(), nil)).RegisterCompatibleRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s, want 401", rec.Code, rec.Body.String())
	}
}

func TestCompatibleEmbeddingEndpointReturnsStructuredUnsupportedOperation(t *testing.T) {
	repo := newFakeGatewayRepo()
	repo.accessToken = AccessTokenContext{ID: uuid.New(), OrganizationID: uuid.New(), Status: "active"}
	router := chi.NewRouter()
	NewHandler(NewService(repo, nil)).RegisterCompatibleRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"gpt-test","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer ak-org")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, body = %s, want 501", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"unsupported_operation"`) {
		t.Fatalf("response body = %s, want unsupported_operation error", rec.Body.String())
	}
}

func TestCompatibleListModelsAuthenticatesAndFiltersAccessTokenModels(t *testing.T) {
	groupID := uuid.New()
	repo := newFakeGatewayRepo()
	repo.accessToken = AccessTokenContext{
		ID:                   uuid.New(),
		OrganizationID:       uuid.New(),
		ModelGroupID:         &groupID,
		AllowedModelPatterns: []string{"gpt-*"},
		Status:               "active",
	}
	repo.catalogModels = []Model{
		{ID: uuid.New(), ProviderID: uuid.New(), ModelKey: "gpt-allowed", Status: "active"},
		{ID: uuid.New(), ProviderID: uuid.New(), ModelKey: "gpt-no-channel", Status: "active"},
		{ID: uuid.New(), ProviderID: uuid.New(), ModelKey: "claude-hidden", Status: "active"},
		{ID: uuid.New(), ProviderID: uuid.New(), ModelKey: "gpt-disabled", Status: "inactive"},
	}
	repo.abilities = []ModelChannelAbility{{ModelGroupID: &groupID, ModelPattern: "gpt-allowed", Enabled: true}}
	router := chi.NewRouter()
	NewHandler(NewService(repo, nil)).RegisterCompatibleRoutes(router)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer ak-org")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"id":"gpt-allowed"`) || strings.Contains(recorder.Body.String(), "gpt-no-channel") || strings.Contains(recorder.Body.String(), "claude-hidden") || strings.Contains(recorder.Body.String(), "gpt-disabled") {
		t.Fatalf("filtered model response = %s", recorder.Body.String())
	}
}

func TestCompatibleCatalogAndUnsupportedRoutesRejectInvalidAccessToken(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(NewService(newFakeGatewayRepo(), nil)).RegisterCompatibleRoutes(router)
	for _, path := range []string{"/v1/models", "/v1/embeddings"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if path != "/v1/models" {
			request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		}
		request.Header.Set("Authorization", "Bearer invalid-token")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d, body = %s, want 403", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestCompatibleRoutesApplyConfiguredRateLimitMiddleware(t *testing.T) {
	router := chi.NewRouter()
	limit := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
		})
	}
	NewHandler(NewService(newFakeGatewayRepo(), nil)).RegisterCompatibleRoutes(router, limit)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
}
