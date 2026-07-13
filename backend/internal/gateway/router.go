package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/aigateway"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/assistant"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/auditretention"
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
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/monitoringagent"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/observability"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/organization"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/procurement"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/project"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/saas"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/sales"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/tenantprojection"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/verification"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/workflow"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/authlimit"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/platformauth"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

type Dependencies struct {
	JWTSecret               string
	IdentityHandler         *identity.Handler
	OrganizationHandler     *organization.Handler
	LayerHandler            *layer.Handler
	CapabilityHandler       *capability.Handler
	CostingHandler          *costing.Handler
	DashboardHandler        *dashboard.Handler
	MetaOrgHandler          *metaorg.Handler
	MetaResourceHandler     *metaresource.Handler
	AssistantHandler        *assistant.Handler
	AIGatewayHandler        *aigateway.Handler
	WorkflowHandler         *workflow.Handler
	ProjectHandler          *project.Handler
	FinanceHandler          *finance.Handler
	InventoryHandler        *inventory.Handler
	IndustryHandler         *industry.Handler
	ProcurementHandler      *procurement.Handler
	SalesHandler            *sales.Handler
	RuntimeHandler          *runtime.Handler
	ToolRuntimeHandler      *toolruntime.Handler
	SaaSHandler             *saas.Handler
	SystemAdminHandler      *systemadmin.Handler
	TenantResolver          middleware.TenantResolver
	PlatformRoleResolver    middleware.PlatformRoleResolver
	ObservabilityHandler    *observability.Handler
	VerificationHandler     *verification.Handler
	GovernanceHandler       *governance.Handler
	EvolutionHandler        *evolution.Handler
	MonitoringAgentHandler  *monitoringagent.Handler
	ErpHandler              *erp.Handler
	TenantPoolStatsProvider interface {
		Stats() tenantdb.PoolRouterStats
	}
	TenantProjectionStatsProvider interface {
		Stats() tenantprojection.WorkerStats
	}
	AuthenticationRateLimitStatsProvider interface {
		Stats() authlimit.Stats
	}
	AuditRetentionStatsProvider interface {
		Stats() auditretention.WorkerStats
	}
	SecurityKernelHealthProvider interface {
		CheckHealth(context.Context) error
	}
	PlatformDatabaseHealthProvider interface {
		Ping(context.Context) error
	}
	PublicInvitationRateLimit    func(http.Handler) http.Handler
	AuthenticatedSensitiveLimit  func(http.Handler) http.Handler
	AIGatewayCompatibleRateLimit func(http.Handler) http.Handler
}

func RegisterRoutes(r *chi.Mux, deps *Dependencies) {
	if deps == nil {
		panic("gateway.RegisterRoutes: deps must not be nil")
	}
	if deps.AIGatewayHandler != nil {
		deps.AIGatewayHandler.RegisterCompatibleRoutes(r, optionalMiddleware(deps.AIGatewayCompatibleRateLimit)...)
	}
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", healthCheck(
			deps.TenantPoolStatsProvider,
			deps.TenantProjectionStatsProvider,
			deps.AuthenticationRateLimitStatsProvider,
			deps.AuditRetentionStatsProvider,
			deps.PlatformDatabaseHealthProvider,
			deps.SecurityKernelHealthProvider,
		))
		if deps.IdentityHandler != nil {
			deps.IdentityHandler.RegisterPublicRoutes(r)
		}
		if deps.SaaSHandler != nil {
			deps.SaaSHandler.RegisterPublicRoutes(r, optionalMiddleware(deps.PublicInvitationRateLimit)...)
		}
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(deps.JWTSecret))
			if deps.AuthenticatedSensitiveLimit != nil {
				r.Use(deps.AuthenticatedSensitiveLimit)
			}
			if deps.IdentityHandler != nil {
				deps.IdentityHandler.RegisterSelfServiceRoutes(r)
			}
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

