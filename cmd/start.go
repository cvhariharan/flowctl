/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/handlers"
	"github.com/cvhariharan/autopilot/internal/repo"
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
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start autopilot server",
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start autopilot worker",
	Run: func(cmd *cobra.Command, args []string) {
		startWorker()
	},
}

func init() {
	startCmd.AddCommand(serverCmd)
	startCmd.AddCommand(workerCmd)
	rootCmd.AddCommand(startCmd)
}

func startServer() {
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

	asynqClient := asynq.NewClientFromRedisClient(redisClient)
	defer asynqClient.Close()

	s := repo.NewPostgresStore(db)

	flows, err := processYAMLFiles("./testdata", s)
	if err != nil {
		log.Fatal(err)
	}

	h := handlers.NewHandler(flows, s, asynqClient)

	e := echo.New()
	views := e.Group("/view")
	views.POST("/trigger/:flow", h.HandleTrigger)
	views.GET("/:flow", h.HandleForm)

	e.Start(":7000")
}

func processYAMLFiles(dirPath string, store repo.Store) (map[string]flow.Flow, error) {
	m := make(map[string]flow.Flow)

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

		var f flow.Flow
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

func startWorker() {
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

	flowRunner := &tasks.FlowRunner{}

	asynqSrv := asynq.NewServerFromRedisClient(redisClient, asynq.Config{
		Concurrency: 8,
	})

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeFlowExecution, flowRunner.HandleFlowExecution)

	log.Fatal(asynqSrv.Run(mux))

	// s := repo.NewPostgresStore(db)
	// q := queue.NewQueue(s)

	// listener := pq.NewListener(fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", viper.GetString("db.user"), viper.GetString("db.password"), viper.GetString("db.host"), viper.GetInt("db.port"), viper.GetString("db.dbname")), 10*time.Second, time.Minute,
	// 	func(event pq.ListenerEventType, err error) {
	// 		if err != nil {
	// 			log.Printf("Error on listener: %v\n", err)
	// 		}
	// 	})
	// defer listener.Close()

	// jchan, err := q.ListenForJobs(context.Background(), listener, queue.DEFAULT_BATCH_INTERVAL, 4)
	// if err != nil {
	// 	log.Fatalf("error listening for jobs: %v", err)
	// }

	// for job := range jchan {

	// 	log.Println(job)
	// }
}
