// Package finder implements the global search across all hardware components.
// Design Decision: Die Suche nutzt einen parallelen Ansatz mit Goroutines,
// die gleichzeitig alle Tabellen durchsuchen. Die Ergebnisse werden über
// Channels zusammengeführt. LIKE-Queries mit Wildcards ermöglichen partielle Matches.
package finder

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Maxsander123/bmcinv/internal/database"
	"github.com/Maxsander123/bmcinv/internal/models"
)

// SearchOptions controls search behavior
type SearchOptions struct {
	ExactMatch  bool // If true, no wildcards - exact match only
	CaseSensitive bool // If true, case-sensitive search
	Limit       int  // Maximum results per table (0 = unlimited)
}

// DefaultSearchOptions returns sensible defaults
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		ExactMatch:    false,
		CaseSensitive: false,
		Limit:         100,
	}
}

// GlobalFind searches for a string across all tables in parallel.
// This is the core of the "find" command - it searches:
// - Server: IP, ChassisSerial, Hostname, Vendor, Model
// - Memory: SerialNumber, Manufacturer, PartNumber
// - Storage: SerialNumber, Model, Manufacturer
// - Network: MacAddress, IPAddress
//
// Returns aggregated results with server context for each match.
func GlobalFind(query string, opts SearchOptions) ([]models.SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	db := database.GetDB()
	
	// Prepare search pattern
	searchPattern := prepareSearchPattern(query, opts)
	
	// Channel to collect results from parallel searches
	resultChan := make(chan []models.SearchResult, 4)
	errChan := make(chan error, 4)
	var wg sync.WaitGroup

	// Launch parallel searches
	wg.Add(4)

	// Search Servers
	go func() {
		defer wg.Done()
		results, err := searchServers(db, searchPattern, opts)
		if err != nil {
			errChan <- fmt.Errorf("server search failed: %w", err)
			return
		}
		resultChan <- results
	}()

	// Search Memory
	go func() {
		defer wg.Done()
		results, err := searchMemory(db, searchPattern, opts)
		if err != nil {
			errChan <- fmt.Errorf("memory search failed: %w", err)
			return
		}
		resultChan <- results
	}()

	// Search Storage
	go func() {
		defer wg.Done()
		results, err := searchStorage(db, searchPattern, opts)
		if err != nil {
			errChan <- fmt.Errorf("storage search failed: %w", err)
			return
		}
		resultChan <- results
	}()

	// Search Networks
	go func() {
		defer wg.Done()
		results, err := searchNetworks(db, searchPattern, opts)
		if err != nil {
			errChan <- fmt.Errorf("network search failed: %w", err)
			return
		}
		resultChan <- results
	}()

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errChan)
	}()

	// Collect results
	var allResults []models.SearchResult
	var errors []error

	for results := range resultChan {
		allResults = append(allResults, results...)
	}

	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		// Return partial results with error
		return allResults, fmt.Errorf("some searches failed: %v", errors)
	}

	return allResults, nil
}

// prepareSearchPattern formats the search query for SQL LIKE
func prepareSearchPattern(query string, opts SearchOptions) string {
	// Normalize MAC address format (remove common separators)
	query = normalizeMAC(query)
	
	if opts.ExactMatch {
		return query
	}
	
	// Wrap with wildcards for partial matching
	return "%" + query + "%"
}

// normalizeMAC handles different MAC address formats
func normalizeMAC(query string) string {
	// Detect if this looks like a MAC address
	cleaned := strings.ReplaceAll(query, ":", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	
	// If it's a 12-char hex string, it's probably a MAC
	if len(cleaned) == 12 && isHexString(cleaned) {
		// Convert to standard format AA:BB:CC:DD:EE:FF
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			cleaned[0:2], cleaned[2:4], cleaned[4:6],
			cleaned[6:8], cleaned[8:10], cleaned[10:12])
	}
	
	return query
}

