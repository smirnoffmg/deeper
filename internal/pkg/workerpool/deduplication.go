package workerpool

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

// DeduplicationConfig holds configuration for the deduplication system
type DeduplicationConfig struct {
	EnableCache     bool
	CacheTTL        time.Duration
	MaxMemorySize   int // Maximum number of items in memory cache
	EnableMetrics   bool
	CleanupInterval time.Duration
	PersistentCache bool // Whether to use database cache
}

// DeduplicationCache provides memory-efficient deduplication with cache integration
type DeduplicationCache struct {
	config        *DeduplicationConfig
	memoryCache   *LRUCache
	dbCache       *database.Cache
	mutex         sync.RWMutex
	metrics       *DeduplicationMetrics
	cleanupTicker *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
}

// LRUCache implements a thread-safe LRU cache
type LRUCache struct {
	maxSize int
	cache   map[string]*list.Element
	list    *list.List
	mutex   sync.RWMutex
	metrics *LRUMetrics
}

// LRUMetrics tracks LRU cache performance
type LRUMetrics struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int64
}

// DeduplicationMetrics tracks deduplication system performance
type DeduplicationMetrics struct {
	MemoryHits  int64
	CacheHits   int64
	CacheMisses int64
	Evictions   int64
	MemoryUsage int64
	CacheSize   int64
	HitRate     float64
}

// NewDeduplicationCache creates a new deduplication cache
func NewDeduplicationCache(config *DeduplicationConfig, dbCache *database.Cache) *DeduplicationCache {
	ctx, cancel := context.WithCancel(context.Background())

	dc := &DeduplicationCache{
		config:      config,
		memoryCache: NewLRUCache(config.MaxMemorySize),
		dbCache:     dbCache,
		metrics:     &DeduplicationMetrics{},
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start cleanup routine if enabled
	if config.CleanupInterval > 0 {
		dc.cleanupTicker = time.NewTicker(config.CleanupInterval)
		go dc.cleanupRoutine()
	}

	return dc
}

// NewLRUCache creates a new LRU cache
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		maxSize: maxSize,
		cache:   make(map[string]*list.Element),
		list:    list.New(),
		metrics: &LRUMetrics{},
	}
}

// IsDuplicate checks if a task is a duplicate using both memory and persistent cache
func (dc *DeduplicationCache) IsDuplicate(ctx context.Context, task *Task) (bool, error) {
	taskID := dc.generateTaskID(task)

	// First check memory cache (fastest)
	if dc.config.EnableCache {
		if dc.memoryCache.Get(taskID) != nil {
			atomic.AddInt64(&dc.metrics.MemoryHits, 1)
			return true, nil
		}
	}

	// Check persistent cache if enabled
	if dc.config.PersistentCache && dc.dbCache != nil {
		duplicate, err := dc.checkPersistentCache(ctx, task, taskID)
		if err != nil {
			log.Warn().Err(err).Str("taskID", taskID).Msg("Failed to check persistent cache")
			// Continue with memory-only check
		} else if duplicate {
			atomic.AddInt64(&dc.metrics.CacheHits, 1)
			// Add to memory cache for future fast access
			dc.memoryCache.Put(taskID, task)
			return true, nil
		} else {
			atomic.AddInt64(&dc.metrics.CacheMisses, 1)
		}
	}

	// Add to memory cache
	if dc.config.EnableCache {
		dc.memoryCache.Put(taskID, task)
	}

	// Store in persistent cache if enabled
	if dc.config.PersistentCache && dc.dbCache != nil {
		go func() {
			if err := dc.storeInPersistentCache(ctx, task, taskID); err != nil {
				log.Warn().Err(err).Str("taskID", taskID).Msg("Failed to store in persistent cache")
			}
		}()
	}

	return false, nil
}

// generateTaskID generates a content-addressable hash for the task
func (dc *DeduplicationCache) generateTaskID(task *Task) string {
	content := fmt.Sprintf("%v", task.Payload)
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:8])
}

