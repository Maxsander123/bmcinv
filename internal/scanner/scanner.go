// Package scanner implements the parallel scanning engine with smart credential selection.
// Design Decision: Worker-Pool Pattern mit konfigurierbarer Größe für parallelen Scan.
// Der Pre-Flight Check (detectVendorHTTP) ermöglicht das dynamische Laden der Credentials
// basierend auf dem erkannten BMC-Typ, ohne dass der Benutzer dies manuell angeben muss.
package scanner

import (
	"context"
	"fmt"
	"net"
	"net/http"
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
	ctx        context.Context
	cancelFunc context.CancelFunc
	httpClient *http.Client
}

// NewScanner creates a scanner with configuration from Viper
func NewScanner() *Scanner {
	cfg := config.GetScanConfig()
	ctx, cancel := context.WithCancel(context.Background())
	timeout := time.Duration(cfg.TimeoutSecs) * time.Second

	return &Scanner{
		workers:    cfg.Workers,
		timeout:    timeout,
		retries:    cfg.RetryAttempts,
		ctx:        ctx,
		cancelFunc: cancel,
		httpClient: newHTTPClient(timeout, cfg.TLSSkipVerify),
	}
}

// ScanCIDR scans all IPs in a CIDR range using a worker pool.
// Workflow: parse CIDR → fan-out to workers via jobs channel → fan-in results.
// Each call creates its own result channel so ScanCIDR is safe to call repeatedly.
func (s *Scanner) ScanCIDR(cidr string) ([]ScanResult, error) {
	ips, err := expandCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IPs in range")
	}

	// Fresh channel per invocation — closing it at end of this call is safe.
	results := make(chan ScanResult, len(ips))
	jobs := make(chan string, len(ips))

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				select {
				case <-s.ctx.Done():
					return
				default:
					results <- s.scanHost(ip)
				}
			}
		}()
	}

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

	go func() {
		wg.Wait()
		close(results)
	}()

	var out []ScanResult
	for r := range results {
		out = append(out, r)
	}
	return out, nil
}

// scanHost performs the complete scan workflow for a single IP:
// 1. Pre-flight vendor detection (unauthenticated)
// 2. Credential selection based on vendor
// 3. Authenticated Redfish data collection with retries
// 4. Database upsert
func (s *Scanner) scanHost(ip string) ScanResult {
	result := ScanResult{IP: ip}

	vendor := detectVendorHTTP(ip, s.httpClient)
	if vendor == config.BMCTypeUnknown {
		result.Error = fmt.Errorf("no Redfish endpoint reachable")
		return result
	}

	cred, err := config.GetCredential(vendor)
	if err != nil {
		result.Error = fmt.Errorf("credential error: %w", err)
		return result
	}

	var server *models.Server
	var lastErr error
	for attempt := 0; attempt <= s.retries; attempt++ {
		server, err = collectRedfishData(ip, vendor, cred, s.httpClient)
		if err == nil {
			break
		}
		lastErr = err
		// Context-aware backoff: don't block on sleep when cancellation is requested.
		select {
		case <-s.ctx.Done():
			result.Error = fmt.Errorf("scan cancelled")
			return result
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	if server == nil {
		result.Error = fmt.Errorf("data collection failed after %d attempts: %w", s.retries+1, lastErr)
		return result
	}

	if err := database.UpsertServer(server); err != nil {
		result.Error = fmt.Errorf("database error: %w", err)
		return result
	}
	if err := database.ReplaceServerComponents(server.ID, server.Memory, server.Storage, server.Networks); err != nil {
		result.Error = fmt.Errorf("component save error: %w", err)
		return result
	}

	result.Success = true
	result.Server = server
	return result
}

// expandCIDR converts CIDR notation to individual IP list.
// A single IP (no mask) is returned as a one-element slice.
func expandCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		if parsed := net.ParseIP(cidr); parsed != nil {
			return []string{cidr}, nil
		}
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {
		if ones, bits := ipnet.Mask.Size(); bits-ones >= 2 {
			ipCopy := make(net.IP, len(ip))
			copy(ipCopy, ip)
			ips = append(ips, ipCopy.String())
		} else {
			ips = append(ips, ip.String())
		}
	}

	// Strip network and broadcast addresses for standard subnets
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}
	return ips, nil
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// Cancel stops all ongoing scan operations
func (s *Scanner) Cancel() {
	s.cancelFunc()
}

// ScanSingleHost scans a single IP address
func (s *Scanner) ScanSingleHost(ip string) ScanResult {
	return s.scanHost(ip)
}
