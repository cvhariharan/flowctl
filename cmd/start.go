package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	casbin_model "github.com/casbin/casbin/v2/model"
	redisadapter "github.com/casbin/redis-adapter/v3"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/handlers"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/tasks"
	"github.com/cvhariharan/flowctl/internal/tasks/scheduler"
	"github.com/cvhariharan/flowctl/internal/utils"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gocloud.dev/secrets"
	_ "gocloud.dev/secrets/localsecrets"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start flowctl server",
	Run: func(cmd *cobra.Command, args []string) {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// start worker
			start(true)
		}()
		// start server
		start(false)
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func start(isWorker bool) {
	loglevel := slog.LevelError
	if os.Getenv("DEBUG_LOG") == "true" {
		loglevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: loglevel,
	}))
	slog.SetDefault(logger)

	dbConnectionString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", viper.GetString("db.user"), viper.GetString("db.password"), viper.GetString("db.host"), viper.GetInt("db.port"), viper.GetString("db.dbname"))
	db, err := sqlx.Connect("postgres", dbConnectionString)
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer db.Close()

	// Initialize casbin
	m, _ := casbin_model.NewModelFromFile("configs/rbac_model.conf")
	casbinAdapterConfig := &redisadapter.Config{
		Network:  "tcp",
		Address:  fmt.Sprintf("%s:%d", viper.GetString("redis.host"), viper.GetInt("redis.port")),
		Password: viper.GetString("redis.password"),
	}
	a, err := redisadapter.NewAdapter(casbinAdapterConfig)
	if err != nil {
		log.Fatalf("error creating casbin adapter: %v", err)
	}

	enforcer, err := casbin.NewEnforcer(m, a)
	if err != nil {
		log.Fatalf("could not initialize casbin enforcer: %v", err)
	}

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{fmt.Sprintf("%s:%d", viper.GetString("redis.host"), viper.GetInt("redis.port"))},
		Password: viper.GetString("redis.password"),
	})
	defer redisClient.Close()

	// Initialize secret keeper
	keeperURL := viper.GetString("app.keystore.keeper_url")
	if keeperURL == "" {
		log.Fatal("app.keystore.keeper_url is not set")
	}

	keeper, err := secrets.OpenKeeper(context.Background(), keeperURL)
	if err != nil {
		log.Fatalf("could not open secrets keeper: %v", err)
	}
	defer keeper.Close()

	asynqClient := asynq.NewClientFromRedisClient(redisClient)
	defer asynqClient.Close()

	s := repo.NewPostgresStore(db)

	co, err := core.NewCore(viper.GetString("app.flows_directory"), s, asynqClient, redisClient, keeper, enforcer)
	if err != nil {
		log.Fatal(err)
	}

	if isWorker {
		startWorker(db, co, redisClient, logger, keeper, enforcer)
	} else {
		startServer(db, co, logger)
	}
}

func startServer(db *sqlx.DB, co *core.Core, logger *slog.Logger) {
	h, err := handlers.NewHandler(logger, db.DB, co, handlers.OIDCAuthConfig{
		Issuer:       viper.GetString("app.oidc.issuer"),
		ClientID:     viper.GetString("app.oidc.client_id"),
		ClientSecret: viper.GetString("app.oidc.client_secret"),
	}, viper.GetString("app.root_url"))
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	// e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
	//   Format: "method=${method}, uri=${uri}, status=${status}\n",
	// }))

	e.GET("/ping", h.HandlePing)
	e.POST("/login", h.HandleLoginPage)

	// oidc
	e.GET("/login/oidc", h.HandleOIDCLogin)
	e.GET("/auth/callback", h.HandleAuthCallback)

	e.Logger.SetLevel(0)

	e.HTTPErrorHandler = h.ErrorHandler

	api := e.Group("/api/v1", h.Authenticate)

	// Global admin endpoints for users and groups
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
	api.GET("/namespaces/:namespaceID", h.HandleGetNamespace, h.AuthorizeNamespaceAction(models.ResourceNamespace, models.RBACActionView))
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
	namespaceGroup.GET("/approvals", h.HandleListApprovals, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionApprove))
	namespaceGroup.POST("/approvals/:approvalID", h.HandleApprovalAction, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionApprove))

	// Namespace management - admins only
	namespaceGroup.GET("/members", h.HandleGetNamespaceMembers, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionView))
	namespaceGroup.POST("/members", h.HandleAddNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionUpdate))
	namespaceGroup.DELETE("/members/:membershipID", h.HandleRemoveNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceMember, models.RBACActionUpdate))

	// admin := e.Group("/admin")
	// admin.Use(h.AuthorizeForRole("superuser"))
	// admin.GET("/settings", h.HandleSettingsView)

	// Serve static assets from SvelteKit build
	e.Static("/_app", "site/build/_app")
	e.File("/robots.txt", "site/build/robots.txt")

	// SPA fallback - serve index.html for all other routes
	e.GET("/*", func(c echo.Context) error {
		return c.File("site/build/index.html")
	})

	rootURL := viper.GetString("app.root_url")
	if !strings.Contains(rootURL, "://") {
		log.Fatal("root_url should contain a scheme")
	}

	u, err := url.Parse(rootURL)
	if err != nil {
		log.Fatalf("invalid root_url: %v", err)
	}

	log.Fatal(e.Start(u.Host))
}

// startWorker creates a worker that processes jobs from redis.
// A single worker automatically uses all available CPU cores for concurrency.
func startWorker(db *sqlx.DB, co *core.Core, redisClient redis.UniversalClient, logger *slog.Logger, keeper *secrets.Keeper, enforcer *casbin.Enforcer) {
	asynqClient := asynq.NewClientFromRedisClient(redisClient)
	defer asynqClient.Close()

	asynqSrv := asynq.NewServerFromRedisClient(redisClient, asynq.Config{
		Concurrency: 0,
		Queues: map[string]int{
			"default": 5,
			"resume":  5,
		},
		Logger: &utils.SlogAdapter{Logger: logger.WithGroup("worker")},
	})

	s := repo.NewPostgresStore(db)

	flowRunner := tasks.NewFlowRunner(redisClient, co.BeforeActionHook, nil, co.GetDecryptedFlowSecrets, logger)

	st := tasks.NewStatusTracker(s)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeFlowExecution, st.TrackerMiddleware(flowRunner.HandleFlowExecution))

	// Create PeriodicTaskManager for scheduled flows
	scheduleProvider := scheduler.NewFlowScheduleProvider(co)
	preEnqueueFunc := scheduler.NewSchedulePreEnqueueFunc(co)

	periodicTaskManager, err := asynq.NewPeriodicTaskManager(asynq.PeriodicTaskManagerOpts{
		RedisUniversalClient:       redisClient,
		PeriodicTaskConfigProvider: scheduleProvider,
		SyncInterval:               2 * time.Minute, // Check for schedule changes every 2 minutes
		SchedulerOpts: &asynq.SchedulerOpts{
			PreEnqueueFunc: preEnqueueFunc,
		},
	})
	if err != nil {
		logger.Error("Failed to create periodic task manager", "error", err)
		log.Fatal(err)
	}

	// Start periodic task manager in a goroutine
	go func() {
		logger.Info("Starting periodic task manager for scheduled flows")
		if err := periodicTaskManager.Run(); err != nil {
			logger.Error("Periodic task manager error", "error", err)
		}
	}()

	// Ensure periodic task manager is shut down when worker exits
	defer func() {
		logger.Info("Shutting down periodic task manager")
		periodicTaskManager.Shutdown()
	}()

	log.Fatal(asynqSrv.Run(mux))
}
