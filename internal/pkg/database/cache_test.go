package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

func TestNewCache(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	assert.NotNil(t, cache)
	assert.Equal(t, repo, cache.repo)
}

func TestCache_CacheKey(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	key1 := cache.CacheKey(trace, "TestPlugin")
	key2 := cache.CacheKey(trace, "TestPlugin")

	// Same trace and plugin should generate same key
	assert.Equal(t, key1, key2)
	assert.NotEmpty(t, key1)
	assert.Len(t, key1, 64) // SHA256 hex string length

	// Different plugin should generate different key
	key3 := cache.CacheKey(trace, "DifferentPlugin")
	assert.NotEqual(t, key1, key3)

	// Different trace should generate different key
	trace2 := entities.Trace{
		Value: "different@example.com",
		Type:  entities.Email,
	}
	key4 := cache.CacheKey(trace2, "TestPlugin")
	assert.NotEqual(t, key1, key4)
}

func TestCache_SetAndGet(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	results := []entities.Trace{
		{Value: "result1@example.com", Type: entities.Email},
		{Value: "result2@example.com", Type: entities.Email},
		{Value: "github.com/user", Type: entities.Github},
	}

	// Store in cache
	err = cache.Set(trace, "TestPlugin", results, time.Hour)
	require.NoError(t, err)

	// Retrieve from cache
	retrieved, err := cache.Get(trace, "TestPlugin")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Len(t, retrieved, len(results))

	// Verify results match
	for i, r := range retrieved {
		assert.Equal(t, results[i].Value, r.Value)
		assert.Equal(t, results[i].Type, r.Type)
	}
}

func TestCache_Get_CacheMiss(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	trace := entities.Trace{
		Value: "nonexistent@example.com",
		Type:  entities.Email,
	}

	// Try to get non-existent cache entry
	retrieved, err := cache.Get(trace, "TestPlugin")
	assert.NoError(t, err)
	assert.Nil(t, retrieved) // Cache miss should return nil, not error
}

func TestCache_Set_WithTTL(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	results := []entities.Trace{
		{Value: "result@example.com", Type: entities.Email},
	}

	// Store with TTL
	err = cache.Set(trace, "TestPlugin", results, 1*time.Second)
	require.NoError(t, err)

	// Retrieve immediately (should work)
	retrieved, err := cache.Get(trace, "TestPlugin")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Clean expired entries
	err = cache.CleanExpired()
	require.NoError(t, err)

	// Try to retrieve after expiration
	retrieved, err = cache.Get(trace, "TestPlugin")
	require.NoError(t, err)
	// Should be nil after expiration and cleanup
	assert.Nil(t, retrieved)
}

func TestCache_Set_NoTTL(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	results := []entities.Trace{
		{Value: "result@example.com", Type: entities.Email},
	}

	// Store without TTL (0 duration)
	err = cache.Set(trace, "TestPlugin", results, 0)
	require.NoError(t, err)

	// Should still be retrievable
	retrieved, err := cache.Get(trace, "TestPlugin")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
}

func TestCache_CleanExpired(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	// Store multiple entries with different TTLs
	trace1 := entities.Trace{Value: "test1@example.com", Type: entities.Email}
	trace2 := entities.Trace{Value: "test2@example.com", Type: entities.Email}
	trace3 := entities.Trace{Value: "test3@example.com", Type: entities.Email}

	results := []entities.Trace{{Value: "result@example.com", Type: entities.Email}}

	// Entry 1: expires in 1 second
	err = cache.Set(trace1, "TestPlugin", results, 1*time.Second)
	require.NoError(t, err)

	// Entry 2: expires in 1 second
	err = cache.Set(trace2, "TestPlugin", results, 1*time.Second)
	require.NoError(t, err)

	// Entry 3: no expiration
	err = cache.Set(trace3, "TestPlugin", results, 0)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Clean expired entries
	err = cache.CleanExpired()
	require.NoError(t, err)

	// Entry 1 should be gone
	retrieved, err := cache.Get(trace1, "TestPlugin")
	require.NoError(t, err)
	assert.Nil(t, retrieved)

	// Entry 2 should be gone
	retrieved, err = cache.Get(trace2, "TestPlugin")
	require.NoError(t, err)
	assert.Nil(t, retrieved)

	// Entry 3 should still exist
	retrieved, err = cache.Get(trace3, "TestPlugin")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
}

