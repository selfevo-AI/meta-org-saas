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
	router := chi.NewRouter()
	NewHandler(NewService(newFakeGatewayRepo(), nil)).RegisterCompatibleRoutes(router)

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
