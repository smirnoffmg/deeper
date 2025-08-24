package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	HTTPTimeout        time.Duration
	MaxConcurrency     int
	RateLimitPerSecond int
	LogLevel           string
	UserAgent          string
	MaxRetries         int
	RetryDelay         time.Duration

	// Worker Pool Configuration
	WorkerPoolConfig WorkerPoolConfig
}

// WorkerPoolConfig holds worker pool specific configuration
type WorkerPoolConfig struct {
	MaxWorkers           int
	QueueSize            int
	DefaultRateLimit     float64
	DefaultBurst         int
	TaskTimeout          time.Duration
	EnableDeduplication  bool
	EnableMetrics        bool
	CircuitBreakerConfig CircuitBreakerConfig
	DomainRateConfigs    []DomainRateConfig
	DeduplicationConfig  DeduplicationConfig
}

// DomainRateConfig holds rate limiting configuration for a specific domain
type DomainRateConfig struct {
	Domain      string
	RateLimit   float64
	Burst       int
	BackoffBase time.Duration
	BackoffMax  time.Duration
	MaxRetries  int
}

// DeduplicationConfig holds deduplication system configuration
type DeduplicationConfig struct {
	EnableCache     bool
	CacheTTL        time.Duration
	MaxMemorySize   int
	EnableMetrics   bool
	CleanupInterval time.Duration
	PersistentCache bool
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	HalfOpenMaxCalls int
	WindowSize       time.Duration
}

// DefaultConfig returns default configuration values
func DefaultConfig() *Config {
	return &Config{
		HTTPTimeout:        30 * time.Second,
		MaxConcurrency:     10,
		RateLimitPerSecond: 5,
		LogLevel:           "info",
		UserAgent:          "Deeper/1.0",
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		WorkerPoolConfig: WorkerPoolConfig{
			MaxWorkers:          20,
			QueueSize:           1000,
			DefaultRateLimit:    10.0,
			DefaultBurst:        5,
			TaskTimeout:         30 * time.Second,
			EnableDeduplication: true,
			EnableMetrics:       true,
			CircuitBreakerConfig: CircuitBreakerConfig{
				FailureThreshold: 5,
				RecoveryTimeout:  60 * time.Second,
				HalfOpenMaxCalls: 3,
				WindowSize:       60 * time.Second,
			},
			DeduplicationConfig: DeduplicationConfig{
				EnableCache:     true,
				CacheTTL:        24 * time.Hour,
				MaxMemorySize:   10000,
				EnableMetrics:   true,
				CleanupInterval: 1 * time.Hour,
				PersistentCache: true,
			},
		},
	}
}

// loadDeduplicationConfig loads deduplication configuration from environment variables
func loadDeduplicationConfig(config *Config) {
	if enableCache := os.Getenv("DEEPER_DEDUP_ENABLE_CACHE"); enableCache != "" {
		if val, err := strconv.ParseBool(enableCache); err == nil {
			config.WorkerPoolConfig.DeduplicationConfig.EnableCache = val
		}
	}

	if cacheTTL := os.Getenv("DEEPER_DEDUP_CACHE_TTL"); cacheTTL != "" {
		if duration, err := time.ParseDuration(cacheTTL); err == nil {
			config.WorkerPoolConfig.DeduplicationConfig.CacheTTL = duration
		}
	}

	if maxMemorySize := os.Getenv("DEEPER_DEDUP_MAX_MEMORY_SIZE"); maxMemorySize != "" {
		if val, err := strconv.Atoi(maxMemorySize); err == nil {
			config.WorkerPoolConfig.DeduplicationConfig.MaxMemorySize = val
		}
	}

	if enableMetrics := os.Getenv("DEEPER_DEDUP_ENABLE_METRICS"); enableMetrics != "" {
		if val, err := strconv.ParseBool(enableMetrics); err == nil {
			config.WorkerPoolConfig.DeduplicationConfig.EnableMetrics = val
		}
	}

	if cleanupInterval := os.Getenv("DEEPER_DEDUP_CLEANUP_INTERVAL"); cleanupInterval != "" {
		if duration, err := time.ParseDuration(cleanupInterval); err == nil {
			config.WorkerPoolConfig.DeduplicationConfig.CleanupInterval = duration
		}
	}

	if persistentCache := os.Getenv("DEEPER_DEDUP_PERSISTENT_CACHE"); persistentCache != "" {
		if val, err := strconv.ParseBool(persistentCache); err == nil {
			config.WorkerPoolConfig.DeduplicationConfig.PersistentCache = val
		}
	}
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	config := DefaultConfig()

	if timeout := os.Getenv("DEEPER_HTTP_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			config.HTTPTimeout = duration
		}
	}

	if concurrency := os.Getenv("DEEPER_MAX_CONCURRENCY"); concurrency != "" {
		if val, err := strconv.Atoi(concurrency); err == nil {
			config.MaxConcurrency = val
		}
	}

	if rateLimit := os.Getenv("DEEPER_RATE_LIMIT"); rateLimit != "" {
		if val, err := strconv.Atoi(rateLimit); err == nil {
			config.RateLimitPerSecond = val
		}
	}

	if logLevel := os.Getenv("DEEPER_LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	if userAgent := os.Getenv("DEEPER_USER_AGENT"); userAgent != "" {
		config.UserAgent = userAgent
	}

	if maxRetries := os.Getenv("DEEPER_MAX_RETRIES"); maxRetries != "" {
		if val, err := strconv.Atoi(maxRetries); err == nil {
			config.MaxRetries = val
		}
	}

	if retryDelay := os.Getenv("DEEPER_RETRY_DELAY"); retryDelay != "" {
		if duration, err := time.ParseDuration(retryDelay); err == nil {
			config.RetryDelay = duration
		}
	}

	// Load worker pool configuration
	loadWorkerPoolConfig(config)

	return config
}

