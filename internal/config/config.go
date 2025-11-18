package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	DB        DBConfig        `koanf:"db"`
	App       AppConfig       `koanf:"app"`
	Keystore  KeystoreConfig  `koanf:"keystore"`
	OIDC      OIDCConfig      `koanf:"oidc"`
	Scheduler SchedulerConfig `koanf:"scheduler"`
	Logger    Logger          `koanf:"logger"`
}

type DBConfig struct {
	DBName   string `koanf:"dbname"`
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
}

type SchedulerConfig struct {
	WorkerCount      int           `koanf:"workers"`
	Backend          string        `koanf:"backend"`
	CronSyncInterval time.Duration `koanf:"cron_sync_interval"`
}

type Logger struct {
	Backend       string        `koanf:"backend"`
	Directory     string        `koanf:"log_directory"`
	MaxSizeBytes  int64         `koanf:"max_size_bytes"`
	RetentionTime time.Duration `koanf:"retention_time"`
	ScanInterval  time.Duration `koanf:"scan_interval"`
}

type AppConfig struct {
	AdminUsername   string `koanf:"admin_username"`
	AdminPassword   string `koanf:"admin_password"`
	RootURL         string `koanf:"root_url"`
	Address         string `koanf:"address"`
	UseTLS          bool   `koanf:"use_tls"`
	HTTPTLSCert     string `koanf:"http_tls_cert"`
	HTTPTLSKey      string `koanf:"http_tls_key"`
	FlowsDirectory  string `koanf:"flows_directory"`
	SecureCookieKey string `koanf:"secure_cookie_key"`
}

type KeystoreConfig struct {
	KeeperURL string `koanf:"keeper_url"`
}

type OIDCConfig struct {
	Issuer       string `koanf:"issuer"`
	AuthURL      string `koanf:"auth_url"`
	TokenURL     string `koanf:"token_url"`
	RedirectURL  string `koanf:"redirect_url"`
	ClientID     string `koanf:"client_id"`
	ClientSecret string `koanf:"client_secret"`
}

func Load(configPath string) (Config, error) {
	k := koanf.New(".")

	defaultConfig := GetDefaultConfig()
	if err := k.Load(structs.Provider(defaultConfig, "koanf"), nil); err != nil {
		return Config{}, fmt.Errorf("error loading default config: %w", err)
	}

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

	return config, nil
}

// WriteConfigFile writes the current configuration to a TOML file
// func WriteConfigFile(filename string) error {
// 	k := koanf.New(".")

// 	defaultConfig := getDefaultConfig()
// 	if err := k.Load(structs.Provider(defaultConfig, "koanf"), nil); err != nil {
// 		return fmt.Errorf("error loading default config: %w", err)
// 	}

// 	data, err := k.Marshal(toml.Parser())
// 	if err != nil {
// 		return fmt.Errorf("error marshaling config: %w", err)
// 	}

// 	if err := os.WriteFile(filename, data, 0644); err != nil {
// 		return fmt.Errorf("error writing config file: %w", err)
// 	}

// 	return nil
// }

// GetDefaultConfig returns the default configuration values
func GetDefaultConfig() Config {
	return Config{
		DB: DBConfig{
			DBName:   "flowctl",
			User:     "flowctl",
			Password: "flowctl",
			Host:     "127.0.0.1",
			Port:     5432,
		},
		App: AppConfig{
			AdminUsername:   "flowctl_admin",
			AdminPassword:   "flowctl_password",
			RootURL:         "http://localhost:7000",
			Address:         ":7000",
			UseTLS:          false,
			HTTPTLSCert:     "server_cert.pem",
			HTTPTLSKey:      "server_key.pem",
			FlowsDirectory:  "flows",
			SecureCookieKey: genKey(16),
		},
		Keystore: KeystoreConfig{
			KeeperURL: fmt.Sprintf("base64key://%s", genKey(32)),
		},
		OIDC: OIDCConfig{
			Issuer:       "",
			ClientID:     "",
			ClientSecret: "",
		},
		Scheduler: SchedulerConfig{
			WorkerCount:      runtime.NumCPU(),
			CronSyncInterval: 5 * time.Minute,
		},
		Logger: Logger{
			Backend:       "file",
			Directory:     "/var/log/flowctl",
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
