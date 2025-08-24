package workerpool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDomainRateLimiter(t *testing.T) {
	config := &DomainRateConfig{
		Domain:      "test.com",
		RateLimit:   5.0,
		Burst:       2,
		BackoffBase: 1 * time.Second,
		BackoffMax:  10 * time.Second,
		MaxRetries:  3,
	}

	limiter := NewDomainRateLimiter(config)
	require.NotNil(t, limiter)
	assert.NotNil(t, limiter.domainExtractor)
	assert.Equal(t, config, limiter.defaultConfig)
}

func TestNewDomainRateLimiter_DefaultConfig(t *testing.T) {
	limiter := NewDomainRateLimiter(nil)
	require.NotNil(t, limiter)
	assert.NotNil(t, limiter.defaultConfig)
	assert.Equal(t, "default", limiter.defaultConfig.Domain)
	assert.Equal(t, 10.0, limiter.defaultConfig.RateLimit)
}

func TestDomainRateLimiter_AddDomainConfig(t *testing.T) {
	limiter := NewDomainRateLimiter(nil)

	tests := []struct {
		name        string
		config      *DomainRateConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &DomainRateConfig{
				Domain:      "example.com",
				RateLimit:   10.0,
				Burst:       5,
				BackoffBase: 1 * time.Second,
				BackoffMax:  60 * time.Second,
				MaxRetries:  3,
			},
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "invalid domain",
			config: &DomainRateConfig{
				Domain:      "invalid@domain",
				RateLimit:   10.0,
				Burst:       5,
				BackoffBase: 1 * time.Second,
				BackoffMax:  60 * time.Second,
				MaxRetries:  3,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := limiter.AddDomainConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify config was added
				addedConfig := limiter.GetDomainConfig(tt.config.Domain)
				assert.Equal(t, tt.config.Domain, addedConfig.Domain)
				assert.Equal(t, tt.config.RateLimit, addedConfig.RateLimit)
			}
		})
	}
}

func TestDomainRateLimiter_GetDomainConfig(t *testing.T) {
	limiter := NewDomainRateLimiter(nil)

	// Add a custom config
	customConfig := &DomainRateConfig{
		Domain:      "custom.com",
		RateLimit:   20.0,
		Burst:       10,
		BackoffBase: 2 * time.Second,
		BackoffMax:  120 * time.Second,
		MaxRetries:  5,
	}
	err := limiter.AddDomainConfig(customConfig)
	require.NoError(t, err)

	tests := []struct {
		name     string
		domain   string
		expected *DomainRateConfig
	}{
		{
			name:     "existing domain",
			domain:   "custom.com",
			expected: customConfig,
		},
		{
			name:     "non-existing domain",
			domain:   "nonexistent.com",
			expected: limiter.defaultConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := limiter.GetDomainConfig(tt.domain)
			assert.Equal(t, tt.expected, config)
		})
	}
}

func TestDomainRateLimiter_Allow(t *testing.T) {
	limiter := NewDomainRateLimiter(nil)

	// Add a domain with low rate limit for testing
	config := &DomainRateConfig{
		Domain:      "test.com",
		RateLimit:   1.0, // 1 request per second
		Burst:       1,
		BackoffBase: 1 * time.Second,
		BackoffMax:  10 * time.Second,
		MaxRetries:  3,
	}
	err := limiter.AddDomainConfig(config)
	require.NoError(t, err)

	// First request should be allowed
	assert.True(t, limiter.Allow("test.com"))

	// Second request should be rate limited
	assert.False(t, limiter.Allow("test.com"))

	// Wait for rate limit to reset
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	assert.True(t, limiter.Allow("test.com"))
}

func TestDomainRateLimiter_Wait(t *testing.T) {
	limiter := NewDomainRateLimiter(nil)

	// Add a domain with low rate limit
	config := &DomainRateConfig{
		Domain:      "test.com",
		RateLimit:   1.0, // 1 request per second
		Burst:       1,
		BackoffBase: 100 * time.Millisecond,
		BackoffMax:  1 * time.Second,
		MaxRetries:  3,
	}
	err := limiter.AddDomainConfig(config)
	require.NoError(t, err)

	ctx := context.Background()

	// First wait should succeed
	err = limiter.Wait(ctx, "test.com")
	assert.NoError(t, err)

	// Second wait should succeed but take time due to rate limiting
	start := time.Now()
	err = limiter.Wait(ctx, "test.com")
	duration := time.Since(start)

	assert.NoError(t, err) // Wait should eventually succeed
	assert.True(t, duration >= 900*time.Millisecond, "Expected wait to take at least 900ms due to rate limiting")
}

