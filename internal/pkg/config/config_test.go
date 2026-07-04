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
	_ = os.Setenv("DEEPER_HTTP_TIMEOUT", "60s")
	_ = os.Setenv("DEEPER_MAX_CONCURRENCY", "20")
	_ = os.Setenv("DEEPER_RATE_LIMIT", "10")
	_ = os.Setenv("DEEPER_LOG_LEVEL", "debug")
	_ = os.Setenv("DEEPER_USER_AGENT", "TestAgent/1.0")
	_ = os.Setenv("DEEPER_MAX_RETRIES", "5")
	_ = os.Setenv("DEEPER_RETRY_DELAY", "2s")

	defer func() {
		_ = os.Unsetenv("DEEPER_HTTP_TIMEOUT")
		_ = os.Unsetenv("DEEPER_MAX_CONCURRENCY")
		_ = os.Unsetenv("DEEPER_RATE_LIMIT")
		_ = os.Unsetenv("DEEPER_LOG_LEVEL")
		_ = os.Unsetenv("DEEPER_USER_AGENT")
		_ = os.Unsetenv("DEEPER_MAX_RETRIES")
		_ = os.Unsetenv("DEEPER_RETRY_DELAY")
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
	_ = os.Setenv("DEEPER_HTTP_TIMEOUT", "invalid")
	_ = os.Setenv("DEEPER_MAX_CONCURRENCY", "invalid")
	_ = os.Setenv("DEEPER_RATE_LIMIT", "invalid")
	_ = os.Setenv("DEEPER_MAX_RETRIES", "invalid")
	_ = os.Setenv("DEEPER_RETRY_DELAY", "invalid")

	defer func() {
		_ = os.Unsetenv("DEEPER_HTTP_TIMEOUT")
		_ = os.Unsetenv("DEEPER_MAX_CONCURRENCY")
		_ = os.Unsetenv("DEEPER_RATE_LIMIT")
		_ = os.Unsetenv("DEEPER_MAX_RETRIES")
		_ = os.Unsetenv("DEEPER_RETRY_DELAY")
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

func TestLoadConfigPluginCredentials(t *testing.T) {
	_ = os.Setenv("DEEPER_GRAVATAR_API_KEY", "gravatar-test-key")
	_ = os.Setenv("DEEPER_GITHUB_TOKEN", "github-test-token")

	defer func() {
		_ = os.Unsetenv("DEEPER_GRAVATAR_API_KEY")
		_ = os.Unsetenv("DEEPER_GITHUB_TOKEN")
	}()

	cfg := LoadConfig()

	if cfg.GravatarAPIKey != "gravatar-test-key" {
		t.Errorf("Expected GravatarAPIKey to be set, got %q", cfg.GravatarAPIKey)
	}

	if cfg.GitHubToken != "github-test-token" {
		t.Errorf("Expected GitHubToken to be set, got %q", cfg.GitHubToken)
	}
}
