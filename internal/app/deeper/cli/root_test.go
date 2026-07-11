package cli

import (
	"testing"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/stretchr/testify/assert"
)

// Regression: --timeout (the scan-wide deadline, see scan.go's
// context.WithTimeout) used to also override cfg.HTTPTimeout — meaning
// every individual HTTP request got the same ~5-minute timeout as the whole
// scan, instead of a sane per-request timeout. A single slow/hung request
// could then occupy a worker for the entire scan budget. --timeout must
// only control the scan-wide deadline.
func TestApplyCLIOverrides_DoesNotOverrideHTTPTimeout(t *testing.T) {
	cfg := config.DefaultConfig()
	originalHTTPTimeout := cfg.HTTPTimeout

	applyCLIOverrides(cfg, 5*time.Minute, 0, 0, "")

	assert.Equal(t, originalHTTPTimeout, cfg.HTTPTimeout)
}

func TestApplyCLIOverrides_AppliesConcurrencyRateLimitLogLevel(t *testing.T) {
	cfg := config.DefaultConfig()

	applyCLIOverrides(cfg, 0, 42, 7, "debug")

	assert.Equal(t, 42, cfg.MaxConcurrency)
	assert.Equal(t, 7, cfg.RateLimitPerSecond)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestApplyCLIOverrides_ZeroValuesLeaveDefaultsUnchanged(t *testing.T) {
	cfg := config.DefaultConfig()
	originalConcurrency := cfg.MaxConcurrency
	originalRateLimit := cfg.RateLimitPerSecond
	originalLogLevel := cfg.LogLevel

	applyCLIOverrides(cfg, 0, 0, 0, "")

	assert.Equal(t, originalConcurrency, cfg.MaxConcurrency)
	assert.Equal(t, originalRateLimit, cfg.RateLimitPerSecond)
	assert.Equal(t, originalLogLevel, cfg.LogLevel)
}
