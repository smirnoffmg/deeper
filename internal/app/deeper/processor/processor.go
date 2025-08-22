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
	"github.com/smirnoffmg/deeper/internal/pkg/worker"
)

// Processor handles trace processing through plugins
type Processor struct {
	config  *config.Config
	metrics *metrics.MetricsCollector
	repo    *database.Repository
	cache   *database.Cache
	pool    *worker.Pool
}

// NewProcessor creates a new trace processor
func NewProcessor(
	cfg *config.Config,
	metricsCollector *metrics.MetricsCollector,
	repo *database.Repository,
	cache *database.Cache,
	pool *worker.Pool,
) *Processor {
	return &Processor{
		config:  cfg,
		metrics: metricsCollector,
		repo:    repo,
		cache:   cache,
		pool:    pool,
	}
}

// ProcessTrace processes a single trace through all applicable plugins
func (p *Processor) ProcessTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	startTime := time.Now()

	plugins, exists := state.ActivePlugins[trace.Type]
	if !exists || len(plugins) == 0 {
		log.Debug().Msgf("No plugins found for trace type %s", trace.Type)
		p.metrics.RecordTraceTypeMetrics(trace.Type, false, 0, time.Since(startTime))
		return []entities.Trace{}, nil
	}

	resultChan := make(chan []entities.Trace, len(plugins))
	errorChan := make(chan error, len(plugins))
	var wg sync.WaitGroup
	wg.Add(len(plugins))

	for _, plugin := range plugins {
		job := p.pool.GetJob()
		job.ID = trace.Value
		job.Execute = func(ctx context.Context) (interface{}, error) {
			pluginInterface, ok := plugin.(interface {
				FollowTrace(trace entities.Trace) ([]entities.Trace, error)
				String() string
			})
			if !ok {
				return nil, errors.NewPluginError("invalid plugin interface", nil)
			}

			pluginStartTime := time.Now()
			log.Debug().Msgf("Processing trace %v with plugin %s", trace, pluginInterface.String())
			newTraces, err := pluginInterface.FollowTrace(trace)
			pluginDuration := time.Since(pluginStartTime)
			p.metrics.RecordPluginExecution(pluginInterface.String(), pluginDuration, err == nil)

			if err != nil {
				return nil, errors.NewPluginError("plugin processing failed", err).WithContext("plugin", pluginInterface.String())
			}

			var validTraces []entities.Trace
			for _, newTrace := range newTraces {
				if newTrace.Value != "" {
					validTraces = append(validTraces, newTrace)
				}
			}
			return validTraces, nil
		}
		job.Callback = func(result interface{}, err error) {
			defer wg.Done()
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- result.([]entities.Trace)
		}
		p.pool.Submit(job)
	}

	wg.Wait()
	close(resultChan)
	close(errorChan)

	var allTraces []entities.Trace
	var allErrors []error

	for traces := range resultChan {
		allTraces = append(allTraces, traces...)
	}

	for err := range errorChan {
		allErrors = append(allErrors, err)
	}

	totalDuration := time.Since(startTime)
	p.metrics.RecordProcessingTime(totalDuration)
	p.metrics.RecordTraceTypeMetrics(trace.Type, true, len(allTraces), totalDuration)
	p.metrics.IncrementTracesProcessed()
	p.metrics.IncrementTracesDiscovered()

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
