package cmd

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/casbin/casbin/v2"
	casbin_model "github.com/casbin/casbin/v2/model"
	sqlxadapter "github.com/memwey/casbin-sqlx-adapter"
	"github.com/cvhariharan/autopilot/internal/core"
	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/handlers"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/cvhariharan/autopilot/internal/runner"
	"github.com/cvhariharan/autopilot/internal/streamlogger"
	"github.com/cvhariharan/autopilot/internal/tasks"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gocloud.dev/secrets"
	_ "gocloud.dev/secrets/localsecrets"
	"gopkg.in/yaml.v3"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start autopilot server or worker",
	Long:  "Use --worker to start a worker. The default command runs the server",
	Run: func(cmd *cobra.Command, args []string) {
		isWorker, _ := cmd.Flags().GetBool("worker")
		start(isWorker)
	},
}

func init() {
	startCmd.Flags().Bool("worker", false, "Start autopilot worker")
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
	a := sqlxadapter.NewAdapter("postgres", dbConnectionString)
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

	if isWorker {
		startWorker(db, redisClient, logger, keeper, enforcer)
	} else {
		startServer(db, redisClient, logger, keeper, enforcer)
	}
}

func startServer(db *sqlx.DB, redisClient redis.UniversalClient, logger *slog.Logger, keeper *secrets.Keeper, enforcer *casbin.Enforcer) {
	asynqClient := asynq.NewClientFromRedisClient(redisClient)
	defer asynqClient.Close()

	s := repo.NewPostgresStore(db)

	flows, err := processYAMLFiles(viper.GetString("app.flows_directory"), s)
	if err != nil {
		log.Fatal(err)
	}

	co := core.NewCore(flows, s, asynqClient, redisClient, keeper, enforcer)

	// Initialize RBAC policies
	if err := co.InitializeRBACPolicies(); err != nil {
		log.Fatalf("could not initialize RBAC policies: %v", err)
	}

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

	e.Renderer = handlers.NewTemplateRenderer("web/**/**/*.html")

	e.Static("/static", "web/static")

	e.GET("/ping", h.HandlePing)
	e.GET("/login", h.HandleLoginView)
	e.POST("/login", h.HandleLoginPage)

	// oidc
	e.GET("/login/oidc", h.HandleOIDCLogin)
	e.GET("/auth/callback", h.HandleAuthCallback)

	e.Logger.SetLevel(0)

	e.HTTPErrorHandler = h.ErrorHandler

	views := e.Group("/view")
	views.Use(h.Authenticate, h.NamespaceMiddleware)

	// views.POST("/trigger/:flow", h.HandleFlowTrigger)
	views.GET("/:namespace/flows/:flow", h.HandleFlowFormView)
	views.GET("/:namespace", h.HandleFlowsListView)
	views.GET("/:namespace/results/:flowID/:logID", h.HandleFlowExecutionResults)
	views.GET("/:namespace/nodes", h.HandleNodesView)
	views.GET("/:namespace/credentials", h.HandleCredentialsView)
	// views.GET("/logs/:logID", h.HandleLogStreaming)
	// views.GET("/summary/:flowID", h.HandleExecutionSummary)

	views.GET("/:namespace/approvals/:approvalID", h.HandleApprovalView, h.ApprovalMiddleware)

	api := e.Group("/api/v1", h.Authenticate)

	// Global admin endpoints for users and groups
	api.GET("/users", h.HandleUserPagination)
	api.GET("/users/profile", h.HandleGetUserProfile)
	api.GET("/users/:userID", h.HandleGetUser)
	api.POST("/users", h.HandleCreateUser)
	api.DELETE("/users/:userID", h.HandleDeleteUser)
	api.PUT("/users/:userID", h.HandleUpdateUser)

	api.GET("/groups", h.HandleGroupPagination)
	api.GET("/groups/:groupID", h.HandleGetGroup)
	api.PUT("/groups/:groupID", h.HandleUpdateGroup)
	api.POST("/groups", h.HandleCreateGroup)
	api.DELETE("/groups/:groupID", h.HandleDeleteGroup)

	// Namespace management
	api.GET("/namespaces", h.HandleListNamespaces)
	api.GET("/namespaces/:namespaceID", h.HandleGetNamespace, h.NamespaceMiddleware)
	api.POST("/namespaces", h.HandleCreateNamespace, h.AuthorizeForRole("admin"))
	api.PUT("/namespaces/:namespaceID", h.HandleUpdateNamespace, h.AuthorizeForRole("admin"))
	api.DELETE("/namespaces/:namespaceID", h.HandleDeleteNamespace, h.AuthorizeForRole("admin"))

	// Namespace-specific resource endpoints using RBAC
	namespaceGroup := api.Group("/:namespace", h.NamespaceMiddleware)

	// Flow routes - users can view and execute
	namespaceGroup.GET("/flows", h.HandleFlowsPagination, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.GET("/flows/:flowID", h.HandleGetFlow, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
	namespaceGroup.GET("/flows/summary/:execID", h.HandleGetExecutionSummary, h.AuthorizeNamespaceAction(models.ResourceFlow, models.RBACActionView))
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
	namespaceGroup.POST("/approvals/:approvalID", h.HandleApprovalAction, h.AuthorizeNamespaceAction(models.ResourceApproval, models.RBACActionApprove), h.ApprovalMiddleware)

	// Namespace management - admins only
	namespaceGroup.GET("/members", h.HandleGetNamespaceMembers, h.AuthorizeNamespaceAction(models.ResourceNamespace, models.RBACActionView))
	namespaceGroup.POST("/members", h.HandleAddNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceNamespace, models.RBACActionUpdate))
	namespaceGroup.DELETE("/members/:membershipID", h.HandleRemoveNamespaceMember, h.AuthorizeNamespaceAction(models.ResourceNamespace, models.RBACActionUpdate))

	admin := e.Group("/admin")
	admin.Use(h.AuthorizeForRole("admin"))

	admin.GET("/users", h.HandleUserManagementView)
	admin.GET("/groups", h.HandleGroupManagementView)

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

func processYAMLFiles(rootDir string, store repo.Store) (map[string]models.Flow, error) {
	m := make(map[string]models.Flow)

	// Read immediate subdirectories
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("error reading root directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(rootDir, entry.Name())
		yamlFound := false

		// Look for YAML files in the project directory
		projectFiles, err := os.ReadDir(projectDir)
		if err != nil {
			log.Printf("error reading project directory %s: %v", projectDir, err)
			continue
		}

		for _, file := range projectFiles {
			if file.IsDir() {
				continue
			}

			filename := strings.ToLower(file.Name())
			if !strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml") {
				continue
			}

			yamlPath := filepath.Join(projectDir, file.Name())
			data, err := os.ReadFile(yamlPath)
			if err != nil {
				log.Printf("error reading file %s: %v", yamlPath, err)
				continue
			}

			// Calculate checksum
			h := sha256.New()
			if _, err := h.Write(data); err != nil {
				log.Printf("error hashing file %s: %v", yamlPath, err)
				continue
			}
			checksum := hex.EncodeToString(h.Sum(nil))

			var f models.Flow
			if err := yaml.Unmarshal(data, &f); err != nil {
				log.Printf("error parsing YAML in %s: %v", yamlPath, err)
				continue
			}

			if err := f.Validate(); err != nil {
				log.Fatalf("validation error in %s: %v", yamlPath, err)
			}

			// Set the source directory
			f.Meta.SrcDir = entry.Name()
			yamlFound = true

			if f.Meta.Namespace == "" {
				f.Meta.Namespace = "default"
			}

			ns, err := store.GetNamespaceByName(context.Background(), f.Meta.Namespace)
			if err != nil {
				log.Printf("error getting namespace %s: %v", f.Meta.Namespace, err)
				continue
			}

			// Database operations
			fd, err := store.GetFlowBySlug(context.Background(), repo.GetFlowBySlugParams{
				Slug: f.Meta.ID,
				Uuid: ns.Uuid,
			})
			if err != nil {
				// Create new flow
				fd, err = store.CreateFlow(context.Background(), repo.CreateFlowParams{
					Slug:        f.Meta.ID,
					Name:        f.Meta.Name,
					Checksum:    checksum,
					Description: sql.NullString{String: f.Meta.Description, Valid: true},
					Name_2:      f.Meta.Namespace,
				})
				if err != nil {
					log.Printf("error creating flow %s: %v", f.Meta.ID, err)
					continue
				}
			} else if fd.Checksum != checksum {
				// Update existing flow if checksum differs
				fd, err = store.UpdateFlow(context.Background(), repo.UpdateFlowParams{
					Name:        f.Meta.Name,
					Description: sql.NullString{String: f.Meta.Description, Valid: true},
					Checksum:    checksum,
					Slug:        f.Meta.ID,
					Name_2:      f.Meta.Namespace,
				})
				if err != nil {
					log.Printf("error updating flow %s: %v", f.Meta.ID, err)
					continue
				}
			}

			f.Meta.DBID = fd.ID

			m[fmt.Sprintf("%s:%s", f.Meta.ID, ns.Uuid.String())] = f
		}

		if !yamlFound {
			log.Printf("no YAML file found in directory: %s", projectDir)
		}
	}

	return m, nil
}

func startWorker(db *sqlx.DB, redisClient redis.UniversalClient, logger *slog.Logger, keeper *secrets.Keeper, enforcer *casbin.Enforcer) {
	asynqClient := asynq.NewClientFromRedisClient(redisClient)
	defer asynqClient.Close()

	asynqSrv := asynq.NewServerFromRedisClient(redisClient, asynq.Config{
		Concurrency: 0,
		Queues: map[string]int{
			"default": 5,
			"resume":  5,
		},
	})

	s := repo.NewPostgresStore(db)
	flows, err := processYAMLFiles(viper.GetString("app.flows_directory"), s)
	if err != nil {
		log.Fatal(err)
	}

	core := core.NewCore(flows, s, asynqClient, redisClient, keeper, enforcer)

	flowLogger := streamlogger.NewStreamLogger(redisClient)
	flowRunner := tasks.NewFlowRunner(flowLogger, runner.NewDockerArtifactsManager("./artifacts"), core.BeforeActionHook, nil, logger)

	st := tasks.NewStatusTracker(s)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeFlowExecution, st.TrackerMiddleware(flowRunner.HandleFlowExecution))

	log.Fatal(asynqSrv.Run(mux))
}
