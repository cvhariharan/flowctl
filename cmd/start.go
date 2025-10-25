package cmd

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	casbin_model "github.com/casbin/casbin/v2/model"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/handlers"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/cvhariharan/flowctl/internal/scheduler/storage"
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
		startServer(shared.DB, shared.Core, shared.Logger)
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

// SharedComponents holds components that are shared between server and worker
type SharedComponents struct {
	DB        *sqlx.DB
	Core      *core.Core
	Scheduler *scheduler.Scheduler
	Logger    *slog.Logger
	Keeper    *secrets.Keeper
}

// Cleanup cleans up all shared resources
func (s *SharedComponents) Cleanup() {
	if s.DB != nil {
		s.DB.Close()
	}
	if s.Keeper != nil {
		s.Keeper.Close()
	}
}

// initializeSharedComponents sets up all shared components (DB, scheduler, core, etc.)
func initializeSharedComponents() *SharedComponents {
	loglevel := slog.LevelError
	if os.Getenv("DEBUG_LOG") == "true" {
		loglevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: loglevel,
	}))
	slog.SetDefault(logger)

	// Create shared FileLogManager instance
	fileLogManager := streamlogger.NewFileLogManager(streamlogger.FileLogManagerCfg{
		RetentionTime: time.Duration(appConfig.App.Logger.RetentionTimeHours) * time.Hour,
		MaxSizeBytes:  appConfig.App.Logger.MaxSizeBytes * 1024 * 1024,
		LogDir:        appConfig.App.Logger.Directory,
		ScanInterval:  appConfig.App.Logger.ScanInterval,
	})
	go fileLogManager.Run(context.Background(), logger.WithGroup("file_log_manager"))

	dbConnectionString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", appConfig.DB.User, appConfig.DB.Password, appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.DBName)
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

	keeperURL := appConfig.App.Keystore.KeeperURL
	if keeperURL == "" {
		log.Fatal("app.keystore.keeper_url is not set")
	}

	keeper, err := secrets.OpenKeeper(context.Background(), keeperURL)
	if err != nil {
		log.Fatalf("could not open secrets keeper: %v", err)
	}

	s := repo.NewPostgresStore(db)

	jobStore := storage.NewPostgresStorage(db)

	// Build scheduler
	sch, err := scheduler.NewSchedulerBuilder(logger.WithGroup("scheduler")).
		WithStore(s).
		WithJobStore(jobStore).
		WithLogManager(fileLogManager).
		WithWorkerCount(appConfig.App.Scheduler.WorkerCount).
		WithCronSyncInterval(appConfig.App.Scheduler.CronSyncInterval).
		Build()

	if err != nil {
		log.Fatal(err)
	}

	co, err := core.NewCore(appConfig.App.FlowsDirectory, s, sch, keeper, enforcer)
	if err != nil {
		log.Fatal(err)
	}

	co.LogManager = fileLogManager

	// Set secrets provider and flow loader after core is created
	sch.SetSecretsProvider(co.GetDecryptedFlowSecrets)
	sch.SetFlowLoader(co.GetSchedulerFlow)

	return &SharedComponents{
		DB:        db,
		Core:      co,
		Scheduler: sch,
		Logger:    logger,
		Keeper:    keeper,
	}
}

