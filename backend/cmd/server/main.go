package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	domainruntime "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/saas"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/sales"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/verification"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/workflow"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/gateway"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/config"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/database"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/secretbox"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/server"
)

func main() {
	cfg := config.Load()

	connCtx, connCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connCancel()

	db, err := database.Connect(connCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(context.Background(), db, cfg.MigrationsPath); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	modelSecretBox, err := secretbox.New(cfg.ModelSecretKey)
	if err != nil {
		log.Fatalf("model secret key invalid: %v", err)
	}
	securityKernel := securitykernel.NewClient(securitykernel.Config{
		URL:             cfg.SecurityKernelURL,
		SharedSecret:    cfg.SecurityKernelSharedSecret,
		EnforcementMode: cfg.SecurityKernelEnforcementMode,
	})

	industryRepo := industry.NewRepository(db)
	industrySvc := industry.NewService(industryRepo)
	industryHandler := industry.NewHandler(industrySvc)

	saasRepo := saas.NewRepository(db)
	saasSvc := saas.NewService(saasRepo, cfg.MetaOrgMode, saas.WithSecurityKernel(securityKernel), saas.WithIndustryPolicy(industrySvc))
	if err := saasSvc.BootstrapPlatformAdmin(context.Background(), cfg.PlatformAdminEmail, cfg.PlatformAdminPasswordHash); err != nil {
		log.Fatalf("platform admin bootstrap failed: %v", err)
	}
	saasHandler := saas.NewHandler(saasSvc)

	systemAdminRepo := systemadmin.NewRepository(db)
	systemAdminSvc := systemadmin.NewService(systemAdminRepo)
	systemAdminHandler := systemadmin.NewHandler(systemAdminSvc)

	identRepo := identity.NewRepository(db)
	identSvc := identity.NewService(identRepo, cfg.JWTSecret, identity.WithSessionProfileProvider(saasSvc))
	identHandler := identity.NewHandler(identSvc)

	govRepo := governance.NewRepository(db)
	govSvc := governance.NewService(govRepo)
	govHandler := governance.NewHandler(govSvc)

	evoRepo := evolution.NewRepository(db)
	evoSvc := evolution.NewService(evoRepo)
	evoHandler := evolution.NewHandler(evoSvc)

	metaResourceRepo := metaresource.NewRepository(db)
	metaResourceSvc := metaresource.NewService(metaResourceRepo)
	metaResourceHandler := metaresource.NewHandler(metaResourceSvc)

	orgRepo := organization.NewRepository(db)
	orgSvc := organization.NewService(
		orgRepo,
		organization.WithGovernanceService(govSvc),
		organization.WithEvolutionService(evoSvc),
		organization.WithMetaResourceService(metaResourceSvc),
	)
	orgHandler := organization.NewHandler(orgSvc)

	layerRepo := layer.NewRepository(db)
	layerClassifier := layer.NewClassifierService(layerRepo)
	layerHandler := layer.NewHandler(layerClassifier)

	capRepo := capability.NewRepository(db)
	capRouter := capability.NewRouter(capRepo)
	capHandler := capability.NewHandler(capRepo, capRouter, evoSvc)

	costRepo := costing.NewRepository(db)
	costSvc := costing.NewService(costRepo)
	costHandler := costing.NewHandler(costSvc)

	dashRepo := dashboard.NewRepository(db)
	dashSvc := dashboard.NewService(dashRepo)
	dashHandler := dashboard.NewHandler(dashSvc)

	metaRepo := metaorg.NewRepository(db)
	metaSvc := metaorg.NewService(metaRepo)
	metaHandler := metaorg.NewHandler(metaSvc)

	obsRepo := observability.NewRepository(db)
	obsSvc := observability.NewService(obsRepo)
	obsHandler := observability.NewHandler(obsSvc)

	aiRepo := aigateway.NewRepository(db, modelSecretBox)
	aiSvc := aigateway.NewService(aiRepo, nil, aigateway.WithObservability(obsSvc), aigateway.WithCostRecorder(costSvc), aigateway.WithSecurityKernel(securityKernel))
	aiHandler := aigateway.NewHandler(aiSvc)

	wfRepo := workflow.NewRepository(db)
	wfSvc := workflow.NewService(wfRepo)
	wfHandler := workflow.NewHandler(wfSvc)

	projectRepo := project.NewRepository(db)
	projectSvc := project.NewService(
		projectRepo,
		project.WithGovernanceService(govSvc),
		project.WithEvolutionService(evoSvc),
		project.WithOrganizationService(orgSvc),
		project.WithWorkflowService(wfSvc),
		project.WithMetaResourceService(metaResourceSvc),
		project.WithCostRecorder(costSvc),
	)
	projectHandler := project.NewHandler(projectSvc)

	financeRepo := finance.NewRepository(db, modelSecretBox)
	financeSvc := finance.NewService(financeRepo, finance.WithCostPoster(projectSvc), finance.WithObservability(obsSvc))
	financeHandler := finance.NewHandler(financeSvc)

	inventoryRepo := inventory.NewRepository(db)
	inventorySvc := inventory.NewService(inventoryRepo)
	inventoryHandler := inventory.NewHandler(inventorySvc)

	procurementRepo := procurement.NewRepository(db)
	procurementSvc := procurement.NewService(
		procurementRepo,
		procurement.WithInventoryPoster(inventorySvc),
		procurement.WithFinancePoster(financeSvc),
	)
	procurementHandler := procurement.NewHandler(procurementSvc)

	salesRepo := sales.NewRepository(db)
	salesSvc := sales.NewService(
		salesRepo,
		sales.WithInventoryPoster(inventorySvc),
		sales.WithFinancePoster(financeSvc),
	)
	salesHandler := sales.NewHandler(salesSvc)

	runtimeRepo := domainruntime.NewRepository(db)
	runtimeSvc := domainruntime.NewService(runtimeRepo)
	runtimeHandler := domainruntime.NewHandler(runtimeSvc)

	erpRepo := erp.NewRepository(db)
	erpSvc := erp.NewService(erpRepo, erp.DefaultCatalog())
	erpHandler := erp.NewHandler(erpSvc)

	toolRepo := toolruntime.NewRepository(db)
	toolSvc := toolruntime.NewService(toolRepo, govSvc, toolruntime.InternalTools(projectSvc, financeSvc, evoSvc), toolruntime.WithObservability(obsSvc), toolruntime.WithSecurityKernel(securityKernel))
	toolHandler := toolruntime.NewHandler(toolSvc)

	assistantRepo := assistant.NewRepository(db)
	contextRepo := assistant.NewContextRepository(db)
	contextResolver := assistant.NewDBContextResolver(db)
	contextEngine := assistant.NewVerifiedContextEngine(assistant.VerifiedContextEngineConfig{
		Resolver:   contextResolver,
		Evaluator:  assistant.NewContextRuleEvaluator(assistant.ContextRuleEvaluatorConfig{AttentionCoreRatio: 0.4}),
		Repository: contextRepo,
		RuleSource: contextRepo,
	})
	dictionarySvc := assistant.NewDictionaryService(contextRepo, nil)
	assistantSvc := assistant.NewService(
		assistantRepo,
		aiSvc,
		toolSvc,
		assistant.WithContextResolver(contextResolver),
		assistant.WithProposalApplicator(assistant.NewDBProposalApplicator(db)),
		assistant.WithDictionaryService(dictionarySvc),
		assistant.WithVerifiedContextEngine(contextEngine),
		assistant.WithSecurityKernel(securityKernel),
	)
	toolRunner := assistant.NewToolRunner(toolSvc, assistant.ToolRunnerConfig{})
	eventSink := assistant.NewMemoryEventSink(assistantRepo)
	assistantRuntime := assistant.NewAssistantRuntime(assistantSvc, contextEngine, toolRunner, eventSink)
	assistantSvc.SetRuntime(assistantRuntime)
	assistantHandler := assistant.NewHandler(assistantSvc)

	verRepo := verification.NewRepository(db)
	verSvc := verification.NewService(verRepo)
	verHandler := verification.NewHandler(verSvc)

	router := server.NewRouter(cfg.CorsOrigins)
	gateway.RegisterRoutes(router, &gateway.Dependencies{
		JWTSecret:            cfg.JWTSecret,
		IdentityHandler:      identHandler,
		OrganizationHandler:  orgHandler,
		LayerHandler:         layerHandler,
		CapabilityHandler:    capHandler,
		CostingHandler:       costHandler,
		DashboardHandler:     dashHandler,
		MetaOrgHandler:       metaHandler,
		MetaResourceHandler:  metaResourceHandler,
		AssistantHandler:     assistantHandler,
		AIGatewayHandler:     aiHandler,
		WorkflowHandler:      wfHandler,
		ProjectHandler:       projectHandler,
		FinanceHandler:       financeHandler,
		InventoryHandler:     inventoryHandler,
		IndustryHandler:      industryHandler,
		ProcurementHandler:   procurementHandler,
		SalesHandler:         salesHandler,
		RuntimeHandler:       runtimeHandler,
		ToolRuntimeHandler:   toolHandler,
		SaaSHandler:          saasHandler,
		SystemAdminHandler:   systemAdminHandler,
		TenantResolver:       saasSvc,
		PlatformRoleResolver: systemAdminRepo,
		ObservabilityHandler: obsHandler,
		VerificationHandler:  verHandler,
		GovernanceHandler:    govHandler,
		EvolutionHandler:     evoHandler,
		ErpHandler:           erpHandler,
	})

	srv := server.New(router, cfg.ServerPort)
	go func() {
		log.Printf("server starting on :%d", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
