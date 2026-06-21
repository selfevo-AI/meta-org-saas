package gateway

import (
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/aigateway"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/runtime"
)

func TestRegisterRoutesDoesNotPanicWithPlatformAdminHandlers(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("RegisterRoutes panicked with platform admin handlers: %v", recovered)
		}
	}()

	RegisterRoutes(chi.NewMux(), &Dependencies{
		JWTSecret:        "test-secret",
		RuntimeHandler:   runtime.NewHandler(nil),
		AIGatewayHandler: aigateway.NewHandler(nil),
	})
}
