package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

// Cache provides caching functionality for plugin results
type Cache struct {
	repo *Repository
}

// NewCache creates a new cache instance
func NewCache(repo *Repository) *Cache {
	return &Cache{repo: repo}
}

// CacheKey generates a cache key for a trace and plugin
func (c *Cache) CacheKey(trace entities.Trace, pluginName string) string {
	data := fmt.Sprintf("%s:%s:%s", trace.Value, trace.Type, pluginName)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Get retrieves cached results for a trace and plugin
func (c *Cache) Get(trace entities.Trace, pluginName string) ([]entities.Trace, error) {
	key := c.CacheKey(trace, pluginName)

	entry, err := c.repo.GetCacheEntry(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}

	if entry == nil {
		return nil, nil // Cache miss
	}

	// Parse cached traces
	var traces []entities.Trace
	if err := json.Unmarshal([]byte(entry.Value), &traces); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached traces: %w", err)
	}

	return traces, nil
}

// Set stores results in cache for a trace and plugin
func (c *Cache) Set(trace entities.Trace, pluginName string, results []entities.Trace, ttl time.Duration) error {
	key := c.CacheKey(trace, pluginName)

	// Marshal results to JSON
	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	// Calculate expiration time
	var expiresAt *time.Time
	if ttl > 0 {
		exp := time.Now().Add(ttl)
		expiresAt = &exp
	}

	entry := &CacheEntry{
		Key:        key,
		Value:      string(data),
		CreatedAt:  time.Now(),
		ExpiresAt:  expiresAt,
		PluginName: pluginName,
	}

	return c.repo.StoreCacheEntry(entry)
}

// Invalidate removes cache entries for a specific plugin
func (c *Cache) Invalidate(pluginName string) error {
	// This would require a new method in the repository
	// For now, we'll clean expired entries
	return c.repo.CleanExpiredCache()
}

// CleanExpired removes expired cache entries
func (c *Cache) CleanExpired() error {
	return c.repo.CleanExpiredCache()
}

// GetStats returns cache statistics
func (c *Cache) GetStats() (*CacheStats, error) {
	// This would require additional repository methods
	// For now, return basic stats
	return &CacheStats{
		TotalEntries:   0,
		ExpiredEntries: 0,
		ValidEntries:   0,
		OldestEntry:    time.Now(),
		NewestEntry:    time.Now(),
	}, nil
}

// DefaultTTL returns the default TTL for cache entries
func (c *Cache) DefaultTTL() time.Duration {
	return 24 * time.Hour // 24 hours default
}

// Plugin-specific TTLs
func (c *Cache) GetTTLForPlugin(pluginName string) time.Duration {
	// Define different TTLs for different plugins
	switch pluginName {
	case "SubdomainPlugin":
		return 12 * time.Hour // Subdomain data changes less frequently
	case "CodeRepositoriesPlugin":
		return 6 * time.Hour // Code repos change more frequently
	case "SocialProfilesPlugin":
		return 1 * time.Hour // Social profiles change frequently
	default:
		return c.DefaultTTL()
	}
}