// isHexString checks if a string is valid hexadecimal
func isHexString(s string) bool {
	for _, c := range strings.ToLower(s) {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// searchServers searches the servers table
func searchServers(_ interface{}, pattern string, opts SearchOptions) ([]models.SearchResult, error) {
	db := database.GetDB()
	var results []models.SearchResult
	
	query := db.Model(&models.Server{}).
		Select("ip, vendor, model, chassis_serial, hostname").
		Where("ip LIKE ? OR chassis_serial LIKE ? OR hostname LIKE ? OR vendor LIKE ? OR model LIKE ?",
			pattern, pattern, pattern, pattern, pattern)
	
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}

	var servers []models.Server
	if err := query.Find(&servers).Error; err != nil {
		return nil, err
	}

	for _, s := range servers {
		// Determine which field matched
		matchedField, matchedValue := determineServerMatch(s, pattern)
		results = append(results, models.SearchResult{
			ServerIP:      s.IP,
			ServerVendor:  s.Vendor,
			ServerModel:   s.Model,
			ComponentType: "server",
			ComponentInfo: fmt.Sprintf("Hostname: %s, Serial: %s", s.Hostname, s.ChassisSerial),
			MatchedField:  matchedField,
			MatchedValue:  matchedValue,
		})
	}

	return results, nil
}

// searchMemory searches the memory table with server JOIN
func searchMemory(_ interface{}, pattern string, opts SearchOptions) ([]models.SearchResult, error) {
	db := database.GetDB()
	var results []models.SearchResult

	type memoryWithServer struct {
		models.Memory
		ServerIP     string
		ServerVendor string
		ServerModel  string
	}

	var matches []memoryWithServer
	
	query := db.Table("memory").
		Select("memory.*, servers.ip as server_ip, servers.vendor as server_vendor, servers.model as server_model").
		Joins("JOIN servers ON memory.server_id = servers.id").
		Where("memory.serial_number LIKE ? OR memory.manufacturer LIKE ? OR memory.part_number LIKE ?",
			pattern, pattern, pattern)
	
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}

	if err := query.Find(&matches).Error; err != nil {
		return nil, err
	}

	for _, m := range matches {
		matchedField, matchedValue := determineMemoryMatch(m.Memory, pattern)
		results = append(results, models.SearchResult{
			ServerIP:      m.ServerIP,
			ServerVendor:  m.ServerVendor,
			ServerModel:   m.ServerModel,
			ComponentType: "memory",
			ComponentInfo: fmt.Sprintf("Slot: %s, %dGB %s %s", m.Slot, m.CapacityGB, m.Manufacturer, m.Type),
			MatchedField:  matchedField,
			MatchedValue:  matchedValue,
		})
	}

	return results, nil
}

// searchStorage searches the storage table with server JOIN
func searchStorage(_ interface{}, pattern string, opts SearchOptions) ([]models.SearchResult, error) {
	db := database.GetDB()
	var results []models.SearchResult

	type storageWithServer struct {
		models.Storage
		ServerIP     string
		ServerVendor string
		ServerModel  string
	}

	var matches []storageWithServer
	
	query := db.Table("storage").
		Select("storage.*, servers.ip as server_ip, servers.vendor as server_vendor, servers.model as server_model").
		Joins("JOIN servers ON storage.server_id = servers.id").
		Where("storage.serial_number LIKE ? OR storage.model LIKE ? OR storage.manufacturer LIKE ?",
			pattern, pattern, pattern)
	
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}

	if err := query.Find(&matches).Error; err != nil {
		return nil, err
	}

	for _, s := range matches {
		matchedField, matchedValue := determineStorageMatch(s.Storage, pattern)
		results = append(results, models.SearchResult{
			ServerIP:      s.ServerIP,
			ServerVendor:  s.ServerVendor,
			ServerModel:   s.ServerModel,
			ComponentType: "storage",
			ComponentInfo: fmt.Sprintf("Slot: %s, %s %dGB %s", s.Slot, s.MediaType, s.CapacityGB, s.Manufacturer),
			MatchedField:  matchedField,
			MatchedValue:  matchedValue,
		})
	}

	return results, nil
}

