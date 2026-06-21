package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRouterAllowsOrganizationHeaderForCORS(t *testing.T) {
	router := NewRouter([]string{"http://localhost:3000"})
	router.Get("/ok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/ok", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Organization-ID")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200", resp.Code)
	}
	allowed := resp.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(strings.ToLower(allowed), "x-organization-id") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want X-Organization-ID", allowed)
	}
}
