package workerpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDeduplicationCache(t *testing.T) {
	config := &DeduplicationConfig{
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		MaxMemorySize:   100,
		EnableMetrics:   true,
		CleanupInterval: 30 * time.Minute,
		PersistentCache: true,
	}

	dc := NewDeduplicationCache(config, nil)

	assert.NotNil(t, dc)
	assert.Equal(t, config, dc.config)
	assert.Nil(t, dc.dbCache)
	assert.NotNil(t, dc.memoryCache)
	assert.NotNil(t, dc.metrics)
	assert.Equal(t, 100, dc.memoryCache.maxSize)
}

func TestLRUCache_BasicOperations(t *testing.T) {
	lru := NewLRUCache(3)

	// Test Put and Get
	lru.Put("key1", "value1")
	lru.Put("key2", "value2")

	assert.Equal(t, "value1", lru.Get("key1"))
	assert.Equal(t, "value2", lru.Get("key2"))
	assert.Nil(t, lru.Get("key3"))

	// Test size
	assert.Equal(t, 2, lru.Size())
}

func TestLRUCache_Eviction(t *testing.T) {
	lru := NewLRUCache(2)

	// Fill cache
	lru.Put("key1", "value1")
	lru.Put("key2", "value2")
	assert.Equal(t, 2, lru.Size())

	// Add one more - should evict key1
	lru.Put("key3", "value3")
	assert.Equal(t, 2, lru.Size())
	assert.Nil(t, lru.Get("key1"))
	assert.Equal(t, "value2", lru.Get("key2"))
	assert.Equal(t, "value3", lru.Get("key3"))

	// Access key2 to make it most recently used
	lru.Get("key2")

	// Add another - should evict key3
	lru.Put("key4", "value4")
	assert.Equal(t, 2, lru.Size())
	assert.Nil(t, lru.Get("key3"))
	assert.Equal(t, "value2", lru.Get("key2"))
	assert.Equal(t, "value4", lru.Get("key4"))
}

func TestLRUCache_UpdateExisting(t *testing.T) {
	lru := NewLRUCache(2)

	lru.Put("key1", "value1")
	lru.Put("key2", "value2")

	// Update existing key
	lru.Put("key1", "updated_value")
	assert.Equal(t, 2, lru.Size())
	assert.Equal(t, "updated_value", lru.Get("key1"))
	assert.Equal(t, "value2", lru.Get("key2"))
}

func TestLRUCache_Metrics(t *testing.T) {
	lru := NewLRUCache(2)

	// Test hits and misses
	lru.Put("key1", "value1")
	lru.Get("key1") // Hit
	lru.Get("key2") // Miss

	metrics := lru.GetMetrics()
	assert.Equal(t, int64(1), metrics.Hits)
	assert.Equal(t, int64(1), metrics.Misses)
	assert.Equal(t, int64(1), metrics.Size)
}

func TestDeduplicationCache_MemoryOnly(t *testing.T) {
	config := &DeduplicationConfig{
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		MaxMemorySize:   10,
		EnableMetrics:   true,
		CleanupInterval: 0, // Disable cleanup for test
		PersistentCache: false,
	}

	dc := NewDeduplicationCache(config, nil)
	ctx := context.Background()

	// Test first submission
	task1 := &Task{Payload: "test-payload"}
	isDuplicate, err := dc.IsDuplicate(ctx, task1)
	assert.NoError(t, err)
	assert.False(t, isDuplicate)

	// Test duplicate submission
	task2 := &Task{Payload: "test-payload"}
	isDuplicate, err = dc.IsDuplicate(ctx, task2)
	assert.NoError(t, err)
	assert.True(t, isDuplicate)

	// Test different payload
	task3 := &Task{Payload: "different-payload"}
	isDuplicate, err = dc.IsDuplicate(ctx, task3)
	assert.NoError(t, err)
	assert.False(t, isDuplicate)
}

func TestDeduplicationCache_WithPersistentCache(t *testing.T) {
	config := &DeduplicationConfig{
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		MaxMemorySize:   10,
		EnableMetrics:   true,
		CleanupInterval: 0,
		PersistentCache: true,
	}

	// Test with nil cache (should fall back to memory-only)
	dc := NewDeduplicationCache(config, nil)
	ctx := context.Background()

	task1 := &Task{Payload: "test-payload"}
	isDuplicate, err := dc.IsDuplicate(ctx, task1)
	assert.NoError(t, err)
	assert.False(t, isDuplicate)

	task2 := &Task{Payload: "test-payload"}
	isDuplicate, err = dc.IsDuplicate(ctx, task2)
	assert.NoError(t, err)
	assert.True(t, isDuplicate)
}

