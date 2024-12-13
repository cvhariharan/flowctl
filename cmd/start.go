/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/handlers"
	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
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

func init() {
	startCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(startCmd)
}

func startServer() {
	flows, err := processYAMLFiles("./testdata")
	if err != nil {
		log.Fatal(err)
	}

	h := handlers.NewHandler(flows)

	e := echo.New()
	views := e.Group("/view")
	views.POST("/trigger/:flow", h.HandleTrigger)
	views.GET("/:flow", h.HandleForm)

	e.Start(":7000")
}

func processYAMLFiles(dirPath string) (map[string]flow.Flow, error) {
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

		var f flow.Flow
		if err := yaml.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("error parsing YAML in %s: %v", path, err)
		}

		m[f.Meta.ID] = f
		if err := f.Validate(); err != nil {
			log.Println(err)
			delete(m, f.Meta.ID)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}
