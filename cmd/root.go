// Package cmd implements the CLI commands using Cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/Maxsander123/bmcinv/internal/config"
	"github.com/Maxsander123/bmcinv/internal/database"
	"github.com/spf13/cobra"
)

var (
	// Used for flags
	cfgFile string
	verbose bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "bmcinv",
	Short: "BMC Inventory - Hardware inventory tool via Redfish API",
	Long: `bmcinv is a read-only hardware inventory tool that queries servers
via their BMC interfaces (iDRAC, iLO, IPMI) using the Redfish API.

It caches detailed hardware information in a local SQLite database,
enabling fast searches across serial numbers, MAC addresses, and
other component identifiers.

Features:
  - Smart credential management per BMC type
  - Parallel scanning of IP ranges
  - Deep component search (RAM, storage, network)
  - SQLite caching for instant queries`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip initialization for completion and help
		if cmd.Name() == "completion" || cmd.Name() == "help" {
			return nil
		}

		// Initialize config
		if err := config.InitConfig(); err != nil {
			return fmt.Errorf("config initialization failed: %w", err)
		}

		// Initialize database
		if _, err := database.InitDB(); err != nil {
			return fmt.Errorf("database initialization failed: %w", err)
		}

		if verbose {
			database.EnableDebugLogging()
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Close database connection
		_ = database.Close()
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.bmcinv/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(statusCmd)
}
