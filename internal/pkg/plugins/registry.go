package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/errors"
)

// PluginStatus represents the health status of a plugin
type PluginStatus string

const (
	StatusHealthy     PluginStatus = "healthy"
	StatusUnhealthy   PluginStatus = "unhealthy"
	StatusUnavailable PluginStatus = "unavailable"
	StatusUnknown     PluginStatus = "unknown"
)

// PluginInfo contains metadata about a plugin
type PluginInfo struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	TraceTypes  []entities.TraceType   `json:"trace_types"`
	Status      PluginStatus           `json:"status"`
	LastCheck   time.Time              `json:"last_check"`
	Metadata    map[string]interface{} `json:"metadata"`
	ErrorCount  int                    `json:"error_count"`
	LastError   string                 `json:"last_error,omitempty"`
}

// PluginRegistry manages plugin lifecycle and health monitoring
type PluginRegistry struct {
	plugins      map[entities.TraceType][]DeeperPlugin
	pluginInfo   map[string]*PluginInfo
	healthChecks map[string]time.Time
	mu           sync.RWMutex

	// Health check configuration
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	stopCh              chan struct{}
	wg                  sync.WaitGroup
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins:             make(map[entities.TraceType][]DeeperPlugin),
		pluginInfo:          make(map[string]*PluginInfo),
		healthChecks:        make(map[string]time.Time),
		healthCheckInterval: 5 * time.Minute,
		healthCheckTimeout:  30 * time.Second,
		stopCh:              make(chan struct{}),
	}
}

// RegisterPlugin registers a plugin with the registry
func (r *PluginRegistry) RegisterPlugin(traceType entities.TraceType, plugin DeeperPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if plugin == nil {
		return errors.NewValidationError("plugin cannot be nil", nil)
	}

	pluginName := plugin.String()
	if pluginName == "" {
		return errors.NewValidationError("plugin must have a valid name", nil)
	}

	// Register the plugin
	r.plugins[traceType] = append(r.plugins[traceType], plugin)

	// Create plugin info
	if _, exists := r.pluginInfo[pluginName]; !exists {
		r.pluginInfo[pluginName] = &PluginInfo{
			Name:        pluginName,
			Version:     "1.0.0", // Default version
			Description: fmt.Sprintf("Plugin for processing %s traces", traceType),
			TraceTypes:  []entities.TraceType{traceType},
			Status:      StatusUnknown,
			Metadata:    make(map[string]interface{}),
		}
	} else {
		// Add trace type to existing plugin info
		info := r.pluginInfo[pluginName]
		found := false
		for _, tt := range info.TraceTypes {
			if tt == traceType {
				found = true
				break
			}
		}
		if !found {
			info.TraceTypes = append(info.TraceTypes, traceType)
		}
	}

	log.Info().Msgf("Registered plugin %s for trace type %s", pluginName, traceType)
	return nil
}

// GetPlugins returns all plugins for a given trace type
func (r *PluginRegistry) GetPlugins(traceType entities.TraceType) []DeeperPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins, exists := r.plugins[traceType]
	if !exists {
		return []DeeperPlugin{}
	}

	// Return a copy to prevent external modification
	result := make([]DeeperPlugin, len(plugins))
	copy(result, plugins)
	return result
}

// GetAllPlugins returns all registered plugins
func (r *PluginRegistry) GetAllPlugins() map[entities.TraceType][]DeeperPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[entities.TraceType][]DeeperPlugin)
	for traceType, plugins := range r.plugins {
		result[traceType] = make([]DeeperPlugin, len(plugins))
		copy(result[traceType], plugins)
	}
	return result
}

// GetPluginInfo returns information about a specific plugin
func (r *PluginRegistry) GetPluginInfo(pluginName string) (*PluginInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.pluginInfo[pluginName]
	if !exists {
		return nil, errors.NewValidationError("plugin not found", nil)
	}

	// Return a copy
	infoCopy := *info
	return &infoCopy, nil
}

