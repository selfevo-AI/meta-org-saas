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
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/auditretention"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/businessai"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/businessaibridge"
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
	domainruntime "github.com/selfevo-AI/meta-org-saas/backend/internal/domain/runtime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/saas"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/sales"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/systemadmin"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/tenantprojection"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/toolruntime"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/verification"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/domain/workflow"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/gateway"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/authlimit"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/config"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/database"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/secretbox"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/securitykernel"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/server"
	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/tenantdb"
)

func main() {
	cfg, err := config.LoadValidated()
	if err != nil {
		log.Fatalf("configuration invalid: %v", err)
	}
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	connCtx, connCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connCancel()

	db, err := database.Connect(connCtx, cfg.PlatformDatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(context.Background(), db, cfg.MigrationsPath); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}
	retentionWorker := auditretention.NewWorker(
		auditretention.NewRepository(db),
		auditretention.WorkerConfig{
			Enabled:       cfg.AuditRetentionWorkerEnabled,
			RetentionDays: cfg.AuditRetentionDays,
			PollInterval:  time.Duration(cfg.AuditRetentionPollSeconds) * time.Second,
			BatchSize:     cfg.AuditRetentionBatchSize,
			MaxBatches:    cfg.AuditRetentionMaxBatches,
		},
	)
	retentionWorker.Start(appCtx)
	clientIPResolver, err := middleware.NewClientIPResolver(cfg.TrustedProxyCIDRs)
	if err != nil {
		log.Fatalf("trusted proxy configuration invalid: %v", err)
	}
	authLimiter := authlimit.NewPostgresLimiter(db)
	sensitiveOperationRateLimit := middleware.RequestRateLimit(
		authLimiter,
		clientIPResolver,
		authlimit.Policy{
			Window:        time.Duration(cfg.SensitiveRateLimitWindowSeconds) * time.Second,
			MaxAttempts:   cfg.SensitiveRateLimitMaxAttempts,
			BlockDuration: time.Duration(cfg.SensitiveRateLimitBlockSeconds) * time.Second,
		},
		middleware.AuthenticatedSensitiveRateLimitBuckets,
	)
	publicInvitationRateLimit := middleware.RequestRateLimit(
		authLimiter,
		clientIPResolver,
		authlimit.Policy{
			Window:        time.Duration(cfg.SensitiveRateLimitWindowSeconds) * time.Second,
			MaxAttempts:   cfg.InvitationAcceptMaxAttempts,
			BlockDuration: time.Duration(cfg.SensitiveRateLimitBlockSeconds) * time.Second,
		},
		middleware.PublicInvitationRateLimitBuckets,
	)
	aiGatewayCompatibleRateLimit := middleware.RequestRateLimit(
		authLimiter,
		clientIPResolver,
		authlimit.Policy{
			Window:        time.Duration(cfg.AIGatewayRateLimitWindowSeconds) * time.Second,
			MaxAttempts:   cfg.AIGatewayRateLimitMaxRequests,
			BlockDuration: time.Duration(cfg.AIGatewayRateLimitBlockSeconds) * time.Second,
		},
		middleware.AIGatewayCompatibleRateLimitBuckets,
	)
	tenantBusinessDB := tenantdb.NewPoolRouterWithConfig(db, cfg.TenantDatabaseAdminURL, tenantdb.PoolRouterConfig{
		MaxCachedPools:        cfg.TenantDatabasePoolMaxEntries,
		MaxConnectionsPerPool: int32(cfg.TenantDatabasePoolMaxConnections),
		MinConnectionsPerPool: int32(cfg.TenantDatabasePoolMinConnections),
		IdlePoolTimeout:       time.Duration(cfg.TenantDatabasePoolIdleSeconds) * time.Second,
		SweepInterval:         time.Duration(cfg.TenantDatabasePoolSweepSeconds) * time.Second,
		ConnectionIdleTimeout: time.Duration(cfg.TenantDatabaseConnIdleSeconds) * time.Second,
		ConnectionLifetime:    time.Duration(cfg.TenantDatabaseConnLifetimeSeconds) * time.Second,
	})
	defer tenantBusinessDB.Close()

	modelSecretBox, err := secretbox.New(cfg.ModelSecretKey)
	if err != nil {
		log.Fatalf("model secret key invalid: %v", err)
	}
	securityKernelConfig := securitykernel.Config{
		URL:             cfg.SecurityKernelURL,
		SharedSecret:    cfg.SecurityKernelSharedSecret,
		EnforcementMode: cfg.SecurityKernelEnforcementMode,
		Required:        cfg.MetaOrgMode == saas.ModeSaaS,
	}
	if err := securitykernel.ValidateConfig(securityKernelConfig); err != nil {
		log.Fatalf("security kernel configuration invalid: %v", err)
	}
	securityKernel := securitykernel.NewClient(securityKernelConfig)

	industryRepo := industry.NewRepository(db)
	industrySvc := industry.NewService(industryRepo)
	industryHandler := industry.NewHandler(industrySvc)

	saasRepo := saas.NewRepository(db, saas.WithTenantDatabaseDefaults(tenantdb.Defaults{
		DeploymentMode:     cfg.TenantDatabaseMode,
		DatabaseNamePrefix: cfg.TenantDatabaseNamePrefix,
		ClusterKey:         cfg.TenantDatabaseDefaultCluster,
		Region:             cfg.TenantDatabaseDefaultRegion,
	}))
	tenantMigrator := tenantdb.FileTenantMigrator{MigrationsDir: cfg.TenantMigrationsPath}
	saasOptions := []saas.ServiceOption{saas.WithSecurityKernel(securityKernel), saas.WithIndustryPolicy(industrySvc)}
	var tenantProvisioningWorker *saas.TenantDatabaseProvisioningWorker
	if cfg.MetaOrgMode == saas.ModeSaaS && cfg.TenantDatabaseMode == tenantdb.DeploymentModeDedicatedDatabase {
		tenantConnCtx, tenantConnCancel := context.WithTimeout(context.Background(), 10*time.Second)
		tenantAdminDB, err := database.Connect(tenantConnCtx, cfg.TenantDatabaseAdminURL)
		tenantConnCancel()
		if err != nil {
			log.Fatalf("tenant database admin connection failed: %v", err)
		}
		defer tenantAdminDB.Close()
		provisioner := tenantdb.NewProvisioner(tenantdb.ProvisionerConfig{
			AdminURL: cfg.TenantDatabaseAdminURL,
			Creator:  tenantdb.NewCatalogDatabaseCreator(tenantdb.NewPGDatabaseCatalog(tenantAdminDB)),
			Migrator: tenantMigrator,
		})
		if cfg.TenantProvisioningWorkerEnabled {
			tenantProvisioningWorker = saas.NewTenantDatabaseProvisioningWorker(
				saasRepo,
				provisioner,
				tenantdb.TenantBootstrapper{AdminURL: cfg.TenantDatabaseAdminURL},
				saas.TenantDatabaseProvisioningWorkerConfig{
					PollInterval:   time.Duration(cfg.TenantProvisioningPollSeconds) * time.Second,
					LeaseDuration:  time.Duration(cfg.TenantProvisioningLeaseSeconds) * time.Second,
					RetryBaseDelay: time.Duration(cfg.TenantProvisioningRetrySeconds) * time.Second,
					RetryMaxDelay:  time.Duration(cfg.TenantProvisioningRetryMaxSeconds) * time.Second,
				},
			)
		}
	}
	var tenantProjectionWorker *tenantprojection.Worker
	if cfg.MetaOrgMode == saas.ModeSaaS &&
		cfg.TenantDatabaseMode == tenantdb.DeploymentModeDedicatedDatabase &&
		cfg.TenantProjectionWorkerEnabled {
		tenantProjectionWorker = tenantprojection.NewWorker(
			tenantprojection.NewRepository(db), tenantBusinessDB, tenantMigrator, saasRepo,
			tenantprojection.WorkerConfig{
				AdminURL:      cfg.TenantDatabaseAdminURL,
				PollInterval:  time.Duration(cfg.TenantProjectionPollSeconds) * time.Second,
				LeaseDuration: time.Duration(cfg.TenantProjectionLeaseSeconds) * time.Second,
				RetryDelay:    time.Duration(cfg.TenantProjectionRetrySeconds) * time.Second,
				BatchSize:     cfg.TenantProjectionBatchSize,
				TargetLimit:   cfg.TenantProjectionTargetLimit,
				ActivityLimit: cfg.TenantProjectionActivityLimit,
				MaxAttempts:   cfg.TenantProjectionMaxAttempts,
			},
		)
	}
	saasSvc := saas.NewService(saasRepo, cfg.MetaOrgMode, saasOptions...)
	if err := saasSvc.BootstrapPlatformAdmin(context.Background(), cfg.PlatformAdminEmail, cfg.PlatformAdminPasswordHash); err != nil {
		log.Fatalf("platform admin bootstrap failed: %v", err)
	}
	if tenantProvisioningWorker != nil {
		tenantProvisioningWorker.Start(appCtx)
	}
	saasHandler := saas.NewHandler(saasSvc)

	systemAdminRepo := systemadmin.NewRepository(db)
	if tenantProjectionWorker != nil {
		tenantProjectionWorker.Start(appCtx)
	}
	systemAdminSvc := systemadmin.NewService(systemAdminRepo)
	systemAdminHandler := systemadmin.NewHandler(systemAdminSvc)

	identRepo := identity.NewRepository(db)
	identSvc := identity.NewService(identRepo, cfg.JWTSecret, identity.WithSessionProfileProvider(saasSvc))
	identHandler := identity.NewHandler(identSvc, identity.WithAuthenticationProtection(
		authLimiter,
		clientIPResolver,
		identity.AuthProtectionConfig{
			AuthenticationPolicy: authlimit.Policy{
				Window:           time.Duration(cfg.AuthRateLimitWindowSeconds) * time.Second,
				MaxAttempts:      cfg.AuthRateLimitMaxAttempts,
				FailureThreshold: cfg.AuthRateLimitFailureThreshold,
				BlockDuration:    time.Duration(cfg.AuthRateLimitBlockSeconds) * time.Second,
			},
			RegistrationPolicy: authlimit.Policy{
				Window:           time.Duration(cfg.AuthRateLimitWindowSeconds) * time.Second,
				MaxAttempts:      cfg.AuthRegistrationMaxAttempts,
				FailureThreshold: cfg.AuthRegistrationMaxAttempts,
				BlockDuration:    time.Duration(cfg.AuthRateLimitBlockSeconds) * time.Second,
			},
		},
	))

	govRepo := governance.NewRepository(db)
	govSvc := governance.NewService(govRepo)
	govHandler := governance.NewHandler(govSvc)

	evoRepo := evolution.NewRepository(db)
	evoSvc := evolution.NewService(evoRepo)
	evoHandler := evolution.NewHandler(evoSvc)

	metaResourceRepo := metaresource.NewRepository(db)
	metaResourceSvc := metaresource.NewService(metaResourceRepo)
	metaResourceHandler := metaresource.NewHandler(metaResourceSvc)

	orgRepo := organization.NewRepository(tenantBusinessDB)
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

	costRepo := costing.NewRepository(tenantBusinessDB)
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

	monitoringRepo := monitoringagent.NewRepository(db)
	monitoringSvc := monitoringagent.NewService(monitoringRepo, monitoringagent.ServiceConfig{
		LookbackHours:    cfg.MonitoringAgentLookbackHours,
		MaxSignalsPerRun: cfg.MonitoringAgentMaxSignalsPerRun,
		SchedulerEnabled: cfg.MonitoringAgentSchedulerEnabled,
		DailyTime:        cfg.MonitoringAgentDailyTime,
	})
	monitoringHandler := monitoringagent.NewHandler(monitoringSvc)
	monitoringScheduler := monitoringagent.NewScheduler(monitoringSvc, monitoringagent.SchedulerConfig{
		Enabled:       cfg.MonitoringAgentSchedulerEnabled,
		DailyTime:     cfg.MonitoringAgentDailyTime,
		LookbackHours: cfg.MonitoringAgentLookbackHours,
	})
	monitoringScheduler.Start(appCtx)

	aiRepo := aigateway.NewRepository(db, modelSecretBox)
	reservationRecoveryWorker := aigateway.NewReservationRecoveryWorker(aiRepo, aigateway.ReservationRecoveryWorkerConfig{
		Enabled:       cfg.AIGatewayRecoveryEnabled,
		StaleAfter:    time.Duration(cfg.AIGatewayReservationStaleSeconds) * time.Second,
		PollInterval:  time.Duration(cfg.AIGatewayReservationPollSeconds) * time.Second,
		LeaseDuration: time.Duration(cfg.AIGatewayReservationLeaseSeconds) * time.Second,
		BatchSize:     cfg.AIGatewayReservationBatchSize,
	})
	reservationRecoveryWorker.Start(appCtx)
	aiSvc := aigateway.NewService(aiRepo, nil, aigateway.WithObservability(obsSvc), aigateway.WithCostRecorder(costSvc), aigateway.WithSecurityKernel(securityKernel), aigateway.WithInvocationTimeouts(
		time.Duration(cfg.AIGatewayInvokeTimeoutSeconds)*time.Second,
		time.Duration(cfg.AIGatewayStreamTimeoutSeconds)*time.Second,
	), aigateway.WithProviderRetryPolicy(cfg.AIGatewayMaxRetries, 100*time.Millisecond))
	aiHandler := aigateway.NewHandler(aiSvc)
	businessAIRepo := businessai.NewRepository(db)
	businessAISvc := businessai.NewService(businessAIRepo, aiSvc, businessai.Config{
		ProviderType: cfg.BusinessAIProviderType,
		Model:        cfg.BusinessAIModel,
		MaxTokens:    cfg.BusinessAIMaxTokens,
	})
	businessAIBridge := businessaibridge.New(businessAISvc)

	wfRepo := workflow.NewRepository(tenantBusinessDB)
	wfSvc := workflow.NewService(wfRepo)
	wfHandler := workflow.NewHandler(wfSvc)

	projectRepo := project.NewRepository(tenantBusinessDB)
	projectSvc := project.NewService(
		projectRepo,
		project.WithGovernanceService(govSvc),
		project.WithEvolutionService(evoSvc),
		project.WithOrganizationService(orgSvc),
		project.WithWorkflowService(wfSvc),
		project.WithMetaResourceService(metaResourceSvc),
		project.WithCostRecorder(costSvc),
		project.WithBusinessAI(businessAISvc),
	)
	projectHandler := project.NewHandler(projectSvc)

	financeRepo := finance.NewRepository(tenantBusinessDB, modelSecretBox)
	financeSvc := finance.NewService(financeRepo, finance.WithCostPoster(projectSvc), finance.WithObservability(obsSvc))
	financeHandler := finance.NewHandler(financeSvc)

	inventoryRepo := inventory.NewRepository(tenantBusinessDB)
	inventorySvc := inventory.NewService(inventoryRepo)
	inventoryHandler := inventory.NewHandler(inventorySvc)

	procurementRepo := procurement.NewRepository(tenantBusinessDB)
	procurementSvc := procurement.NewService(
		procurementRepo,
		procurement.WithInventoryPoster(inventorySvc),
		procurement.WithFinancePoster(financeSvc),
	)
	procurementHandler := procurement.NewHandler(procurementSvc)

	salesRepo := sales.NewRepository(tenantBusinessDB)
	salesSvc := sales.NewService(
		salesRepo,
		sales.WithInventoryPoster(inventorySvc),
		sales.WithFinancePoster(financeSvc),
	)
	salesHandler := sales.NewHandler(salesSvc)

	runtimeRepo := domainruntime.NewRepository(db)
	runtimeSvc := domainruntime.NewService(runtimeRepo)
	runtimeHandler := domainruntime.NewHandler(runtimeSvc)

	erpRepo := erp.NewRepository(tenantBusinessDB)
	erpSvc := erp.NewService(erpRepo, erp.DefaultCatalog())
	erpHandler := erp.NewHandler(erpSvc)

	toolRepo := toolruntime.NewRepository(db)
	toolSvc := toolruntime.NewService(
		toolRepo,
		govSvc,
		toolruntime.InternalToolsWithPlatform(projectSvc, financeSvc, evoSvc, toolruntime.PlatformToolServices{
			ERP:                      erpSvc,
			IndustrySolutionVerifier: systemAdminSvc,
			Runtime:                  runtimeSvc,
		}),
		toolruntime.WithObservability(obsSvc),
		toolruntime.WithSecurityKernel(securityKernel),
		toolruntime.WithExecutionObserver(businessAIBridge),
	)
	businessAIBridge.SetToolService(toolSvc)
	businessAISvc.SetProposalExecutor(businessAIBridge)
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
		assistant.WithContextDiagnosticsRepository(contextRepo),
		assistant.WithSecurityKernel(securityKernel),
	)
	toolRunner := assistant.NewToolRunner(toolSvc, assistant.ToolRunnerConfig{})
	eventSink := assistant.NewMemoryEventSink(assistantRepo)
	assistantRuntime := assistant.NewAssistantRuntime(assistantSvc, contextEngine, toolRunner, eventSink)
	assistantSvc.SetRuntime(assistantRuntime)
	toolSvc.RegisterAdapters(toolruntime.ContextProposalTools(assistantSvc))
	assistantHandler := assistant.NewHandler(assistantSvc, assistant.WithStreamTimeout(time.Duration(cfg.AIGatewayStreamTimeoutSeconds)*time.Second))

	verRepo := verification.NewRepository(db)
	verSvc := verification.NewService(verRepo)
	verHandler := verification.NewHandler(verSvc)

	router := server.NewRouter(cfg.CorsOrigins)
	gateway.RegisterRoutes(router, &gateway.Dependencies{
		JWTSecret:                            cfg.JWTSecret,
		IdentityHandler:                      identHandler,
		OrganizationHandler:                  orgHandler,
		LayerHandler:                         layerHandler,
		CapabilityHandler:                    capHandler,
		CostingHandler:                       costHandler,
		DashboardHandler:                     dashHandler,
		MetaOrgHandler:                       metaHandler,
		MetaResourceHandler:                  metaResourceHandler,
		AssistantHandler:                     assistantHandler,
		AIGatewayHandler:                     aiHandler,
		WorkflowHandler:                      wfHandler,
		ProjectHandler:                       projectHandler,
		FinanceHandler:                       financeHandler,
		InventoryHandler:                     inventoryHandler,
		IndustryHandler:                      industryHandler,
		ProcurementHandler:                   procurementHandler,
		SalesHandler:                         salesHandler,
		RuntimeHandler:                       runtimeHandler,
		ToolRuntimeHandler:                   toolHandler,
		SaaSHandler:                          saasHandler,
		SystemAdminHandler:                   systemAdminHandler,
		TenantResolver:                       saasSvc,
		PlatformRoleResolver:                 systemAdminRepo,
		ObservabilityHandler:                 obsHandler,
		VerificationHandler:                  verHandler,
		GovernanceHandler:                    govHandler,
		EvolutionHandler:                     evoHandler,
		MonitoringAgentHandler:               monitoringHandler,
		ErpHandler:                           erpHandler,
		TenantPoolStatsProvider:              tenantBusinessDB,
		TenantProjectionStatsProvider:        tenantProjectionWorker,
		AuthenticationRateLimitStatsProvider: authLimiter,
		AuditRetentionStatsProvider:          retentionWorker,
		AIGatewayReservationStatsProvider:    reservationRecoveryWorker,
		PlatformDatabaseHealthProvider:       db,
		SecurityKernelHealthProvider:         securityKernel.(securitykernel.HealthChecker),
		PublicInvitationRateLimit:            publicInvitationRateLimit,
		AuthenticatedSensitiveLimit:          sensitiveOperationRateLimit,
		AIGatewayCompatibleRateLimit:         aiGatewayCompatibleRateLimit,
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
	appCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
