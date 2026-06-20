package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type staticTenantResolver struct {
	tenant *TenantContext
	err    error
}

func (r staticTenantResolver) ResolveTenant(ctx context.Context, user AuthenticatedUser, requestedOrganizationID string) (*TenantContext, error) {
	return r.tenant, r.err
}

func TestTenantMiddlewareModuleGate(t *testing.T) {
	orgID := uuid.New()
	baseTenant := &TenantContext{
		Mode:           "saas",
		UserID:         uuid.New(),
		OrganizationID: &orgID,
		EnabledModules: map[string]bool{
			"organization": false,
			"project":      false,
			"inventory":    false,
			"procurement":  false,
			"sales":        false,
		},
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "blocks disabled business module route",
			path:       "/api/v1/projects",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "blocks disabled agent management route",
			path:       "/api/v1/agents",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "blocks disabled inventory module route",
			path:       "/api/v1/inventory/items",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "blocks disabled procurement module route",
			path:       "/api/v1/procurement/orders",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "blocks disabled sales module route",
			path:       "/api/v1/sales/orders",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "allows saas module management route",
			path:       "/api/v1/organizations/" + orgID.String() + "/modules",
			wantStatus: http.StatusOK,
		},
		{
			name:       "allows saas invitation route",
			path:       "/api/v1/organizations/" + orgID.String() + "/invitations",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler := TenantMiddleware(staticTenantResolver{tenant: baseTenant})(next)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req = req.WithContext(context.WithValue(req.Context(), UserContextKey, AuthenticatedUser{
				ID:   uuid.New().String(),
				Type: "human",
				Name: "Tester",
			}))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestTenantMiddlewareOnboardingRequired(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TenantMiddleware(staticTenantResolver{err: ErrOnboardingRequired})(next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, AuthenticatedUser{
		ID:   uuid.New().String(),
		Type: "human",
		Name: "Tester",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusPreconditionRequired)
	}
}
