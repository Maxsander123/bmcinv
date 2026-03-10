// Package scanner implements the parallel scanning engine with smart credential selection.
// Design Decision: Worker-Pool Pattern mit konfigurierbarer Größe für parallelen Scan.
// Der Pre-Flight Check (DetectVendor) ermöglicht das dynamische Laden der Credentials
// basierend auf dem erkannten BMC-Typ, ohne dass der Benutzer dies manuell angeben muss.
package scanner

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Maxsander123/bmcinv/internal/config"
	"github.com/Maxsander123/bmcinv/internal/database"
	"github.com/Maxsander123/bmcinv/internal/models"
)

// ScanResult holds the outcome of a single server scan
type ScanResult struct {
	IP      string
	Success bool
	Error   error
	Server  *models.Server
}

// Scanner orchestrates parallel BMC scanning operations
type Scanner struct {
	workers    int
	timeout    time.Duration
	retries    int
	results    chan ScanResult
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewScanner creates a scanner with configuration from Viper
func NewScanner() *Scanner {
	cfg := config.GetScanConfig()
	ctx, cancel := context.WithCancel(context.Background())

	return &Scanner{
		workers:    cfg.Workers,
		timeout:    time.Duration(cfg.TimeoutSecs) * time.Second,
		retries:    cfg.RetryAttempts,
		results:    make(chan ScanResult, 100),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// ScanCIDR scans all IPs in a CIDR range using a worker pool.
// The workflow is:
// 1. Parse CIDR and generate IP list
// 2. Start worker goroutines (fan-out)
// 3. Feed IPs to workers via channel
// 4. Collect results via results channel (fan-in)
func (s *Scanner) ScanCIDR(cidr string) ([]ScanResult, error) {
	ips, err := expandCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IPs in range")
	}

	// Job channel for workers
	jobs := make(chan string, len(ips))

	// Start worker pool
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(jobs)
	}

	// Feed jobs
	go func() {
		for _, ip := range ips {
			select {
			case jobs <- ip:
			case <-s.ctx.Done():
				return
			}
		}
		close(jobs)
	}()

	// Collect results
	var results []ScanResult
	done := make(chan struct{})

	go func() {
		s.wg.Wait()
		close(s.results)
	}()

	go func() {
		for result := range s.results {
			results = append(results, result)
		}
		close(done)
	}()

	<-done
	return results, nil
}

// worker processes IPs from the jobs channel
func (s *Scanner) worker(jobs <-chan string) {
	defer s.wg.Done()

	for ip := range jobs {
		select {
		case <-s.ctx.Done():
			return
		default:
			result := s.scanHost(ip)
			s.results <- result
		}
	}
}

// scanHost performs the complete scan workflow for a single IP:
// 1. Pre-flight vendor detection (unauthenticated)
// 2. Credential selection based on vendor
// 3. Authenticated data collection
// 4. Database upsert
func (s *Scanner) scanHost(ip string) ScanResult {
	result := ScanResult{IP: ip}

	// Step 1: Pre-flight check - detect BMC vendor
	vendor := DetectVendor(ip)
	if vendor == config.BMCTypeUnknown {
		result.Error = fmt.Errorf("unable to detect BMC type")
		return result
	}

	// Step 2: Get credentials for this BMC type
	cred, err := config.GetCredential(vendor)
	if err != nil {
		result.Error = fmt.Errorf("credential error: %w", err)
		return result
	}

	// Step 3: Collect data with retries
	var server *models.Server
	var lastErr error

	for attempt := 0; attempt <= s.retries; attempt++ {
		server, err = collectServerData(ip, vendor, cred)
		if err == nil {
			break
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * time.Second) // Exponential backoff
	}

	if server == nil {
		result.Error = fmt.Errorf("data collection failed after %d attempts: %w", s.retries+1, lastErr)
		return result
	}

	// Step 4: Upsert to database
	if err := database.UpsertServer(server); err != nil {
		result.Error = fmt.Errorf("database error: %w", err)
		return result
	}

	// Replace components
	if err := database.ReplaceServerComponents(server.ID, server.Memory, server.Storage, server.Networks); err != nil {
		result.Error = fmt.Errorf("component save error: %w", err)
		return result
	}

	result.Success = true
	result.Server = server
	return result
}

// DetectVendor performs an unauthenticated check to identify the BMC type.
// This is the "Pre-Flight Check" - it examines HTTP headers, response patterns,
// and endpoints to determine if this is an iDRAC, iLO, or IPMI system.
//
// In a real implementation, this would:
// - Make HTTP requests to common BMC endpoints
// - Check Server headers (e.g., "iDRAC/9", "iLO/5")
// - Examine response body for vendor-specific strings
// - Check SSL certificate details
func DetectVendor(ip string) config.BMCType {
	// CONCEPTUAL IMPLEMENTATION
	// In production, this would make actual HTTP requests like:
	//
	// resp, err := http.Get(fmt.Sprintf("https://%s/redfish/v1/", ip))
	// if err == nil {
	//     // Check headers
	//     server := resp.Header.Get("Server")
	//     if strings.Contains(server, "iDRAC") {
	//         return config.BMCTypeIDRAC
	//     }
	//     if strings.Contains(server, "iLO") {
	//         return config.BMCTypeILO
	//     }
	//
	//     // Check response body for vendor-specific identifiers
	//     body, _ := io.ReadAll(resp.Body)
	//     if strings.Contains(string(body), "Dell") {
	//         return config.BMCTypeIDRAC
	//     }
	//     if strings.Contains(string(body), "Hewlett") {
	//         return config.BMCTypeILO
	//     }
	// }

	// For demo: Simulate detection based on IP pattern
	// In production, replace with actual HTTP probing
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		lastOctet := parts[3]
		// Simulate: even IPs are Dell, odd are HP, multiples of 5 are IPMI
		if lastOctet[len(lastOctet)-1] == '0' || lastOctet[len(lastOctet)-1] == '5' {
			return config.BMCTypeIPMI
		}
		if lastOctet[len(lastOctet)-1]%2 == 0 {
			return config.BMCTypeIDRAC
		}
		return config.BMCTypeILO
	}

	return config.BMCTypeUnknown
}

