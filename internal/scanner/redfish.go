package scanner

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Maxsander123/bmcinv/internal/config"
	"github.com/Maxsander123/bmcinv/internal/models"
)

// newHTTPClient creates an HTTP client for BMC communication.
// skipTLS is typically required in data centers with self-signed BMC certificates.
func newHTTPClient(timeout time.Duration, skipTLS bool) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLS}, //nolint:gosec
		DialContext: (&net.Dialer{
			Timeout: timeout / 2,
		}).DialContext,
		TLSHandshakeTimeout: timeout / 2,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// --- Redfish JSON response types ---

type rfLink struct {
	OdataID string `json:"@odata.id"`
}

type rfCollection struct {
	Members []rfLink `json:"Members"`
}

type rfRoot struct {
	Systems  rfLink `json:"Systems"`
	Managers rfLink `json:"Managers"`
}

type rfSystem struct {
	Manufacturer       string `json:"Manufacturer"`
	Model              string `json:"Model"`
	SerialNumber       string `json:"SerialNumber"`
	BiosVersion        string `json:"BiosVersion"`
	HostName           string `json:"HostName"`
	Memory             rfLink `json:"Memory"`
	Storage            rfLink `json:"Storage"`
	EthernetInterfaces rfLink `json:"EthernetInterfaces"`
}

type rfManager struct {
	FirmwareVersion string `json:"FirmwareVersion"`
}

type rfStatus struct {
	Health string `json:"Health"`
}

type rfMemoryItem struct {
	ID                string   `json:"Id"`
	DeviceLocator     string   `json:"DeviceLocator"`
	CapacityMiB       int      `json:"CapacityMiB"`
	OperatingSpeedMhz int      `json:"OperatingSpeedMhz"`
	MemoryDeviceType  string   `json:"MemoryDeviceType"`
	Manufacturer      string   `json:"Manufacturer"`
	PartNumber        string   `json:"PartNumber"`
	SerialNumber      string   `json:"SerialNumber"`
	Status            rfStatus `json:"Status"`
}

type rfStorageController struct {
	Drives []rfLink `json:"Drives"`
}

type rfDrive struct {
	MediaType        string   `json:"MediaType"`
	Protocol         string   `json:"Protocol"`
	CapacityBytes    int64    `json:"CapacityBytes"`
	Manufacturer     string   `json:"Manufacturer"`
	Model            string   `json:"Model"`
	SerialNumber     string   `json:"SerialNumber"`
	Revision         string   `json:"Revision"`
	Status           rfStatus `json:"Status"`
	FailurePredicted bool     `json:"FailurePredicted"`
}

type rfIPv4Address struct {
	Address string `json:"Address"`
}

type rfEthernetInterface struct {
	ID            string          `json:"Id"`
	MACAddress    string          `json:"MACAddress"`
	IPv4Addresses []rfIPv4Address `json:"IPv4Addresses"`
	LinkStatus    string          `json:"LinkStatus"`
	SpeedMbps     int             `json:"SpeedMbps"`
	Name          string          `json:"Name"`
}

// --- HTTP helpers ---

// getJSON performs an authenticated GET request and decodes the JSON response.
func getJSON(client *http.Client, url, username, password string, out interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// --- Vendor detection ---

// detectVendorHTTP probes the BMC's Redfish root endpoint to determine vendor.
// It checks the Server header first, then falls back to response body inspection.
func detectVendorHTTP(ip string, client *http.Client) config.BMCType {
	url := fmt.Sprintf("https://%s/redfish/v1/", ip)
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return config.BMCTypeUnknown
	}
	defer resp.Body.Close()

	serverHdr := strings.ToLower(resp.Header.Get("Server"))
	switch {
	case strings.Contains(serverHdr, "idrac"):
		return config.BMCTypeIDRAC
	case strings.Contains(serverHdr, "ilo"):
		return config.BMCTypeILO
	}

	if resp.StatusCode != http.StatusOK {
		return config.BMCTypeUnknown
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return config.BMCTypeUnknown
	}
	bodyStr := strings.ToLower(string(body))

	switch {
	case strings.Contains(bodyStr, "dell") || strings.Contains(bodyStr, "idrac"):
		return config.BMCTypeIDRAC
	case strings.Contains(bodyStr, "hewlett") || strings.Contains(bodyStr, "hpe") || strings.Contains(bodyStr, "ilo"):
		return config.BMCTypeILO
	default:
		// Any BMC that answers Redfish but whose vendor we can't identify is treated as IPMI-generic.
		return config.BMCTypeIPMI
	}
}

// --- Data collection ---