// loadWorkerPoolConfig loads worker pool configuration from environment variables
func loadWorkerPoolConfig(config *Config) {
	if maxWorkers := os.Getenv("DEEPER_WORKER_POOL_MAX_WORKERS"); maxWorkers != "" {
		if val, err := strconv.Atoi(maxWorkers); err == nil {
			config.WorkerPoolConfig.MaxWorkers = val
		}
	}

	if queueSize := os.Getenv("DEEPER_WORKER_POOL_QUEUE_SIZE"); queueSize != "" {
		if val, err := strconv.Atoi(queueSize); err == nil {
			config.WorkerPoolConfig.QueueSize = val
		}
	}

	if rateLimit := os.Getenv("DEEPER_WORKER_POOL_RATE_LIMIT"); rateLimit != "" {
		if val, err := strconv.ParseFloat(rateLimit, 64); err == nil {
			config.WorkerPoolConfig.DefaultRateLimit = val
		}
	}

	if burst := os.Getenv("DEEPER_WORKER_POOL_BURST"); burst != "" {
		if val, err := strconv.Atoi(burst); err == nil {
			config.WorkerPoolConfig.DefaultBurst = val
		}
	}

	if taskTimeout := os.Getenv("DEEPER_WORKER_POOL_TASK_TIMEOUT"); taskTimeout != "" {
		if duration, err := time.ParseDuration(taskTimeout); err == nil {
			config.WorkerPoolConfig.TaskTimeout = duration
		}
	}

	if enableDedup := os.Getenv("DEEPER_WORKER_POOL_ENABLE_DEDUP"); enableDedup != "" {
		if val, err := strconv.ParseBool(enableDedup); err == nil {
			config.WorkerPoolConfig.EnableDeduplication = val
		}
	}

	if enableMetrics := os.Getenv("DEEPER_WORKER_POOL_ENABLE_METRICS"); enableMetrics != "" {
		if val, err := strconv.ParseBool(enableMetrics); err == nil {
			config.WorkerPoolConfig.EnableMetrics = val
		}
	}

	// Load circuit breaker configuration
	if failureThreshold := os.Getenv("DEEPER_CIRCUIT_BREAKER_FAILURE_THRESHOLD"); failureThreshold != "" {
		if val, err := strconv.Atoi(failureThreshold); err == nil {
			config.WorkerPoolConfig.CircuitBreakerConfig.FailureThreshold = val
		}
	}

	if recoveryTimeout := os.Getenv("DEEPER_CIRCUIT_BREAKER_RECOVERY_TIMEOUT"); recoveryTimeout != "" {
		if duration, err := time.ParseDuration(recoveryTimeout); err == nil {
			config.WorkerPoolConfig.CircuitBreakerConfig.RecoveryTimeout = duration
		}
	}

	if halfOpenMaxCalls := os.Getenv("DEEPER_CIRCUIT_BREAKER_HALF_OPEN_MAX_CALLS"); halfOpenMaxCalls != "" {
		if val, err := strconv.Atoi(halfOpenMaxCalls); err == nil {
			config.WorkerPoolConfig.CircuitBreakerConfig.HalfOpenMaxCalls = val
		}
	}

	if windowSize := os.Getenv("DEEPER_CIRCUIT_BREAKER_WINDOW_SIZE"); windowSize != "" {
		if duration, err := time.ParseDuration(windowSize); err == nil {
			config.WorkerPoolConfig.CircuitBreakerConfig.WindowSize = duration
		}
	}

	// Load deduplication configuration
	loadDeduplicationConfig(config)
}
