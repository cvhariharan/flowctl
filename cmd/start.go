package cmd

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	casbin_model "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/util"
	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/handlers"
	"github.com/cvhariharan/flowctl/internal/messengers"
	"github.com/cvhariharan/flowctl/internal/metrics"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/cvhariharan/flowctl/internal/scheduler/storage"
	artifactsStorage "github.com/cvhariharan/flowctl/internal/storage"
	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	sqlxadapter "github.com/memwey/casbin-sqlx-adapter"
	"github.com/spf13/cobra"
	"gocloud.dev/secrets"
	_ "gocloud.dev/secrets/localsecrets"
)

// StaticFiles will be set from the main package
var StaticFiles embed.FS

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start flowctl server",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		if err := LoadConfig(configPath); err != nil {
			log.Fatal(err)
		}

		// Initialize shared components once
		shared := initializeSharedComponents()
		defer shared.Cleanup()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// start worker
			startWorker(shared.Scheduler, shared.Logger)
		}()
		// start server
		startServer(shared.DB, shared.Core, shared.Metrics, shared.Logger, shared.ExecutorSigningKey)
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

// SharedComponents holds components that are shared between server and worker
type SharedComponents struct {
	DB                 *sqlx.DB
	Core               *core.Core
	Scheduler          *scheduler.Scheduler
	Metrics            *metrics.Manager
	Logger             *slog.Logger
	Keeper             *secrets.Keeper
	Messengers         map[string]messengers.Messenger
	ExecutorSigningKey []byte
}

// Cleanup cleans up all shared resources
func (s *SharedComponents) Cleanup() {
	CleanupPlugins()
	if s.DB != nil {
		s.DB.Close()
	}
	if s.Keeper != nil {
		s.Keeper.Close()
	}
	for _, m := range s.Messengers {
		if m != nil {
			m.Close()
		}
	}
}

// initMessengers creates and returns all enabled messengers as a map keyed by channel name.
func initMessengers(cfg config.MessengersConfig, groupResolver messengers.GroupResolver, logger *slog.Logger) map[string]messengers.Messenger {
	m := make(map[string]messengers.Messenger)

	if cfg.Email.Enabled {
		emailMessenger, err := messengers.NewEmailMessenger(cfg.Email, groupResolver, logger.WithGroup("email_messenger"), appConfig.App.RootURL)
		if err != nil {
			logger.Error("failed to create email messenger", "error", err)
		} else {
			m["email"] = emailMessenger
			messengers.RegisterSchema("email", messengers.GetEmailNotifySchema())
			logger.Info("email messenger initialized")
		}
	}

	if cfg.Webhook.Enabled {
		webhookMessenger, err := messengers.NewWebhookMessenger(cfg.Webhook, logger.WithGroup("webhook_messenger"))
		if err != nil {
			logger.Error("failed to create webhook messenger", "error", err)
		} else {
			m["webhook"] = webhookMessenger
			messengers.RegisterSchema("webhook", messengers.GetWebhookNotifySchema())
			logger.Info("webhook messenger initialized")
		}
	}

	return m
}

