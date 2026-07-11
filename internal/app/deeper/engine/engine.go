package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/app/deeper/processor"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
)

// Engine orchestrates the trace processing workflow
type Engine struct {
	config    *config.Config
	processor *processor.Processor
	metrics   *metrics.MetricsCollector
	repo      *database.Repository
}

// NewEngine creates a new trace processing engine
func NewEngine(cfg *config.Config, metricsCollector *metrics.MetricsCollector, repo *database.Repository, cache *database.Cache) *Engine {
	return &Engine{
		config:    cfg,
		processor: processor.NewProcessor(cfg, metricsCollector, repo, cache),
		metrics:   metricsCollector,
		repo:      repo,
	}
}

// ProcessInput processes an input string and returns all discovered traces
func (e *Engine) ProcessInput(ctx context.Context, input string, scanID int64) ([]entities.Trace, error) {
	initialTrace := entities.NewTrace(input)

	rootID, err := e.repo.GetOrCreateTrace(initialTrace)
	if err != nil {
		return nil, fmt.Errorf("failed to persist root trace: %w", err)
	}
	if err := e.repo.InsertEdge(&database.TraceEdge{
		ChildTraceID: rootID,
		PluginName:   database.SeedPluginName,
		ScanID:       scanID,
		DiscoveredAt: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("failed to persist seed edge: %w", err)
	}

	stack := []entities.Trace{initialTrace}
	// The seed is marked seen and included in results up front: previously
	// it was excluded from allTraces entirely (only plugin-discovered
	// children were ever appended), so a scan's own starting point never
	// appeared in its own results unless some plugin happened to
	// rediscover the identical (value, type) pair as a "new" child
	// elsewhere in the graph. Marking it seen here also prevents that kind
	// of rediscovery from re-queuing and reprocessing the seed a second time.
	seen := map[entities.Trace]bool{initialTrace: true}
	allTraces := []entities.Trace{initialTrace}

	var processedCount int
	var errorCount int

	for len(stack) > 0 {
		batchSize := min(len(stack), e.config.MaxConcurrency)
		batch := stack[:batchSize]
		stack = stack[batchSize:]

		discoveries, err := e.processBatch(ctx, batch)
		if err != nil {
			log.Error().Err(err).Msg("Failed to process batch")
			errorCount++
			continue
		}

		if err := e.repo.PersistDiscoveries(scanID, discoveries); err != nil {
			return nil, fmt.Errorf("failed to persist discoveries: %w", err)
		}

		for _, d := range discoveries {
			if !seen[d.Child] {
				seen[d.Child] = true
				allTraces = append(allTraces, d.Child)
				stack = append(stack, d.Child)
			}
		}

		processedCount += len(batch)
	}

	log.Info().Msgf("Processing complete. Processed %d traces, found %d unique traces, %d errors",
		processedCount, len(allTraces), errorCount)

	return allTraces, nil
}

// processBatch processes a batch of traces concurrently, bounded by MaxConcurrency
func (e *Engine) processBatch(ctx context.Context, traces []entities.Trace) ([]entities.Discovery, error) {
	var (
		allResults []entities.Discovery
		errors     []error
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	concurrency := e.config.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)

	for _, trace := range traces {
		wg.Add(1)
		go func(trace entities.Trace) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			results, err := e.processor.ProcessTrace(ctx, trace)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Error().Err(err).Msgf("Failed to process trace %v", trace)
				errors = append(errors, err)
				return
			}
			allResults = append(allResults, results...)
		}(trace)
	}

	wg.Wait()

	if len(errors) > 0 {
		log.Warn().Msgf("Encountered %d errors in batch processing", len(errors))
	}

	return allResults, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Shutdown gracefully shuts down the engine and its processor
func (e *Engine) Shutdown(timeout time.Duration) error {
	if e.processor != nil {
		return e.processor.Shutdown(timeout)
	}
	return nil
}

// GetWorkerPoolMetrics returns worker pool metrics from the processor
func (e *Engine) GetWorkerPoolMetrics() interface{} {
	if e.processor != nil {
		return e.processor.GetWorkerPoolMetrics()
	}
	return nil
}
