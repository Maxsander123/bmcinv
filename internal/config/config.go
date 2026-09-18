// Package config handles Viper-based configuration loading with smart credential management.
// Design Decision: Das Config-System nutzt eine hierarchische YAML-Struktur, die verschiedene
// BMC-Typen (iDRAC, iLO, IPMI) mit separaten Credentials unterstützt. Dies ist essentiell
// für Rechenzentren, wo unterschiedliche Hardware-Generationen verschiedene Passwörter haben.
//
// Passwords can be supplied via environment variables instead of the config file:
//
//	BMCINV_IDRAC_PASSWORD, BMCINV_ILO_PASSWORD, BMCINV_IPMI_PASSWORD
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// BMCType represents supported BMC vendors
type BMCType string

const (
	BMCTypeIDRAC       BMCType = "idrac"       // Dell iDRAC (PowerEdge)
	BMCTypeILO         BMCType = "ilo"         // HPE iLO (ProLiant)
	BMCTypeIPMI        BMCType = "ipmi"        // Generic IPMI / Redfish (ASUS, MSI, unknown)
	BMCTypeSupermicro  BMCType = "supermicro"  // Supermicro IPMI / Redfish
	BMCTypeUnknown     BMCType = "unknown"
)

// Credential holds authentication data for a specific BMC type
type Credential struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// ScanConfig holds scanner-specific settings
type ScanConfig struct {
	Workers       int  `mapstructure:"workers"`
	TimeoutSecs   int  `mapstructure:"timeout_secs"`
	RetryAttempts int  `mapstructure:"retry_attempts"`
	TLSSkipVerify bool `mapstructure:"tls_skip_verify"`
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
// Called early in the CLI lifecycle (rootCmd.PersistentPreRunE).
func InitConfig() error {
	configDir := ConfigDir()

	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")

	setDefaults()
	bindEnvVars()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if err := createDefaultConfig(); err != nil {
				return fmt.Errorf("failed to create default config: %w", err)
			}
			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read config after creation: %w", err)
			}
		} else {
			return fmt.Errorf("failed to read config: %w", err)
		}
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply environment variable overrides for passwords after unmarshal
	applyEnvPasswords()

	return nil
}

// setDefaults configures Viper with sensible default values
func setDefaults() {
	viper.SetDefault("credentials.idrac.username", "root")
	viper.SetDefault("credentials.idrac.password", "calvin")
	viper.SetDefault("credentials.ilo.username", "Administrator")
	viper.SetDefault("credentials.ilo.password", "")
	viper.SetDefault("credentials.supermicro.username", "ADMIN")
	viper.SetDefault("credentials.supermicro.password", "ADMIN")
	viper.SetDefault("credentials.ipmi.username", "ADMIN")
	viper.SetDefault("credentials.ipmi.password", "ADMIN")

	viper.SetDefault("database.path", filepath.Join(ConfigDir(), "inventory.db"))

	viper.SetDefault("scan.workers", 10)
	viper.SetDefault("scan.timeout_secs", 30)
	viper.SetDefault("scan.retry_attempts", 2)
	// Default true: BMCs in data centers almost always use self-signed certificates.
	viper.SetDefault("scan.tls_skip_verify", true)
}

// bindEnvVars wires environment variables to Viper keys.
// Viper's AutomaticEnv does not resolve nested keys reliably, so we bind explicitly.
func bindEnvVars() {
	viper.SetEnvPrefix("BMCINV")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	_ = viper.BindEnv("credentials.idrac.username", "BMCINV_IDRAC_USERNAME")
	_ = viper.BindEnv("credentials.idrac.password", "BMCINV_IDRAC_PASSWORD")
	_ = viper.BindEnv("credentials.ilo.username", "BMCINV_ILO_USERNAME")
	_ = viper.BindEnv("credentials.ilo.password", "BMCINV_ILO_PASSWORD")
	_ = viper.BindEnv("credentials.supermicro.username", "BMCINV_SUPERMICRO_USERNAME")
	_ = viper.BindEnv("credentials.supermicro.password", "BMCINV_SUPERMICRO_PASSWORD")
	_ = viper.BindEnv("credentials.ipmi.username", "BMCINV_IPMI_USERNAME")
	_ = viper.BindEnv("credentials.ipmi.password", "BMCINV_IPMI_PASSWORD")
}

// applyEnvPasswords overrides credential passwords from environment variables
// after Viper has unmarshalled the config struct. This is needed because Viper's
// BindEnv for nested map keys is not reliably picked up by Unmarshal.
func applyEnvPasswords() {
	if AppConfig == nil {
		return
	}
	if AppConfig.Credentials == nil {
		AppConfig.Credentials = make(map[string]Credential)
	}

	envMap := map[string][2]string{
		"idrac":      {"BMCINV_IDRAC_USERNAME", "BMCINV_IDRAC_PASSWORD"},
		"ilo":        {"BMCINV_ILO_USERNAME", "BMCINV_ILO_PASSWORD"},
		"supermicro": {"BMCINV_SUPERMICRO_USERNAME", "BMCINV_SUPERMICRO_PASSWORD"},
		"ipmi":       {"BMCINV_IPMI_USERNAME", "BMCINV_IPMI_PASSWORD"},
	}

	for key, vars := range envMap {
		cred := AppConfig.Credentials[key]
		if v := os.Getenv(vars[0]); v != "" {
			cred.Username = v
		}
		if v := os.Getenv(vars[1]); v != "" {
			cred.Password = v
		}
		AppConfig.Credentials[key] = cred
	}
}

// createDefaultConfig writes a template YAML config file with usage hints
func createDefaultConfig() error {
	configContent := `# BMC Inventory Configuration
#
# SECURITY: Store passwords via environment variables instead of this file:
#   export BMCINV_IDRAC_PASSWORD="yourpassword"     # Dell iDRAC
#   export BMCINV_ILO_PASSWORD="yourpassword"       # HPE iLO
#   export BMCINV_SUPERMICRO_PASSWORD="yourpassword" # Supermicro
#   export BMCINV_IPMI_PASSWORD="yourpassword"      # Generic IPMI / ASUS / MSI

credentials:
  idrac:        # Dell PowerEdge (iDRAC 7+)
    username: root
    password: ""  # default: calvin — prefer BMCINV_IDRAC_PASSWORD env var
  ilo:          # HPE ProLiant (iLO 4+)
    username: Administrator
    password: ""  # prefer BMCINV_ILO_PASSWORD env var
  supermicro:   # Supermicro (X10/X11/X12/H12/H13)
    username: ADMIN
    password: ""  # default: ADMIN — prefer BMCINV_SUPERMICRO_PASSWORD env var
  ipmi:         # Generic Redfish/IPMI fallback (ASUS ASMB, MSI, unknown vendors)
    username: ADMIN
    password: ""  # prefer BMCINV_IPMI_PASSWORD env var

database:
  path: ~/.bmcinv/inventory.db

scan:
  workers: 10
  timeout_secs: 30
  retry_attempts: 2
  # BMCs in data centers typically use self-signed TLS certificates.
  # Set to false only if your BMCs have trusted certificates.
  tls_skip_verify: true
`
	return os.WriteFile(ConfigPath(), []byte(configContent), 0600)
}

// GetCredential retrieves credentials for a specific BMC type.
// Returns an error if the BMC type is unconfigured or has no username.
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
			TLSSkipVerify: true,
		}
	}
	return AppConfig.Scan
}
