package workerpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicationSystem_Integration(t *testing.T) {
	// Test the complete deduplication system integration
	config := &Config{
		MaxWorkers:          5,
		QueueSize:           100,
		DefaultRateLimit:    100,
		DefaultBurst:        10,
		TaskTimeout:         5 * time.Second,
		EnableDeduplication: true,
		EnableMetrics:       true,
		DeduplicationConfig: DeduplicationConfig{
			EnableCache:     true,
			CacheTTL:        1 * time.Hour,
			MaxMemorySize:   1000,
			EnableMetrics:   true,
			CleanupInterval: 0, // Disable cleanup for test
			PersistentCache: false,
		},
	}

	wp := NewWorkerPool(config)

	// Initialize deduplication cache
	dedupCache := NewDeduplicationCache(&config.DeduplicationConfig, nil)
	wp.SetDeduplicationCache(dedupCache)

	defer wp.Shutdown(10 * time.Second)

	ctx := context.Background()

	// Test 1: Basic deduplication
	t.Run("Basic Deduplication", func(t *testing.T) {
		// Submit identical tasks
		task1 := &Task{Payload: "identical-content"}
		task2 := &Task{Payload: "identical-content"}

		err1 := wp.Submit(ctx, task1)
		err2 := wp.Submit(ctx, task2)

		assert.NoError(t, err1)
		assert.NoError(t, err2) // Should be deduplicated

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Check metrics
		metrics := wp.GetMetrics()
		assert.Equal(t, int64(1), metrics.DeduplicationHits)
		assert.NotNil(t, metrics.DeduplicationMetrics)
		assert.Equal(t, int64(1), metrics.DeduplicationMetrics.MemoryHits)
	})

	// Test 2: Memory-efficient storage
	t.Run("Memory Efficient Storage", func(t *testing.T) {
		// Submit many unique tasks to test LRU eviction
		for i := 0; i < 50; i++ {
			task := &Task{Payload: fmt.Sprintf("unique-task-%d", i)}
			err := wp.Submit(ctx, task)
			assert.NoError(t, err)
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// Check that memory usage is controlled
		metrics := wp.GetMetrics()
		assert.NotNil(t, metrics.DeduplicationMetrics)
		assert.True(t, metrics.DeduplicationMetrics.MemoryUsage <= int64(config.DeduplicationConfig.MaxMemorySize))
	})

	// Test 3: Content-addressable hashing
	t.Run("Content Addressable Hashing", func(t *testing.T) {
		// Test that different content types generate different hashes
		task1 := &Task{Payload: "email@example.com"}
		task2 := &Task{Payload: "https://example.com"}
		task3 := &Task{Payload: "example.com"}

		// All should be unique
		err1 := wp.Submit(ctx, task1)
		err2 := wp.Submit(ctx, task2)
		err3 := wp.Submit(ctx, task3)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)

		// Submit duplicates
		task1Dup := &Task{Payload: "email@example.com"}
		task2Dup := &Task{Payload: "https://example.com"}

		err1Dup := wp.Submit(ctx, task1Dup)
		err2Dup := wp.Submit(ctx, task2Dup)

		assert.NoError(t, err1Dup) // Should be deduplicated
		assert.NoError(t, err2Dup) // Should be deduplicated

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Check deduplication metrics
		metrics := wp.GetMetrics()
		assert.True(t, metrics.DeduplicationHits >= 2) // At least 2 duplicates
	})

	// Test 4: Concurrent deduplication
	t.Run("Concurrent Deduplication", func(t *testing.T) {
		// Submit the same task concurrently from multiple goroutines
		const numGoroutines = 10
		const numTasks = 5

		for i := 0; i < numTasks; i++ {
			content := fmt.Sprintf("concurrent-task-%d", i)

			// Submit the same content from multiple goroutines
			for j := 0; j < numGoroutines; j++ {
				go func(content string) {
					task := &Task{Payload: content}
					wp.Submit(ctx, task)
				}(content)
			}
		}

		// Wait for all submissions
		time.Sleep(500 * time.Millisecond)

		// Check that deduplication worked correctly
		metrics := wp.GetMetrics()
		assert.NotNil(t, metrics.DeduplicationMetrics)
		assert.True(t, metrics.DeduplicationMetrics.MemoryHits > 0)
	})

	// Test 5: Metrics and monitoring
	t.Run("Metrics and Monitoring", func(t *testing.T) {
		// Submit some tasks to generate metrics
		for i := 0; i < 10; i++ {
			task := &Task{Payload: fmt.Sprintf("metrics-test-%d", i)}
			wp.Submit(ctx, task)
		}

		// Submit some duplicates
		for i := 0; i < 5; i++ {
			task := &Task{Payload: "metrics-test-0"} // Duplicate
			wp.Submit(ctx, task)
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// Check comprehensive metrics
		metrics := wp.GetMetrics()
		assert.NotNil(t, metrics.DeduplicationMetrics)

		// Verify hit rate calculation
		totalRequests := metrics.DeduplicationMetrics.MemoryHits + metrics.DeduplicationMetrics.CacheHits + metrics.DeduplicationMetrics.CacheMisses
		if totalRequests > 0 {
			expectedHitRate := float64(metrics.DeduplicationMetrics.MemoryHits+metrics.DeduplicationMetrics.CacheHits) / float64(totalRequests)
			assert.Equal(t, expectedHitRate, metrics.DeduplicationMetrics.HitRate)
		}

		// Verify memory usage is tracked
		assert.True(t, metrics.DeduplicationMetrics.MemoryUsage >= 0)
		assert.True(t, metrics.DeduplicationMetrics.CacheSize >= 0)
	})
}

func TestDeduplicationSystem_Performance(t *testing.T) {
	// Test performance characteristics of the deduplication system
	config := &Config{
		MaxWorkers:          10,
		QueueSize:           1000,
		DefaultRateLimit:    1000,
		DefaultBurst:        100,
		TaskTimeout:         10 * time.Second,
		EnableDeduplication: true,
		EnableMetrics:       true,
		DeduplicationConfig: DeduplicationConfig{
			EnableCache:     true,
			CacheTTL:        1 * time.Hour,
			MaxMemorySize:   10000,
			EnableMetrics:   true,
			CleanupInterval: 0,
			PersistentCache: false,
		},
	}

	wp := NewWorkerPool(config)
	dedupCache := NewDeduplicationCache(&config.DeduplicationConfig, nil)
	wp.SetDeduplicationCache(dedupCache)
	defer wp.Shutdown(10 * time.Second)

	ctx := context.Background()

	// Performance test: Submit many tasks quickly
	t.Run("High Throughput Deduplication", func(t *testing.T) {
		start := time.Now()

		// Submit 1000 unique tasks
		for i := 0; i < 1000; i++ {
			task := &Task{Payload: fmt.Sprintf("perf-test-%d", i)}
			err := wp.Submit(ctx, task)
			assert.NoError(t, err)
		}

		// Submit 500 duplicates
		for i := 0; i < 500; i++ {
			task := &Task{Payload: fmt.Sprintf("perf-test-%d", i%100)} // Duplicates of first 100
			err := wp.Submit(ctx, task)
			assert.NoError(t, err)
		}

		duration := time.Since(start)

		// Wait for processing
		time.Sleep(1 * time.Second)

		// Check performance metrics
		metrics := wp.GetMetrics()
		assert.NotNil(t, metrics.DeduplicationMetrics)

		// Verify deduplication worked
		assert.True(t, metrics.DeduplicationMetrics.MemoryHits > 0)

		// Performance should be reasonable (sub-second for 1500 submissions)
		assert.True(t, duration < 2*time.Second, "Deduplication system should handle high throughput")

		t.Logf("Processed 1500 tasks (1000 unique, 500 duplicates) in %v", duration)
		t.Logf("Deduplication hits: %d", metrics.DeduplicationMetrics.MemoryHits)
		t.Logf("Memory usage: %d items", metrics.DeduplicationMetrics.MemoryUsage)
	})
}

func TestDeduplicationSystem_EdgeCases(t *testing.T) {
	config := &Config{
		MaxWorkers:          2,
		QueueSize:           10,
		DefaultRateLimit:    100,
		DefaultBurst:        10,
		TaskTimeout:         5 * time.Second,
		EnableDeduplication: true,
		EnableMetrics:       true,
		DeduplicationConfig: DeduplicationConfig{
			EnableCache:     true,
			CacheTTL:        1 * time.Hour,
			MaxMemorySize:   5, // Small cache for testing eviction
			EnableMetrics:   true,
			CleanupInterval: 0,
			PersistentCache: false,
		},
	}

	wp := NewWorkerPool(config)
	dedupCache := NewDeduplicationCache(&config.DeduplicationConfig, nil)
	wp.SetDeduplicationCache(dedupCache)
	defer wp.Shutdown(10 * time.Second)

	ctx := context.Background()

	// Test edge cases
	t.Run("Empty Payload", func(t *testing.T) {
		task1 := &Task{Payload: ""}
		task2 := &Task{Payload: ""}

		err1 := wp.Submit(ctx, task1)
		err2 := wp.Submit(ctx, task2)

		assert.NoError(t, err1)
		assert.NoError(t, err2) // Should be deduplicated

		time.Sleep(100 * time.Millisecond)
		metrics := wp.GetMetrics()
		assert.Equal(t, int64(1), metrics.DeduplicationHits)
	})

	t.Run("Very Large Payload", func(t *testing.T) {
		largePayload := string(make([]byte, 10000)) // 10KB payload
		task1 := &Task{Payload: largePayload}
		task2 := &Task{Payload: largePayload}

		err1 := wp.Submit(ctx, task1)
		err2 := wp.Submit(ctx, task2)

		assert.NoError(t, err1)
		assert.NoError(t, err2) // Should be deduplicated

		time.Sleep(100 * time.Millisecond)
		metrics := wp.GetMetrics()
		assert.True(t, metrics.DeduplicationHits > 0)
	})

	t.Run("LRU Eviction", func(t *testing.T) {
		// Fill cache beyond capacity
		for i := 0; i < 10; i++ {
			task := &Task{Payload: fmt.Sprintf("eviction-test-%d", i)}
			wp.Submit(ctx, task)
		}

		time.Sleep(100 * time.Millisecond)

		// Submit duplicates of early items (should not be found due to eviction)
		task1 := &Task{Payload: "eviction-test-0"}
		task2 := &Task{Payload: "eviction-test-1"}

		wp.Submit(ctx, task1)
		wp.Submit(ctx, task2)

		time.Sleep(100 * time.Millisecond)

		// Check that some items were evicted
		metrics := wp.GetMetrics()
		assert.NotNil(t, metrics.DeduplicationMetrics)
		assert.True(t, metrics.DeduplicationMetrics.Evictions > 0)
	})
}
