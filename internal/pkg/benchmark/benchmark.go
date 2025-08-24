package benchmark

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/workerpool"
	"golang.org/x/time/rate"
)

// BenchmarkResult holds benchmark test results
type BenchmarkResult struct {
	TestName          string
	Duration          time.Duration
	TracesProcessed   int
	TracesDiscovered  int
	Errors            int
	MemoryUsage       uint64
	WorkerPoolMetrics *workerpool.Metrics
	Throughput        float64 // traces per second
	ErrorRate         float64 // percentage of errors
}

// BenchmarkSuite runs performance benchmarks
type BenchmarkSuite struct {
	config *config.Config
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(cfg *config.Config) *BenchmarkSuite {
	return &BenchmarkSuite{
		config: cfg,
	}
}

// RunWorkerPoolBenchmark benchmarks the worker pool implementation
func (bs *BenchmarkSuite) RunWorkerPoolBenchmark(ctx context.Context, numTraces int) (*BenchmarkResult, error) {
	startTime := time.Now()

	// Create worker pool configuration
	wpConfig := &workerpool.Config{
		MaxWorkers:          bs.config.WorkerPoolConfig.MaxWorkers,
		QueueSize:           bs.config.WorkerPoolConfig.QueueSize,
		DefaultRateLimit:    rate.Limit(bs.config.WorkerPoolConfig.DefaultRateLimit),
		DefaultBurst:        bs.config.WorkerPoolConfig.DefaultBurst,
		TaskTimeout:         bs.config.WorkerPoolConfig.TaskTimeout,
		EnableDeduplication: bs.config.WorkerPoolConfig.EnableDeduplication,
		EnableMetrics:       bs.config.WorkerPoolConfig.EnableMetrics,
		CircuitBreakerConfig: workerpool.CircuitBreakerConfig{
			FailureThreshold: bs.config.WorkerPoolConfig.CircuitBreakerConfig.FailureThreshold,
			RecoveryTimeout:  bs.config.WorkerPoolConfig.CircuitBreakerConfig.RecoveryTimeout,
			HalfOpenMaxCalls: bs.config.WorkerPoolConfig.CircuitBreakerConfig.HalfOpenMaxCalls,
			WindowSize:       bs.config.WorkerPoolConfig.CircuitBreakerConfig.WindowSize,
		},
	}

	workerPool := workerpool.NewWorkerPool(wpConfig)
	defer workerPool.Shutdown(10 * time.Second)

	// Generate test traces
	traces := bs.generateTestTraces(numTraces)

	var wg sync.WaitGroup
	results := make(chan *workerpool.TaskResult, numTraces)
	errors := make(chan error, numTraces)

	// Submit tasks
	for i, trace := range traces {
		wg.Add(1)
		go func(id int, t entities.Trace) {
			defer wg.Done()

			task := &workerpool.Task{
				ID:      fmt.Sprintf("benchmark-task-%d", id),
				Payload: t,
			}

			err := workerPool.Submit(ctx, task)
			if err != nil {
				errors <- err
				return
			}

			// Get result
			result, err := workerPool.GetResult(ctx)
			if err != nil {
				errors <- err
				return
			}

			results <- result
		}(i, trace)
	}

	// Wait for completion
	wg.Wait()
	close(results)
	close(errors)

	// Collect results
	var processedTraces int
	var discoveredTraces int
	var errorCount int

	for result := range results {
		processedTraces++
		if result.Error == nil {
			discoveredTraces++
		}
	}

	for range errors {
		errorCount++
	}

	duration := time.Since(startTime)
	metrics := workerPool.GetMetrics()

	result := &BenchmarkResult{
		TestName:          "Worker Pool Benchmark",
		Duration:          duration,
		TracesProcessed:   processedTraces,
		TracesDiscovered:  discoveredTraces,
		Errors:            errorCount,
		WorkerPoolMetrics: metrics,
		Throughput:        float64(processedTraces) / duration.Seconds(),
		ErrorRate:         float64(errorCount) / float64(numTraces) * 100,
	}

	return result, nil
}

// RunConcurrencyBenchmark benchmarks different concurrency levels
func (bs *BenchmarkSuite) RunConcurrencyBenchmark(ctx context.Context, numTraces int, concurrencyLevels []int) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	for _, concurrency := range concurrencyLevels {
		log.Info().Int("concurrency", concurrency).Msg("Running concurrency benchmark")

		// Create config with specific concurrency
		testConfig := *bs.config
		testConfig.WorkerPoolConfig.MaxWorkers = concurrency

		benchmarkSuite := NewBenchmarkSuite(&testConfig)
		result, err := benchmarkSuite.RunWorkerPoolBenchmark(ctx, numTraces)
		if err != nil {
			return results, err
		}

		result.TestName = fmt.Sprintf("Concurrency Benchmark (%d workers)", concurrency)
		results = append(results, result)
	}

	return results, nil
}

