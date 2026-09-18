package scanner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Maxsander123/bmcinv/internal/config"
	"github.com/Maxsander123/bmcinv/internal/models"
)

// --- Mock server helpers ---

// redfishMux builds a net/http.ServeMux that serves a complete Redfish v1 API.
// serverHeader is set on the /redfish/v1/ root response (e.g. "iDRAC/9 1.0").
// bodyVendor is embedded in the root JSON body (e.g. "Dell").
// storage controls whether standard /Storage/ drives are populated;
// if false, /Storage/ returns an empty member list (simulates iLO 4).
// smartStorage controls whether HPE SmartStorage path is populated.
func redfishMux(serverHeader, bodyVendor string, storage, smartStorage bool) http.Handler {
	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	// Redfish root
	mux.HandleFunc("/redfish/v1/", func(w http.ResponseWriter, r *http.Request) {
		if serverHeader != "" {
			w.Header().Set("Server", serverHeader)
		}
		writeJSON(w, map[string]interface{}{
			"Systems":  map[string]string{"@odata.id": "/redfish/v1/Systems"},
			"Managers": map[string]string{"@odata.id": "/redfish/v1/Managers"},
			"Oem":      map[string]string{"Vendor": bodyVendor},
			// Embed vendor string so body detection works
			"Description": bodyVendor + " Redfish Service",
		})
	})

	// Systems collection
	mux.HandleFunc("/redfish/v1/Systems", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Members": []map[string]string{{"@odata.id": "/redfish/v1/Systems/1"}},
		})
	})

	// System detail
	mux.HandleFunc("/redfish/v1/Systems/1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Manufacturer": bodyVendor,
			"Model":        "TestServer-2000",
			"SerialNumber": "SN-TEST-0001",
			"BiosVersion":  "3.0.0",
			"HostName":     "testserver.example.com",
			"Memory":       map[string]string{"@odata.id": "/redfish/v1/Systems/1/Memory"},
			"Storage":      map[string]string{"@odata.id": "/redfish/v1/Systems/1/Storage"},
			"EthernetInterfaces": map[string]string{"@odata.id": "/redfish/v1/Systems/1/EthernetInterfaces"},
		})
	})

	// Memory collection
	mux.HandleFunc("/redfish/v1/Systems/1/Memory", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Members": []map[string]string{
				{"@odata.id": "/redfish/v1/Systems/1/Memory/DIMM1"},
				{"@odata.id": "/redfish/v1/Systems/1/Memory/DIMM2"},
				{"@odata.id": "/redfish/v1/Systems/1/Memory/DIMM_EMPTY"},
			},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/Memory/DIMM1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Id": "DIMM1", "DeviceLocator": "DIMM_A1",
			"CapacityMiB": 32768, "OperatingSpeedMhz": 3200,
			"MemoryDeviceType": "DDR4", "Manufacturer": "Samsung",
			"PartNumber": "M393A4K40EB3-CWE", "SerialNumber": "S1MEM001",
			"Status": map[string]string{"Health": "OK"},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/Memory/DIMM2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Id": "DIMM2", "DeviceLocator": "DIMM_B1",
			"CapacityMiB": 16384, "OperatingSpeedMhz": 3200,
			"MemoryDeviceType": "DDR4", "Manufacturer": "Micron",
			"PartNumber": "MTA18ASF2G72PDZ-3G2R", "SerialNumber": "S1MEM002",
			"Status": map[string]string{"Health": "OK"},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/Memory/DIMM_EMPTY", func(w http.ResponseWriter, r *http.Request) {
		// Empty slot — CapacityMiB == 0, must be skipped
		writeJSON(w, map[string]interface{}{
			"Id": "DIMM_EMPTY", "CapacityMiB": 0,
			"Status": map[string]string{"Health": ""},
		})
	})

	// Storage — standard Redfish path
	mux.HandleFunc("/redfish/v1/Systems/1/Storage", func(w http.ResponseWriter, r *http.Request) {
		members := []map[string]string{}
		if storage {
			members = append(members, map[string]string{"@odata.id": "/redfish/v1/Systems/1/Storage/RAID1"})
		}
		writeJSON(w, map[string]interface{}{"Members": members})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/Storage/RAID1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Drives": []map[string]string{
				{"@odata.id": "/redfish/v1/Systems/1/Storage/RAID1/Drives/0"},
			},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/Storage/RAID1/Drives/0", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"MediaType": "SSD", "Protocol": "NVMe",
			"CapacityBytes": int64(960 * 1000 * 1000 * 1000),
			"Manufacturer": "Samsung", "Model": "MZQL2960HCJR",
			"SerialNumber": "S5GJNA0T000001", "Revision": "GDC5302Q",
			"Status": map[string]string{"Health": "OK"}, "FailurePredicted": false,
		})
	})

	// HPE SmartStorage path (iLO 4)
	mux.HandleFunc("/redfish/v1/Systems/1/SmartStorage/", func(w http.ResponseWriter, r *http.Request) {
		if !smartStorage {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]interface{}{
			"ArrayControllers": map[string]string{"@odata.id": "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/"},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/SmartStorage/ArrayControllers/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Members": []map[string]string{{"@odata.id": "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/"}},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Members": []map[string]string{{"@odata.id": "/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/0/"}},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/SmartStorage/ArrayControllers/0/DiskDrives/0/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"CapacityMiB": 1907200, "MediaType": "HDD", "InterfaceType": "SAS",
			"Model": "EG1800FHJTU", "SerialNumber": "S0M5HPE00001",
			"Location": "1I:1:1",
			"Status":   map[string]string{"Health": "OK"},
			"FirmwareVersion": map[string]interface{}{
				"Current": map[string]string{"VersionString": "HPD8"},
			},
		})
	})

	// EthernetInterfaces
	mux.HandleFunc("/redfish/v1/Systems/1/EthernetInterfaces", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Members": []map[string]string{
				{"@odata.id": "/redfish/v1/Systems/1/EthernetInterfaces/1"},
				{"@odata.id": "/redfish/v1/Systems/1/EthernetInterfaces/NOMAC"},
			},
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/EthernetInterfaces/1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Id": "1", "MACAddress": "aa:bb:cc:dd:ee:ff",
			"IPv4Addresses": []map[string]string{{"Address": "10.0.0.1"}},
			"LinkStatus": "Up", "SpeedMbps": 10000,
			"Name": "NIC.Integrated.1-1",
		})
	})
	mux.HandleFunc("/redfish/v1/Systems/1/EthernetInterfaces/NOMAC", func(w http.ResponseWriter, r *http.Request) {
		// Interface without MAC — must be skipped
		writeJSON(w, map[string]interface{}{"Id": "NOMAC", "MACAddress": ""})
	})

	// Managers
	mux.HandleFunc("/redfish/v1/Managers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"Members": []map[string]string{{"@odata.id": "/redfish/v1/Managers/1"}},
		})
	})
	mux.HandleFunc("/redfish/v1/Managers/1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"FirmwareVersion": "5.10.30.20"})
	})

	return mux
}

