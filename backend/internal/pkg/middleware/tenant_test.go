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
			name:       "blocks disabled erp module route",
			path:       "/api/v1/erp/catalog",
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

func TestTenantMiddlewareInvalidOrganization(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TenantMiddleware(staticTenantResolver{err: ErrTenantInvalid})(next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, AuthenticatedUser{
		ID:   uuid.New().String(),
		Type: "human",
		Name: "Tester",
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if got := rr.Body.String(); got != "{\"error\":\"invalid_organization\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestTenantMiddlewarePlatformTenantPermissions(t *testing.T) {
	orgID := uuid.New()
	tests := []struct {
		name        string
		method      string
		permissions map[string]bool
		wantStatus  int
	}{
		{
			name:        "auditor can read tenant data",
			method:      http.MethodGet,
			permissions: map[string]bool{"tenant.data.read": true},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "auditor cannot mutate tenant data",
			method:      http.MethodPost,
			permissions: map[string]bool{"tenant.data.read": true},
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "operator can mutate tenant data",
			method:      http.MethodPost,
			permissions: map[string]bool{"tenant.data.read": true, "tenant.data.manage": true},
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler := TenantMiddleware(staticTenantResolver{tenant: &TenantContext{
				Mode:                "saas",
				UserID:              uuid.New(),
				OrganizationID:      &orgID,
				IsPlatformAdmin:     true,
				PlatformRole:        "auditor",
				PlatformPermissions: tt.permissions,
				EnabledModules:      map[string]bool{"project": true},
			}})(next)
			req := httptest.NewRequest(tt.method, "/api/v1/projects", nil)
			req = req.WithContext(context.WithValue(req.Context(), UserContextKey, AuthenticatedUser{
				ID:   uuid.New().String(),
				Type: "human",
			}))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}
