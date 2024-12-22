package cmd

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/autopilot/internal/auth"
	"github.com/cvhariharan/autopilot/internal/core"
	"github.com/cvhariharan/autopilot/internal/handlers"
	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/cvhariharan/autopilot/internal/runner"
	"github.com/cvhariharan/autopilot/internal/tasks"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
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
		startWorker(db, redisClient)
	} else {
		startServer(db, redisClient)
	}
}

func startServer(db *sqlx.DB, redisClient redis.UniversalClient) {
	asynqClient := asynq.NewClientFromRedisClient(redisClient)
	defer asynqClient.Close()

	s := repo.NewPostgresStore(db)

	flows, err := processYAMLFiles("./testdata", s)
	if err != nil {
		log.Fatal(err)
	}

	co := core.NewCore(flows, s, asynqClient, redisClient)

	h := handlers.NewHandler(co)

	ah, err := auth.NewAuthHandler(db.DB, co, auth.OIDCAuthConfig{
		Issuer:       viper.GetString("app.oidc.issuer"),
		ClientID:     viper.GetString("app.oidc.client_id"),
		ClientSecret: viper.GetString("app.oidc.client_secret"),
	})
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.GET("/login", ah.HandleLoginPage)
	e.POST("/login", ah.HandleLoginPage)

	// oidc
	e.GET("/login/oidc", ah.HandleOIDCLogin)
	e.GET("/auth/callback", ah.HandleAuthCallback)

	e.Logger.SetLevel(0)

	e.HTTPErrorHandler = handlers.ErrorHandler

	views := e.Group("/view")
	views.Use(ah.Authenticate)

	views.POST("/trigger/:flow", h.HandleFlowTrigger)
	views.GET("/:flow", h.HandleFlowForm)
	views.GET("/", h.HandleFlowsList)
	views.GET("/results/:flowID/:logID", h.HandleFlowExecutionResults)
	views.GET("/logs/:logID", h.HandleLogStreaming)
	views.GET("/summary/:flowID", h.HandleExecutionSummary)

	admin := e.Group("/admin")
	admin.GET("/groups", h.HandleGroup)
	admin.POST("/groups", h.HandleCreateGroup)
	admin.DELETE("/groups/:groupID", h.HandleDeleteGroup)
	admin.GET("/groups/search", h.HandleGroupSearch)

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

func processYAMLFiles(dirPath string, store repo.Store) (map[string]models.Flow, error) {
	m := make(map[string]models.Flow)

	if err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(path), ".yml") &&
			!strings.HasSuffix(strings.ToLower(path), ".yaml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %v", path, err)
		}

		h := sha256.New()
		if _, err := h.Write(data); err != nil {
			return fmt.Errorf("error hashing file %s: %v", path, err)
		}
		checksum := hex.EncodeToString(h.Sum(nil))

		var f models.Flow
		if err := yaml.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("error parsing YAML in %s: %v", path, err)
		}
		if err := f.Validate(); err != nil {
			log.Println(err)
		} else {
			// Insert into db
			fd, err := store.GetFlowBySlug(context.Background(), f.Meta.ID)
			// Create if flow doesn't exist
			if err != nil {
				fd, err = store.CreateFlow(context.Background(), repo.CreateFlowParams{
					Slug:        f.Meta.ID,
					Name:        f.Meta.Name,
					Checksum:    checksum,
					Description: sql.NullString{String: f.Meta.Description, Valid: true},
				})
				if err != nil {
					return fmt.Errorf("error creating flow %s: %v", f.Meta.ID, err)
				}
			}

			if fd.Checksum != checksum {
				fd, err = store.UpdateFlow(context.Background(), repo.UpdateFlowParams{
					Name:        f.Meta.Name,
					Description: sql.NullString{String: f.Meta.Description, Valid: true},
					Checksum:    checksum,
					Slug:        f.Meta.ID,
				})
				if err != nil {
					return fmt.Errorf("error updating flow %s: %v", f.Meta.ID, err)
				}
			}
			f.Meta.DBID = fd.ID
			m[f.Meta.ID] = f
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

func startWorker(db *sqlx.DB, redisClient redis.UniversalClient) {
	flowLogger := runner.NewStreamLogger(redisClient)
	flowRunner := tasks.NewFlowRunner(flowLogger, runner.NewDockerArtifactsManager("./artifacts"))

	asynqSrv := asynq.NewServerFromRedisClient(redisClient, asynq.Config{
		Concurrency: 0,
	})

	s := repo.NewPostgresStore(db)
	st := tasks.NewStatusTracker(s)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeFlowExecution, st.TrackerMiddleware(flowRunner.HandleFlowExecution))

	log.Fatal(asynqSrv.Run(mux))
}
