package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Perform DB migration",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		if err := readConfig(configPath); err != nil {
			log.Fatal(err)
		}

		db, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", viper.GetString("db.user"), viper.GetString("db.password"), viper.GetString("db.host"), viper.GetInt("db.port"), viper.GetString("db.dbname")))
		if err != nil {
			log.Fatalf("could not connect to database: %v", err)
		}
		defer db.Close()

		if err := initDB(db); err != nil {
			log.Fatal(err)
		}

		s := repo.NewPostgresStore(db)
		if err := initAdmin(s); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func readConfig(configPath string) error {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("config file not found, using default config")
		} else {
			return fmt.Errorf("could not read config file: %w", err)
		}
	}

	return nil
}

func initDB(db *sqlx.DB) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver instance: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Get current version before attempting migration
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// If database is in a dirty state, force the version
	if dirty {
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}
	}

	// Attempt to migrate to the latest version
	if err := m.Up(); err != nil {
		// ErrNoChange means we're at the latest version - this is fine
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func initAdmin(store repo.Store) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(viper.GetString("app.admin_password")), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error hashing admin password: %w", err)
	}

	_, err = store.GetUserByUsername(context.Background(), viper.GetString("app.admin_username"))
	if err != nil {
		_, err = store.CreateUser(context.Background(), repo.CreateUserParams{
			Username:  viper.GetString("app.admin_username"),
			Password:  sql.NullString{String: string(hashedPassword), Valid: true},
			LoginType: "standard",
			Role:      "admin",
			Name:      "admin",
		})
		if err != nil {
			return err
		}
	}

	return nil
}