func optionalMiddleware(value func(http.Handler) http.Handler) []func(http.Handler) http.Handler {
	if value == nil {
		return nil
	}
	return []func(http.Handler) http.Handler{value}
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
				deps.IdentityHandler.RegisterPlatformManagementRoutes(r)
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
			if deps.MonitoringAgentHandler != nil {
				deps.MonitoringAgentHandler.RegisterRoutes(r)
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
	if deps.MetaResourceHandler != nil {
		deps.MetaResourceHandler.RegisterRoutes(r)
	}
	if deps.AssistantHandler != nil {
		deps.AssistantHandler.RegisterRoutes(r)
	}
	if deps.AIGatewayHandler != nil {
		deps.AIGatewayHandler.RegisterTenantRoutes(r)
	}
	if deps.ToolRuntimeHandler != nil {
		deps.ToolRuntimeHandler.RegisterTenantRoutes(r)
	}
	if deps.OrganizationHandler != nil {
		deps.OrganizationHandler.RegisterTenantDepartmentRoutes(r)
	}
	if deps.RuntimeHandler != nil {
		deps.RuntimeHandler.RegisterTenantReadRoutes(r)
	}
	if deps.ErpHandler != nil {
		deps.ErpHandler.RegisterRoutes(r)
	}
	if deps.CostingHandler != nil {
		deps.CostingHandler.RegisterRoutes(r)
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
}

type healthResponse struct {
	Status                   string                        `json:"status"`
	TenantDatabasePools      *tenantdb.PoolRouterStats     `json:"tenant_database_pools,omitempty"`
	TenantProjectionWorker   *tenantprojection.WorkerStats `json:"tenant_projection_worker,omitempty"`
	RequestRateLimits        *authlimit.Stats              `json:"request_rate_limits,omitempty"`
	AuthenticationRateLimits *authlimit.Stats              `json:"authentication_rate_limits,omitempty"`
	AuditRetentionWorker     *auditretention.WorkerStats   `json:"audit_retention_worker,omitempty"`
	PlatformDatabase         *dependencyHealth             `json:"platform_database,omitempty"`
	SecurityKernel           *dependencyHealth             `json:"security_kernel,omitempty"`
}

type dependencyHealth struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func healthCheck(poolProvider interface {
	Stats() tenantdb.PoolRouterStats
}, projectionProvider interface {
	Stats() tenantprojection.WorkerStats
}, authLimitProvider interface {
	Stats() authlimit.Stats
}, retentionProvider interface {
	Stats() auditretention.WorkerStats
}, platformDatabaseProvider interface {
	Ping(context.Context) error
}, securityKernelProvider interface {
	CheckHealth(context.Context) error
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := healthResponse{Status: "ok"}
		statusCode := http.StatusOK
		healthCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if poolProvider != nil {
			stats := poolProvider.Stats()
			response.TenantDatabasePools = &stats
		}
		if projectionProvider != nil {
			stats := projectionProvider.Stats()
			response.TenantProjectionWorker = &stats
		}
		if authLimitProvider != nil {
			stats := authLimitProvider.Stats()
			response.RequestRateLimits = &stats
			response.AuthenticationRateLimits = &stats
		}
		if retentionProvider != nil {
			stats := retentionProvider.Stats()
			response.AuditRetentionWorker = &stats
		}
		if platformDatabaseProvider != nil {
			if err := platformDatabaseProvider.Ping(healthCtx); err != nil {
				log.Printf("platform database health check failed: %v", err)
				response.Status = "unavailable"
				response.PlatformDatabase = &dependencyHealth{Status: "unavailable", Reason: "platform_database_unavailable"}
				statusCode = http.StatusServiceUnavailable
			} else {
				response.PlatformDatabase = &dependencyHealth{Status: "ok"}
			}
		}
		if securityKernelProvider != nil {
			err := securityKernelProvider.CheckHealth(healthCtx)
			if err != nil {
				log.Printf("security kernel health check failed: %v", err)
				response.Status = "unavailable"
				response.SecurityKernel = &dependencyHealth{Status: "unavailable", Reason: "security_kernel_unavailable"}
				statusCode = http.StatusServiceUnavailable
			} else {
				response.SecurityKernel = &dependencyHealth{Status: "ok"}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("health check write error: %v", err)
		}
	}
}