// GetAllPluginInfo returns information about all plugins
func (r *PluginRegistry) GetAllPluginInfo() map[string]*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*PluginInfo)
	for name, info := range r.pluginInfo {
		infoCopy := *info
		result[name] = &infoCopy
	}
	return result
}

// StartHealthChecks begins periodic health checking of plugins
func (r *PluginRegistry) StartHealthChecks(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ticker := time.NewTicker(r.healthCheckInterval)
		defer ticker.Stop()

		// Initial health check
		r.performHealthChecks(ctx)

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("Stopping plugin health checks due to context cancellation")
				return
			case <-r.stopCh:
				log.Info().Msg("Stopping plugin health checks")
				return
			case <-ticker.C:
				r.performHealthChecks(ctx)
			}
		}
	}()
}

// StopHealthChecks stops the health checking process
func (r *PluginRegistry) StopHealthChecks() {
	close(r.stopCh)
	r.wg.Wait()
}

// performHealthChecks performs health checks on all registered plugins
func (r *PluginRegistry) performHealthChecks(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Debug().Msg("Starting plugin health checks")

	for traceType, plugins := range r.plugins {
		for _, plugin := range plugins {
			r.checkPluginHealth(ctx, traceType, plugin)
		}
	}

	log.Debug().Msg("Completed plugin health checks")
}

// checkPluginHealth performs a health check on a single plugin
func (r *PluginRegistry) checkPluginHealth(ctx context.Context, traceType entities.TraceType, plugin DeeperPlugin) {
	pluginName := plugin.String()

	// Create a timeout context for this health check
	healthCtx, cancel := context.WithTimeout(ctx, r.healthCheckTimeout)
	defer cancel()

	// Update last check time
	now := time.Now()
	r.healthChecks[pluginName] = now

	info, exists := r.pluginInfo[pluginName]
	if !exists {
		log.Warn().Msgf("No plugin info found for %s", pluginName)
		return
	}

	info.LastCheck = now

	// Perform health check by trying to process a safe test trace
	testTrace := entities.Trace{
		Value: "healthcheck",
		Type:  traceType,
	}

	// Use a goroutine to respect the timeout
	resultCh := make(chan error, 1)
	go func() {
		_, err := plugin.FollowTrace(testTrace)
		resultCh <- err
	}()

	select {
	case err := <-resultCh:
		if err != nil {
			info.Status = StatusUnhealthy
			info.ErrorCount++
			info.LastError = err.Error()
			log.Warn().Err(err).Msgf("Plugin %s health check failed", pluginName)
		} else {
			info.Status = StatusHealthy
			info.LastError = ""
			log.Debug().Msgf("Plugin %s health check passed", pluginName)
		}
	case <-healthCtx.Done():
		info.Status = StatusUnavailable
		info.ErrorCount++
		info.LastError = "Health check timeout"
		log.Warn().Msgf("Plugin %s health check timed out", pluginName)
	}
}

// GetHealthySummary returns a summary of plugin health
func (r *PluginRegistry) GetHealthySummary() map[PluginStatus]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	summary := map[PluginStatus]int{
		StatusHealthy:     0,
		StatusUnhealthy:   0,
		StatusUnavailable: 0,
		StatusUnknown:     0,
	}

	for _, info := range r.pluginInfo {
		summary[info.Status]++
	}

	return summary
}

// SetPluginMetadata sets metadata for a plugin
func (r *PluginRegistry) SetPluginMetadata(pluginName string, key string, value interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.pluginInfo[pluginName]
	if !exists {
		return errors.NewValidationError("plugin not found", nil)
	}

	if info.Metadata == nil {
		info.Metadata = make(map[string]interface{})
	}
	info.Metadata[key] = value

	return nil
}

// GetPluginCount returns the total number of registered plugins
func (r *PluginRegistry) GetPluginCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.pluginInfo)
}

// GetTraceTypeCount returns the number of supported trace types
func (r *PluginRegistry) GetTraceTypeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.plugins)
}
