package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type tenantContextKey string

const TenantContextKey tenantContextKey = "tenant"

var (
	ErrTenantRequired     = errors.New("organization context is required")
	ErrOnboardingRequired = errors.New("onboarding is required")
	ErrTenantForbidden    = errors.New("organization access forbidden")
)

type TenantContext struct {
	Mode             string
	UserID           uuid.UUID
	OrganizationID   *uuid.UUID
	IsPlatformAdmin  bool
	PlatformRole     string
	MembershipID     *uuid.UUID
	AuthorityTier    string
	EnabledModules   map[string]bool
	OnboardingStatus string

	TenantDatabaseDeploymentMode string
	TenantDatabaseClusterKey     string
	TenantDatabaseRegion         string
	TenantDatabaseName           string
	TenantDatabaseStatus         string
	TenantSchemaName             string
}

type TenantResolver interface {
	ResolveTenant(ctx context.Context, user AuthenticatedUser, requestedOrganizationID string) (*TenantContext, error)
}

func TenantFromContext(ctx context.Context) (*TenantContext, bool) {
	tenant, ok := ctx.Value(TenantContextKey).(*TenantContext)
	return tenant, ok
}

func TenantMiddleware(resolver TenantResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if resolver == nil {
				writeTenantError(w, http.StatusInternalServerError, "tenant resolver is not configured")
				return
			}
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeTenantError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			tenant, err := resolver.ResolveTenant(r.Context(), user, r.Header.Get("X-Organization-ID"))
			if err != nil {
				switch {
				case errors.Is(err, ErrOnboardingRequired):
					writeTenantError(w, http.StatusPreconditionRequired, "onboarding_required")
				case errors.Is(err, ErrTenantRequired):
					writeTenantError(w, http.StatusBadRequest, "organization_required")
				case errors.Is(err, ErrTenantForbidden):
					writeTenantError(w, http.StatusForbidden, "organization_forbidden")
				default:
					writeTenantError(w, http.StatusInternalServerError, "tenant_resolution_failed")
				}
				return
			}
			if moduleKey := moduleForPath(r.URL.Path); tenant.Mode == "saas" && moduleKey != "" && !tenant.EnabledModules[moduleKey] {
				writeTenantError(w, http.StatusForbidden, "module_disabled")
				return
			}
			ctx := context.WithValue(r.Context(), TenantContextKey, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func moduleForPath(path string) string {
	if isSaaSOrganizationAdminPath(path) {
		return ""
	}
	switch {
	case strings.HasPrefix(path, "/api/v1/organizations"),
		strings.HasPrefix(path, "/api/v1/organization"),
		strings.HasPrefix(path, "/api/v1/agents"),
		strings.HasPrefix(path, "/api/v1/departments"),
		strings.HasPrefix(path, "/api/v1/positions"),
		strings.HasPrefix(path, "/api/v1/position-assignments"),
		strings.HasPrefix(path, "/api/v1/memberships"),
		strings.HasPrefix(path, "/api/v1/external-members"),
		strings.HasPrefix(path, "/api/v1/muvrs"),
		strings.HasPrefix(path, "/api/v1/relationships"):
		return "organization"
	case strings.HasPrefix(path, "/api/v1/requirements"),
		strings.HasPrefix(path, "/api/v1/requirement-documents"),
		strings.HasPrefix(path, "/api/v1/projects"),
		strings.HasPrefix(path, "/api/v1/deliverables"):
		return "project"
	case strings.HasPrefix(path, "/api/v1/workflows"),
		strings.HasPrefix(path, "/api/v1/tasks"),
		strings.HasPrefix(path, "/api/v1/task-matrix-assignments"):
		return "workflow"
	case strings.HasPrefix(path, "/api/v1/governance"):
		return "governance"
	case strings.HasPrefix(path, "/api/v1/evolution"):
		return "evolution"
	case strings.HasPrefix(path, "/api/v1/capabilities"):
		return "capability"
	case strings.HasPrefix(path, "/api/v1/meta-resources"),
		strings.HasPrefix(path, "/api/v1/demand-profiles"),
		strings.HasPrefix(path, "/api/v1/pdca-"):
		return "meta_resource"
	case strings.HasPrefix(path, "/api/v1/assistant"):
		return "assistant"
	case strings.HasPrefix(path, "/api/v1/model-providers"),
		strings.HasPrefix(path, "/api/v1/model-provider-channels"),
		strings.HasPrefix(path, "/api/v1/models"),
		strings.HasPrefix(path, "/api/v1/ai-gateway"):
		return "ai_gateway"
	case strings.HasPrefix(path, "/api/v1/tools"),
		strings.HasPrefix(path, "/api/v1/tool-"),
		strings.HasPrefix(path, "/api/v1/interface-files"):
		return "toolruntime"
	case strings.HasPrefix(path, "/api/v1/finance"):
		return "finance"
	case strings.HasPrefix(path, "/api/v1/costing"):
		return "costing"
	case strings.HasPrefix(path, "/api/v1/inventory"):
		return "inventory"
	case strings.HasPrefix(path, "/api/v1/procurement"):
		return "procurement"
	case strings.HasPrefix(path, "/api/v1/sales"):
		return "sales"
	default:
		return ""
	}
}

func isSaaSOrganizationAdminPath(path string) bool {
	const prefix = "/api/v1/organizations/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return false
	}
	switch parts[1] {
	case "subscription", "entitlements", "modules", "invitations":
		return true
	default:
		return false
	}
}

func writeTenantError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
