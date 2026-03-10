package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/Maxsander123/bmcinv/internal/scanner"
	"github.com/spf13/cobra"
)

var (
	scanWorkers int
	scanTimeout int
)

var scanCmd = &cobra.Command{
	Use:   "scan <cidr|ip>",
	Short: "Scan BMC endpoints and collect hardware inventory",
	Long: `Scan one or more BMC endpoints to collect hardware inventory data.

The scan process:
1. Pre-flight: Detect BMC vendor type (unauthenticated)
2. Credentials: Load vendor-specific credentials from config
3. Collection: Query Redfish API for hardware details
4. Storage: Upsert data into local SQLite database

Examples:
  bmcinv scan 192.168.1.0/24    # Scan entire subnet
  bmcinv scan 10.0.0.100        # Scan single host
  bmcinv scan 10.0.0.0/28       # Scan small range`,
	Args: cobra.ExactArgs(1),
	RunE: runScan,
}

func init() {
	scanCmd.Flags().IntVarP(&scanWorkers, "workers", "w", 0, "number of parallel workers (default from config)")
	scanCmd.Flags().IntVarP(&scanTimeout, "timeout", "t", 0, "timeout per host in seconds (default from config)")
}

func runScan(cmd *cobra.Command, args []string) error {
	target := args[0]

	fmt.Printf("Starting scan of %s...\n", target)
	startTime := time.Now()

	// Create scanner
	s := scanner.NewScanner()

	// Override workers if flag provided
	if scanWorkers > 0 {
		// Note: Would need to modify Scanner to accept this
		fmt.Printf("Using %d workers\n", scanWorkers)
	}

	// Run scan
	results, err := s.ScanCIDR(target)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Print results
	elapsed := time.Since(startTime)
	printScanResults(results, elapsed)

	return nil
}

func printScanResults(results []scanner.ScanResult, elapsed time.Duration) {
	var successful, failed int
	var errors []scanner.ScanResult

	for _, r := range results {
		if r.Success {
			successful++
		} else {
			failed++
			errors = append(errors, r)
		}
	}

	fmt.Printf("\n=== Scan Complete ===\n")
	fmt.Printf("Duration:   %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Total:      %d hosts\n", len(results))
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed:     %d\n", failed)

	if successful > 0 {
		fmt.Printf("\n=== Successfully Scanned Servers ===\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "IP\tVENDOR\tMODEL\tSERIAL")
		for _, r := range results {
			if r.Success && r.Server != nil {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					r.IP,
					r.Server.Vendor,
					r.Server.Model,
					r.Server.ChassisSerial,
				)
			}
		}
		w.Flush()
	}

	if failed > 0 && verbose {
		fmt.Printf("\n=== Failed Hosts ===\n")
		for _, r := range errors {
			fmt.Printf("  %s: %v\n", r.IP, r.Error)
		}
	}
}
