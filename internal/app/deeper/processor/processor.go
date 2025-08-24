package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/app/deeper/processor/tasks"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/errors"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/smirnoffmg/deeper/internal/pkg/workerpool"
	"golang.org/x/time/rate"
)

// Processor handles trace processing through plugins
type Processor struct {
	config     *config.Config
	metrics    *metrics.MetricsCollector
	repo       *database.Repository
	cache      *database.Cache
	workerPool *workerpool.WorkerPool
}

// NewProcessor creates a new trace processor
func NewProcessor(cfg *config.Config, metricsCollector *metrics.MetricsCollector, repo *database.Repository, cache *database.Cache) *Processor {
	// Create worker pool configuration
	wpConfig := &workerpool.Config{
		MaxWorkers:          cfg.WorkerPoolConfig.MaxWorkers,
		QueueSize:           cfg.WorkerPoolConfig.QueueSize,
		DefaultRateLimit:    rate.Limit(cfg.WorkerPoolConfig.DefaultRateLimit),
		DefaultBurst:        cfg.WorkerPoolConfig.DefaultBurst,
		TaskTimeout:         cfg.WorkerPoolConfig.TaskTimeout,
		EnableDeduplication: cfg.WorkerPoolConfig.EnableDeduplication,
		EnableMetrics:       cfg.WorkerPoolConfig.EnableMetrics,
		CircuitBreakerConfig: workerpool.CircuitBreakerConfig{
			FailureThreshold: cfg.WorkerPoolConfig.CircuitBreakerConfig.FailureThreshold,
			RecoveryTimeout:  cfg.WorkerPoolConfig.CircuitBreakerConfig.RecoveryTimeout,
			HalfOpenMaxCalls: cfg.WorkerPoolConfig.CircuitBreakerConfig.HalfOpenMaxCalls,
			WindowSize:       cfg.WorkerPoolConfig.CircuitBreakerConfig.WindowSize,
		},
	}

	workerPool := workerpool.NewWorkerPool(wpConfig)

	// Initialize deduplication cache if enabled
	if cfg.WorkerPoolConfig.EnableDeduplication {
		dedupConfig := &workerpool.DeduplicationConfig{
			EnableCache:     cfg.WorkerPoolConfig.DeduplicationConfig.EnableCache,
			CacheTTL:        cfg.WorkerPoolConfig.DeduplicationConfig.CacheTTL,
			MaxMemorySize:   cfg.WorkerPoolConfig.DeduplicationConfig.MaxMemorySize,
			EnableMetrics:   cfg.WorkerPoolConfig.DeduplicationConfig.EnableMetrics,
			CleanupInterval: cfg.WorkerPoolConfig.DeduplicationConfig.CleanupInterval,
			PersistentCache: cfg.WorkerPoolConfig.DeduplicationConfig.PersistentCache,
		}
		dedupCache := workerpool.NewDeduplicationCache(dedupConfig, cache)
		workerPool.SetDeduplicationCache(dedupCache)
	}

	return &Processor{
		config:     cfg,
		metrics:    metricsCollector,
		repo:       repo,
		cache:      cache,
		workerPool: workerPool,
	}
}

