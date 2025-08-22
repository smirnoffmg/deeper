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

	return config
}
