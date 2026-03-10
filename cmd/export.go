package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Maxsander123/bmcinv/internal/database"
	"github.com/Maxsander123/bmcinv/internal/models"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOutput string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export inventory data to CSV or JSON",
	Long: `Export all inventory data from the database to CSV or JSON format.

Creates multiple CSV files (one per table) or a single JSON file.

Examples:
  bmcinv export                      # Export to CSV in current directory
  bmcinv export -f json              # Export to JSON
  bmcinv export -o /tmp/inventory    # Export to specific directory
  bmcinv export -f csv -o ~/backup   # Export CSV to backup folder`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "csv", "output format: csv or json")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", ".", "output directory or file")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	db := database.GetDB()

	// Load all data
	var servers []models.Server
	if err := db.Preload("Memory").Preload("Storage").Preload("Networks").Find(&servers).Error; err != nil {
		return fmt.Errorf("failed to load data: %w", err)
	}

	if len(servers) == 0 {
		fmt.Println("No data to export. Run 'bmcinv scan' first.")
		return nil
	}

	switch exportFormat {
	case "csv":
		return exportCSV(servers)
	case "json":
		return exportJSON(servers)
	default:
		return fmt.Errorf("unknown format: %s (use 'csv' or 'json')", exportFormat)
	}
}

func exportCSV(servers []models.Server) error {
	// Create output directory
	if err := os.MkdirAll(exportOutput, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")

	// Export servers
	serversFile := filepath.Join(exportOutput, fmt.Sprintf("servers_%s.csv", timestamp))
	if err := writeServersCSV(serversFile, servers); err != nil {
		return err
	}
	fmt.Printf("✓ Exported %d servers to %s\n", len(servers), serversFile)

	// Export memory
	var allMemory []memoryExport
	for _, s := range servers {
		for _, m := range s.Memory {
			allMemory = append(allMemory, memoryExport{Memory: m, ServerIP: s.IP})
		}
	}
	if len(allMemory) > 0 {
		memoryFile := filepath.Join(exportOutput, fmt.Sprintf("memory_%s.csv", timestamp))
		if err := writeMemoryCSV(memoryFile, allMemory); err != nil {
			return err
		}
		fmt.Printf("✓ Exported %d memory modules to %s\n", len(allMemory), memoryFile)
	}

	// Export storage
	var allStorage []storageExport
	for _, s := range servers {
		for _, st := range s.Storage {
			allStorage = append(allStorage, storageExport{Storage: st, ServerIP: s.IP})
		}
	}
	if len(allStorage) > 0 {
		storageFile := filepath.Join(exportOutput, fmt.Sprintf("storage_%s.csv", timestamp))
		if err := writeStorageCSV(storageFile, allStorage); err != nil {
			return err
		}
		fmt.Printf("✓ Exported %d storage devices to %s\n", len(allStorage), storageFile)
	}

	// Export networks
	var allNetworks []networkExport
	for _, s := range servers {
		for _, n := range s.Networks {
			allNetworks = append(allNetworks, networkExport{Network: n, ServerIP: s.IP})
		}
	}
	if len(allNetworks) > 0 {
		networksFile := filepath.Join(exportOutput, fmt.Sprintf("networks_%s.csv", timestamp))
		if err := writeNetworksCSV(networksFile, allNetworks); err != nil {
			return err
		}
		fmt.Printf("✓ Exported %d network interfaces to %s\n", len(allNetworks), networksFile)
	}

	return nil
}

type memoryExport struct {
	models.Memory
	ServerIP string
}

type storageExport struct {
	models.Storage
	ServerIP string
}

type networkExport struct {
	models.Network
	ServerIP string
}

func writeServersCSV(filename string, servers []models.Server) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	w.Write([]string{"IP", "Vendor", "Model", "ChassisSerial", "BiosVersion", "BMCVersion", "Hostname", "LastScanned"})

	for _, s := range servers {
		w.Write([]string{
			s.IP,
			s.Vendor,
			s.Model,
			s.ChassisSerial,
			s.BiosVersion,
			s.BMCVersion,
			s.Hostname,
			s.LastScanned.Format(time.RFC3339),
		})
	}
	return w.Error()
}

func writeMemoryCSV(filename string, memory []memoryExport) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	w.Write([]string{"ServerIP", "Slot", "CapacityGB", "Speed", "Type", "Manufacturer", "PartNumber", "SerialNumber", "Health"})

	for _, m := range memory {
		w.Write([]string{
			m.ServerIP,
			m.Slot,
			strconv.Itoa(m.CapacityGB),
			strconv.Itoa(m.Speed),
			m.Type,
			m.Manufacturer,
			m.PartNumber,
			m.SerialNumber,
			m.Health,
		})
	}
	return w.Error()
}

func writeStorageCSV(filename string, storage []storageExport) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	w.Write([]string{"ServerIP", "Slot", "MediaType", "Protocol", "CapacityGB", "Manufacturer", "Model", "SerialNumber", "FirmwareRev", "Health", "PredFailure"})

	for _, s := range storage {
		predFailure := "false"
		if s.PredFailure {
			predFailure = "true"
		}
		w.Write([]string{
			s.ServerIP,
			s.Slot,
			s.MediaType,
			s.Protocol,
			strconv.Itoa(s.CapacityGB),
			s.Manufacturer,
			s.Model,
			s.SerialNumber,
			s.FirmwareRev,
			s.Health,
			predFailure,
		})
	}
	return w.Error()
}

func writeNetworksCSV(filename string, networks []networkExport) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	w.Write([]string{"ServerIP", "Port", "MacAddress", "IPAddress", "LinkStatus", "LinkSpeedMbps", "Manufacturer", "Model", "FirmwareRev"})

	for _, n := range networks {
		w.Write([]string{
			n.ServerIP,
			n.Port,
			n.MacAddress,
			n.IPAddress,
			n.LinkStatus,
			strconv.Itoa(n.LinkSpeedMbps),
			n.Manufacturer,
			n.Model,
			n.FirmwareRev,
		})
	}
	return w.Error()
}

func exportJSON(servers []models.Server) error {
	timestamp := time.Now().Format("20060102_150405")
	
	var outputFile string
	if info, err := os.Stat(exportOutput); err == nil && info.IsDir() {
		outputFile = filepath.Join(exportOutput, fmt.Sprintf("inventory_%s.json", timestamp))
	} else {
		outputFile = exportOutput
		if filepath.Ext(outputFile) == "" {
			outputFile += ".json"
		}
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✓ Exported %d servers to %s\n", len(servers), outputFile)
	return nil
}
