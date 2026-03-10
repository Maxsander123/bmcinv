package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/Maxsander123/bmcinv/internal/finder"
	"github.com/Maxsander123/bmcinv/internal/models"
	"github.com/spf13/cobra"
)

var (
	findExact bool
	findLimit int
	findType  string
)

var findCmd = &cobra.Command{
	Use:   "find <search-string>",
	Short: "Search for a string across all hardware components",
	Long: `Search for a string (serial number, MAC address, hostname, etc.)
across all hardware inventory tables.

The search is performed in parallel across:
  - Servers (IP, hostname, chassis serial)
  - Memory (serial number, manufacturer, part number)
  - Storage (serial number, model, manufacturer)
  - Networks (MAC address, IP address)

MAC addresses can be provided in any format:
  - AA:BB:CC:DD:EE:FF
  - AA-BB-CC-DD-EE-FF
  - AABBCCDDEEFF

Examples:
  bmcinv find MEM12345678         # Find by RAM serial
  bmcinv find 00:1B:21:AB:CD:EF   # Find by MAC address
  bmcinv find "Dell Inc."         # Find all Dell servers
  bmcinv find Samsung --type memory   # Find Samsung RAM only`,
	Args: cobra.ExactArgs(1),
	RunE: runFind,
}

func init() {
	findCmd.Flags().BoolVarP(&findExact, "exact", "e", false, "exact match only (no wildcards)")
	findCmd.Flags().IntVarP(&findLimit, "limit", "l", 100, "maximum results per table")
	findCmd.Flags().StringVarP(&findType, "type", "T", "", "limit search to type: server, memory, storage, network")
}

func runFind(cmd *cobra.Command, args []string) error {
	query := args[0]

	if verbose {
		fmt.Printf("Searching for: %q\n", query)
	}

	opts := finder.SearchOptions{
		ExactMatch: findExact,
		Limit:      findLimit,
	}

	results, err := finder.GlobalFind(query, opts)
	if err != nil {
		// May return partial results with error
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Filter by type if specified
	if findType != "" {
		results = filterByType(results, findType)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	printFindResults(results)
	return nil
}

func filterByType(results []models.SearchResult, typeFilter string) []models.SearchResult {
	typeFilter = strings.ToLower(typeFilter)
	var filtered []models.SearchResult
	for _, r := range results {
		if strings.ToLower(r.ComponentType) == typeFilter {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func printFindResults(results []models.SearchResult) {
	fmt.Printf("\n=== Found %d Result(s) ===\n\n", len(results))

	// Group by component type for better readability
	grouped := make(map[string][]models.SearchResult)
	for _, r := range results {
		grouped[r.ComponentType] = append(grouped[r.ComponentType], r)
	}

	// Print order
	typeOrder := []string{"server", "memory", "storage", "network"}

	for _, typ := range typeOrder {
		items, ok := grouped[typ]
		if !ok || len(items) == 0 {
			continue
		}

		fmt.Printf("─── %s (%d) ───\n", strings.ToUpper(typ), len(items))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		switch typ {
		case "server":
			fmt.Fprintln(w, "IP\tVENDOR\tMODEL\tMATCHED\tVALUE")
			for _, r := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					r.ServerIP, r.ServerVendor, r.ServerModel,
					r.MatchedField, r.MatchedValue)
			}

		case "memory":
			fmt.Fprintln(w, "SERVER IP\tCOMPONENT\tMATCHED\tVALUE")
			for _, r := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					r.ServerIP, r.ComponentInfo,
					r.MatchedField, r.MatchedValue)
			}

		case "storage":
			fmt.Fprintln(w, "SERVER IP\tCOMPONENT\tMATCHED\tVALUE")
			for _, r := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					r.ServerIP, r.ComponentInfo,
					r.MatchedField, r.MatchedValue)
			}

		case "network":
			fmt.Fprintln(w, "SERVER IP\tCOMPONENT\tMATCHED\tVALUE")
			for _, r := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					r.ServerIP, r.ComponentInfo,
					r.MatchedField, r.MatchedValue)
			}
		}
		w.Flush()
		fmt.Println()
	}
}