// collectServerData retrieves hardware information via Redfish API.
// In production, this would make authenticated API calls to endpoints like:
// - /redfish/v1/Systems/System.Embedded.1
// - /redfish/v1/Systems/System.Embedded.1/Memory
// - /redfish/v1/Systems/System.Embedded.1/Storage
// - /redfish/v1/Systems/System.Embedded.1/EthernetInterfaces
func collectServerData(ip string, vendor config.BMCType, cred config.Credential) (*models.Server, error) {
	// CONCEPTUAL IMPLEMENTATION
	// In production:
	//
	// client := &http.Client{Timeout: 30 * time.Second}
	// req, _ := http.NewRequest("GET", fmt.Sprintf("https://%s/redfish/v1/Systems/System.Embedded.1", ip), nil)
	// req.SetBasicAuth(cred.Username, cred.Password)
	// resp, err := client.Do(req)
	// ... parse JSON response ...

	// For demo: Generate realistic dummy data
	server := generateDummyServerData(ip, vendor, cred)
	return server, nil
}

// generateDummyServerData creates realistic test data for development.
// This simulates what would come from the Redfish API.
func generateDummyServerData(ip string, vendor config.BMCType, cred config.Credential) *models.Server {
	_ = cred // Would be used for authentication in real implementation

	var vendorName, modelPrefix string
	switch vendor {
	case config.BMCTypeIDRAC:
		vendorName = "Dell Inc."
		modelPrefix = "PowerEdge R"
	case config.BMCTypeILO:
		vendorName = "HPE"
		modelPrefix = "ProLiant DL"
	default:
		vendorName = "Supermicro"
		modelPrefix = "SYS-"
	}

	server := &models.Server{
		IP:            ip,
		Vendor:        vendorName,
		Model:         fmt.Sprintf("%s%d", modelPrefix, 640+rand.Intn(100)),
		ChassisSerial: generateSerial("CHS"),
		BiosVersion:   fmt.Sprintf("%d.%d.%d", rand.Intn(3), rand.Intn(10), rand.Intn(20)),
		BMCVersion:    fmt.Sprintf("%d.%02d.%02d.%02d", rand.Intn(6)+1, rand.Intn(100), rand.Intn(100), rand.Intn(100)),
		Hostname:      fmt.Sprintf("server-%s", strings.ReplaceAll(ip, ".", "-")),
		LastScanned:   time.Now(),
	}

	// Generate memory modules (8-32 DIMMs typical)
	dimmCount := (rand.Intn(4) + 1) * 8 // 8, 16, 24, or 32
	server.Memory = make([]models.Memory, dimmCount)
	memManufacturers := []string{"Samsung", "Micron", "SK Hynix", "Kingston"}
	for i := 0; i < dimmCount; i++ {
		server.Memory[i] = models.Memory{
			Slot:         fmt.Sprintf("DIMM_%c%d", 'A'+rune(i/8), (i%8)+1),
			CapacityGB:   []int{16, 32, 64, 128}[rand.Intn(4)],
			Speed:        []int{2666, 2933, 3200}[rand.Intn(3)],
			Type:         "DDR4",
			Manufacturer: memManufacturers[rand.Intn(len(memManufacturers))],
			PartNumber:   fmt.Sprintf("M393A%dG40BB4-CWE", rand.Intn(9)+1),
			SerialNumber: generateSerial("MEM"),
			Health:       "OK",
		}
	}

	// Generate storage devices (2-24 drives typical)
	driveCount := (rand.Intn(4) + 1) * 2 // 2, 4, 6, or 8
	server.Storage = make([]models.Storage, driveCount)
	storageManufacturers := []string{"Samsung", "Intel", "Micron", "Western Digital", "Seagate"}
	mediaTypes := []string{"SSD", "NVMe", "HDD"}
	for i := 0; i < driveCount; i++ {
		mediaType := mediaTypes[rand.Intn(len(mediaTypes))]
		var capacityGB int
		var protocol string
		switch mediaType {
		case "NVMe":
			capacityGB = []int{480, 960, 1920, 3840}[rand.Intn(4)]
			protocol = "NVMe"
		case "SSD":
			capacityGB = []int{480, 960, 1920, 3840}[rand.Intn(4)]
			protocol = "SATA"
		default:
			capacityGB = []int{2000, 4000, 8000, 16000}[rand.Intn(4)]
			protocol = "SAS"
		}
		server.Storage[i] = models.Storage{
			Slot:         fmt.Sprintf("Bay %d", i+1),
			MediaType:    mediaType,
			Protocol:     protocol,
			CapacityGB:   capacityGB,
			Manufacturer: storageManufacturers[rand.Intn(len(storageManufacturers))],
			Model:        fmt.Sprintf("MZQL%dTHBLA-00AD3", capacityGB/100),
			SerialNumber: generateSerial("DSK"),
			FirmwareRev:  fmt.Sprintf("GDC%d%c0%dQ", rand.Intn(6)+1, 'A'+rune(rand.Intn(26)), rand.Intn(10)),
			Health:       "OK",
			PredFailure:  rand.Float32() < 0.02, // 2% chance of predicted failure
		}
	}

	// Generate network interfaces (2-8 typical)
	nicCount := (rand.Intn(3) + 1) * 2 // 2, 4, or 6
	server.Networks = make([]models.Network, nicCount)
	nicManufacturers := []string{"Intel Corporation", "Broadcom Inc.", "Mellanox Technologies"}
	for i := 0; i < nicCount; i++ {
		server.Networks[i] = models.Network{
			Port:          fmt.Sprintf("NIC.Integrated.1-%d", i+1),
			MacAddress:    generateMAC(),
			IPAddress:     "", // Often empty in BMC data
			LinkStatus:    []string{"Up", "Down"}[rand.Intn(2)],
			LinkSpeedMbps: []int{1000, 10000, 25000}[rand.Intn(3)],
			Manufacturer:  nicManufacturers[rand.Intn(len(nicManufacturers))],
			Model:         fmt.Sprintf("X710 %d-port", []int{2, 4}[rand.Intn(2)]),
			FirmwareRev:   fmt.Sprintf("%d.%d.%d", rand.Intn(20)+1, rand.Intn(10), rand.Intn(100)),
		}
	}

	return server
}

