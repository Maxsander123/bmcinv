// Package models defines GORM database models for hardware inventory.
// Design Decision: Relationale Struktur mit Foreign Keys für Datenintegrität.
// Composite Indizes auf SerialNumber und MacAddress für schnelle Suche.
// GORM's AutoMigrate wird verwendet, um Schema-Updates zu handhaben.
package models

import (
	"time"

	"gorm.io/gorm"
)

// Server represents a physical server with its BMC/management interface.
// This is the root entity - all components (Memory, Storage, Network) belong to a Server.
type Server struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	IP            string         `gorm:"uniqueIndex;size:45;not null" json:"ip"` // IPv4/IPv6
	Vendor        string         `gorm:"index;size:50" json:"vendor"`            // Dell, HPE, Supermicro
	Model         string         `gorm:"size:100" json:"model"`
	ChassisSerial string         `gorm:"index;size:50" json:"chassis_serial"`
	BiosVersion   string         `gorm:"size:50" json:"bios_version"`
	BMCVersion    string         `gorm:"size:50" json:"bmc_version"`
	Hostname      string         `gorm:"size:255" json:"hostname"`
	LastScanned   time.Time      `gorm:"index" json:"last_scanned"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships (has-many)
	Memory   []Memory  `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE" json:"memory,omitempty"`
	Storage  []Storage `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE" json:"storage,omitempty"`
	Networks []Network `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE" json:"networks,omitempty"`
}

// Memory represents a DIMM module installed in a server.
// Index on SerialNumber enables fast serial number lookups across the fleet.
type Memory struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ServerID     uint           `gorm:"index;not null" json:"server_id"`
	Slot         string         `gorm:"size:50" json:"slot"`               // e.g., "DIMM_A1", "P1-DIMMA1"
	CapacityGB   int            `gorm:"index" json:"capacity_gb"`          // Index for "find all 64GB DIMMs"
	Speed        int            `json:"speed"`                             // MHz
	Type         string         `gorm:"size:20" json:"type"`               // DDR4, DDR5
	Manufacturer string         `gorm:"index;size:50" json:"manufacturer"` // Samsung, Micron, SK Hynix
	PartNumber   string         `gorm:"size:50" json:"part_number"`
	SerialNumber string         `gorm:"index;size:50" json:"serial_number"` // Critical for RMA tracking!
	Health       string         `gorm:"size:20" json:"health"`              // OK, Warning, Critical
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Storage represents a storage device (SSD, HDD, NVMe) in a server.
// Supports multiple device types with unified serial number tracking.
type Storage struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ServerID     uint           `gorm:"index;not null" json:"server_id"`
	Slot         string         `gorm:"size:50" json:"slot"`                // e.g., "Bay 1", "Slot 0"
	MediaType    string         `gorm:"index;size:20" json:"media_type"`    // SSD, HDD, NVMe
	Protocol     string         `gorm:"size:20" json:"protocol"`            // SATA, SAS, NVMe
	CapacityGB   int            `gorm:"index" json:"capacity_gb"`           // Index for capacity queries
	Manufacturer string         `gorm:"index;size:50" json:"manufacturer"`  // Samsung, WD, Seagate
	Model        string         `gorm:"size:100" json:"model"`
	SerialNumber string         `gorm:"index;size:50" json:"serial_number"` // Critical for RMA!
	FirmwareRev  string         `gorm:"size:50" json:"firmware_rev"`
	Health       string         `gorm:"index;size:20" json:"health"`        // OK, Warning, Critical, Failed
	PredFailure  bool           `json:"pred_failure"`                       // SMART predicted failure
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Network represents a network interface on a server.
// MAC address index is critical for network debugging and DHCP/ARP correlation.
type Network struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ServerID     uint           `gorm:"index;not null" json:"server_id"`
	Port         string         `gorm:"size:50" json:"port"`                         // e.g., "NIC.Integrated.1-1"
	MacAddress   string         `gorm:"index;size:17" json:"mac_address"`            // Format: AA:BB:CC:DD:EE:FF
	IPAddress    string         `gorm:"size:45" json:"ip_address"`                   // Assigned IP if known
	LinkStatus   string         `gorm:"index;size:20" json:"link_status"`            // Up, Down, Unknown
	LinkSpeedMbps int           `json:"link_speed_mbps"`                              // 1000, 10000, 25000
	Manufacturer string         `gorm:"size:50" json:"manufacturer"`                 // Intel, Broadcom, Mellanox
	Model        string         `gorm:"size:100" json:"model"`
	FirmwareRev  string         `gorm:"size:50" json:"firmware_rev"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// SearchResult represents unified search results across all component types.
// Used by the "find" command to return matched components with their parent server.
type SearchResult struct {
	ServerIP      string `json:"server_ip"`
	ServerVendor  string `json:"server_vendor"`
	ServerModel   string `json:"server_model"`
	ComponentType string `json:"component_type"` // "memory", "storage", "network", "server"
	ComponentInfo string `json:"component_info"` // Human-readable component description
	MatchedField  string `json:"matched_field"`  // Which field matched (e.g., "serial_number", "mac_address")
	MatchedValue  string `json:"matched_value"`  // The actual matched value
}

// AllModels returns all model types for AutoMigrate
func AllModels() []interface{} {
	return []interface{}{
		&Server{},
		&Memory{},
		&Storage{},
		&Network{},
	}
}

// TableName overrides for explicit table naming (optional but good practice)
func (Server) TableName() string  { return "servers" }
func (Memory) TableName() string  { return "memory" }
func (Storage) TableName() string { return "storage" }
func (Network) TableName() string { return "networks" }