// initializeSharedComponents sets up all shared components (DB, scheduler, core, etc.)
func initializeSharedComponents() *SharedComponents {
	loglevel := slog.LevelInfo
	if os.Getenv("DEBUG_LOG") == "true" {
		loglevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: loglevel,
	}))
	slog.SetDefault(logger)

	// Create the log directory and instantiate log manager
	if err := os.MkdirAll(appConfig.Logger.Directory, 0755); err != nil {
		log.Fatalf("could not create log directory: %v", err)
	}
	fileLogManager := streamlogger.NewFileLogManager(streamlogger.FileLogManagerCfg{
		RetentionTime: appConfig.Logger.RetentionTime,
		MaxSizeBytes:  appConfig.Logger.MaxSizeBytes * 1024 * 1024,
		LogDir:        appConfig.Logger.Directory,
		ScanInterval:  appConfig.Logger.ScanInterval,
	})
	go fileLogManager.Run(context.Background(), logger.WithGroup("file_log_manager"))

	dbConnectionString := appConfig.DB.ConnectionString()
	db, err := sqlx.Connect("postgres", dbConnectionString)
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}

	// Initialize casbin with sqlx adapter
	modelContent, err := StaticFiles.ReadFile("configs/rbac_model.conf")
	if err != nil {
		log.Fatalf("could not read rbac_model.conf from embedded FS: %v", err)
	}
	m, err := casbin_model.NewModelFromString(string(modelContent))
	if err != nil {
		log.Fatalf("could not create casbin model: %v", err)
	}

	a := sqlxadapter.NewAdapter("postgres", dbConnectionString)

	enforcer, err := casbin.NewEnforcer(m, a)
	if err != nil {
		log.Fatalf("could not initialize casbin enforcer: %v", err)
	}
	enforcer.AddNamedDomainMatchingFunc("g", "keyMatch2", util.KeyMatch2)

	keeperURL := appConfig.Keystore.KeeperURL
	if keeperURL == "" {
		log.Fatal("app.keystore.keeper_url is not set")
	}

	keeper, err := secrets.OpenKeeper(context.Background(), keeperURL)
	if err != nil {
		log.Fatalf("could not open secrets keeper: %v", err)
	}

	s := repo.NewPostgresStore(db)

	jobStore := storage.NewPostgresStorage(db)

	// Initialize metrics
	var metricsManager *metrics.Manager
	if appConfig.Metrics.Enabled {
		metricsManager = metrics.NewManager()
		metricsManager.Register()
	}

	// Build scheduler
	sch, err := scheduler.NewSchedulerBuilder(logger.WithGroup("scheduler")).
		WithJobStore(jobStore).
		WithWorkerCount(appConfig.Scheduler.WorkerCount).
		WithCronSyncInterval(appConfig.Scheduler.CronSyncInterval).
		WithRetryOptions(scheduler.DefaultRetryOptions()).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// Create core with scheduler
	co, err := core.NewCore(appConfig.App.FlowsDirectory, s, sch, keeper, enforcer)
	if err != nil {
		log.Fatal(err)
	}
	co.LogManager = fileLogManager

	messengersMap := initMessengers(appConfig.Messengers, co, logger)

	executorSigningKey, err := core.GenerateSigningKey()
	if err != nil {
		log.Fatalf("failed to generate executor signing key: %v", err)
	}
	executorKeys := registerPlugins(appConfig.App.PluginDir, executorSigningKey)

	var storeInstance artifactsStorage.Storage
	if appConfig.Artifacts.Retain {
		var err error
		storeInstance, err = artifactsStorage.NewLocalStorage(appConfig.Artifacts.Directory)
		if err != nil {
			log.Fatalf("failed to initialize artifacts storage: %v", err)
		}

		artifactMgr := scheduler.NewArtifactManager(
			s,
			storeInstance,
			appConfig.Artifacts.RetentionTime,
			appConfig.Artifacts.ScanInterval,
			logger.WithGroup("artifact_manager"),
		)
		go artifactMgr.Run(context.Background())
	}

	// Create flow execution handler with core's secrets provider
	flowHandler := scheduler.NewFlowExecutionHandler(scheduler.FlowHandlerConfig{
		Store:                s,
		SecretsProvider:      co.GetMergedSecretsForFlow,
		LogManager:           fileLogManager,
		Logger:               logger.WithGroup("flow_handler"),
		Metrics:              metricsManager,
		FlowExecutionTimeout: appConfig.Scheduler.FlowExecutionTimeout,
		ExecutorKeys:         executorKeys,
		APIBaseURL:           appConfig.App.RootURL,
		Storage:              storeInstance,
	})

	// Set handler and queue config on scheduler
	if err := sch.SetHandler(flowHandler); err != nil {
		log.Fatal(err)
	}

	queueWeights := []scheduler.QueueWeight{
		{PayloadType: scheduler.PayloadTypeFlowExecution, Weight: 100},
	}

	if len(messengersMap) > 0 {
		// Create and register notification handler
		notificationHandler := scheduler.NewNotificationHandler(messengersMap, s, logger.WithGroup("notification_handler"))
		if err := sch.SetHandler(notificationHandler); err != nil {
			log.Fatal(err)
		}

		// Update queue weights to include notifications
		queueWeights = []scheduler.QueueWeight{
			{PayloadType: scheduler.PayloadTypeFlowExecution, Weight: 90},
			{PayloadType: scheduler.PayloadTypeNotification, Weight: 10},
		}

		logger.Info("notifications enabled", "channels", len(messengersMap))
	}

	if err := sch.SetQueueConfig(scheduler.QueueConfig{Queues: queueWeights}); err != nil {
		log.Fatal(err)
	}

	// Set task queuer on flow handler for notification enqueueing
	flowHandler.SetTaskQueuer(sch)

	// Set job syncer for cron scheduling
	sch.SetJobSyncer(co.SyncScheduledFlowJobs)

	return &SharedComponents{
		DB:                 db,
		Core:               co,
		Scheduler:          sch,
		Metrics:            metricsManager,
		Logger:             logger,
		Keeper:             keeper,
		Messengers:         messengersMap,
		ExecutorSigningKey: executorSigningKey,
	}
}

