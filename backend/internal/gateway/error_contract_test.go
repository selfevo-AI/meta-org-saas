package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestInternalAPIRoutesUseStableNotFoundContract(t *testing.T) {
	router := chi.NewRouter()
	RegisterRoutes(router, &Dependencies{})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))

	if recorder.Code != http.StatusNotFound || recorder.Header().Get("X-Error-Code") != "not_found" || recorder.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status/headers = %d %#v", recorder.Code, recorder.Header())
	}
	var response struct {
		Error     string `json:"error"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error != "route not found" || response.Code != "not_found" || response.RequestID == "" {
		t.Fatalf("response = %#v", response)
	}
}

func TestInternalAPIRoutesUseStableMethodContract(t *testing.T) {
	router := chi.NewRouter()
	RegisterRoutes(router, &Dependencies{})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))

	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("X-Error-Code") != "method_not_allowed" {
		t.Fatalf("response = %d %#v %s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}
