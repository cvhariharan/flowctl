package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	DB         DBConfig         `koanf:"db"`
	App        AppConfig        `koanf:"app"`
	Keystore   KeystoreConfig   `koanf:"keystore"`
	OIDC       []OIDCConfig     `koanf:"oidc" validate:"dive"`
	Scheduler  SchedulerConfig  `koanf:"scheduler"`
	Logger     Logger           `koanf:"logger"`
	Metrics    Metrics          `koanf:"metrics"`
	Messengers MessengersConfig `koanf:"messengers"`
	Artifacts  ArtifactsConfig  `koanf:"artifacts"`
}

func (c *Config) Validate() error {
	validate := validator.New()

	if err := validate.Struct(c); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	if err := validateOIDCProviders(c.OIDC); err != nil {
		return fmt.Errorf("invalid oidc configuration: %w", err)
	}

	if c.Artifacts.Retain {
		if c.Artifacts.Directory == "" {
			return fmt.Errorf("artifacts.directory is required when retain is true")
		}
	}

	return nil
}

type Metrics struct {
	Enabled bool   `koanf:"enabled"`
	Path    string `koanf:"path"`
}

type DBConfig struct {
	DSN         string `koanf:"dsn"`
	DBName      string `koanf:"dbname" validate:"required_without=DSN"`
	User        string `koanf:"user" validate:"required_without=DSN"`
	Password    string `koanf:"password" validate:"required_without=DSN"`
	Host        string `koanf:"host" validate:"required_without=DSN"`
	Port        int    `koanf:"port" validate:"required_without=DSN,omitempty,min=1,max=65535"`
	SSLMode     string `koanf:"sslmode" validate:"omitempty,oneof=disable allow prefer require verify-ca verify-full"`
	SSLCert     string `koanf:"sslcert"`
	SSLKey      string `koanf:"sslkey"`
	SSLRootCert string `koanf:"sslrootcert"`
}

// ConnectionString returns the database connection string.
// If DSN is set, it returns the DSN directly else it builds a URL.
func (db DBConfig) ConnectionString() string {
	if db.DSN != "" {
		return db.DSN
	}

	userInfo := url.UserPassword(db.User, db.Password)
	dbURL := &url.URL{
		Scheme: "postgres",
		User:   userInfo,
		Host:   fmt.Sprintf("%s:%d", db.Host, db.Port),
		Path:   db.DBName,
	}

	query := dbURL.Query()
	sslMode := db.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	query.Add("sslmode", sslMode)

	if db.SSLCert != "" {
		query.Add("sslcert", db.SSLCert)
	}
	if db.SSLKey != "" {
		query.Add("sslkey", db.SSLKey)
	}
	if db.SSLRootCert != "" {
		query.Add("sslrootcert", db.SSLRootCert)
	}

	dbURL.RawQuery = query.Encode()

	return dbURL.String()
}

type SchedulerConfig struct {
	WorkerCount          int           `koanf:"workers" validate:"min=1"`
	Backend              string        `koanf:"backend"`
	CronSyncInterval     time.Duration `koanf:"cron_sync_interval" validate:"min=1s"`
	FlowExecutionTimeout time.Duration `koanf:"flow_execution_timeout" validate:"min=1s"`
	DefaultTimezone      string        `koanf:"default_timezone" validate:"required,timezone"`
}

type Logger struct {
	Backend       string        `koanf:"backend"`
	Directory     string        `koanf:"log_directory" validate:"required"`
	MaxSizeBytes  int64         `koanf:"max_size_bytes" validate:"min=0"`
	RetentionTime time.Duration `koanf:"retention_time" validate:"min=0"`
	ScanInterval  time.Duration `koanf:"scan_interval" validate:"min=1s"`
}

type AppConfig struct {
	AdminUsername     string `koanf:"admin_username" validate:"required,min=1"`
	AdminPassword     string `koanf:"admin_password" validate:"required,min=8"`
	RootURL           string `koanf:"root_url" validate:"required,url"`
	Address           string `koanf:"address" validate:"required"`
	UseTLS            bool   `koanf:"use_tls"`
	HTTPTLSCert       string `koanf:"http_tls_cert" validate:"required_if=UseTLS true"`
	HTTPTLSKey        string `koanf:"http_tls_key" validate:"required_if=UseTLS true"`
	FlowsDirectory    string `koanf:"flows_directory" validate:"required"`
	MaxFileUploadSize int64  `koanf:"max_file_upload_size" validate:"required,min=1"`
	PluginDir         string `koanf:"plugin_dir"`
}

type KeystoreConfig struct {
	KeeperURL string `koanf:"keeper_url" validate:"required"`
}

type OIDCAutoCreateConfig struct {
	Enabled        bool     `koanf:"enabled"`
	Namespace      string   `koanf:"namespace"`
	Role           string   `koanf:"role"`
	Groups         []string `koanf:"groups"`
	AllowedDomains []string `koanf:"allowed_domains"`
}

