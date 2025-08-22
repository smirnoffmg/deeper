package deeper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/smirnoffmg/deeper/internal/app/deeper/cli"
	"github.com/smirnoffmg/deeper/internal/app/deeper/display"
	"github.com/smirnoffmg/deeper/internal/app/deeper/engine"
	"github.com/smirnoffmg/deeper/internal/app/deeper/processor"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/smirnoffmg/deeper/internal/pkg/worker"
)

// App represents the main application with all its components
type App struct {
	app    *fx.App
	logger *zap.Logger
}

// NewApp creates a new application instance with uber-fx
func NewApp() *App {
	app := fx.New(
		// Provide core dependencies
		fx.Provide(
			provideLogger,
			provideConfig,
			provideDatabase,
			provideRepository,
			provideCache,
			provideHTTPClient,
			provideMetricsCollector,
			providePluginRegistry,
			provideWorkerPool, // Add worker pool provider
			provideProcessor,
			provideDisplay,
			provideEngine,
		),
		// Invoke startup functions
		fx.Invoke(
			startupLogger,
			startupPluginRegistry,
			startupMetrics,
			startupWorkerPool, // Add worker pool startup
		),
		// Lifecycle hooks
		fx.StartTimeout(30*time.Second),
		fx.StopTimeout(30*time.Second),
	)

	return &App{
		app: app,
	}
}

// Run starts the application and executes the CLI
func (a *App) Run() error {
	// Start the application
	if err := a.app.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Execute CLI directly (this will handle its own lifecycle)
	cli.Execute()

	// Graceful shutdown
	if err := a.app.Stop(context.Background()); err != nil {
		return fmt.Errorf("failed to stop application: %w", err)
	}

	return nil
}

// provideLogger provides a configured logger
func provideLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return logger, nil
}

// provideConfig provides application configuration
func provideConfig() (*config.Config, error) {
	cfg := config.LoadConfig()
	return cfg, nil
}

// provideDatabase provides a database connection
func provideDatabase(cfg *config.Config) (*database.Database, error) {
	// Use default database path in user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(homeDir, ".deeper", "deeper.db")
	return database.NewDatabase(dbPath)
}

// provideRepository provides a database repository
func provideRepository(db *database.Database) *database.Repository {
	return database.NewRepository(db)
}

// provideCache provides a cache instance
func provideCache(repo *database.Repository) *database.Cache {
	return database.NewCache(repo)
}

// provideHTTPClient provides a configured HTTP client
func provideHTTPClient(cfg *config.Config) (http.Client, error) {
	client := http.NewClient(cfg)
	return client, nil
}

// provideMetricsCollector provides a metrics collector
func provideMetricsCollector() *metrics.MetricsCollector {
	return metrics.GetGlobalMetrics()
}

// providePluginRegistry provides a plugin registry
func providePluginRegistry() *plugins.PluginRegistry {
	return plugins.NewPluginRegistry()
}

// provideProcessor provides a trace processor
func provideProcessor(
	cfg *config.Config,
	metricsCollector *metrics.MetricsCollector,
	repo *database.Repository,
	cache *database.Cache,
	pool *worker.Pool, // Inject worker pool
) *processor.Processor {
	return processor.NewProcessor(cfg, metricsCollector, repo, cache, pool)
}

// provideDisplay provides a result display
func provideDisplay() *display.Display {
	return display.NewDisplay(os.Stdout)
}

// provideEngine provides the main processing engine
func provideEngine(
	cfg *config.Config,
	metricsCollector *metrics.MetricsCollector,
	repo *database.Repository,
	cache *database.Cache,
	pool *worker.Pool, // Inject worker pool
) *engine.Engine {
	return engine.NewEngine(cfg, metricsCollector, repo, cache, pool)
}

// provideCLI provides the CLI interface
func provideCLI() {
	// CLI is handled separately through cobra
}

// startupLogger logs application startup
func startupLogger(logger *zap.Logger) {
	logger.Info("Application starting up")
}

// startupPluginRegistry initializes the plugin registry
func startupPluginRegistry(registry *plugins.PluginRegistry, logger *zap.Logger) {
	logger.Info("Initializing plugin registry")

	// Start health checks
	registry.StartHealthChecks(context.Background())

	logger.Info("Plugin registry initialized",
		zap.Int("plugin_count", registry.GetPluginCount()),
		zap.Int("trace_type_count", registry.GetTraceTypeCount()))
}

// startupMetrics initializes metrics collection
func startupMetrics(collector *metrics.MetricsCollector, logger *zap.Logger) {
	logger.Info("Initializing metrics collection")
	// Metrics collector is ready to use
}

// provideWorkerPool provides a worker pool
func provideWorkerPool(cfg *config.Config) *worker.Pool {
	return worker.NewPool(cfg.MaxConcurrency)
}

// startupWorkerPool starts the worker pool
func startupWorkerPool(lc fx.Lifecycle, pool *worker.Pool, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting worker pool")
			pool.Start(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Stopping worker pool")
			pool.Stop()
			return nil
		},
	})
}