// generateSerial creates a realistic-looking serial number
func generateSerial(prefix string) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ0123456789"
	result := make([]byte, 10)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return prefix + string(result)
}

// generateMAC creates a realistic MAC address
func generateMAC() string {
	// Use common vendor OUIs
	ouis := []string{"00:1B:21", "00:25:90", "00:50:56", "24:6E:96", "EC:F4:BB"}
	oui := ouis[rand.Intn(len(ouis))]
	return fmt.Sprintf("%s:%02X:%02X:%02X", oui, rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

// expandCIDR converts a CIDR notation to a list of individual IPs
func expandCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		// Maybe it's a single IP?
		if parsed := net.ParseIP(cidr); parsed != nil {
			return []string{cidr}, nil
		}
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {
		// Skip network and broadcast addresses for /24 and larger
		if ones, bits := ipnet.Mask.Size(); bits-ones >= 2 {
			ipCopy := make(net.IP, len(ip))
			copy(ipCopy, ip)
			ips = append(ips, ipCopy.String())
		} else {
			ips = append(ips, ip.String())
		}
	}

	// Remove network and broadcast for standard subnets
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips, nil
}

// incrementIP increments an IP address by one
func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// Cancel stops all scanning operations
func (s *Scanner) Cancel() {
	s.cancelFunc()
}

// ScanSingleHost scans a single IP address
func (s *Scanner) ScanSingleHost(ip string) ScanResult {
	return s.scanHost(ip)
}