type OIDCConfig struct {
	Name            string               `koanf:"name" validate:"required,alpha"`
	Issuer          string               `koanf:"issuer" validate:"required,url"`
	AuthURL         string               `koanf:"auth_url" validate:"omitempty,url"`
	TokenURL        string               `koanf:"token_url" validate:"omitempty,url"`
	RedirectURL     string               `koanf:"redirect_url" validate:"omitempty,url"`
	ClientID        string               `koanf:"client_id" validate:"required"`
	ClientSecret    string               `koanf:"client_secret" validate:"required"`
	Label           string               `koanf:"label"`
	AutoCreateUsers OIDCAutoCreateConfig `koanf:"auto_create_users"`
}

type MessengersConfig struct {
	Email   SMTPConfig    `koanf:"email"`
	Webhook WebhookConfig `koanf:"webhook"`
}

type WebhookConfig struct {
	Enabled    bool          `koanf:"enabled"`
	SigningKey string        `koanf:"signing_key" validate:"required_if=Enabled true"`
	Timeout    time.Duration `koanf:"timeout"`
}

type SMTPConfig struct {
	Enabled     bool   `koanf:"enabled"`
	Host        string `koanf:"host" validate:"required_if=Enabled true"`
	Port        int    `koanf:"port" validate:"required_if=Enabled true,min=1,max=65535"`
	Username    string `koanf:"username"`
	Password    string `koanf:"password"`
	FromAddress string `koanf:"from_address" validate:"required_if=Enabled true,email"`
	FromName    string `koanf:"from_name"`
	MaxConns    int    `koanf:"max_conns" validate:"min=1"`
	SSL         string `koanf:"ssl" validate:"omitempty,oneof=none tls starttls"`
}

type ArtifactsConfig struct {
	Retain        bool          `koanf:"retain"`
	Directory     string        `koanf:"directory"`
	RetentionTime time.Duration `koanf:"retention_time"`
	ScanInterval  time.Duration `koanf:"scan_interval"`
}


func Load(configPath string) (Config, error) {
	k := koanf.New(".")

	if configPath != "" {
		if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
			return Config{}, fmt.Errorf("error loading config file %s: %w", configPath, err)
		}
	} else {
		if err := k.Load(file.Provider("config.toml"), toml.Parser()); err != nil {
			log.Println("config file not found, using defaults and environment variables")
		}
	}

	if err := k.Load(env.Provider("FLOWCTL_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "FLOWCTL_")), "__", ".")
	}), nil); err != nil {
		return Config{}, fmt.Errorf("error loading environment variables: %w", err)
	}
	var config Config
	if err := k.Unmarshal("", &config); err != nil {
		return Config{}, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("error validating config: %w", err)
	}

	return config, nil
}

// GetDefaultConfig returns the default configuration values
func GetDefaultConfig() Config {
	return Config{
		DB: DBConfig{
			DSN:         "",
			DBName:      "flowctl",
			User:        "flowctl",
			Password:    "flowctl",
			Host:        "127.0.0.1",
			Port:        5432,
			SSLMode:     "disable",
			SSLCert:     "",
			SSLKey:      "",
			SSLRootCert: "",
		},
		App: AppConfig{
			AdminUsername:     "flowctl_admin",
			AdminPassword:     "flowctl_password",
			RootURL:           "http://localhost:7000",
			Address:           ":7000",
			UseTLS:            false,
			HTTPTLSCert:       "server_cert.pem",
			HTTPTLSKey:        "server_key.pem",
			FlowsDirectory:    "flows",
			MaxFileUploadSize: 100 * 1024 * 1024, // 100MB
			PluginDir:         "",
		},
		Keystore: KeystoreConfig{
			KeeperURL: fmt.Sprintf("base64key://%s", genKey(32)),
		},
		OIDC: []OIDCConfig{
			{
				Issuer:       "",
				ClientID:     "",
				ClientSecret: "",
			},
		},
		Scheduler: SchedulerConfig{
			WorkerCount:          runtime.NumCPU(),
			CronSyncInterval:     5 * time.Minute,
			FlowExecutionTimeout: time.Hour,
			DefaultTimezone:      "UTC",
		},
		Logger: Logger{
			Backend:       "file",
			Directory:     "/var/log/flowctl",
			RetentionTime: 0,
			ScanInterval:  1 * time.Hour,
		},
		Messengers: MessengersConfig{
			Email: SMTPConfig{
				Enabled:  false,
				Host:     "localhost",
				Port:     587,
				MaxConns: 10,
				SSL:      "none",
			},
			Webhook: WebhookConfig{
				Enabled: false,
				Timeout: 30 * time.Second,
			},
		},
		Artifacts: ArtifactsConfig{
			Retain:        false,
			Directory:     "artifacts",
			RetentionTime: 0,
			ScanInterval:  1 * time.Hour,
		},
	}
}

// genKey generates a random key of the specified size
func genKey(bytes int) string {
	key := make([]byte, bytes)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("could not generate random key for securecookie encryption: %v", err)
	}
	return base64.URLEncoding.EncodeToString(key)
}

// validateOIDCProviders ensures OIDC array has no duplicate names
func validateOIDCProviders(providers []OIDCConfig) error {
	names := make(map[string]bool)

	for _, provider := range providers {
		if names[provider.Name] {
			return fmt.Errorf("duplicate provider name: %s", provider.Name)
		}
		names[provider.Name] = true
	}

	return nil
}