// checkPersistentCache checks if task exists in persistent cache
func (dc *DeduplicationCache) checkPersistentCache(ctx context.Context, task *Task, taskID string) (bool, error) {
	// Create a trace for cache lookup
	trace := entities.Trace{
		Value: taskID,
		Type:  entities.TraceType("deduplication"),
	}

	// Check if we have cached results for this task
	results, err := dc.dbCache.Get(trace, "deduplication")
	if err != nil {
		return false, fmt.Errorf("failed to get from persistent cache: %w", err)
	}

	return len(results) > 0, nil
}

// storeInPersistentCache stores task in persistent cache
func (dc *DeduplicationCache) storeInPersistentCache(ctx context.Context, task *Task, taskID string) error {
	// Create a trace for cache storage
	trace := entities.Trace{
		Value: taskID,
		Type:  entities.TraceType("deduplication"),
	}

	// Store empty result to mark as processed
	results := []entities.Trace{}
	return dc.dbCache.Set(trace, "deduplication", results, dc.config.CacheTTL)
}

// GetMetrics returns current deduplication metrics
func (dc *DeduplicationCache) GetMetrics() *DeduplicationMetrics {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	// Calculate hit rate
	totalRequests := dc.metrics.MemoryHits + dc.metrics.CacheHits + dc.metrics.CacheMisses
	if totalRequests > 0 {
		dc.metrics.HitRate = float64(dc.metrics.MemoryHits+dc.metrics.CacheHits) / float64(totalRequests)
	}

	// Get memory cache metrics
	lruMetrics := dc.memoryCache.GetMetrics()
	dc.metrics.Evictions = lruMetrics.Evictions
	dc.metrics.MemoryUsage = lruMetrics.Size

	return dc.metrics
}

// cleanupRoutine periodically cleans up expired entries
func (dc *DeduplicationCache) cleanupRoutine() {
	for {
		select {
		case <-dc.cleanupTicker.C:
			dc.cleanup()
		case <-dc.ctx.Done():
			return
		}
	}
}

// cleanup removes expired entries from persistent cache
func (dc *DeduplicationCache) cleanup() {
	if dc.config.PersistentCache && dc.dbCache != nil {
		if err := dc.dbCache.CleanExpired(); err != nil {
			log.Warn().Err(err).Msg("Failed to clean expired cache entries")
		}
	}
}

// Shutdown gracefully shuts down the deduplication cache
func (dc *DeduplicationCache) Shutdown() {
	dc.cancel()
	if dc.cleanupTicker != nil {
		dc.cleanupTicker.Stop()
	}
}

// LRU Cache Methods

// Get retrieves a value from the LRU cache
func (lru *LRUCache) Get(key string) interface{} {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	if element, exists := lru.cache[key]; exists {
		// Move to front (most recently used)
		lru.list.MoveToFront(element)
		atomic.AddInt64(&lru.metrics.Hits, 1)
		return element.Value
	}

	atomic.AddInt64(&lru.metrics.Misses, 1)
	return nil
}

// Put adds a value to the LRU cache
func (lru *LRUCache) Put(key string, value interface{}) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	// Check if key already exists
	if element, exists := lru.cache[key]; exists {
		// Update value and move to front
		element.Value = value
		lru.list.MoveToFront(element)
		return
	}

	// Add new element to front
	element := lru.list.PushFront(value)
	lru.cache[key] = element
	atomic.AddInt64(&lru.metrics.Size, 1)

	// Check if we need to evict
	if lru.list.Len() > lru.maxSize {
		// Remove least recently used element
		back := lru.list.Back()
		if back != nil {
			lru.list.Remove(back)
			// Remove from map (we need to find the key)
			for k, v := range lru.cache {
				if v == back {
					delete(lru.cache, k)
					break
				}
			}
			atomic.AddInt64(&lru.metrics.Evictions, 1)
			atomic.AddInt64(&lru.metrics.Size, -1)
		}
	}
}

// GetMetrics returns LRU cache metrics
func (lru *LRUCache) GetMetrics() *LRUMetrics {
	return lru.metrics
}

// Size returns the current size of the LRU cache
func (lru *LRUCache) Size() int {
	lru.mutex.RLock()
	defer lru.mutex.RUnlock()
	return lru.list.Len()
}

// Clear removes all entries from the LRU cache
func (lru *LRUCache) Clear() {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()
	lru.cache = make(map[string]*list.Element)
	lru.list.Init()
	atomic.StoreInt64(&lru.metrics.Size, 0)
}
