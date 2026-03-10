package cmd

import (
	"fmt"

	"github.com/Maxsander123/bmcinv/internal/config"
	"github.com/Maxsander123/bmcinv/internal/database"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show database and configuration status",
	Long:  `Display current inventory statistics and configuration details.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("=== BMC Inventory Status ===\n")

	// Config info
	fmt.Println("Configuration:")
	fmt.Printf("  Config file:  %s\n", config.ConfigPath())
	fmt.Printf("  Database:     %s\n", config.DatabasePath())
	
	cfg := config.GetScanConfig()
	fmt.Printf("  Workers:      %d\n", cfg.Workers)
	fmt.Printf("  Timeout:      %ds\n", cfg.TimeoutSecs)
	fmt.Println()

	// Credentials info (without showing passwords)
	fmt.Println("Configured Credentials:")
	if config.AppConfig != nil {
		for bmcType, cred := range config.AppConfig.Credentials {
			fmt.Printf("  %s: user=%s\n", bmcType, cred.Username)
		}
	}
	fmt.Println()

	// Database stats
	stats, err := database.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}

	fmt.Println("Inventory Statistics:")
	fmt.Printf("  Servers:      %d\n", stats.TotalServers)
	fmt.Printf("  Memory DIMMs: %d\n", stats.TotalMemory)
	fmt.Printf("  Storage:      %d\n", stats.TotalStorage)
	fmt.Printf("  NICs:         %d\n", stats.TotalNetworks)

	return nil
}