func startServer(db *sqlx.DB, co *core.Core, logger *slog.Logger) {
	h, err := handlers.NewHandler(logger, db.DB, co, appConfig)
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Use(middleware.Recover())

	e.GET("/ping", h.HandlePing)
	e.POST("/login", h.HandleLoginPage)
	e.POST("/logout", h.HandleLogout)

	// oidc
	e.GET("/login/oidc", h.HandleOIDCLogin)
	e.GET("/auth/callback", h.HandleAuthCallback)

	e.Logger.SetLevel(0)

	e.HTTPErrorHandler = h.ErrorHandler

	api := e.Group("/api/v1", h.Authenticate)

	api.GET("/users", h.HandleUserPagination, h.AuthorizeForRole("superuser"))
	api.GET("/users/profile", h.HandleGetUserProfile)
	api.GET("/users/:userID", h.HandleGetUser, h.AuthorizeForRole("superuser"))
	api.POST("/users", h.HandleCreateUser, h.AuthorizeForRole("superuser"))
	api.DELETE("/users/:userID", h.HandleDeleteUser, h.AuthorizeForRole("superuser"))
	api.PUT("/users/:userID", h.HandleUpdateUser, h.AuthorizeForRole("superuser"))

	api.GET("/groups", h.HandleGroupPagination, h.AuthorizeForRole("superuser"))
	api.GET("/groups/:groupID", h.HandleGetGroup, h.AuthorizeForRole("superuser"))
	api.PUT("/groups/:groupID", h.HandleUpdateGroup, h.AuthorizeForRole("superuser"))
	api.POST("/groups", h.HandleCreateGroup, h.AuthorizeForRole("superuser"))
	api.DELETE("/groups/:groupID", h.HandleDeleteGroup, h.AuthorizeForRole("superuser"))

	// No authorization required
	api.GET("/executors/:executor/config", h.HandleGetExecutorConfig)
	api.GET("/executors", h.HandleListExecutors)
	api.GET("/permissions", h.HandleGetCasbinPermissions)

	// Namespace management
	api.GET("/namespaces", h.HandleListNamespaces)
	api.GET("/namespaces/:namespaceID", h.HandleGetNamespace, h.AuthorizeForRole("superuser"))
	api.POST("/namespaces", h.HandleCreateNamespace, h.AuthorizeForRole("superuser"))
	api.PUT("/namespaces/:namespaceID", h.HandleUpdateNamespace, h.AuthorizeForRole("superuser"))
	api.DELETE("/namespaces/:namespaceID", h.HandleDeleteNamespace, h.AuthorizeForRole("superuser"))

	// Namespace-specific resource endpoints using RBAC
	namespaceGroup := api.Group("/:namespace", h.NamespaceMiddleware)

	// Flow routes - users can view and execute
	namespaceGroup.GET("/flows", h.HandleFlowsPagination, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.POST("/flows", h.HandleCreateFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionCreate))
	namespaceGroup.GET("/flows/:flowID", h.HandleGetFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.PUT("/flows/:flowID", h.HandleUpdateFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionUpdate))
	namespaceGroup.DELETE("/flows/:flowID", h.HandleDeleteFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionDelete))
	namespaceGroup.GET("/flows/executions/:execID", h.HandleGetExecutionSummary, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.POST("/flows/executions/:execID/cancel", h.HandleCancelExecution, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionUpdate))
	namespaceGroup.GET("/flows/:flowID/executions", h.HandleExecutionsPagination, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))
	namespaceGroup.GET("/flows/executions", h.HandleAllExecutionsPagination, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/inputs", h.HandleGetFlowInputs, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/meta", h.HandleGetFlowMeta, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/config", h.HandleGetFlowConfig, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionCreate))

	// Flow secrets routes - only admins can manage secrets
	namespaceGroup.GET("/flows/:flowID/secrets", h.HandleListFlowSecrets, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID/secrets/:secretID", h.HandleGetFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionView))
	namespaceGroup.POST("/flows/:flowID/secrets", h.HandleCreateFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionCreate))
	namespaceGroup.PUT("/flows/:flowID/secrets/:secretID", h.HandleUpdateFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionUpdate))
	namespaceGroup.DELETE("/flows/:flowID/secrets/:secretID", h.HandleDeleteFlowSecret, h.AuthorizeNamespaceAction(models.ResourceFlowSecret, models.RBACActionDelete))
	namespaceGroup.POST("/trigger/:flow", h.HandleFlowTrigger, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionExecute))
	namespaceGroup.GET("/logs/:logID", h.HandleLogStreaming, h.AuthorizeNamespaceAction(models.ResourceExecution, models.RBACActionView))

	// Node routes - only admins can create/update/delete
	namespaceGroup.GET("/nodes", h.HandleListNodes, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionView))
	namespaceGroup.GET("/nodes/stats", h.HandleGetNodeStats, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionView))
	namespaceGroup.GET("/nodes/:nodeID", h.HandleGetNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionView))
	namespaceGroup.POST("/nodes", h.HandleCreateNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionCreate))
	namespaceGroup.PUT("/nodes/:nodeID", h.HandleUpdateNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionUpdate))
	namespaceGroup.DELETE("/nodes/:nodeID", h.HandleDeleteNode, h.AuthorizeNamespaceAction(models.ResourceNode, models.RBACActionDelete))

	// Credential routes - only admins can create/update/delete
	namespaceGroup.GET("/credentials", h.HandleListCredentials, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionView))
	namespaceGroup.GET("/credentials/:credID", h.HandleGetCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionView))
	namespaceGroup.POST("/credentials", h.HandleCreateCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionCreate))
	namespaceGroup.PUT("/credentials/:credID", h.HandleUpdateCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionUpdate))
	namespaceGroup.DELETE("/credentials/:credID", h.HandleDeleteCredential, h.AuthorizeNamespaceAction(models.ResourceCredential, models.RBACActionDelete))

	// Approval routes - operators and admins
	namespaceGroup.GET("/approvals", h.HandleListApprovals, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionView))
	namespaceGroup.GET("/approvals/:approvalID", h.HandleGetApproval, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionView))
	namespaceGroup.POST("/approvals/:approvalID", h.HandleApprovalAction, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionApprove))

	// Namespace management - admins only
	namespaceGroup.GET("/members", h.HandleGetNamespaceMembers, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionView))
	namespaceGroup.POST("/members", h.HandleAddNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionCreate))
	namespaceGroup.PUT("/members/:membershipID", h.HandleUpdateNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionUpdate))
	namespaceGroup.DELETE("/members/:membershipID", h.HandleRemoveNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionDelete))

	buildFS, err := fs.Sub(StaticFiles, "site/build")
	if err != nil {
		log.Fatal(err)
	}

	// Serve static assets from embedded FS
	e.GET("/_app/*", echo.WrapHandler(http.FileServer(http.FS(buildFS))))
	e.GET("/robots.txt", echo.WrapHandler(http.StripPrefix("/", http.FileServer(http.FS(buildFS)))))

	// SPA fallback - serve index.html for all other routes
	e.GET("/*", func(c echo.Context) error {
		indexFile, err := buildFS.Open("index.html")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to open index.html")
		}
		defer indexFile.Close()
		return c.Stream(http.StatusOK, "text/html; charset=utf-8", indexFile)
	})

	rootURL := appConfig.App.RootURL
	if !strings.Contains(rootURL, "://") {
		log.Fatal("root_url should contain a scheme")
	}

	u, err := url.Parse(rootURL)
	if err != nil {
		log.Fatalf("invalid root_url: %v", err)
	}

	log.Fatal(e.Start(u.Host))
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
