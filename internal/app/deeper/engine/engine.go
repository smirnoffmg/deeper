package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/app/deeper/processor"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"

	"golang.org/x/sync/errgroup"
)

// Engine orchestrates the trace processing workflow
type Engine struct {
	config    *config.Config
	processor *processor.Processor
	metrics   *metrics.MetricsCollector
	repo      *database.Repository
}

// NewEngine creates a new trace processing engine
func NewEngine(cfg *config.Config, registry plugins.Registry, metricsCollector *metrics.MetricsCollector, repo *database.Repository, cache *database.Cache) *Engine {
	return &Engine{
		config:    cfg,
		processor: processor.NewProcessor(cfg, registry, metricsCollector, repo, cache),
		metrics:   metricsCollector,
		repo:      repo,
	}
}

// ProcessInput processes an input string and returns all discovered traces
func (e *Engine) ProcessInput(ctx context.Context, input string, scanID int64) ([]entities.Trace, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("scan input must not be empty")
	}

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
		Strength:     entities.StrengthProof,
		Basis:        "seed",
	}); err != nil {
		return nil, fmt.Errorf("failed to persist seed edge: %w", err)
	}

	queue := []entities.Trace{initialTrace}
	// The seed is registered as queued and included in results up front: previously
	// it was excluded from allTraces entirely (only plugin-discovered
	// children were ever appended), so a scan's own starting point never
	// appeared in its own results unless some plugin happened to
	// rediscover the identical (value, type) pair as a "new" child
	// elsewhere in the graph. Registering it here also prevents that kind
	// of rediscovery from re-queuing and reprocessing the seed a second time.
	nodes := map[entities.Trace]*nodeState{initialTrace: {strength: entities.StrengthProof, queued: true}}
	children := map[entities.Trace][]entities.Discovery{}
	allTraces := []entities.Trace{initialTrace}

	var processedCount int

	for len(queue) > 0 {
		batchSize := min(len(queue), max(e.config.MaxConcurrency, 1))
		batch := queue[:batchSize]
		queue = queue[batchSize:]

		discoveries := e.processBatch(ctx, batch)

		if err := e.repo.PersistDiscoveries(scanID, discoveries); err != nil {
			return nil, fmt.Errorf("failed to persist discoveries: %w", err)
		}

		for _, d := range discoveries {
			if _, known := nodes[d.Child]; !known {
				allTraces = append(allTraces, d.Child)
			}
			queue = append(queue, admit(nodes, children, d)...)
		}

		processedCount += len(batch)
	}

	log.Info().Msgf("Processing complete. Processed %d traces, found %d unique traces",
		processedCount, len(allTraces))

	return allTraces, nil
}

// processBatch processes a batch of traces concurrently, bounded by
// MaxConcurrency. A trace that fails is logged and skipped so the rest of the
// batch still contributes to the graph.
func (e *Engine) processBatch(ctx context.Context, traces []entities.Trace) []entities.Discovery {
	concurrency := e.config.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	var (
		allResults []entities.Discovery
		errCount   atomic.Int64
		mu         sync.Mutex
		g          errgroup.Group
	)
	g.SetLimit(concurrency)

	for _, trace := range traces {
		// A per-trace failure is logged and skipped, not propagated as a group
		// error: one bad trace must not cancel the rest of the batch.
		g.Go(func() error {
			results, err := e.processor.ProcessTrace(ctx, trace)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to process trace %v", trace)
				errCount.Add(1)
				return nil
			}
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()

	if n := errCount.Load(); n > 0 {
		log.Warn().Msgf("Encountered %d errors in batch processing", n)
	}

	return allResults
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

type nodeState struct {
	strength entities.Strength
	queued   bool
}

func rank(s entities.Strength) int {
	switch s {
	case entities.StrengthProof:
		return 2
	case entities.StrengthStrong:
		return 1
	default:
		return 0
	}
}

// admit applies d to nodes and returns the traces that newly become enqueueable.
func admit(nodes map[entities.Trace]*nodeState, children map[entities.Trace][]entities.Discovery, d entities.Discovery) []entities.Trace {
	children[d.Parent] = append(children[d.Parent], d)
	return propagate(nodes, children, d)
}

func propagate(nodes map[entities.Trace]*nodeState, children map[entities.Trace][]entities.Discovery, d entities.Discovery) []entities.Trace {
	parent := nodes[d.Parent]
	s := d.Strength
	if rank(parent.strength) < rank(s) {
		s = parent.strength
	}

	var enqueued []entities.Trace
	promoted := false
	child, known := nodes[d.Child]
	switch {
	case !known:
		child = &nodeState{strength: s}
		nodes[d.Child] = child
	case rank(s) > rank(child.strength):
		child.strength = s
		promoted = true
	}

	if !child.queued && parent.strength != entities.StrengthWeak {
		child.queued = true
		enqueued = append(enqueued, d.Child)
	}

	if promoted {
		for _, cd := range children[d.Child] {
			enqueued = append(enqueued, propagate(nodes, children, cd)...)
		}
	}
	return enqueued
}
