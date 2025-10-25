package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Perform DB migration",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		if err := LoadConfig(configPath); err != nil {
			log.Fatal(err)
		}

		db, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", appConfig.DB.User, appConfig.DB.Password, appConfig.DB.Host, appConfig.DB.Port, appConfig.DB.DBName))
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

// initDB performs DB migrations and can be safely run multiple times
func initDB(db *sqlx.DB) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver instance: %w", err)
	}

	migrationsFS, err := fs.Sub(StaticFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations sub-filesystem: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create iofs source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
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

// initAdmin creates the admin user as defined in the config file
// User will only be created if the user doesn't already exist
func initAdmin(store repo.Store) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(appConfig.App.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error hashing admin password: %w", err)
	}

	_, err = store.GetUserByUsername(context.Background(), appConfig.App.AdminUsername)
	if err != nil {
		_, err = store.CreateUser(context.Background(), repo.CreateUserParams{
			Username:  appConfig.App.AdminUsername,
			Password:  sql.NullString{String: string(hashedPassword), Valid: len(hashedPassword) > 0},
			LoginType: "standard",
			Role:      "superuser",
			Name:      "admin",
		})
		return err
	}
	// If the user already exists, reset the password
	_, err = store.UpdateUserPasswordByUsername(context.Background(), repo.UpdateUserPasswordByUsernameParams{
		Username: appConfig.App.AdminUsername,
		Password: sql.NullString{String: string(hashedPassword), Valid: len(hashedPassword) > 0},
	})
	return err
}
