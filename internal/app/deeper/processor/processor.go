package processor

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/errors"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

// Processor handles trace processing through plugins
type Processor struct {
	config  *config.Config
	metrics *metrics.MetricsCollector
	repo    *database.Repository
	cache   *database.Cache
}

// NewProcessor creates a new trace processor
func NewProcessor(cfg *config.Config, metricsCollector *metrics.MetricsCollector, repo *database.Repository, cache *database.Cache) *Processor {
	return &Processor{
		config:  cfg,
		metrics: metricsCollector,
		repo:    repo,
		cache:   cache,
	}
}

// ProcessTrace processes a single trace through all applicable plugins
func (p *Processor) ProcessTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	startTime := time.Now()

	plugins, exists := state.ActivePlugins[trace.Type]
	if !exists || len(plugins) == 0 {
		log.Debug().Msgf("No plugins found for trace type %s", trace.Type)
		// Record metrics for skipped trace
		p.metrics.RecordTraceTypeMetrics(trace.Type, false, 0, time.Since(startTime))
		return []entities.Trace{}, nil
	}

	// Use a semaphore to limit concurrency
	semaphore := make(chan struct{}, p.config.MaxConcurrency)

	// Use channels for thread-safe result collection
	resultChan := make(chan []entities.Trace, len(plugins))
	errorChan := make(chan error, len(plugins))

	var wg sync.WaitGroup

	// Process each plugin concurrently
	for _, plugin := range plugins {
		wg.Add(1)
		go func(plugin interface{}) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Type assertion to get the plugin interface
			pluginInterface, ok := plugin.(interface {
				FollowTrace(trace entities.Trace) ([]entities.Trace, error)
				String() string
			})
			if !ok {
				log.Error().Msgf("Plugin does not implement required interface")
				errorChan <- errors.NewPluginError("invalid plugin interface", nil)
				return
			}

			pluginStartTime := time.Now()
			log.Debug().Msgf("Processing trace %v with plugin %s", trace, pluginInterface.String())

			newTraces, err := pluginInterface.FollowTrace(trace)
			pluginDuration := time.Since(pluginStartTime)

			// Record plugin metrics
			p.metrics.RecordPluginExecution(pluginInterface.String(), pluginDuration, err == nil)

			if err != nil {
				log.Error().Err(err).Msgf("Plugin %s failed to process trace", pluginInterface.String())
				errorChan <- errors.NewPluginError("plugin processing failed", err).WithContext("plugin", pluginInterface.String())
				return
			}

			// Filter out empty traces
			var validTraces []entities.Trace
			for _, newTrace := range newTraces {
				if newTrace.Value != "" {
					validTraces = append(validTraces, newTrace)
				}
			}

			resultChan <- validTraces
		}(plugin)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	var allTraces []entities.Trace
	var allErrors []error

	// Collect traces
	for traces := range resultChan {
		allTraces = append(allTraces, traces...)
	}

	// Collect errors
	for err := range errorChan {
		allErrors = append(allErrors, err)
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