func TestDeduplicationCache_ContentAddressableHashing(t *testing.T) {
	config := &DeduplicationConfig{
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		MaxMemorySize:   10,
		EnableMetrics:   true,
		CleanupInterval: 0,
		PersistentCache: false,
	}

	dc := NewDeduplicationCache(config, nil)
	ctx := context.Background()

	// Same content should generate same hash
	task1 := &Task{Payload: "identical-content"}
	task2 := &Task{Payload: "identical-content"}

	// First submission
	isDuplicate, err := dc.IsDuplicate(ctx, task1)
	assert.NoError(t, err)
	assert.False(t, isDuplicate)

	// Second submission with same content
	isDuplicate, err = dc.IsDuplicate(ctx, task2)
	assert.NoError(t, err)
	assert.True(t, isDuplicate)

	// Different content should not be deduplicated
	task3 := &Task{Payload: "different-content"}
	isDuplicate, err = dc.IsDuplicate(ctx, task3)
	assert.NoError(t, err)
	assert.False(t, isDuplicate)
}

func TestDeduplicationCache_Metrics(t *testing.T) {
	config := &DeduplicationConfig{
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		MaxMemorySize:   10,
		EnableMetrics:   true,
		CleanupInterval: 0,
		PersistentCache: false,
	}

	dc := NewDeduplicationCache(config, nil)
	ctx := context.Background()

	// Submit unique tasks
	for i := 0; i < 3; i++ {
		task := &Task{Payload: fmt.Sprintf("task-%d", i)}
		isDuplicate, err := dc.IsDuplicate(ctx, task)
		assert.NoError(t, err)
		assert.False(t, isDuplicate)
	}

	// Submit duplicates
	for i := 0; i < 2; i++ {
		task := &Task{Payload: "task-0"} // Duplicate of first task
		isDuplicate, err := dc.IsDuplicate(ctx, task)
		assert.NoError(t, err)
		assert.True(t, isDuplicate)
	}

	metrics := dc.GetMetrics()
	assert.Equal(t, int64(2), metrics.MemoryHits)
	assert.Equal(t, int64(0), metrics.CacheHits)
	assert.Equal(t, int64(0), metrics.CacheMisses)
	assert.True(t, metrics.HitRate > 0)
}

func TestDeduplicationCache_Shutdown(t *testing.T) {
	config := &DeduplicationConfig{
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		MaxMemorySize:   10,
		EnableMetrics:   true,
		CleanupInterval: 100 * time.Millisecond, // Short interval for test
		PersistentCache: true,
	}

	dc := NewDeduplicationCache(config, nil)

	// Let cleanup run once
	time.Sleep(150 * time.Millisecond)

	// Shutdown
	dc.Shutdown()

	// Test should complete without errors
	assert.True(t, true)
}

func TestDeduplicationCache_ErrorHandling(t *testing.T) {
	config := &DeduplicationConfig{
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		MaxMemorySize:   10,
		EnableMetrics:   true,
		CleanupInterval: 0,
		PersistentCache: true,
	}

	dc := NewDeduplicationCache(config, nil)
	ctx := context.Background()

	task := &Task{Payload: "test-payload"}
	isDuplicate, err := dc.IsDuplicate(ctx, task)
	assert.NoError(t, err) // Should work with memory-only cache
	assert.False(t, isDuplicate)
}

func TestLRUCache_Clear(t *testing.T) {
	lru := NewLRUCache(5)

	// Add some items
	lru.Put("key1", "value1")
	lru.Put("key2", "value2")
	assert.Equal(t, 2, lru.Size())

	// Clear cache
	lru.Clear()
	assert.Equal(t, 0, lru.Size())
	assert.Nil(t, lru.Get("key1"))
	assert.Nil(t, lru.Get("key2"))

	// Verify metrics are reset
	metrics := lru.GetMetrics()
	assert.Equal(t, int64(0), metrics.Size)
}
