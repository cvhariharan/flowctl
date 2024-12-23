package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "autopilot",
	Short: "Self-service workflow execution engine",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		if err := readConfig(configPath); err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if ok, _ := cmd.Flags().GetBool("new-config"); ok {
			if err := viper.WriteConfigAs("config.toml"); err != nil {
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

func init() {
	rootCmd.Flags().Bool("new-config", false, "Generate a new default config.toml file")
	rootCmd.PersistentFlags().String("config", "", "Path to config file")

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")

	// Default Config
	viper.SetDefault("db.dbname", "autopilot")
	viper.SetDefault("db.user", "autopilot")
	viper.SetDefault("db.password", "autopilot")
	viper.SetDefault("db.host", "127.0.0.1")
	viper.SetDefault("db.port", 5432)

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")

	viper.SetDefault("app.admin_username", "autopilot_admin")
	viper.SetDefault("app.admin_password", "autopilot_password")
	viper.SetDefault("app.root_url", "http://localhost:7000")
	viper.SetDefault("app.use_tls", false)
	viper.SetDefault("app.http_tls_cert", "server_cert.pem")
	viper.SetDefault("app.http_tls_key", "server_key.pem")
	viper.SetDefault("app.flows_directory", "flows")

	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("could not generate random key for securecookie encryption: %v", err)
	}
	viper.SetDefault("app.secure_cookie_key", hex.EncodeToString(key))
}