// searchNetworks searches the networks table with server JOIN
func searchNetworks(_ interface{}, pattern string, opts SearchOptions) ([]models.SearchResult, error) {
	db := database.GetDB()
	var results []models.SearchResult

	type networkWithServer struct {
		models.Network
		ServerIP     string
		ServerVendor string
		ServerModel  string
	}

	var matches []networkWithServer
	
	query := db.Table("networks").
		Select("networks.*, servers.ip as server_ip, servers.vendor as server_vendor, servers.model as server_model").
		Joins("JOIN servers ON networks.server_id = servers.id").
		Where("networks.mac_address LIKE ? OR networks.ip_address LIKE ?",
			pattern, pattern)
	
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}

	if err := query.Find(&matches).Error; err != nil {
		return nil, err
	}

	for _, n := range matches {
		matchedField, matchedValue := determineNetworkMatch(n.Network, pattern)
		results = append(results, models.SearchResult{
			ServerIP:      n.ServerIP,
			ServerVendor:  n.ServerVendor,
			ServerModel:   n.ServerModel,
			ComponentType: "network",
			ComponentInfo: fmt.Sprintf("Port: %s, MAC: %s, Status: %s", n.Port, n.MacAddress, n.LinkStatus),
			MatchedField:  matchedField,
			MatchedValue:  matchedValue,
		})
	}

	return results, nil
}

// Helper functions to determine which field matched

func determineServerMatch(s models.Server, pattern string) (string, string) {
	p := strings.Trim(pattern, "%")
	p = strings.ToLower(p)
	
	if strings.Contains(strings.ToLower(s.IP), p) {
		return "ip", s.IP
	}
	if strings.Contains(strings.ToLower(s.ChassisSerial), p) {
		return "chassis_serial", s.ChassisSerial
	}
	if strings.Contains(strings.ToLower(s.Hostname), p) {
		return "hostname", s.Hostname
	}
	if strings.Contains(strings.ToLower(s.Vendor), p) {
		return "vendor", s.Vendor
	}
	if strings.Contains(strings.ToLower(s.Model), p) {
		return "model", s.Model
	}
	return "unknown", ""
}

func determineMemoryMatch(m models.Memory, pattern string) (string, string) {
	p := strings.Trim(pattern, "%")
	p = strings.ToLower(p)
	
	if strings.Contains(strings.ToLower(m.SerialNumber), p) {
		return "serial_number", m.SerialNumber
	}
	if strings.Contains(strings.ToLower(m.Manufacturer), p) {
		return "manufacturer", m.Manufacturer
	}
	if strings.Contains(strings.ToLower(m.PartNumber), p) {
		return "part_number", m.PartNumber
	}
	return "unknown", ""
}

func determineStorageMatch(s models.Storage, pattern string) (string, string) {
	p := strings.Trim(pattern, "%")
	p = strings.ToLower(p)
	
	if strings.Contains(strings.ToLower(s.SerialNumber), p) {
		return "serial_number", s.SerialNumber
	}
	if strings.Contains(strings.ToLower(s.Model), p) {
		return "model", s.Model
	}
	if strings.Contains(strings.ToLower(s.Manufacturer), p) {
		return "manufacturer", s.Manufacturer
	}
	return "unknown", ""
}

func determineNetworkMatch(n models.Network, pattern string) (string, string) {
	p := strings.Trim(pattern, "%")
	p = strings.ToLower(p)
	
	if strings.Contains(strings.ToLower(n.MacAddress), p) {
		return "mac_address", n.MacAddress
	}
	if strings.Contains(strings.ToLower(n.IPAddress), p) {
		return "ip_address", n.IPAddress
	}
	return "unknown", ""
}

// FindByMAC is a convenience function for MAC address lookup
func FindByMAC(mac string) ([]models.SearchResult, error) {
	opts := DefaultSearchOptions()
	opts.ExactMatch = false // Allow partial MAC matches
	return GlobalFind(mac, opts)
}

// FindBySerial is a convenience function for serial number lookup
func FindBySerial(serial string) ([]models.SearchResult, error) {
	opts := DefaultSearchOptions()
	opts.ExactMatch = false
	return GlobalFind(serial, opts)
}
