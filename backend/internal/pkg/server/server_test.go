package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRouterExposesAPIContractHeadersForCORS(t *testing.T) {
	router := NewRouter([]string{"http://localhost:3000"})
	router.Get("/ok", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "request-id")
		w.Header().Set("X-Error-Code", "invalid_request")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/ok", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Organization-ID, X-Request-ID")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200", resp.Code)
	}
	allowed := resp.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(strings.ToLower(allowed), "x-organization-id") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want X-Organization-ID", allowed)
	}
	if !strings.Contains(strings.ToLower(allowed), "x-request-id") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want X-Request-ID", allowed)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/ok", nil)
	getRequest.Header.Set("Origin", "http://localhost:3000")
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)
	exposed := strings.ToLower(getResponse.Header().Get("Access-Control-Expose-Headers"))
	for _, header := range []string{"x-request-id", "x-error-code", "retry-after"} {
		if !strings.Contains(exposed, header) {
			t.Fatalf("Access-Control-Expose-Headers = %q, want %s", exposed, header)
		}
	}
}
