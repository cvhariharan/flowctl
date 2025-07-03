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
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: loglevel,
	}))
	slog.SetDefault(logger)

	db, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", viper.GetString("db.user"), viper.GetString("db.password"), viper.GetString("db.host"), viper.GetInt("db.port"), viper.GetString("db.dbname")))
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer db.Close()

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{fmt.Sprintf("%s:%d", viper.GetString("redis.host"), viper.GetInt("redis.port"))},
		Password: viper.GetString("redis.password"),
	})
	defer redisClient.Close()

	if isWorker {
		startWorker(db, redisClient, logger)
	} else {
		startServer(db, redisClient, logger)
	}
}

func startServer(db *sqlx.DB, redisClient redis.UniversalClient, logger *slog.Logger) {
	asynqClient := asynq.NewClientFromRedisClient(redisClient)
	defer asynqClient.Close()

	s := repo.NewPostgresStore(db)

	flows, err := processYAMLFiles(viper.GetString("app.flows_directory"), s)
	if err != nil {
		log.Fatal(err)
	}

	co := core.NewCore(flows, s, asynqClient, redisClient)

	h, err := handlers.NewHandler(logger, db.DB, co, handlers.OIDCAuthConfig{
		Issuer:       viper.GetString("app.oidc.issuer"),
		ClientID:     viper.GetString("app.oidc.client_id"),
		ClientSecret: viper.GetString("app.oidc.client_secret"),
	})
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Use(middleware.Logger())

	e.GET("/ping", h.HandlePing)
	e.GET("/login", h.HandleLoginPage)
	e.POST("/login", h.HandleLoginPage)

	// oidc
	e.GET("/login/oidc", h.HandleOIDCLogin)
	e.GET("/auth/callback", h.HandleAuthCallback)

	e.Logger.SetLevel(0)

	e.HTTPErrorHandler = h.ErrorHandler

	views := e.Group("/view")
	views.Use(h.Authenticate)

	// views.POST("/trigger/:flow", h.HandleFlowTrigger)
	// views.GET("/:flow", h.HandleFlowForm)
	// views.GET("/", h.HandleFlowsList)
	// views.GET("/results/:flowID/:logID", h.HandleFlowExecutionResults)
	// views.GET("/logs/:logID", h.HandleLogStreaming)
	// views.GET("/summary/:flowID", h.HandleExecutionSummary)

	views.GET("/approvals/:approvalID", h.HandleApprovalRequest, h.ApprovalMiddleware)
	views.POST("/approvals/:approvalID/:action", h.HandleApprovalAction, h.ApprovalMiddleware)

	admin := e.Group("/admin")
	admin.Use(h.AuthorizeForRole("admin"))
	admin.GET("/groups", h.HandleGroup)
	admin.POST("/groups", h.HandleCreateGroup)
	admin.DELETE("/groups/:groupID", h.HandleDeleteGroup)
	admin.GET("/groups/search", h.HandleGroupSearch)

	admin.GET("/users", h.HandleUser)
	admin.POST("/users", h.HandleCreateUser)
	admin.GET("/users/search", h.HandleUserSearch)
	admin.DELETE("/users/:userID", h.HandleDeleteUser)
	admin.PUT("/users/:userID", h.HandleUpdateUser)

	admin.GET("/requests/:execID", h.HandleApprovalRequest)

	rootURL := viper.GetString("app.root_url")
	if !strings.Contains(rootURL, "://") {
		log.Fatal("root_url should contain a scheme")
	}

	u, err := url.Parse(rootURL)
	if err != nil {
		log.Fatalf("invalid root_url: %v", err)
	}

	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Root:   "web",
		Index:  "index.html",
		HTML5:  true,
		Browse: false,
	}))

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
				log.Printf("validation error in %s: %v", yamlPath, err)
				continue
			}

			// Set the source directory
			f.Meta.SrcDir = entry.Name()
			yamlFound = true

			// Database operations
			fd, err := store.GetFlowBySlug(context.Background(), f.Meta.ID)
			if err != nil {
				// Create new flow
				fd, err = store.CreateFlow(context.Background(), repo.CreateFlowParams{
					Slug:        f.Meta.ID,
					Name:        f.Meta.Name,
					Checksum:    checksum,
					Description: sql.NullString{String: f.Meta.Description, Valid: true},
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
				})
				if err != nil {
					log.Printf("error updating flow %s: %v", f.Meta.ID, err)
					continue
				}
			}

			f.Meta.DBID = fd.ID
			m[f.Meta.ID] = f
		}

		if !yamlFound {
			log.Printf("no YAML file found in directory: %s", projectDir)
		}
	}

	return m, nil
}

func startWorker(db *sqlx.DB, redisClient redis.UniversalClient, logger *slog.Logger) {
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

	core := core.NewCore(flows, s, asynqClient, redisClient)

	flowLogger := streamlogger.NewStreamLogger(redisClient)
	flowRunner := tasks.NewFlowRunner(flowLogger, runner.NewDockerArtifactsManager("./artifacts"), core.BeforeActionHook, nil)

	st := tasks.NewStatusTracker(s)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeFlowExecution, st.TrackerMiddleware(flowRunner.HandleFlowExecution))

	log.Fatal(asynqSrv.Run(mux))
}
