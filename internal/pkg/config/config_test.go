package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("Expected HTTPTimeout to be 30s, got %v", cfg.HTTPTimeout)
	}

	if cfg.MaxConcurrency != 10 {
		t.Errorf("Expected MaxConcurrency to be 10, got %d", cfg.MaxConcurrency)
	}

	if cfg.RateLimitPerSecond != 5 {
		t.Errorf("Expected RateLimitPerSecond to be 5, got %d", cfg.RateLimitPerSecond)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be 'info', got %s", cfg.LogLevel)
	}

	if cfg.UserAgent != "Deeper/1.0" {
		t.Errorf("Expected UserAgent to be 'Deeper/1.0', got %s", cfg.UserAgent)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", cfg.MaxRetries)
	}

	if cfg.RetryDelay != 1*time.Second {
		t.Errorf("Expected RetryDelay to be 1s, got %v", cfg.RetryDelay)
	}
}

func TestLoadConfigWithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("DEEPER_HTTP_TIMEOUT", "60s")
	os.Setenv("DEEPER_MAX_CONCURRENCY", "20")
	os.Setenv("DEEPER_RATE_LIMIT", "10")
	os.Setenv("DEEPER_LOG_LEVEL", "debug")
	os.Setenv("DEEPER_USER_AGENT", "TestAgent/1.0")
	os.Setenv("DEEPER_MAX_RETRIES", "5")
	os.Setenv("DEEPER_RETRY_DELAY", "2s")

	defer func() {
		os.Unsetenv("DEEPER_HTTP_TIMEOUT")
		os.Unsetenv("DEEPER_MAX_CONCURRENCY")
		os.Unsetenv("DEEPER_RATE_LIMIT")
		os.Unsetenv("DEEPER_LOG_LEVEL")
		os.Unsetenv("DEEPER_USER_AGENT")
		os.Unsetenv("DEEPER_MAX_RETRIES")
		os.Unsetenv("DEEPER_RETRY_DELAY")
	}()

	cfg := LoadConfig()

	if cfg.HTTPTimeout != 60*time.Second {
		t.Errorf("Expected HTTPTimeout to be 60s, got %v", cfg.HTTPTimeout)
	}

	if cfg.MaxConcurrency != 20 {
		t.Errorf("Expected MaxConcurrency to be 20, got %d", cfg.MaxConcurrency)
	}

	if cfg.RateLimitPerSecond != 10 {
		t.Errorf("Expected RateLimitPerSecond to be 10, got %d", cfg.RateLimitPerSecond)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel to be 'debug', got %s", cfg.LogLevel)
	}

	if cfg.UserAgent != "TestAgent/1.0" {
		t.Errorf("Expected UserAgent to be 'TestAgent/1.0', got %s", cfg.UserAgent)
	}

	if cfg.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries to be 5, got %d", cfg.MaxRetries)
	}

	if cfg.RetryDelay != 2*time.Second {
		t.Errorf("Expected RetryDelay to be 2s, got %v", cfg.RetryDelay)
	}
}

func TestLoadConfigWithInvalidValues(t *testing.T) {
	// Set invalid environment variables
	os.Setenv("DEEPER_HTTP_TIMEOUT", "invalid")
	os.Setenv("DEEPER_MAX_CONCURRENCY", "invalid")
	os.Setenv("DEEPER_RATE_LIMIT", "invalid")
	os.Setenv("DEEPER_MAX_RETRIES", "invalid")
	os.Setenv("DEEPER_RETRY_DELAY", "invalid")

	defer func() {
		os.Unsetenv("DEEPER_HTTP_TIMEOUT")
		os.Unsetenv("DEEPER_MAX_CONCURRENCY")
		os.Unsetenv("DEEPER_RATE_LIMIT")
		os.Unsetenv("DEEPER_MAX_RETRIES")
		os.Unsetenv("DEEPER_RETRY_DELAY")
	}()

	cfg := LoadConfig()

	// Should fall back to defaults
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("Expected HTTPTimeout to fall back to 30s, got %v", cfg.HTTPTimeout)
	}

	if cfg.MaxConcurrency != 10 {
		t.Errorf("Expected MaxConcurrency to fall back to 10, got %d", cfg.MaxConcurrency)
	}

	if cfg.RateLimitPerSecond != 5 {
		t.Errorf("Expected RateLimitPerSecond to fall back to 5, got %d", cfg.RateLimitPerSecond)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to fall back to 3, got %d", cfg.MaxRetries)
	}

	if cfg.RetryDelay != 1*time.Second {
		t.Errorf("Expected RetryDelay to fall back to 1s, got %v", cfg.RetryDelay)
	}
}
