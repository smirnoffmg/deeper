package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

func TestNewDatabase(t *testing.T) {
	// Create a temporary database for testing
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Check if database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Check if we can get stats
	stats, err := db.Stats()
	if err != nil {
		t.Fatalf("Failed to get database stats: %v", err)
	}

	// Check if stats contain expected keys
	expectedKeys := []string{"total_traces", "total_sessions", "total_cache_entries"}
	for _, key := range expectedKeys {
		if _, exists := stats[key]; !exists {
			t.Errorf("Stats missing expected key: %s", key)
		}
	}
}

func TestRepository_StoreAndGetTrace(t *testing.T) {
	// Create a temporary database for testing
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Create a test trace
	trace := &Trace{
		Value:        "test@example.com",
		Type:         entities.Email,
		SourcePlugin: "TestPlugin",
		DiscoveredAt: time.Now(),
		Depth:        1,
	}

	// Store the trace
	err = repo.StoreTrace(trace)
	if err != nil {
		t.Fatalf("Failed to store trace: %v", err)
	}

	// Retrieve the trace
	retrieved, err := repo.GetTraceByValue("test@example.com", entities.Email)
	if err != nil {
		t.Fatalf("Failed to get trace: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved trace is nil")
	}

	if retrieved.Value != trace.Value {
		t.Errorf("Expected value %s, got %s", trace.Value, retrieved.Value)
	}

	if retrieved.Type != trace.Type {
		t.Errorf("Expected type %s, got %s", trace.Type, retrieved.Type)
	}
}

func TestCache_StoreAndGet(t *testing.T) {
	// Create a temporary database for testing
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)
	cache := NewCache(repo)

	// Create a test trace
	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	// Create test results
	results := []entities.Trace{
		{Value: "result1@example.com", Type: entities.Email},
		{Value: "result2@example.com", Type: entities.Email},
	}

	// Store in cache
	err = cache.Set(trace, "TestPlugin", results, time.Hour)
	if err != nil {
		t.Fatalf("Failed to store in cache: %v", err)
	}

	// Retrieve from cache
	retrieved, err := cache.Get(trace, "TestPlugin")
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}

	if len(retrieved) != len(results) {
		t.Errorf("Expected %d results, got %d", len(results), len(retrieved))
	}

	// Test cache miss
	nonExistentTrace := entities.Trace{
		Value: "nonexistent@example.com",
		Type:  entities.Email,
	}

	retrieved, err = cache.Get(nonExistentTrace, "TestPlugin")
	if err != nil {
		t.Fatalf("Failed to get non-existent trace: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil for non-existent trace")
	}
}

func TestRepository_ScanSession(t *testing.T) {
	// Create a temporary database for testing
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Create a scan session
	session, err := repo.CreateScanSession("test@example.com")
	if err != nil {
		t.Fatalf("Failed to create scan session: %v", err)
	}

	if session.ID == 0 {
		t.Error("Session ID should not be zero")
	}

	if session.Input != "test@example.com" {
		t.Errorf("Expected input %s, got %s", "test@example.com", session.Input)
	}

	// Update the session
	session.Status = "completed"
	session.TotalTraces = 10
	session.UniqueTraces = 8
	session.Errors = 2
	completedAt := time.Now()
	session.CompletedAt = &completedAt

	err = repo.UpdateScanSession(session)
	if err != nil {
		t.Fatalf("Failed to update scan session: %v", err)
	}

	// Retrieve the session
	retrieved, err := repo.GetScanSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to get scan session: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved session is nil")
	}

	if retrieved.Status != "completed" {
		t.Errorf("Expected status %s, got %s", "completed", retrieved.Status)
	}

	if retrieved.TotalTraces != 10 {
		t.Errorf("Expected total traces %d, got %d", 10, retrieved.TotalTraces)
	}
}