// collectRedfishData fetches hardware inventory from the Redfish API.
// It follows the standard Redfish v1 path: Systems → Memory / Storage / EthernetInterfaces.
func collectRedfishData(ip string, cred config.Credential, client *http.Client) (*models.Server, error) {
	base := fmt.Sprintf("https://%s", ip)
	u, p := cred.Username, cred.Password

	// 1. Redfish root — discover Systems and Managers paths
	var root rfRoot
	if err := getJSON(client, base+"/redfish/v1/", u, p, &root); err != nil {
		return nil, fmt.Errorf("redfish root: %w", err)
	}

	systemsPath := root.Systems.OdataID
	if systemsPath == "" {
		systemsPath = "/redfish/v1/Systems"
	}

	// 2. Systems collection — use the first member
	var sysColl rfCollection
	if err := getJSON(client, base+systemsPath, u, p, &sysColl); err != nil {
		return nil, fmt.Errorf("systems collection: %w", err)
	}
	if len(sysColl.Members) == 0 {
		return nil, fmt.Errorf("no systems reported by Redfish")
	}

	// 3. System details
	sysPath := sysColl.Members[0].OdataID
	var sys rfSystem
	if err := getJSON(client, base+sysPath, u, p, &sys); err != nil {
		return nil, fmt.Errorf("system details: %w", err)
	}

	server := &models.Server{
		IP:            ip,
		Vendor:        sys.Manufacturer,
		Model:         sys.Model,
		ChassisSerial: sys.SerialNumber,
		BiosVersion:   sys.BiosVersion,
		Hostname:      sys.HostName,
		LastScanned:   time.Now(),
	}

	// 4. BMC firmware version from Managers (best-effort)
	if root.Managers.OdataID != "" {
		var mgrs rfCollection
		if err := getJSON(client, base+root.Managers.OdataID, u, p, &mgrs); err == nil && len(mgrs.Members) > 0 {
			var mgr rfManager
			if err := getJSON(client, base+mgrs.Members[0].OdataID, u, p, &mgr); err == nil {
				server.BMCVersion = mgr.FirmwareVersion
			}
		}
	}

	// 5. Memory (best-effort — missing slots are silently skipped)
	memPath := sys.Memory.OdataID
	if memPath == "" {
		memPath = sysPath + "/Memory"
	}
	server.Memory = fetchMemory(client, base, memPath, u, p)

	// 6. Storage controllers + drives (best-effort)
	storagePath := sys.Storage.OdataID
	if storagePath == "" {
		storagePath = sysPath + "/Storage"
	}
	server.Storage = fetchStorage(client, base, storagePath, u, p)

	// 7. Ethernet interfaces (best-effort)
	ethPath := sys.EthernetInterfaces.OdataID
	if ethPath == "" {
		ethPath = sysPath + "/EthernetInterfaces"
	}
	server.Networks = fetchNetworks(client, base, ethPath, u, p)

	return server, nil
}

func fetchMemory(client *http.Client, base, path, u, p string) []models.Memory {
	var coll rfCollection
	if err := getJSON(client, base+path, u, p, &coll); err != nil {
		return nil
	}

	var result []models.Memory
	for _, link := range coll.Members {
		var item rfMemoryItem
		if err := getJSON(client, base+link.OdataID, u, p, &item); err != nil {
			continue
		}
		if item.CapacityMiB == 0 {
			continue // empty DIMM slot
		}
		slot := item.DeviceLocator
		if slot == "" {
			slot = item.ID
		}
		result = append(result, models.Memory{
			Slot:         slot,
			CapacityGB:   item.CapacityMiB / 1024,
			Speed:        item.OperatingSpeedMhz,
			Type:         item.MemoryDeviceType,
			Manufacturer: item.Manufacturer,
			PartNumber:   strings.TrimSpace(item.PartNumber),
			SerialNumber: strings.TrimSpace(item.SerialNumber),
			Health:       healthOrOK(item.Status.Health),
		})
	}
	return result
}

func fetchStorage(client *http.Client, base, path, u, p string) []models.Storage {
	var controllers rfCollection
	if err := getJSON(client, base+path, u, p, &controllers); err != nil {
		return nil
	}

	var result []models.Storage
	for _, ctrlLink := range controllers.Members {
		var ctrl rfStorageController
		if err := getJSON(client, base+ctrlLink.OdataID, u, p, &ctrl); err != nil {
			continue
		}
		for i, driveLink := range ctrl.Drives {
			var drive rfDrive
			if err := getJSON(client, base+driveLink.OdataID, u, p, &drive); err != nil {
				continue
			}
			result = append(result, models.Storage{
				Slot:         fmt.Sprintf("Bay %d", i+1),
				MediaType:    drive.MediaType,
				Protocol:     drive.Protocol,
				CapacityGB:   int(drive.CapacityBytes / (1000 * 1000 * 1000)),
				Manufacturer: drive.Manufacturer,
				Model:        drive.Model,
				SerialNumber: strings.TrimSpace(drive.SerialNumber),
				FirmwareRev:  drive.Revision,
				Health:       healthOrOK(drive.Status.Health),
				PredFailure:  drive.FailurePredicted,
			})
		}
	}
	return result
}

func fetchNetworks(client *http.Client, base, path, u, p string) []models.Network {
	var coll rfCollection
	if err := getJSON(client, base+path, u, p, &coll); err != nil {
		return nil
	}

	var result []models.Network
	for _, link := range coll.Members {
		var iface rfEthernetInterface
		if err := getJSON(client, base+link.OdataID, u, p, &iface); err != nil {
			continue
		}
		if iface.MACAddress == "" {
			continue // skip virtual/management-only interfaces with no MAC
		}
		ipAddr := ""
		if len(iface.IPv4Addresses) > 0 {
			ipAddr = iface.IPv4Addresses[0].Address
		}
		result = append(result, models.Network{
			Port:          iface.ID,
			MacAddress:    strings.ToUpper(iface.MACAddress),
			IPAddress:     ipAddr,
			LinkStatus:    iface.LinkStatus,
			LinkSpeedMbps: iface.SpeedMbps,
			Model:         iface.Name,
		})
	}
	return result
}

func healthOrOK(health string) string {
	if health == "" {
		return "OK"
	}
	return health
}
