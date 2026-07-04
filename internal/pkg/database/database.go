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

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
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
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Close()
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
