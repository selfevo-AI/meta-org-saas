package gateway

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/aigateway"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/assistant"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/capability"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/costing"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/dashboard"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/evolution"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/finance"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/governance"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/identity"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/inventory"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/layer"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/metaorg"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/metaresource"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/observability"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/organization"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/procurement"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/project"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/saas"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/sales"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/systemadmin"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/toolruntime"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/verification"
	"github.com/selfevo-AI/meta-org/backend/internal/domain/workflow"
	"github.com/selfevo-AI/meta-org/backend/internal/pkg/middleware"
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
	ProcurementHandler   *procurement.Handler
	SalesHandler         *sales.Handler
	RuntimeHandler       *runtime.Handler
	ToolRuntimeHandler   *toolruntime.Handler
	SaaSHandler          *saas.Handler
	SystemAdminHandler   *systemadmin.Handler
	TenantResolver       middleware.TenantResolver
	ObservabilityHandler *observability.Handler
	VerificationHandler  *verification.Handler
	GovernanceHandler    *governance.Handler
	EvolutionHandler     *evolution.Handler
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
		if deps.FinanceHandler != nil {
			deps.FinanceHandler.RegisterPublicRoutes(r)
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
			r.Group(func(r chi.Router) {
				r.Use(middleware.TenantMiddleware(deps.TenantResolver))
				if deps.IdentityHandler != nil {
					deps.IdentityHandler.RegisterProtectedRoutes(r)
				}
				if deps.SaaSHandler != nil {
					deps.SaaSHandler.RegisterTenantRoutes(r)
				}
				if deps.OrganizationHandler != nil {
					deps.OrganizationHandler.RegisterRoutes(r)
				}
				if deps.LayerHandler != nil {
					deps.LayerHandler.RegisterRoutes(r)
				}
				if deps.CapabilityHandler != nil {
					deps.CapabilityHandler.RegisterRoutes(r)
				}
				if deps.CostingHandler != nil {
					deps.CostingHandler.RegisterRoutes(r)
				}
				if deps.DashboardHandler != nil {
					deps.DashboardHandler.RegisterRoutes(r)
				}
				if deps.MetaOrgHandler != nil {
					deps.MetaOrgHandler.RegisterRoutes(r)
				}
				if deps.MetaResourceHandler != nil {
					deps.MetaResourceHandler.RegisterRoutes(r)
				}
				if deps.AssistantHandler != nil {
					deps.AssistantHandler.RegisterRoutes(r)
				}
				if deps.AIGatewayHandler != nil {
					deps.AIGatewayHandler.RegisterRoutes(r)
				}
				if deps.WorkflowHandler != nil {
					deps.WorkflowHandler.RegisterRoutes(r)
				}
				if deps.ProjectHandler != nil {
					deps.ProjectHandler.RegisterRoutes(r)
				}
				if deps.FinanceHandler != nil {
					deps.FinanceHandler.RegisterRoutes(r)
				}
				if deps.InventoryHandler != nil {
					deps.InventoryHandler.RegisterRoutes(r)
				}
				if deps.ProcurementHandler != nil {
					deps.ProcurementHandler.RegisterRoutes(r)
				}
				if deps.SalesHandler != nil {
					deps.SalesHandler.RegisterRoutes(r)
				}
				if deps.RuntimeHandler != nil {
					deps.RuntimeHandler.RegisterRoutes(r)
				}
				if deps.ToolRuntimeHandler != nil {
					deps.ToolRuntimeHandler.RegisterRoutes(r)
				}
				if deps.VerificationHandler != nil {
					deps.VerificationHandler.RegisterRoutes(r)
				}
				if deps.ObservabilityHandler != nil {
					deps.ObservabilityHandler.RegisterRoutes(r)
				}
				if deps.GovernanceHandler != nil {
					deps.GovernanceHandler.RegisterRoutes(r)
				}
				if deps.EvolutionHandler != nil {
					deps.EvolutionHandler.RegisterRoutes(r)
				}
			})
		})
	})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("health check write error: %v", err)
	}
}
