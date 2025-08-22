package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

// MetricsCollector collects and stores application metrics
type MetricsCollector struct {
	// Counters
	tracesProcessed  uint64
	tracesDiscovered uint64
	pluginExecutions uint64
	pluginErrors     uint64
	networkRequests  uint64
	networkErrors    uint64

	// Histograms and timing data
	processingTimes     []time.Duration
	pluginResponseTimes map[string][]time.Duration

	// Trace type metrics
	traceTypeMetrics map[entities.TraceType]*TraceTypeMetrics

	// Plugin metrics
	pluginMetrics map[string]*PluginMetrics

	// Mutex for complex data structures
	mu sync.RWMutex

	// Start time for uptime calculation
	startTime time.Time
}

// TraceTypeMetrics holds metrics for a specific trace type
type TraceTypeMetrics struct {
	Processed   uint64
	Discovered  uint64
	SuccessRate float64
	AvgTime     time.Duration
}

// PluginMetrics holds metrics for a specific plugin
type PluginMetrics struct {
	Name          string
	Executions    uint64
	Errors        uint64
	TotalTime     time.Duration
	AvgTime       time.Duration
	SuccessRate   float64
	LastExecution time.Time
}

// Summary provides a comprehensive metrics summary
type Summary struct {
	Uptime            time.Duration                            `json:"uptime"`
	TracesProcessed   uint64                                   `json:"traces_processed"`
	TracesDiscovered  uint64                                   `json:"traces_discovered"`
	PluginExecutions  uint64                                   `json:"plugin_executions"`
	PluginErrors      uint64                                   `json:"plugin_errors"`
	NetworkRequests   uint64                                   `json:"network_requests"`
	NetworkErrors     uint64                                   `json:"network_errors"`
	SuccessRate       float64                                  `json:"success_rate"`
	AvgProcessingTime time.Duration                            `json:"avg_processing_time"`
	TraceTypes        map[entities.TraceType]*TraceTypeMetrics `json:"trace_types"`
	Plugins           map[string]*PluginMetrics                `json:"plugins"`
	RequestsPerSecond float64                                  `json:"requests_per_second"`
	ErrorRate         float64                                  `json:"error_rate"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		pluginResponseTimes: make(map[string][]time.Duration),
		traceTypeMetrics:    make(map[entities.TraceType]*TraceTypeMetrics),
		pluginMetrics:       make(map[string]*PluginMetrics),
		startTime:           time.Now(),
	}
}

// Global metrics collector instance
var globalMetrics = NewMetricsCollector()

// GetGlobalMetrics returns the global metrics collector
func GetGlobalMetrics() *MetricsCollector {
	return globalMetrics
}

// IncrementTracesProcessed increments the processed traces counter
func (m *MetricsCollector) IncrementTracesProcessed() {
	atomic.AddUint64(&m.tracesProcessed, 1)
}

// IncrementTracesDiscovered increments the discovered traces counter
func (m *MetricsCollector) IncrementTracesDiscovered() {
	atomic.AddUint64(&m.tracesDiscovered, 1)
}

// IncrementPluginExecutions increments the plugin executions counter
func (m *MetricsCollector) IncrementPluginExecutions() {
	atomic.AddUint64(&m.pluginExecutions, 1)
}

// IncrementPluginErrors increments the plugin errors counter
func (m *MetricsCollector) IncrementPluginErrors() {
	atomic.AddUint64(&m.pluginErrors, 1)
}

// IncrementNetworkRequests increments the network requests counter
func (m *MetricsCollector) IncrementNetworkRequests() {
	atomic.AddUint64(&m.networkRequests, 1)
}

// IncrementNetworkErrors increments the network errors counter
func (m *MetricsCollector) IncrementNetworkErrors() {
	atomic.AddUint64(&m.networkErrors, 1)
}

// RecordProcessingTime records a processing time measurement
func (m *MetricsCollector) RecordProcessingTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processingTimes = append(m.processingTimes, duration)

	// Keep only last 1000 measurements to prevent memory growth
	if len(m.processingTimes) > 1000 {
		m.processingTimes = m.processingTimes[len(m.processingTimes)-1000:]
	}
}

// RecordPluginExecution records metrics for a plugin execution
func (m *MetricsCollector) RecordPluginExecution(pluginName string, duration time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize plugin metrics if not exists
	if _, exists := m.pluginMetrics[pluginName]; !exists {
		m.pluginMetrics[pluginName] = &PluginMetrics{
			Name: pluginName,
		}
	}

	plugin := m.pluginMetrics[pluginName]
	plugin.Executions++
	plugin.TotalTime += duration
	plugin.AvgTime = time.Duration(int64(plugin.TotalTime) / int64(plugin.Executions))
	plugin.LastExecution = time.Now()

	if !success {
		plugin.Errors++
	}

	plugin.SuccessRate = float64(plugin.Executions-plugin.Errors) / float64(plugin.Executions) * 100

	// Record response time
	if _, exists := m.pluginResponseTimes[pluginName]; !exists {
		m.pluginResponseTimes[pluginName] = make([]time.Duration, 0)
	}
	m.pluginResponseTimes[pluginName] = append(m.pluginResponseTimes[pluginName], duration)

	// Keep only last 100 measurements per plugin
	if len(m.pluginResponseTimes[pluginName]) > 100 {
		m.pluginResponseTimes[pluginName] = m.pluginResponseTimes[pluginName][len(m.pluginResponseTimes[pluginName])-100:]
	}
}

// RecordTraceTypeMetrics records metrics for a trace type
func (m *MetricsCollector) RecordTraceTypeMetrics(traceType entities.TraceType, processed bool, discovered int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize trace type metrics if not exists
	if _, exists := m.traceTypeMetrics[traceType]; !exists {
		m.traceTypeMetrics[traceType] = &TraceTypeMetrics{}
	}

	metrics := m.traceTypeMetrics[traceType]

	if processed {
		metrics.Processed++
	}

	metrics.Discovered += uint64(discovered)

	// Update average time (simple moving average)
	if metrics.Processed > 0 {
		metrics.AvgTime = time.Duration((int64(metrics.AvgTime)*int64(metrics.Processed-1) + int64(duration)) / int64(metrics.Processed))
	} else {
		metrics.AvgTime = duration
	}
}

// GetSummary returns a comprehensive metrics summary
func (m *MetricsCollector) GetSummary() *Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.startTime)

	// Calculate success rate
	var successRate float64
	if m.pluginExecutions > 0 {
		successRate = float64(m.pluginExecutions-m.pluginErrors) / float64(m.pluginExecutions) * 100
	}

	// Calculate average processing time
	var avgProcessingTime time.Duration
	if len(m.processingTimes) > 0 {
		var total int64
		for _, t := range m.processingTimes {
			total += int64(t)
		}
		avgProcessingTime = time.Duration(total / int64(len(m.processingTimes)))
	}

	// Calculate requests per second
	var requestsPerSecond float64
	if uptime.Seconds() > 0 {
		requestsPerSecond = float64(m.networkRequests) / uptime.Seconds()
	}

	// Calculate error rate
	var errorRate float64
	if m.networkRequests > 0 {
		errorRate = float64(m.networkErrors) / float64(m.networkRequests) * 100
	}

	// Copy trace type metrics
	traceTypes := make(map[entities.TraceType]*TraceTypeMetrics)
	for tt, metrics := range m.traceTypeMetrics {
		traceTypes[tt] = &TraceTypeMetrics{
			Processed:   metrics.Processed,
			Discovered:  metrics.Discovered,
			SuccessRate: metrics.SuccessRate,
			AvgTime:     metrics.AvgTime,
		}
	}

	// Copy plugin metrics
	plugins := make(map[string]*PluginMetrics)
	for name, metrics := range m.pluginMetrics {
		plugins[name] = &PluginMetrics{
			Name:          metrics.Name,
			Executions:    metrics.Executions,
			Errors:        metrics.Errors,
			TotalTime:     metrics.TotalTime,
			AvgTime:       metrics.AvgTime,
			SuccessRate:   metrics.SuccessRate,
			LastExecution: metrics.LastExecution,
		}
	}

	return &Summary{
		Uptime:            uptime,
		TracesProcessed:   atomic.LoadUint64(&m.tracesProcessed),
		TracesDiscovered:  atomic.LoadUint64(&m.tracesDiscovered),
		PluginExecutions:  atomic.LoadUint64(&m.pluginExecutions),
		PluginErrors:      atomic.LoadUint64(&m.pluginErrors),
		NetworkRequests:   atomic.LoadUint64(&m.networkRequests),
		NetworkErrors:     atomic.LoadUint64(&m.networkErrors),
		SuccessRate:       successRate,
		AvgProcessingTime: avgProcessingTime,
		TraceTypes:        traceTypes,
		Plugins:           plugins,
		RequestsPerSecond: requestsPerSecond,
		ErrorRate:         errorRate,
	}
}

// Reset clears all metrics (useful for testing)
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.StoreUint64(&m.tracesProcessed, 0)
	atomic.StoreUint64(&m.tracesDiscovered, 0)
	atomic.StoreUint64(&m.pluginExecutions, 0)
	atomic.StoreUint64(&m.pluginErrors, 0)
	atomic.StoreUint64(&m.networkRequests, 0)
	atomic.StoreUint64(&m.networkErrors, 0)

	m.processingTimes = make([]time.Duration, 0)
	m.pluginResponseTimes = make(map[string][]time.Duration)
	m.traceTypeMetrics = make(map[entities.TraceType]*TraceTypeMetrics)
	m.pluginMetrics = make(map[string]*PluginMetrics)
	m.startTime = time.Now()
}

// GetPluginMetrics returns metrics for a specific plugin
func (m *MetricsCollector) GetPluginMetrics(pluginName string) (*PluginMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.pluginMetrics[pluginName]
	if !exists {
		return nil, false
	}

	// Return a copy
	return &PluginMetrics{
		Name:          metrics.Name,
		Executions:    metrics.Executions,
		Errors:        metrics.Errors,
		TotalTime:     metrics.TotalTime,
		AvgTime:       metrics.AvgTime,
		SuccessRate:   metrics.SuccessRate,
		LastExecution: metrics.LastExecution,
	}, true
}

// GetTraceTypeMetrics returns metrics for a specific trace type
func (m *MetricsCollector) GetTraceTypeMetrics(traceType entities.TraceType) (*TraceTypeMetrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.traceTypeMetrics[traceType]
	if !exists {
		return nil, false
	}

	// Return a copy
	return &TraceTypeMetrics{
		Processed:   metrics.Processed,
		Discovered:  metrics.Discovered,
		SuccessRate: metrics.SuccessRate,
		AvgTime:     metrics.AvgTime,
	}, true
}
