/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/handlers"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
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

	s := repo.NewPostgresStore(db)

	flows, err := processYAMLFiles("./testdata", s)
	if err != nil {
		log.Fatal(err)
	}

	h := handlers.NewHandler(flows, s)

	e := echo.New()
	views := e.Group("/view")
	views.POST("/trigger/:flow", h.HandleTrigger)
	views.GET("/:flow", h.HandleForm)

	e.Start(":7000")
}

func processYAMLFiles(dirPath string, store repo.Store) (map[string]flow.Flow, error) {
	m := make(map[string]flow.Flow)

	// Clear all flows
	err := store.DeleteAllFlows(context.Background())
	if err != nil {
		return nil, fmt.Errorf("could not empty flows table: %w", err)
	}

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

		var f flow.Flow
		if err := yaml.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("error parsing YAML in %s: %v", path, err)
		}
		if err := f.Validate(); err != nil {
			log.Println(err)
		} else {
			// Insert into db
			fd, err := store.GetFlowBySlug(context.Background(), f.Meta.ID)
			if err != nil {
				fd, err = store.CreateFlow(context.Background(), repo.CreateFlowParams{
					Slug:        f.Meta.ID,
					Name:        f.Meta.Name,
					Description: sql.NullString{String: f.Meta.Description, Valid: true},
				})
				if err != nil {
					return err
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
	listener := pq.NewListener(fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", viper.GetString("db.user"), viper.GetString("db.password"), viper.GetString("db.host"), viper.GetInt("db.port"), viper.GetString("db.dbname")), 10*time.Second, time.Minute,
		func(event pq.ListenerEventType, err error) {
			if err != nil {
				log.Printf("Error on listener: %v\n", err)
			}
		})
	defer listener.Close()

	err := listener.Listen("new_flow")
	if err != nil {
		log.Fatalf("error starting postgres listener for notifications: %w", err)
	}

	for {
		log.Println(<-listener.Notify)
	}
}
