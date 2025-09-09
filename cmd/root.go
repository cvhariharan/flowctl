package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/spf13/cobra"
)

var appConfig config.Config

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "flowctl",
	Short: "Self-service workflow execution engine",
	Run: func(cmd *cobra.Command, args []string) {
		if ok, _ := cmd.Flags().GetBool("new-config"); ok {
			if err := config.WriteConfigFile("config.toml"); err != nil {
				log.Fatal(err)
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func LoadConfig(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	appConfig = cfg
	return nil
}

func init() {
	rootCmd.Flags().Bool("new-config", false, "Generate a new default config.toml file")
	rootCmd.PersistentFlags().String("config", "", "Path to config file")
}