func startServer(db *sqlx.DB, co *core.Core, metricsManager *metrics.Manager, logger *slog.Logger, executorSigningKey []byte) {
	h, err := handlers.NewHandler(logger, db.DB, co, appConfig, executorSigningKey, version, commit, date)
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "header:X-CSRF-Token",
		CookieName:     "_csrf",
		CookiePath:     "/",
		CookieHTTPOnly: false,
		CookieSameSite: http.SameSiteStrictMode,
		Skipper: func(c echo.Context) bool {
			// Browsers cannot set Authorization headers on cross-origin requests without triggering a
			// CORS preflight and the server's CORS policy would block that preflight.
			// A request with an Authorization header that reaches this server is therefore not a browser-initiated forged request
			return strings.HasPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
		},
	}))

	if metricsManager != nil {
		e.Use(metricsManager.HTTPMetricsMiddleware())
	}

	e.GET("/ping", h.HandlePing)
	e.GET("/info", h.HandleGetInfo)
	e.POST("/login", h.HandleLoginPage)
	e.POST("/logout", h.HandleLogout)
	e.GET("/sso-providers", h.HandleGetSSOProviders)

	e.GET("/login/oidc/:provider", h.HandleOIDCLogin)
	e.GET("/auth/callback", h.HandleAuthCallback)

	if metricsManager != nil {
		metricsPath := appConfig.Metrics.Path
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		e.GET(metricsPath, echo.WrapHandler(metricsManager.GetHandler()))
	}

	e.Logger.SetLevel(0)

	e.HTTPErrorHandler = h.ErrorHandler

	api := e.Group("/api/v1", h.Authenticate)

	api.GET("/messengers", h.HandleGetMessengers)

	api.GET("/users", h.HandleUserPagination, h.AuthorizeNamespaceAdmins())
	api.GET("/users/profile", h.HandleGetUserProfile)
	api.POST("/users/change-password", h.HandleChangePassword)
	api.GET("/users/me/api-tokens", h.HandleListAPITokens, h.RequireSessionAuth)
	api.POST("/users/me/api-tokens", h.HandleCreateAPIToken, h.RequireSessionAuth)
	api.DELETE("/users/me/api-tokens/:tokenID", h.HandleRevokeAPIToken, h.RequireSessionAuth)
	api.GET("/users/:userID", h.HandleGetUser, h.AuthorizeForRole("superuser"))
	api.POST("/users", h.HandleCreateUser, h.AuthorizeForRole("superuser"))
	api.DELETE("/users/:userID", h.HandleDeleteUser, h.AuthorizeForRole("superuser"))
	api.PUT("/users/:userID", h.HandleUpdateUser, h.AuthorizeForRole("superuser"))

	api.GET("/groups", h.HandleGroupPagination, h.AuthorizeNamespaceAdmins())
	api.GET("/groups/:groupID", h.HandleGetGroup, h.AuthorizeForRole("superuser"))
	api.PUT("/groups/:groupID", h.HandleUpdateGroup, h.AuthorizeForRole("superuser"))
	api.POST("/groups", h.HandleCreateGroup, h.AuthorizeForRole("superuser"))
	api.DELETE("/groups/:groupID", h.HandleDeleteGroup, h.AuthorizeForRole("superuser"))

	api.PUT("/executor/kv/:bucket", h.HandleSetExecutorKV, h.AuthorizeExecutorOnly())
	api.GET("/executor/kv/:bucket/:key", h.HandleGetExecutorKV, h.AuthorizeExecutorOnly())
	api.GET("/executor/kv/:bucket", h.HandleListExecutorKV, h.AuthorizeExecutorOnly())
	api.DELETE("/executor/kv/:bucket/:key", h.HandleDeleteExecutorKV, h.AuthorizeExecutorOnly())

	// No authorization required
	api.GET("/executors/:executor/config", h.HandleGetExecutorConfig)
	api.GET("/executors", h.HandleListExecutors)
	api.GET("/permissions", h.HandleGetCasbinPermissions)
	api.POST("/permissions/check", h.HandleCheckPermissions)

	api.GET("/namespaces", h.HandleListNamespaces)
	api.GET("/namespaces/:namespaceID", h.HandleGetNamespace, h.AuthorizeForRole("superuser"))
	api.POST("/namespaces", h.HandleCreateNamespace, h.AuthorizeForRole("superuser"))
	api.PUT("/namespaces/:namespaceID", h.HandleUpdateNamespace, h.AuthorizeForRole("superuser"))
	api.DELETE("/namespaces/:namespaceID", h.HandleDeleteNamespace, h.AuthorizeForRole("superuser"))

	namespaceGroup := api.Group("/:namespace", h.NamespaceMiddleware)
	namespaceGroup.GET("/flows", h.HandleFlowsPagination, h.AuthorizeNamespaceAction(models.ResourceNamespace, models.RBACActionView))
	namespaceGroup.POST("/flows", h.HandleCreateFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionCreate))

	namespaceGroup.GET("/flows/groups/me", h.HandleListMyFlowGroups, h.AuthorizeNamespaceAction(models.ResourceNamespace, models.RBACActionView))
	namespaceGroup.GET("/flows/groups/:group", h.HandleGetFlowGroup, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.GET("/flows/groups", h.HandleListFlowGroups, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.POST("/flows/groups", h.HandleCreateFlowGroup, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionCreate))
	namespaceGroup.PUT("/flows/groups/:groupID", h.HandleUpdateFlowGroup, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionUpdate))
	namespaceGroup.DELETE("/flows/groups/:groupID", h.HandleDeleteFlowGroup, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionDelete))

	namespaceGroup.GET("/flows/:flowID", h.HandleGetFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.PUT("/flows/:flowID", h.HandleUpdateFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionUpdate))
	namespaceGroup.DELETE("/flows/:flowID", h.HandleDeleteFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionDelete))

	namespaceGroup.GET("/flows/executions/:execID", h.HandleGetExecutionSummary, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))
	namespaceGroup.POST("/flows/executions/:execID/cancel", h.HandleCancelExecution, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionUpdate))
	namespaceGroup.POST("/flows/executions/:execID/retry", h.HandleRetryExecution, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionUpdate))
	namespaceGroup.GET("/flows/executions/:execID/artifacts", h.HandleListArtifacts, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))
	namespaceGroup.GET("/flows/executions/:execID/artifacts/download", h.HandleDownloadArtifact, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/executions", h.HandleExecutionsPagination, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))
	namespaceGroup.GET("/flows/executions", h.HandleAllExecutionsPagination, h.AuthorizeNamespaceAction(models.ResourceNamespace, models.RBACActionView))

	namespaceGroup.GET("/flows/:flowID/inputs", h.HandleGetFlowInputs, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/meta", h.HandleGetFlowMeta, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/config", h.HandleGetFlowConfig, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionViewConfig))

	namespaceGroup.GET("/flows/:flowID/secrets", h.HandleListFlowSecrets, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/secrets/:secretID", h.HandleGetFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionView))
	namespaceGroup.POST("/flows/:flowID/secrets", h.HandleCreateFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionCreate))
	namespaceGroup.PUT("/flows/:flowID/secrets/:secretID", h.HandleUpdateFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionUpdate))
	namespaceGroup.DELETE("/flows/:flowID/secrets/:secretID", h.HandleDeleteFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionDelete))

	namespaceGroup.GET("/flows/:flowID/schedules", h.HandleListSchedules, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionExecute))
	namespaceGroup.GET("/flows/:flowID/schedules/:schedule_id", h.HandleGetSchedule, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionExecute))
	namespaceGroup.POST("/flows/:flowID/schedules", h.HandleCreateSchedule, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionExecute))
	namespaceGroup.PUT("/flows/:flowID/schedules/:schedule_id", h.HandleUpdateSchedule, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionExecute))
	namespaceGroup.DELETE("/flows/:flowID/schedules/:schedule_id", h.HandleDeleteSchedule, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionExecute))

	namespaceGroup.POST("/trigger/:flow", h.HandleFlowTrigger, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionExecute))
	namespaceGroup.GET("/logs/:logID", h.HandleLogStreaming, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))
	namespaceGroup.GET("/logs/:logID/download", h.HandleLogDownload, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))

	namespaceGroup.GET("/nodes", h.HandleListNodes, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionView))
	namespaceGroup.GET("/nodes/stats", h.HandleGetNodeStats, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionView))
	namespaceGroup.GET("/nodes/:nodeID", h.HandleGetNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionView))
	namespaceGroup.POST("/nodes", h.HandleCreateNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionCreate))
	namespaceGroup.PUT("/nodes/:nodeID", h.HandleUpdateNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionUpdate))
	namespaceGroup.DELETE("/nodes/:nodeID", h.HandleDeleteNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionDelete))

	namespaceGroup.GET("/credentials", h.HandleListCredentials, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionView))
	namespaceGroup.GET("/credentials/:credID", h.HandleGetCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionView))
	namespaceGroup.POST("/credentials", h.HandleCreateCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionCreate))
	namespaceGroup.PUT("/credentials/:credID", h.HandleUpdateCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionUpdate))
	namespaceGroup.DELETE("/credentials/:credID", h.HandleDeleteCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionDelete))

	namespaceGroup.GET("/approvals", h.HandleListApprovals, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionView))
	namespaceGroup.GET("/approvals/:approvalID", h.HandleGetApproval, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionView))
	namespaceGroup.POST("/approvals/:approvalID", h.HandleApprovalAction, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionApprove))

	namespaceGroup.GET("/members", h.HandleGetNamespaceMembers, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionView))
	namespaceGroup.POST("/members", h.HandleAddNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionCreate))
	namespaceGroup.PUT("/members/:membershipID", h.HandleUpdateNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionUpdate))
	namespaceGroup.DELETE("/members/:membershipID", h.HandleRemoveNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionDelete))

	namespaceGroup.GET("/members/:membershipID/groups", h.HandleGetMemberGroups, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionView))
	namespaceGroup.POST("/members/:membershipID/groups", h.HandleGrantGroupAccess, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionUpdate))
	namespaceGroup.DELETE("/members/:membershipID/groups/:group", h.HandleRevokeGroupAccess, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionUpdate))

	namespaceGroup.GET("/secrets", h.HandleListNamespaceSecrets, h.AuthorizeNamespaceAction(models.ResourceNamespaceSecret, models.RBACActionView))
	namespaceGroup.GET("/secrets/:secretID", h.HandleGetNamespaceSecret, h.AuthorizeNamespaceAction(models.ResourceNamespaceSecret, models.RBACActionView))
	namespaceGroup.POST("/secrets", h.HandleCreateNamespaceSecret, h.AuthorizeNamespaceAction(models.ResourceNamespaceSecret, models.RBACActionCreate))
	namespaceGroup.PUT("/secrets/:secretID", h.HandleUpdateNamespaceSecret, h.AuthorizeNamespaceAction(models.ResourceNamespaceSecret, models.RBACActionUpdate))
	namespaceGroup.DELETE("/secrets/:secretID", h.HandleDeleteNamespaceSecret, h.AuthorizeNamespaceAction(models.ResourceNamespaceSecret, models.RBACActionDelete))

	buildFS, err := fs.Sub(StaticFiles, "site/build")
	if err != nil {
		log.Fatal(err)
	}

	// Serve static assets from embedded FS
	e.GET("/_app/*", echo.WrapHandler(http.FileServer(http.FS(buildFS))))
	e.GET("/robots.txt", echo.WrapHandler(http.StripPrefix("/", http.FileServer(http.FS(buildFS)))))
	e.GET("/fonts.css", echo.WrapHandler(http.StripPrefix("/", http.FileServer(http.FS(buildFS)))))
	e.GET("/fonts/*", echo.WrapHandler(http.StripPrefix("/", http.FileServer(http.FS(buildFS)))))

	// SPA fallback - serve index.html for all other routes
	e.GET("/*", func(c echo.Context) error {
		indexFile, err := buildFS.Open("index.html")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to open index.html")
		}
		defer indexFile.Close()
		return c.Stream(http.StatusOK, "text/html; charset=utf-8", indexFile)
	})

	address := appConfig.App.Address
	if appConfig.App.UseTLS {
		log.Fatal(e.StartTLS(address, appConfig.App.HTTPTLSCert, appConfig.App.HTTPTLSKey))
	} else {
		log.Fatal(e.Start(address))
	}
}

// startWorker creates a worker that processes jobs using the shared scheduler.
func startWorker(sch scheduler.TaskScheduler, logger *slog.Logger) {
	logger.Info("Starting scheduler worker")
	if err := sch.Start(context.Background()); err != nil {
		logger.Error("Failed to start scheduler", "error", err)
		log.Fatal(err)
	}

	select {}
}