// ProcessTrace processes a single trace through all applicable plugins using worker pool
func (p *Processor) ProcessTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	startTime := time.Now()

	plugins, exists := state.ActivePlugins[trace.Type]
	if !exists || len(plugins) == 0 {
		log.Debug().Msgf("No plugins found for trace type %s", trace.Type)
		// Record metrics for skipped trace
		p.metrics.RecordTraceTypeMetrics(trace.Type, false, 0, time.Since(startTime))
		return []entities.Trace{}, nil
	}

	// Create tasks for each plugin
	var allTraces []entities.Trace
	var allErrors []error

	// Submit tasks to worker pool
	for _, plugin := range plugins {
		pluginInterface, ok := plugin.(interface {
			FollowTrace(trace entities.Trace) ([]entities.Trace, error)
			String() string
		})
		if !ok {
			log.Error().Msgf("Plugin does not implement required interface")
			allErrors = append(allErrors, errors.NewPluginError("invalid plugin interface", nil))
			continue
		}

		// Create task for this plugin
		task := &workerpool.Task{
			ID: trace.Value + ":" + pluginInterface.String(),
			Payload: &tasks.TraceProcessingTask{
				Trace:     trace,
				PluginKey: pluginInterface.String(),
				Plugin:    pluginInterface,
			},
		}

		// Submit task to worker pool
		err := p.workerPool.Submit(ctx, task)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to submit task for plugin %s", pluginInterface.String())
			allErrors = append(allErrors, err)
			continue
		}
	}

	// Collect results from worker pool
	for i := 0; i < len(plugins); i++ {
		result, err := p.workerPool.GetResult(ctx)
		if err != nil {
			if err == context.Canceled {
				return allTraces, ctx.Err()
			}
			allErrors = append(allErrors, err)
			continue
		}

		if result.Error != nil {
			allErrors = append(allErrors, result.Error)
		} else {
			// Extract traces from result
			if taskPayload, ok := result.Result.(*tasks.TraceProcessingTask); ok {
				pluginInterface := taskPayload.Plugin.(interface {
					FollowTrace(trace entities.Trace) ([]entities.Trace, error)
					String() string
				})

				pluginStartTime := time.Now()
				newTraces, err := pluginInterface.FollowTrace(taskPayload.Trace)
				pluginDuration := time.Since(pluginStartTime)

				// Record plugin metrics
				p.metrics.RecordPluginExecution(pluginInterface.String(), pluginDuration, err == nil)

				if err != nil {
					log.Error().Err(err).Msgf("Plugin %s failed to process trace", pluginInterface.String())
					allErrors = append(allErrors, errors.NewPluginError("plugin processing failed", err).WithContext("plugin", pluginInterface.String()))
				} else {
					// Filter out empty traces
					for _, newTrace := range newTraces {
						if newTrace.Value != "" {
							allTraces = append(allTraces, newTrace)
						}
					}
				}
			}
		}
	}

	// Record final metrics
	totalDuration := time.Since(startTime)
	p.metrics.RecordProcessingTime(totalDuration)
	p.metrics.RecordTraceTypeMetrics(trace.Type, true, len(allTraces), totalDuration)
	p.metrics.IncrementTracesProcessed()
	p.metrics.IncrementTracesDiscovered()

	// If there were any errors, log them but don't fail the entire operation
	if len(allErrors) > 0 {
		log.Warn().Msgf("Encountered %d errors during trace processing", len(allErrors))
		for _, err := range allErrors {
			log.Error().Err(err).Msg("Trace processing error")
		}
	}

	return allTraces, nil
}

// ProcessTraces processes multiple traces
func (p *Processor) ProcessTraces(ctx context.Context, traces []entities.Trace) ([]entities.Trace, error) {
	var allResults []entities.Trace

	for _, trace := range traces {
		results, err := p.ProcessTrace(ctx, trace)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to process trace %v", trace)
			continue
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// Shutdown gracefully shuts down the processor and its worker pool
func (p *Processor) Shutdown(timeout time.Duration) error {
	if p.workerPool != nil {
		return p.workerPool.Shutdown(timeout)
	}
	return nil
}

// GetWorkerPoolMetrics returns worker pool metrics
func (p *Processor) GetWorkerPoolMetrics() *workerpool.Metrics {
	if p.workerPool != nil {
		return p.workerPool.GetMetrics()
	}
	return nil
}

// ConfigureDomainRateLimit configures rate limiting for a specific domain
func (p *Processor) ConfigureDomainRateLimit(domain string, rateLimit float64, burst int, backoffBase, backoffMax time.Duration, maxRetries int) error {
	if p.workerPool != nil {
		config := &workerpool.DomainRateConfig{
			Domain:      domain,
			RateLimit:   rateLimit,
			Burst:       burst,
			BackoffBase: backoffBase,
			BackoffMax:  backoffMax,
			MaxRetries:  maxRetries,
		}
		return p.workerPool.ConfigureDomainRateLimit(config)
	}
	return fmt.Errorf("worker pool not initialized")
}
