package gateway

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/assistant"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/capability"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/costing"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/dashboard"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/erp"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/evolution"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/governance"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/identity"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/industry"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/inventory"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/layer"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/metaorg"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/metaresource"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/observability"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/organization"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/procurement"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/project"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/saas"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/sales"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/verification"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/workflow"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
)

type Dependencies struct {
	JWTSecret            string
	IdentityHandler      *identity.Handler
	OrganizationHandler  *organization.Handler
	LayerHandler         *layer.Handler
	CapabilityHandler    *capability.Handler
	CostingHandler       *costing.Handler
	DashboardHandler     *dashboard.Handler
	MetaOrgHandler       *metaorg.Handler
	MetaResourceHandler  *metaresource.Handler
	AssistantHandler     *assistant.Handler
	AIGatewayHandler     *aigateway.Handler
	WorkflowHandler      *workflow.Handler
	ProjectHandler       *project.Handler
	FinanceHandler       *finance.Handler
	InventoryHandler     *inventory.Handler
	IndustryHandler      *industry.Handler
	ProcurementHandler   *procurement.Handler
	SalesHandler         *sales.Handler
	RuntimeHandler       *runtime.Handler
	ToolRuntimeHandler   *toolruntime.Handler
	SaaSHandler          *saas.Handler
	SystemAdminHandler   *systemadmin.Handler
	TenantResolver       middleware.TenantResolver
	PlatformRoleResolver middleware.PlatformRoleResolver
	ObservabilityHandler *observability.Handler
	VerificationHandler  *verification.Handler
	GovernanceHandler    *governance.Handler
	EvolutionHandler     *evolution.Handler
	ErpHandler           *erp.Handler
}

func RegisterRoutes(r *chi.Mux, deps *Dependencies) {
	if deps == nil {
		panic("gateway.RegisterRoutes: deps must not be nil")
	}
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", healthCheck)
		if deps.IdentityHandler != nil {
			deps.IdentityHandler.RegisterPublicRoutes(r)
		}
		if deps.SaaSHandler != nil {
			deps.SaaSHandler.RegisterPublicRoutes(r)
		}
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(deps.JWTSecret))
			if deps.SaaSHandler != nil {
				deps.SaaSHandler.RegisterAuthenticatedRoutes(r)
			}
			if deps.SystemAdminHandler != nil {
				deps.SystemAdminHandler.RegisterAuthenticatedRoutes(r)
			}
			if deps.IndustryHandler != nil {
				deps.IndustryHandler.RegisterAuthenticatedRoutes(r)
			}
			registerPlatformAssistantRoutes(r, deps)
			registerPlatformAdminRoutes(r, deps)
			r.Group(func(r chi.Router) {
				r.Use(middleware.TenantMiddleware(deps.TenantResolver))
				registerTenantRoutes(r, deps)
			})
		})
	})
}

func registerPlatformAssistantRoutes(r chi.Router, deps *Dependencies) {
	if deps.AssistantHandler == nil && deps.ToolRuntimeHandler == nil {
		return
	}
	r.Group(func(r chi.Router) {
		r.Use(middleware.PlatformPermissionMiddleware(deps.PlatformRoleResolver, platformauth.PermissionAssistantRun))
		if deps.AssistantHandler != nil {
			deps.AssistantHandler.RegisterPlatformRoutes(r)
		}
		if deps.ToolRuntimeHandler != nil {
			deps.ToolRuntimeHandler.RegisterPlatformRoutes(r)
		}
	})
}

func registerPlatformAdminRoutes(r chi.Router, deps *Dependencies) {
	r.Route("/platform/admin", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.PlatformPermissionMiddleware(deps.PlatformRoleResolver, platformauth.PermissionPlatformRead))
			if deps.IdentityHandler != nil {
				deps.IdentityHandler.RegisterProtectedRoutes(r)
			}
			if deps.LayerHandler != nil {
				deps.LayerHandler.RegisterRoutes(r)
			}
			if deps.CapabilityHandler != nil {
				deps.CapabilityHandler.RegisterRoutes(r)
			}
			if deps.GovernanceHandler != nil {
				deps.GovernanceHandler.RegisterRoutes(r)
			}
			if deps.EvolutionHandler != nil {
				deps.EvolutionHandler.RegisterRoutes(r)
			}
			if deps.VerificationHandler != nil {
				deps.VerificationHandler.RegisterRoutes(r)
			}
			if deps.ObservabilityHandler != nil {
				deps.ObservabilityHandler.RegisterRoutes(r)
			}
			if deps.MetaOrgHandler != nil {
				deps.MetaOrgHandler.RegisterPlatformRoutes(r)
			}
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.PlatformPermissionMiddleware(deps.PlatformRoleResolver, platformauth.PermissionRuntimeManage))
			if deps.RuntimeHandler != nil {
				deps.RuntimeHandler.RegisterRoutes(r)
			}
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.PlatformPermissionMiddleware(deps.PlatformRoleResolver, platformauth.PermissionOrganizationManage))
			if deps.OrganizationHandler != nil {
				deps.OrganizationHandler.RegisterPlatformManagementRoutes(r)
			}
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.PlatformPermissionMiddleware(deps.PlatformRoleResolver, platformauth.PermissionModelManage))
			if deps.AIGatewayHandler != nil {
				deps.AIGatewayHandler.RegisterRoutes(r)
			}
		})
	})
}

func registerTenantRoutes(r chi.Router, deps *Dependencies) {
	if deps.SaaSHandler != nil {
		deps.SaaSHandler.RegisterTenantRoutes(r)
	}
	if deps.DashboardHandler != nil {
		deps.DashboardHandler.RegisterRoutes(r)
	}
	if deps.MetaOrgHandler != nil {
		deps.MetaOrgHandler.RegisterRoutes(r)
	}
	if deps.OrganizationHandler != nil {
		deps.OrganizationHandler.RegisterTenantDepartmentRoutes(r)
	}
	if deps.ErpHandler != nil {
		deps.ErpHandler.RegisterRoutes(r)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("health check write error: %v", err)
	}
}
