package toolruntime

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRegisterPlatformRoutesMountsApprovalReviewRoutes(t *testing.T) {
	router := chi.NewRouter()
	NewHandler(nil).RegisterPlatformRoutes(router)

	tests := []struct {
		name string
		path string
	}{
		{name: "approve", path: "/platform/admin/tool-approvals/not-a-uuid/approve"},
		{name: "reject", path: "/platform/admin/tool-approvals/not-a-uuid/reject"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("platform approval route returned 404")
			}
		})
	}
}