func TestDomainRateLimiter_ExtractDomainAndWait(t *testing.T) {
	limiter := NewDomainRateLimiter(nil)

	// Add domain config
	config := &DomainRateConfig{
		Domain:      "example.com",
		RateLimit:   10.0,
		Burst:       5,
		BackoffBase: 100 * time.Millisecond,
		BackoffMax:  1 * time.Second,
		MaxRetries:  3,
	}
	err := limiter.AddDomainConfig(config)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		task        *Task
		expectError bool
	}{
		{
			name: "email task",
			task: &Task{
				ID:      "email-task",
				Payload: "user@example.com",
			},
			expectError: false,
		},
		{
			name: "url task",
			task: &Task{
				ID:      "url-task",
				Payload: "https://api.github.com/user/repos",
			},
			expectError: false,
		},
		{
			name:        "nil task",
			task:        nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := limiter.ExtractDomainAndWait(ctx, tt.task)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, domain)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, domain)
			}
		})
	}
}

func TestDomainRateLimiter_GetMetrics(t *testing.T) {
	limiter := NewDomainRateLimiter(nil)

	// Add some domain configs
	configs := []*DomainRateConfig{
		{
			Domain:      "example.com",
			RateLimit:   10.0,
			Burst:       5,
			BackoffBase: 1 * time.Second,
			BackoffMax:  60 * time.Second,
			MaxRetries:  3,
		},
		{
			Domain:      "test.com",
			RateLimit:   5.0,
			Burst:       2,
			BackoffBase: 500 * time.Millisecond,
			BackoffMax:  30 * time.Second,
			MaxRetries:  2,
		},
	}

	for _, config := range configs {
		err := limiter.AddDomainConfig(config)
		require.NoError(t, err)
	}

	// Trigger some rate limiting to generate metrics
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		limiter.Wait(ctx, "example.com")
	}

	metrics := limiter.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Len(t, metrics, 3) // default + 2 custom domains

	// Check that metrics exist for our domains
	assert.Contains(t, metrics, "example.com")
	assert.Contains(t, metrics, "test.com")
	assert.Contains(t, metrics, "default")

	// Check metric structure
	exampleMetrics := metrics["example.com"]
	assert.Equal(t, "example.com", exampleMetrics.Domain)
	assert.Equal(t, 10.0, exampleMetrics.RateLimit)
	assert.Equal(t, 5, exampleMetrics.Burst)
}

func TestBackoffTracker(t *testing.T) {
	config := &DomainRateConfig{
		Domain:      "test.com",
		RateLimit:   10.0,
		Burst:       5,
		BackoffBase: 1 * time.Second,
		BackoffMax:  10 * time.Second,
		MaxRetries:  3,
	}

	tracker := &BackoffTracker{}

	// Initially not in backoff
	assert.False(t, tracker.isInBackoff())
	assert.Equal(t, 0, tracker.getFailureCount())
	assert.Equal(t, time.Duration(0), tracker.getCurrentBackoff())

	// Record a failure
	tracker.recordFailure(config)
	assert.Equal(t, 1, tracker.getFailureCount())
	assert.True(t, tracker.isInBackoff())
	assert.Equal(t, 1*time.Second, tracker.getCurrentBackoff())

	// Record another failure
	tracker.recordFailure(config)
	assert.Equal(t, 2, tracker.getFailureCount())
	assert.Equal(t, 2*time.Second, tracker.getCurrentBackoff())

	// Record success
	tracker.recordSuccess()
	assert.Equal(t, 0, tracker.getFailureCount())
	assert.False(t, tracker.isInBackoff())
	assert.Equal(t, time.Duration(0), tracker.getCurrentBackoff())
}
