package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/smirnoffmg/deeper/internal/pkg/benchmark"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run performance benchmarks",
	Long:  "Run performance benchmarks to test worker pool implementation and measure performance improvements",
	RunE:  runBenchmark,
}

var (
	benchmarkNumTraces     int
	benchmarkConcurrency   []int
	benchmarkRateLimits    []float64
	benchmarkFailureRates  []float64
	benchmarkTimeout       time.Duration
)

func init() {
	benchmarkCmd.Flags().IntVarP(&benchmarkNumTraces, "traces", "t", 100, "Number of traces to process in benchmark")
	benchmarkCmd.Flags().IntSliceVarP(&benchmarkConcurrency, "concurrency", "c", []int{5, 10, 20, 50}, "Concurrency levels to test")
	benchmarkCmd.Flags().Float64SliceVarP(&benchmarkRateLimits, "rate-limits", "r", []float64{1, 5, 10, 20}, "Rate limits to test (requests per second)")
	benchmarkCmd.Flags().Float64SliceVarP(&benchmarkFailureRates, "failure-rates", "f", []float64{0.1, 0.2, 0.5}, "Failure rates to test for circuit breaker")
	benchmarkCmd.Flags().DurationVarP(&benchmarkTimeout, "timeout", "o", 5*time.Minute, "Benchmark timeout")
	
	rootCmd.AddCommand(benchmarkCmd)
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), benchmarkTimeout)
	defer cancel()

	// Load configuration
	cfg := config.LoadConfig()
	
	// Create benchmark suite
	benchmarkSuite := benchmark.NewBenchmarkSuite(cfg)

	log.Info().Msg("Starting performance benchmarks...")
	log.Info().Int("numTraces", benchmarkNumTraces).Msg("Benchmark configuration")

	var allResults []*benchmark.BenchmarkResult

	// Run basic worker pool benchmark
	log.Info().Msg("Running basic worker pool benchmark...")
	basicResult, err := benchmarkSuite.RunWorkerPoolBenchmark(ctx, benchmarkNumTraces)
	if err != nil {
		return fmt.Errorf("basic benchmark failed: %w", err)
	}
	allResults = append(allResults, basicResult)

	// Run concurrency benchmarks
	if len(benchmarkConcurrency) > 0 {
		log.Info().Msg("Running concurrency benchmarks...")
		concurrencyResults, err := benchmarkSuite.RunConcurrencyBenchmark(ctx, benchmarkNumTraces, benchmarkConcurrency)
		if err != nil {
			return fmt.Errorf("concurrency benchmark failed: %w", err)
		}
		allResults = append(allResults, concurrencyResults...)
	}

	// Run rate limit benchmarks
	if len(benchmarkRateLimits) > 0 {
		log.Info().Msg("Running rate limit benchmarks...")
		rateLimitResults, err := benchmarkSuite.RunRateLimitBenchmark(ctx, benchmarkNumTraces, benchmarkRateLimits)
		if err != nil {
			return fmt.Errorf("rate limit benchmark failed: %w", err)
		}
		allResults = append(allResults, rateLimitResults...)
	}

	// Run circuit breaker benchmarks
	if len(benchmarkFailureRates) > 0 {
		log.Info().Msg("Running circuit breaker benchmarks...")
		circuitBreakerResults, err := benchmarkSuite.RunCircuitBreakerBenchmark(ctx, benchmarkNumTraces, benchmarkFailureRates)
		if err != nil {
			return fmt.Errorf("circuit breaker benchmark failed: %w", err)
		}
		allResults = append(allResults, circuitBreakerResults...)
	}

	// Print results
	benchmark.PrintBenchmarkResults(allResults)

	// Generate summary
	generateBenchmarkSummary(allResults)

	return nil
}

func generateBenchmarkSummary(results []*benchmark.BenchmarkResult) {
	fmt.Println("\n=== Benchmark Summary ===")
	
	// Find best performing configuration
	var bestThroughput float64
	var bestResult *benchmark.BenchmarkResult
	
	for _, result := range results {
		if result.Throughput > bestThroughput {
			bestThroughput = result.Throughput
			bestResult = result
		}
	}
	
	if bestResult != nil {
		fmt.Printf("Best Performance: %s\n", bestResult.TestName)
		fmt.Printf("  Throughput: %.2f traces/second\n", bestResult.Throughput)
		fmt.Printf("  Error Rate: %.2f%%\n", bestResult.ErrorRate)
		fmt.Printf("  Duration: %s\n", bestResult.Duration)
	}
	
	// Calculate averages
	var totalThroughput float64
	var totalErrorRate float64
	var totalDuration time.Duration
	
	for _, result := range results {
		totalThroughput += result.Throughput
		totalErrorRate += result.ErrorRate
		totalDuration += result.Duration
	}
	
	avgThroughput := totalThroughput / float64(len(results))
	avgErrorRate := totalErrorRate / float64(len(results))
	avgDuration := totalDuration / time.Duration(len(results))
	
	fmt.Printf("\nAverage Performance:\n")
	fmt.Printf("  Throughput: %.2f traces/second\n", avgThroughput)
	fmt.Printf("  Error Rate: %.2f%%\n", avgErrorRate)
	fmt.Printf("  Duration: %s\n", avgDuration)
	
	// Performance recommendations
	fmt.Println("\n=== Performance Recommendations ===")
	
	// Find optimal concurrency
	var optimalConcurrency int
	var maxConcurrencyThroughput float64
	
	for _, result := range results {
		if result.TestName[:25] == "Concurrency Benchmark (" {
			if result.Throughput > maxConcurrencyThroughput {
				maxConcurrencyThroughput = result.Throughput
				// Extract concurrency number from test name
				if len(result.TestName) > 25 {
					concurrencyStr := result.TestName[25 : len(result.TestName)-9] // Remove " workers)"
					if concurrency, err := strconv.Atoi(concurrencyStr); err == nil {
						optimalConcurrency = concurrency
					}
				}
			}
		}
	}
	
	if optimalConcurrency > 0 {
		fmt.Printf("Optimal Worker Pool Size: %d workers\n", optimalConcurrency)
	}
	
	// Rate limiting recommendations
	var optimalRateLimit float64
	var maxRateLimitThroughput float64
	
	for _, result := range results {
		if result.TestName[:22] == "Rate Limit Benchmark (" {
			if result.Throughput > maxRateLimitThroughput {
				maxRateLimitThroughput = result.Throughput
				// Extract rate limit from test name
				if len(result.TestName) > 22 {
					rateLimitStr := result.TestName[22 : len(result.TestName)-8] // Remove " req/s)"
					if rateLimit, err := strconv.ParseFloat(rateLimitStr, 64); err == nil {
						optimalRateLimit = rateLimit
					}
				}
			}
		}
	}
	
	if optimalRateLimit > 0 {
		fmt.Printf("Optimal Rate Limit: %.1f requests/second\n", optimalRateLimit)
	}
	
	fmt.Println("\nBenchmark completed successfully!")
}