// hostPort strips the https:// scheme from a test server URL.
func hostPort(u string) string {
	return strings.TrimPrefix(u, "https://")
}

// --- Vendor detection tests ---

func TestDetectVendor_Dell(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("iDRAC/9 1.0", "Dell Inc.", true, false))
	defer srv.Close()

	vendor := detectVendorHTTP(hostPort(srv.URL), srv.Client())
	if vendor != config.BMCTypeIDRAC {
		t.Errorf("got %q, want %q", vendor, config.BMCTypeIDRAC)
	}
}

func TestDetectVendor_HPE_Header(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("iLO/5 1.0", "HPE", true, false))
	defer srv.Close()

	vendor := detectVendorHTTP(hostPort(srv.URL), srv.Client())
	if vendor != config.BMCTypeILO {
		t.Errorf("got %q, want %q", vendor, config.BMCTypeILO)
	}
}

func TestDetectVendor_HPE_Body(t *testing.T) {
	// No Server header — detection must fall back to body
	srv := httptest.NewTLSServer(redfishMux("", "Hewlett Packard Enterprise", true, false))
	defer srv.Close()

	vendor := detectVendorHTTP(hostPort(srv.URL), srv.Client())
	if vendor != config.BMCTypeILO {
		t.Errorf("got %q, want %q", vendor, config.BMCTypeILO)
	}
}

func TestDetectVendor_Supermicro(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("", "Supermicro", true, false))
	defer srv.Close()

	vendor := detectVendorHTTP(hostPort(srv.URL), srv.Client())
	if vendor != config.BMCTypeSupermicro {
		t.Errorf("got %q, want %q", vendor, config.BMCTypeSupermicro)
	}
}

