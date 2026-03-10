// Package database handles GORM/SQLite initialization and provides a connection singleton.
// Design Decision: Singleton-Pattern für DB-Connection, da SQLite keine Connection-Pools
// benötigt und die CLI-Anwendung einen einzelnen Execution-Context hat.
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Maxsander123/bmcinv/internal/config"
	"github.com/Maxsander123/bmcinv/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

// InitDB initializes the SQLite database connection and runs migrations.
// Thread-safe via sync.Once - can be called multiple times safely.
func InitDB() (*gorm.DB, error) {
	var initErr error

	once.Do(func() {
		dbPath := config.DatabasePath()

		// Ensure directory exists
		dbDir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dbDir, 0750); err != nil {
			initErr = fmt.Errorf("failed to create database directory: %w", err)
			return
		}

		// SQLite connection with WAL mode for better concurrent read performance
		// WAL (Write-Ahead Logging) allows readers to not block writers
		dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=10000", dbPath)

		gormConfig := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent), // Disable logging by default
		}

		var err error
		db, err = gorm.Open(sqlite.Open(dsn), gormConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to open database: %w", err)
			return
		}

		// Run migrations
		if err := db.AutoMigrate(models.AllModels()...); err != nil {
			initErr = fmt.Errorf("failed to run migrations: %w", err)
			return
		}

		// Create additional indexes that GORM doesn't auto-create
		if err := createCustomIndexes(db); err != nil {
			initErr = fmt.Errorf("failed to create indexes: %w", err)
			return
		}
	})

	if initErr != nil {
		return nil, initErr
	}
	return db, nil
}

// createCustomIndexes creates composite and specialized indexes for search performance
func createCustomIndexes(db *gorm.DB) error {
	// These indexes optimize the "find" command across all tables
	indexes := []string{
		// Composite index for full-text-like serial searches
		"CREATE INDEX IF NOT EXISTS idx_memory_search ON memory(serial_number, manufacturer, part_number)",
		"CREATE INDEX IF NOT EXISTS idx_storage_search ON storage(serial_number, manufacturer, model)",
		"CREATE INDEX IF NOT EXISTS idx_network_mac ON networks(mac_address)",
		// Index for server lookups
		"CREATE INDEX IF NOT EXISTS idx_server_search ON servers(chassis_serial, hostname)",
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetDB returns the initialized database connection.
// Panics if called before InitDB() - this is intentional to catch initialization bugs.
func GetDB() *gorm.DB {
	if db == nil {
		panic("database not initialized - call InitDB() first")
	}
	return db
}

// Close closes the database connection gracefully
func Close() error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// EnableDebugLogging enables SQL query logging for troubleshooting
func EnableDebugLogging() {
	if db != nil {
		db = db.Session(&gorm.Session{
			Logger: logger.Default.LogMode(logger.Info),
		})
	}
}

// UpsertServer creates or updates a server record by IP address.
// This is the core "upsert" logic for the scanner - existing servers are updated,
// new servers are created. Uses GORM's Save with conflict handling.
func UpsertServer(server *models.Server) error {
	db := GetDB()

	// Try to find existing server by IP
	var existing models.Server
	result := db.Where("ip = ?", server.IP).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// New server - create
		return db.Create(server).Error
	} else if result.Error != nil {
		return result.Error
	}

	// Existing server - update fields
	server.ID = existing.ID
	server.CreatedAt = existing.CreatedAt
	return db.Save(server).Error
}

// ReplaceServerComponents replaces all components for a server.
// This ensures we have a clean slate after each scan (removes stale components).
func ReplaceServerComponents(serverID uint, memory []models.Memory, storage []models.Storage, networks []models.Network) error {
	db := GetDB()

	return db.Transaction(func(tx *gorm.DB) error {
		// Delete existing components
		if err := tx.Where("server_id = ?", serverID).Delete(&models.Memory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", serverID).Delete(&models.Storage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("server_id = ?", serverID).Delete(&models.Network{}).Error; err != nil {
			return err
		}

		// Set ServerID and insert new components
		for i := range memory {
			memory[i].ServerID = serverID
		}
		for i := range storage {
			storage[i].ServerID = serverID
		}
		for i := range networks {
			networks[i].ServerID = serverID
		}

		if len(memory) > 0 {
			if err := tx.Create(&memory).Error; err != nil {
				return err
			}
		}
		if len(storage) > 0 {
			if err := tx.Create(&storage).Error; err != nil {
				return err
			}
		}
		if len(networks) > 0 {
			if err := tx.Create(&networks).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetStats returns database statistics for the status command
type DBStats struct {
	TotalServers  int64
	TotalMemory   int64
	TotalStorage  int64
	TotalNetworks int64
}

func GetStats() (DBStats, error) {
	db := GetDB()
	var stats DBStats

	if err := db.Model(&models.Server{}).Count(&stats.TotalServers).Error; err != nil {
		return stats, err
	}
	if err := db.Model(&models.Memory{}).Count(&stats.TotalMemory).Error; err != nil {
		return stats, err
	}
	if err := db.Model(&models.Storage{}).Count(&stats.TotalStorage).Error; err != nil {
		return stats, err
	}
	if err := db.Model(&models.Network{}).Count(&stats.TotalNetworks).Error; err != nil {
		return stats, err
	}

	return stats, nil
}
