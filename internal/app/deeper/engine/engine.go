package engine

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/app/deeper/processor"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/worker"
)

// Engine orchestrates the trace processing workflow
type Engine struct {
	config    *config.Config
	processor *processor.Processor
	metrics   *metrics.MetricsCollector
	pool      *worker.Pool
}

// NewEngine creates a new trace processing engine
func NewEngine(
	cfg *config.Config,
	metricsCollector *metrics.MetricsCollector,
	repo *database.Repository,
	cache *database.Cache,
	pool *worker.Pool,
) *Engine {
	return &Engine{
		config:    cfg,
		processor: processor.NewProcessor(cfg, metricsCollector, repo, cache, pool),
		metrics:   metricsCollector,
		pool:      pool,
	}
}

// ProcessInput processes an input string and returns all discovered traces
func (e *Engine) ProcessInput(ctx context.Context, input string) ([]entities.Trace, error) {
	// Create initial trace from input
	initialTrace := entities.NewTrace(input)

	// Use a stack-based approach for breadth-first processing
	stack := []entities.Trace{initialTrace}
	seen := make(map[entities.Trace]bool)
	var allTraces []entities.Trace

	// Track processing statistics
	var processedCount int
	var errorCount int

	for len(stack) > 0 {
		// Process traces in batches to avoid memory issues
		batchSize := min(len(stack), e.config.MaxConcurrency)
		batch := stack[:batchSize]
		stack = stack[batchSize:]

		// Process batch concurrently
		results, err := e.processBatch(ctx, batch)
		if err != nil {
			log.Error().Err(err).Msg("Failed to process batch")
			errorCount++
			continue
		}

		// Add new traces to stack and results
		for _, trace := range results {
			if !seen[trace] {
				seen[trace] = true
				allTraces = append(allTraces, trace)
				stack = append(stack, trace)
			}
		}

		processedCount += len(batch)
	}

	log.Info().Msgf("Processing complete. Processed %d traces, found %d unique traces, %d errors",
		processedCount, len(allTraces), errorCount)

	return allTraces, nil
}

// processBatch processes a batch of traces concurrently
func (e *Engine) processBatch(ctx context.Context, traces []entities.Trace) ([]entities.Trace, error) {
	var wg sync.WaitGroup
	resultChan := make(chan []entities.Trace, len(traces))
	errorChan := make(chan error, len(traces))

	wg.Add(len(traces))

	for _, trace := range traces {
		job := worker.Job{
			ID: trace.Value,
			Execute: func(ctx context.Context) (interface{}, error) {
				return e.processor.ProcessTrace(ctx, trace)
			},
			Callback: func(result interface{}, err error) {
				defer wg.Done()
				if err != nil {
					log.Error().Err(err).Msgf("Failed to process trace %v", trace)
					errorChan <- err
					return
				}
				resultChan <- result.([]entities.Trace)
			},
		}
		e.pool.Submit(job)
	}

	// Wait for all jobs to complete
	wg.Wait()
	close(resultChan)
	close(errorChan)

	// Collect results
	var allResults []entities.Trace
	var allErrors []error

	for results := range resultChan {
		allResults = append(allResults, results...)
	}

	for err := range errorChan {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		log.Warn().Msgf("Encountered %d errors in batch processing", len(allErrors))
	}

	return allResults, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
