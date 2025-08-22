package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database represents the main database interface
type Database struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure database settings
	db.SetMaxOpenConns(1) // SQLite doesn't support multiple writers
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	database := &Database{
		db:   db,
		path: dbPath,
	}

	// Run migrations
	if err := database.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Close()
}

// migrate runs database migrations
func (d *Database) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS traces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			value TEXT NOT NULL,
			type TEXT NOT NULL,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			source_plugin TEXT,
			metadata TEXT,
			scan_id INTEGER,
			depth INTEGER DEFAULT 0,
			UNIQUE(value, type)
		)`,
		`CREATE TABLE IF NOT EXISTS scan_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			input TEXT NOT NULL,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			status TEXT DEFAULT 'running',
			total_traces INTEGER DEFAULT 0,
			unique_traces INTEGER DEFAULT 0,
			errors INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS cache_entries (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			plugin_name TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_type ON traces(type)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_discovered_at ON traces(discovered_at)`,
		`CREATE INDEX IF NOT EXISTS idx_traces_scan_id ON traces(scan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_expires_at ON cache_entries(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_plugin ON cache_entries(plugin_name)`,
	}

	for i, migration := range migrations {
		if _, err := d.db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	return nil
}

// GetDB returns the underlying sql.DB instance
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// GetPath returns the database file path
func (d *Database) GetPath() string {
	return d.path
}

// Stats returns database statistics
func (d *Database) Stats() (map[string]interface{}, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := make(map[string]interface{})

	// Get trace count
	var traceCount int
	err := d.db.QueryRow("SELECT COUNT(*) FROM traces").Scan(&traceCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get trace count: %w", err)
	}
	stats["total_traces"] = traceCount

	// Get scan session count
	var sessionCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM scan_sessions").Scan(&sessionCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get session count: %w", err)
	}
	stats["total_sessions"] = sessionCount

	// Get cache entry count
	var cacheCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM cache_entries").Scan(&cacheCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache count: %w", err)
	}
	stats["total_cache_entries"] = cacheCount

	// Get database size
	fileInfo, err := os.Stat(d.path)
	if err == nil {
		stats["database_size_bytes"] = fileInfo.Size()
	}

	return stats, nil
}