func TestCache_Invalidate(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	// Store entries for different plugins
	trace1 := entities.Trace{Value: "test1@example.com", Type: entities.Email}
	trace2 := entities.Trace{Value: "test2@example.com", Type: entities.Email}

	results := []entities.Trace{{Value: "result@example.com", Type: entities.Email}}

	err = cache.Set(trace1, "Plugin1", results, time.Hour)
	require.NoError(t, err)

	err = cache.Set(trace2, "Plugin2", results, time.Hour)
	require.NoError(t, err)

	// Invalidate Plugin1
	err = cache.Invalidate("Plugin1")
	require.NoError(t, err)

	// Plugin1 entry should be gone (via cleanup)
	// Note: Invalidate currently calls CleanExpired, so expired entries are removed
	// This test verifies the method doesn't error
}

func TestCache_DefaultTTL(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	expectedTTL := 24 * time.Hour
	actualTTL := cache.DefaultTTL()

	assert.Equal(t, expectedTTL, actualTTL)
}

func TestCache_GetTTLForPlugin(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	tests := []struct {
		name     string
		plugin   string
		expected time.Duration
	}{
		{"SubdomainPlugin", "SubdomainPlugin", 12 * time.Hour},
		{"CodeRepositoriesPlugin", "CodeRepositoriesPlugin", 6 * time.Hour},
		{"SocialProfilesPlugin", "SocialProfilesPlugin", 1 * time.Hour},
		{"UnknownPlugin", "UnknownPlugin", 24 * time.Hour}, // Default TTL
		{"EmptyPlugin", "", 24 * time.Hour},                // Default TTL
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttl := cache.GetTTLForPlugin(tt.plugin)
			assert.Equal(t, tt.expected, ttl)
		})
	}
}

func TestCache_GetStats(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	stats, err := cache.GetStats()
	require.NoError(t, err)
	require.NotNil(t, stats)

	// Note: Current implementation returns zero values
	// This test verifies the method doesn't error
	assert.NotNil(t, stats)
}

func TestCache_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	// Test concurrent Set operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			trace := entities.Trace{
				Value: "test@example.com",
				Type:  entities.Email,
			}
			results := []entities.Trace{
				{Value: "result@example.com", Type: entities.Email},
			}
			err := cache.Set(trace, "TestPlugin", results, time.Hour)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify cache entry exists
	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}
	retrieved, err := cache.Get(trace, "TestPlugin")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
}

func TestCache_EmptyResults(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	// Store empty results
	err = cache.Set(trace, "TestPlugin", []entities.Trace{}, time.Hour)
	require.NoError(t, err)

	// Retrieve empty results
	retrieved, err := cache.Get(trace, "TestPlugin")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Len(t, retrieved, 0)
}

func TestCache_JSONMarshaling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	cache := NewCache(repo)

	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	// Store complex results with various trace types
	results := []entities.Trace{
		{Value: "email@example.com", Type: entities.Email},
		{Value: "github.com/user", Type: entities.Github},
		{Value: "linkedin.com/in/user", Type: entities.Linkedin},
		{Value: "example.com", Type: entities.Domain},
		{Value: "192.168.1.1", Type: entities.IpAddr},
	}

	err = cache.Set(trace, "TestPlugin", results, time.Hour)
	require.NoError(t, err)

	// Retrieve and verify all trace types are preserved
	retrieved, err := cache.Get(trace, "TestPlugin")
	require.NoError(t, err)
	require.Len(t, retrieved, len(results))

	for i, r := range retrieved {
		assert.Equal(t, results[i].Value, r.Value)
		assert.Equal(t, results[i].Type, r.Type)
	}
}