func TestDetectVendor_ASUS(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("", "ASUS", true, false))
	defer srv.Close()

	vendor := detectVendorHTTP(hostPort(srv.URL), srv.Client())
	if vendor != config.BMCTypeIPMI {
		t.Errorf("ASUS should map to IPMI, got %q", vendor)
	}
}

func TestDetectVendor_Unknown_FallsToIPMI(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("", "SomeUnknownVendor", true, false))
	defer srv.Close()

	vendor := detectVendorHTTP(hostPort(srv.URL), srv.Client())
	if vendor != config.BMCTypeIPMI {
		t.Errorf("unknown vendor should fall to IPMI, got %q", vendor)
	}
}

func TestDetectVendor_Unreachable(t *testing.T) {
	// Use an IP that immediately refuses connections
	client := &http.Client{}
	vendor := detectVendorHTTP("127.0.0.1:19999", client)
	if vendor != config.BMCTypeUnknown {
		t.Errorf("unreachable host should return Unknown, got %q", vendor)
	}
}

// --- collectRedfishData tests ---

func testCred() config.Credential {
	return config.Credential{Username: "admin", Password: "password"}
}

func TestCollectRedfishData_Dell(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("iDRAC/9 1.0", "Dell Inc.", true, false))
	defer srv.Close()

	server, err := collectRedfishData(hostPort(srv.URL), config.BMCTypeIDRAC, testCred(), srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertServerBase(t, server, "Dell Inc.", "TestServer-2000", "SN-TEST-0001")
	assertMemory(t, server.Memory)
	assertStorage(t, server.Storage)
	assertNetworks(t, server.Networks)

	if server.BMCVersion != "5.10.30.20" {
		t.Errorf("BMCVersion = %q, want 5.10.30.20", server.BMCVersion)
	}
}

func TestCollectRedfishData_HPE_iLO5_StandardStorage(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("iLO/5 1.0", "HPE", true, false))
	defer srv.Close()

	server, err := collectRedfishData(hostPort(srv.URL), config.BMCTypeILO, testCred(), srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertServerBase(t, server, "HPE", "TestServer-2000", "SN-TEST-0001")
	assertStorage(t, server.Storage)
}

