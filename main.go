package main

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
	"gopkg.in/yaml.v3"
)

func main() {
	flows, err := processYAMLFiles("./testdata")
	if err != nil {
		log.Fatal(err)
	}

	h := handlers.NewHandler(flows)

	e := echo.New()
	e.POST("/api/:flow", h.HandleTrigger)

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
		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}
