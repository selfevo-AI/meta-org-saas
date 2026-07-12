package saas

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestPublicInvitationRouteAppliesConfiguredRateLimitMiddleware(t *testing.T) {
	router := chi.NewRouter()
	limit := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
		})
	}
	NewHandler(nil).RegisterPublicRoutes(router, limit)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/auth/invitations/token/accept", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
}
