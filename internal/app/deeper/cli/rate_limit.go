package cli

import (
	"fmt"
	"time"

	"github.com/smirnoffmg/deeper/internal/app/deeper/processor"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/spf13/cobra"
)

var rateLimitCmd = &cobra.Command{
	Use:   "rate-limit",
	Short: "Configure domain-specific rate limiting",
	Long:  "Configure rate limiting settings for specific domains to prevent API abuse and implement automatic backoff",
}

var (
	rateLimitDomain      string
	rateLimitRate        float64
	rateLimitBurst       int
	rateLimitBackoffBase time.Duration
	rateLimitBackoffMax  time.Duration
	rateLimitMaxRetries  int
	rateLimitList        bool
)

func init() {
	rateLimitCmd.Flags().StringVarP(&rateLimitDomain, "domain", "d", "", "Domain to configure (e.g., api.github.com)")
	rateLimitCmd.Flags().Float64VarP(&rateLimitRate, "rate", "r", 10.0, "Rate limit (requests per second)")
	rateLimitCmd.Flags().IntVarP(&rateLimitBurst, "burst", "b", 5, "Burst limit")
	rateLimitCmd.Flags().DurationVarP(&rateLimitBackoffBase, "backoff-base", "B", 1*time.Second, "Base backoff duration")
	rateLimitCmd.Flags().DurationVarP(&rateLimitBackoffMax, "backoff-max", "M", 60*time.Second, "Maximum backoff duration")
	rateLimitCmd.Flags().IntVarP(&rateLimitMaxRetries, "max-retries", "R", 3, "Maximum retry attempts")
	rateLimitCmd.Flags().BoolVarP(&rateLimitList, "list", "l", false, "List current domain rate limit configurations")

	_ = rateLimitCmd.MarkFlagRequired("domain")

	rootCmd.AddCommand(rateLimitCmd)
}

func runRateLimit(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db, err := database.NewDatabase("deeper.db")
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Initialize processor with worker pool
	metricsCollector := metrics.NewMetricsCollector()
	repo := database.NewRepository(db)
	cache := database.NewCache(repo)
	proc := processor.NewProcessor(cfg, metricsCollector, repo, cache)

	if rateLimitList {
		return listDomainRateLimits(proc)
	}

	err = proc.ConfigureDomainRateLimit(
		rateLimitDomain,
		rateLimitRate,
		rateLimitBurst,
		rateLimitBackoffBase,
		rateLimitBackoffMax,
		rateLimitMaxRetries,
	)

	if err != nil {
		return fmt.Errorf("failed to configure domain rate limit: %w", err)
	}

	fmt.Printf("✅ Successfully configured rate limiting for domain '%s':\n", rateLimitDomain)
	fmt.Printf("   Rate Limit: %.1f requests/second\n", rateLimitRate)
	fmt.Printf("   Burst: %d requests\n", rateLimitBurst)
	fmt.Printf("   Backoff Base: %s\n", rateLimitBackoffBase)
	fmt.Printf("   Backoff Max: %s\n", rateLimitBackoffMax)
	fmt.Printf("   Max Retries: %d\n", rateLimitMaxRetries)

	return nil
}

func listDomainRateLimits(processor *processor.Processor) error {
	metrics := processor.GetWorkerPoolMetrics()
	if metrics == nil || metrics.DomainRateMetrics == nil {
		fmt.Println("No domain rate limit configurations found.")
		return nil
	}

	fmt.Println("Current Domain Rate Limit Configurations:")
	fmt.Println("========================================")

	for domain, domainMetrics := range metrics.DomainRateMetrics {
		fmt.Printf("\nDomain: %s\n", domain)
		fmt.Printf("  Rate Limit: %.1f requests/second\n", domainMetrics.RateLimit)
		fmt.Printf("  Burst: %d requests\n", domainMetrics.Burst)
		fmt.Printf("  Failure Count: %d\n", domainMetrics.FailureCount)
		fmt.Printf("  Current Backoff: %s\n", domainMetrics.CurrentBackoff)
		fmt.Printf("  In Backoff: %t\n", domainMetrics.IsInBackoff)
	}

	return nil
}

// Example usage commands
func init() {
	rateLimitCmd.AddCommand(&cobra.Command{
		Use:   "github",
		Short: "Configure GitHub API rate limiting",
		RunE: func(cmd *cobra.Command, args []string) error {
			rateLimitDomain = "api.github.com"
			rateLimitRate = 2.0
			rateLimitBurst = 1
			rateLimitBackoffBase = 5 * time.Second
			rateLimitBackoffMax = 300 * time.Second
			rateLimitMaxRetries = 5
			return runRateLimit(cmd, args)
		},
	})

	rateLimitCmd.AddCommand(&cobra.Command{
		Use:   "google",
		Short: "Configure Google API rate limiting",
		RunE: func(cmd *cobra.Command, args []string) error {
			rateLimitDomain = "google.com"
			rateLimitRate = 3.0
			rateLimitBurst = 1
			rateLimitBackoffBase = 3 * time.Second
			rateLimitBackoffMax = 180 * time.Second
			rateLimitMaxRetries = 4
			return runRateLimit(cmd, args)
		},
	})
}