func TestCollectRedfishData_HPE_iLO4_SmartStorageFallback(t *testing.T) {
	// Standard /Storage/ is empty; SmartStorage must kick in
	srv := httptest.NewTLSServer(redfishMux("iLO/4 1.0", "HPE", false, true))
	defer srv.Close()

	server, err := collectRedfishData(hostPort(srv.URL), config.BMCTypeILO, testCred(), srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(server.Storage) == 0 {
		t.Fatal("expected storage via SmartStorage fallback, got none")
	}
	disk := server.Storage[0]
	if disk.MediaType != "HDD" {
		t.Errorf("MediaType = %q, want HDD", disk.MediaType)
	}
	if disk.Protocol != "SAS" {
		t.Errorf("Protocol = %q, want SAS", disk.Protocol)
	}
	if disk.SerialNumber != "S0M5HPE00001" {
		t.Errorf("SerialNumber = %q, want S0M5HPE00001", disk.SerialNumber)
	}
	if disk.FirmwareRev != "HPD8" {
		t.Errorf("FirmwareRev = %q, want HPD8", disk.FirmwareRev)
	}
	// 1907200 MiB / 1024 ≈ 1862 GB
	if disk.CapacityGB < 1800 {
		t.Errorf("CapacityGB = %d, want ~1862", disk.CapacityGB)
	}
}

func TestCollectRedfishData_Supermicro(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("", "Supermicro", true, false))
	defer srv.Close()

	server, err := collectRedfishData(hostPort(srv.URL), config.BMCTypeSupermicro, testCred(), srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertServerBase(t, server, "Supermicro", "TestServer-2000", "SN-TEST-0001")
	assertStorage(t, server.Storage)
}

func TestCollectRedfishData_Memory_EmptySlotSkipped(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("iDRAC/9 1.0", "Dell Inc.", true, false))
	defer srv.Close()

	server, err := collectRedfishData(hostPort(srv.URL), config.BMCTypeIDRAC, testCred(), srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 DIMM endpoints, but DIMM_EMPTY has CapacityMiB==0 → only 2 DIMMs
	if len(server.Memory) != 2 {
		t.Errorf("len(Memory) = %d, want 2 (empty slot must be skipped)", len(server.Memory))
	}
}

func TestCollectRedfishData_NIC_NoMACSkipped(t *testing.T) {
	srv := httptest.NewTLSServer(redfishMux("iDRAC/9 1.0", "Dell Inc.", true, false))
	defer srv.Close()

	server, err := collectRedfishData(hostPort(srv.URL), config.BMCTypeIDRAC, testCred(), srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 NIC endpoints, one has no MAC → only 1 NIC
	if len(server.Networks) != 1 {
		t.Errorf("len(Networks) = %d, want 1 (no-MAC interface must be skipped)", len(server.Networks))
	}
	// MAC is stored uppercase
	if server.Networks[0].MacAddress != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MacAddress = %q, want AA:BB:CC:DD:EE:FF", server.Networks[0].MacAddress)
	}
}

// --- assertion helpers ---

func assertServerBase(t *testing.T, s *models.Server, vendor, model, serial string) {
	t.Helper()
	if s == nil {
		t.Fatal("server is nil")
	}
	if s.Vendor != vendor {
		t.Errorf("Vendor = %q, want %q", s.Vendor, vendor)
	}
	if s.Model != model {
		t.Errorf("Model = %q, want %q", s.Model, model)
	}
	if s.ChassisSerial != serial {
		t.Errorf("ChassisSerial = %q, want %q", s.ChassisSerial, serial)
	}
	if s.Hostname != "testserver.example.com" {
		t.Errorf("Hostname = %q, want testserver.example.com", s.Hostname)
	}
	if s.BiosVersion != "3.0.0" {
		t.Errorf("BiosVersion = %q, want 3.0.0", s.BiosVersion)
	}
}

func assertMemory(t *testing.T, mem []models.Memory) {
	t.Helper()
	if len(mem) == 0 {
		t.Fatal("no memory modules collected")
	}
	found := false
	for _, m := range mem {
		if m.SerialNumber == "S1MEM001" {
			found = true
			if m.CapacityGB != 32 {
				t.Errorf("DIMM1 CapacityGB = %d, want 32", m.CapacityGB)
			}
			if m.Manufacturer != "Samsung" {
				t.Errorf("DIMM1 Manufacturer = %q, want Samsung", m.Manufacturer)
			}
			if m.Speed != 3200 {
				t.Errorf("DIMM1 Speed = %d, want 3200", m.Speed)
			}
			if m.Health != "OK" {
				t.Errorf("DIMM1 Health = %q, want OK", m.Health)
			}
		}
	}
	if !found {
		t.Error("DIMM1 (S1MEM001) not found in memory results")
	}
}

func assertStorage(t *testing.T, storage []models.Storage) {
	t.Helper()
	if len(storage) == 0 {
		t.Fatal("no storage devices collected")
	}
	disk := storage[0]
	if disk.MediaType != "SSD" {
		t.Errorf("MediaType = %q, want SSD", disk.MediaType)
	}
	if disk.Protocol != "NVMe" {
		t.Errorf("Protocol = %q, want NVMe", disk.Protocol)
	}
	if disk.CapacityGB != 960 {
		t.Errorf("CapacityGB = %d, want 960", disk.CapacityGB)
	}
	if disk.SerialNumber != "S5GJNA0T000001" {
		t.Errorf("SerialNumber = %q, want S5GJNA0T000001", disk.SerialNumber)
	}
}

func assertNetworks(t *testing.T, nets []models.Network) {
	t.Helper()
	if len(nets) == 0 {
		t.Fatal("no network interfaces collected")
	}
	nic := nets[0]
	if nic.MacAddress != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MacAddress = %q, want AA:BB:CC:DD:EE:FF", nic.MacAddress)
	}
	if nic.LinkStatus != "Up" {
		t.Errorf("LinkStatus = %q, want Up", nic.LinkStatus)
	}
	if nic.LinkSpeedMbps != 10000 {
		t.Errorf("LinkSpeedMbps = %d, want 10000", nic.LinkSpeedMbps)
	}
	if nic.IPAddress != "10.0.0.1" {
		t.Errorf("IPAddress = %q, want 10.0.0.1", nic.IPAddress)
	}
}