// RunRateLimitBenchmark benchmarks different rate limiting configurations
func (bs *BenchmarkSuite) RunRateLimitBenchmark(ctx context.Context, numTraces int, rateLimits []float64) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	for _, rateLimit := range rateLimits {
		log.Info().Float64("rateLimit", rateLimit).Msg("Running rate limit benchmark")

		// Create config with specific rate limit
		testConfig := *bs.config
		testConfig.WorkerPoolConfig.DefaultRateLimit = rateLimit

		benchmarkSuite := NewBenchmarkSuite(&testConfig)
		result, err := benchmarkSuite.RunWorkerPoolBenchmark(ctx, numTraces)
		if err != nil {
			return results, err
		}

		result.TestName = fmt.Sprintf("Rate Limit Benchmark (%.1f req/s)", rateLimit)
		results = append(results, result)
	}

	return results, nil
}

// RunCircuitBreakerBenchmark benchmarks circuit breaker behavior
func (bs *BenchmarkSuite) RunCircuitBreakerBenchmark(ctx context.Context, numTraces int, failureRates []float64) ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	for _, failureRate := range failureRates {
		log.Info().Float64("failureRate", failureRate).Msg("Running circuit breaker benchmark")

		// Create config with specific failure threshold
		testConfig := *bs.config
		testConfig.WorkerPoolConfig.CircuitBreakerConfig.FailureThreshold = int(1 / failureRate)

		benchmarkSuite := NewBenchmarkSuite(&testConfig)
		result, err := benchmarkSuite.RunWorkerPoolBenchmark(ctx, numTraces)
		if err != nil {
			return results, err
		}

		result.TestName = fmt.Sprintf("Circuit Breaker Benchmark (%.1f%% failure rate)", failureRate*100)
		results = append(results, result)
	}

	return results, nil
}

// generateTestTraces generates test traces for benchmarking
func (bs *BenchmarkSuite) generateTestTraces(count int) []entities.Trace {
	traces := make([]entities.Trace, count)

	for i := 0; i < count; i++ {
		traces[i] = entities.Trace{
			Value: fmt.Sprintf("test-trace-%d", i),
			Type:  entities.Email, // Use a common trace type
		}
	}

	return traces
}

// PrintBenchmarkResults prints benchmark results in a formatted way
func PrintBenchmarkResults(results []*BenchmarkResult) {
	fmt.Println("\n=== Benchmark Results ===")
	fmt.Printf("%-40s %-12s %-12s %-12s %-12s %-12s\n",
		"Test Name", "Duration", "Processed", "Discovered", "Throughput", "Error Rate")
	fmt.Println(string(make([]byte, 100, 100)))

	for _, result := range results {
		fmt.Printf("%-40s %-12s %-12d %-12d %-12.2f %-12.2f%%\n",
			result.TestName,
			result.Duration.String(),
			result.TracesProcessed,
			result.TracesDiscovered,
			result.Throughput,
			result.ErrorRate)

		if result.WorkerPoolMetrics != nil {
			fmt.Printf("  └─ Active Workers: %d, Queue Size: %d/%d, Rate Limit Hits: %d, Dedup Hits: %d, Circuit Breaker Trips: %d\n",
				result.WorkerPoolMetrics.ActiveWorkers,
				result.WorkerPoolMetrics.QueueSize,
				result.WorkerPoolMetrics.QueueCapacity,
				result.WorkerPoolMetrics.RateLimitHits,
				result.WorkerPoolMetrics.DeduplicationHits,
				result.WorkerPoolMetrics.CircuitBreakerTrips)
		}
	}
	fmt.Println()
}
