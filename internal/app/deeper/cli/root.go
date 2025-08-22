package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/smirnoffmg/deeper/internal/app/deeper/display"
	"github.com/smirnoffmg/deeper/internal/app/deeper/engine"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
)

var (
	cfgFile     string
	logLevel    string
	timeout     time.Duration
	concurrency int
	rateLimit   int
	output      string
	verbose     bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "deeper [input]",
	Short: "A powerful OSINT tool for gathering information from various online sources",
	Long: `Deeper is an OSINT (Open Source Intelligence) tool designed to help users 
gather information from various online sources. The tool operates based on the 
concept of "traces" - pieces of information such as emails, phone numbers, domains, 
or usernames that can be followed to discover new traces.

Examples:
  deeper scan username123
  deeper scan test@example.com --output json
  deeper scan github.com --concurrency 20 --timeout 60s
  deeper plugins list
  deeper health`,
	Args: cobra.MinimumNArgs(0),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.deeper.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 5*time.Minute, "operation timeout")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 10, "maximum concurrent operations")
	rootCmd.PersistentFlags().IntVar(&rateLimit, "rate-limit", 5, "requests per second")
	rootCmd.PersistentFlags().StringVar(&output, "output", "table", "output format (table, json, csv)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(pluginsCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(databaseCmd)
}

func initConfig() {
	// Initialize configuration here if needed
}

func setupLogging() {
	// Set up console logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if verbose {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05",
		})
	}

	// Set log level
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		log.Warn().Msgf("Invalid log level %s, using info", logLevel)
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
}

func createEngine() *engine.Engine {
	// Create configuration with CLI flags
	cfg := config.LoadConfig()

	// Override with CLI flags if provided
	if timeout != 0 {
		cfg.HTTPTimeout = timeout
	}
	if concurrency != 0 {
		cfg.MaxConcurrency = concurrency
	}
	if rateLimit != 0 {
		cfg.RateLimitPerSecond = rateLimit
	}
	if logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Get global metrics collector
	metricsCollector := metrics.GetGlobalMetrics()

	// Create database and cache (for CLI mode)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get home directory")
		return nil
	}

	dbPath := filepath.Join(homeDir, ".deeper", "deeper.db")
	db, err := database.NewDatabase(dbPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create database")
		return nil
	}

	repo := database.NewRepository(db)
	cache := database.NewCache(repo)

	return engine.NewEngine(cfg, metricsCollector, repo, cache)
}

func createDisplay() *display.Display {
	return display.NewDisplay(os.Stdout)
}

func formatOutput(traces []interface{}, format string, display *display.Display) error {
	switch format {
	case "table":
		if traceList, ok := traces[0].([]interface{}); ok {
			// Handle slice of traces
			_ = traceList
			// display.PrintTracesAsTable(traceList) // We'd need to update the display package
		}
		return nil
	case "json":
		return outputJSON(traces)
	case "csv":
		return outputCSV(traces)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func outputJSON(data []interface{}) error {
	// Implementation for JSON output
	fmt.Println("JSON output not yet implemented")
	return nil
}

func outputCSV(data []interface{}) error {
	// Implementation for CSV output
	fmt.Println("CSV output not yet implemented")
	return nil
}
