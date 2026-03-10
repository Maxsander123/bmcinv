// Package config handles Viper-based configuration loading with smart credential management.
// Design Decision: Das Config-System nutzt eine hierarchische YAML-Struktur, die verschiedene
// BMC-Typen (iDRAC, iLO, IPMI) mit separaten Credentials unterstützt. Dies ist essentiell
// für Rechenzentren, wo unterschiedliche Hardware-Generationen verschiedene Passwörter haben.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// BMCType represents supported BMC vendors
type BMCType string

const (
	BMCTypeIDRAC   BMCType = "idrac"
	BMCTypeILO     BMCType = "ilo"
	BMCTypeIPMI    BMCType = "ipmi"
	BMCTypeUnknown BMCType = "unknown"
)

// Credential holds authentication data for a specific BMC type
type Credential struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// ScanConfig holds scanner-specific settings
type ScanConfig struct {
	Workers       int `mapstructure:"workers"`
	TimeoutSecs   int `mapstructure:"timeout_secs"`
	RetryAttempts int `mapstructure:"retry_attempts"`
}

// Config is the root configuration structure that Viper deserializes into.
// The Credentials map allows dynamic lookup based on detected BMC vendor.
type Config struct {
	Credentials map[string]Credential `mapstructure:"credentials"`
	Database    DatabaseConfig        `mapstructure:"database"`
	Scan        ScanConfig            `mapstructure:"scan"`
}

// DatabaseConfig holds DB connection settings
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// AppConfig holds the loaded application configuration (singleton pattern)
var AppConfig *Config

// ConfigDir returns the path to the config directory (~/.bmcinv/)
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".bmcinv"
	}
	return filepath.Join(home, ".bmcinv")
}

// ConfigPath returns the full path to the config file
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// DatabasePath returns the path to the SQLite database
func DatabasePath() string {
	if AppConfig != nil && AppConfig.Database.Path != "" {
		return AppConfig.Database.Path
	}
	return filepath.Join(ConfigDir(), "inventory.db")
}

// InitConfig initializes Viper and loads the configuration.
// This is called early in the CLI lifecycle (e.g., in rootCmd.PersistentPreRun).
func InitConfig() error {
	configDir := ConfigDir()

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Setup Viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")

	// Set sensible defaults
	setDefaults()

	// Try to read config file (create if not exists)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Create default config file
			if err := createDefaultConfig(); err != nil {
				return fmt.Errorf("failed to create default config: %w", err)
			}
			// Re-read the newly created config
			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read config after creation: %w", err)
			}
		} else {
			return fmt.Errorf("failed to read config: %w", err)
		}
	}

	// Unmarshal into struct
	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// setDefaults configures Viper with sensible default values
func setDefaults() {
	// Default credentials (should be overridden in config)
	viper.SetDefault("credentials.idrac.username", "root")
	viper.SetDefault("credentials.idrac.password", "calvin")
	viper.SetDefault("credentials.ilo.username", "Administrator")
	viper.SetDefault("credentials.ilo.password", "")
	viper.SetDefault("credentials.ipmi.username", "ADMIN")
	viper.SetDefault("credentials.ipmi.password", "ADMIN")

	// Database defaults
	viper.SetDefault("database.path", filepath.Join(ConfigDir(), "inventory.db"))

	// Scan defaults
	viper.SetDefault("scan.workers", 10)
	viper.SetDefault("scan.timeout_secs", 30)
	viper.SetDefault("scan.retry_attempts", 2)
}

// createDefaultConfig writes a template YAML config file
func createDefaultConfig() error {
	configContent := `# BMC Inventory Configuration
# Credentials for different BMC types - customize per your environment
credentials:
  idrac:
    username: root
    password: calvin
  ilo:
    username: Administrator
    password: ""
  ipmi:
    username: ADMIN
    password: ADMIN

# Database settings
database:
  path: ~/.bmcinv/inventory.db

# Scanner settings
scan:
  workers: 10
  timeout_secs: 30
  retry_attempts: 2
`
	return os.WriteFile(ConfigPath(), []byte(configContent), 0600)
}

// GetCredential retrieves credentials for a specific BMC type.
// This is the core of the "Smart Credential Management" - the scanner
// first detects the BMC type, then calls this function to get matching credentials.
func GetCredential(bmcType BMCType) (Credential, error) {
	if AppConfig == nil {
		return Credential{}, fmt.Errorf("configuration not initialized")
	}

	key := string(bmcType)
	cred, exists := AppConfig.Credentials[key]
	if !exists {
		return Credential{}, fmt.Errorf("no credentials configured for BMC type: %s", bmcType)
	}

	if cred.Username == "" {
		return Credential{}, fmt.Errorf("username not configured for BMC type: %s", bmcType)
	}

	return cred, nil
}

// GetScanConfig returns the scanner configuration with fallback defaults
func GetScanConfig() ScanConfig {
	if AppConfig == nil {
		return ScanConfig{
			Workers:       10,
			TimeoutSecs:   30,
			RetryAttempts: 2,
		}
	}
	return AppConfig.Scan
}
