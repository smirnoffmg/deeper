package workerpool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// DomainRateConfig holds rate limiting configuration for a specific domain
type DomainRateConfig struct {
	Domain      string
	RateLimit   float64
	Burst       int
	BackoffBase time.Duration
	BackoffMax  time.Duration
	MaxRetries  int
}

// DomainRateLimiter manages rate limiting for different domains
type DomainRateLimiter struct {
	configs         map[string]*DomainRateConfig
	limiters        map[string]*rate.Limiter
	backoffTrackers map[string]*BackoffTracker
	mux             sync.RWMutex
	defaultConfig   *DomainRateConfig
	domainExtractor *DomainExtractor
}

// BackoffTracker tracks backoff state for a domain
type BackoffTracker struct {
	LastFailure    time.Time
	CurrentBackoff time.Duration
	FailureCount   int
	mux            sync.Mutex
}

// NewDomainRateLimiter creates a new domain-specific rate limiter
func NewDomainRateLimiter(defaultConfig *DomainRateConfig) *DomainRateLimiter {
	if defaultConfig == nil {
		defaultConfig = &DomainRateConfig{
			Domain:      "default",
			RateLimit:   10.0,
			Burst:       5,
			BackoffBase: 1 * time.Second,
			BackoffMax:  60 * time.Second,
			MaxRetries:  3,
		}
	}

	drl := &DomainRateLimiter{
		configs:         make(map[string]*DomainRateConfig),
		limiters:        make(map[string]*rate.Limiter),
		backoffTrackers: make(map[string]*BackoffTracker),
		defaultConfig:   defaultConfig,
		domainExtractor: NewDomainExtractor(),
	}

	// Initialize default limiter
	defaultLimiter := rate.NewLimiter(rate.Limit(defaultConfig.RateLimit), defaultConfig.Burst)
	drl.limiters[defaultConfig.Domain] = defaultLimiter

	return drl
}

// AddDomainConfig adds or updates rate limiting configuration for a domain
func (drl *DomainRateLimiter) AddDomainConfig(config *DomainRateConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if !drl.domainExtractor.ValidateDomain(config.Domain) {
		return fmt.Errorf("invalid domain: %s", config.Domain)
	}

	drl.mux.Lock()
	defer drl.mux.Unlock()

	drl.configs[config.Domain] = config

	// Create or update rate limiter for this domain
	limiter := rate.NewLimiter(rate.Limit(config.RateLimit), config.Burst)
	drl.limiters[config.Domain] = limiter

	// Initialize backoff tracker if it doesn't exist
	if _, exists := drl.backoffTrackers[config.Domain]; !exists {
		drl.backoffTrackers[config.Domain] = &BackoffTracker{}
	}

	log.Info().Str("domain", config.Domain).
		Float64("rateLimit", config.RateLimit).
		Int("burst", config.Burst).
		Msg("Added domain rate limiting configuration")

	return nil
}

// GetDomainConfig returns the rate limiting configuration for a domain
func (drl *DomainRateLimiter) GetDomainConfig(domain string) *DomainRateConfig {
	drl.mux.RLock()
	defer drl.mux.RUnlock()

	if config, exists := drl.configs[domain]; exists {
		return config
	}

	return drl.defaultConfig
}

// Allow checks if a request is allowed for the given domain
func (drl *DomainRateLimiter) Allow(domain string) bool {
	drl.mux.RLock()
	limiter, exists := drl.limiters[domain]
	drl.mux.RUnlock()

	if !exists {
		// Use default limiter
		drl.mux.RLock()
		limiter = drl.limiters[drl.defaultConfig.Domain]
		drl.mux.RUnlock()
	}

	return limiter.Allow()
}

// Wait waits for rate limit allowance with backoff
func (drl *DomainRateLimiter) Wait(ctx context.Context, domain string) error {
	config := drl.GetDomainConfig(domain)
	backoffTracker := drl.getBackoffTracker(domain)

	// Check if we're in backoff period
	if backoffTracker.isInBackoff() {
		backoffDuration := backoffTracker.getCurrentBackoff()
		log.Debug().Str("domain", domain).
			Dur("backoff", backoffDuration).
			Msg("Domain in backoff period")

		select {
		case <-time.After(backoffDuration):
			// Backoff period completed
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Try to get rate limit allowance
	drl.mux.RLock()
	limiter, exists := drl.limiters[domain]
	drl.mux.RUnlock()

	if !exists {
		// Use default limiter
		drl.mux.RLock()
		limiter = drl.limiters[drl.defaultConfig.Domain]
		drl.mux.RUnlock()
	}

	// Wait for rate limit allowance
	err := limiter.Wait(ctx)
	if err != nil {
		// Rate limit exceeded, trigger backoff
		backoffTracker.recordFailure(config)
		return fmt.Errorf("rate limit exceeded for domain %s", domain)
	}

	// Success, reset backoff
	backoffTracker.recordSuccess()
	return nil
}

// ExtractDomainAndWait extracts domain from task and waits for rate limit allowance
func (drl *DomainRateLimiter) ExtractDomainAndWait(ctx context.Context, task *Task) (string, error) {
	domain, err := drl.domainExtractor.ExtractDomain(task)
	if err != nil {
		return "", fmt.Errorf("failed to extract domain: %w", err)
	}

	err = drl.Wait(ctx, domain)
	return domain, err
}

// GetMetrics returns rate limiting metrics for all domains
func (drl *DomainRateLimiter) GetMetrics() map[string]DomainRateMetrics {
	drl.mux.RLock()
	defer drl.mux.RUnlock()

	metrics := make(map[string]DomainRateMetrics)

	// Always include default domain
	metrics[drl.defaultConfig.Domain] = DomainRateMetrics{
		Domain:         drl.defaultConfig.Domain,
		RateLimit:      drl.defaultConfig.RateLimit,
		Burst:          drl.defaultConfig.Burst,
		FailureCount:   0,
		CurrentBackoff: 0,
		IsInBackoff:    false,
	}

	// Add custom domains
	for domain, tracker := range drl.backoffTrackers {
		if domain != drl.defaultConfig.Domain { // Skip default as it's already added
			config := drl.GetDomainConfig(domain)
			metrics[domain] = DomainRateMetrics{
				Domain:         domain,
				RateLimit:      config.RateLimit,
				Burst:          config.Burst,
				FailureCount:   tracker.getFailureCount(),
				CurrentBackoff: tracker.getCurrentBackoff(),
				IsInBackoff:    tracker.isInBackoff(),
			}
		}
	}

	return metrics
}

// getBackoffTracker gets or creates a backoff tracker for a domain
func (drl *DomainRateLimiter) getBackoffTracker(domain string) *BackoffTracker {
	drl.mux.RLock()
	tracker, exists := drl.backoffTrackers[domain]
	drl.mux.RUnlock()

	if !exists {
		drl.mux.Lock()
		defer drl.mux.Unlock()

		// Double-check after acquiring write lock
		if tracker, exists = drl.backoffTrackers[domain]; !exists {
			tracker = &BackoffTracker{}
			drl.backoffTrackers[domain] = tracker
		}
	}

	return tracker
}

// DomainRateMetrics holds metrics for domain rate limiting
type DomainRateMetrics struct {
	Domain         string
	RateLimit      float64
	Burst          int
	FailureCount   int
	CurrentBackoff time.Duration
	IsInBackoff    bool
}

// recordFailure records a rate limit failure and increases backoff
func (bt *BackoffTracker) recordFailure(config *DomainRateConfig) {
	bt.mux.Lock()
	defer bt.mux.Unlock()

	bt.LastFailure = time.Now()
	bt.FailureCount++

	// Calculate exponential backoff
	backoff := config.BackoffBase * time.Duration(bt.FailureCount)
	if backoff > config.BackoffMax {
		backoff = config.BackoffMax
	}
	bt.CurrentBackoff = backoff
}

// recordSuccess records a successful request and resets backoff
func (bt *BackoffTracker) recordSuccess() {
	bt.mux.Lock()
	defer bt.mux.Unlock()

	bt.FailureCount = 0
	bt.CurrentBackoff = 0
}

// isInBackoff checks if the domain is currently in a backoff period
func (bt *BackoffTracker) isInBackoff() bool {
	bt.mux.Lock()
	defer bt.mux.Unlock()

	if bt.FailureCount == 0 {
		return false
	}

	// Check if backoff period has expired
	return time.Since(bt.LastFailure) < bt.CurrentBackoff
}

// getCurrentBackoff returns the current backoff duration
func (bt *BackoffTracker) getCurrentBackoff() time.Duration {
	bt.mux.Lock()
	defer bt.mux.Unlock()
	return bt.CurrentBackoff
}

// getFailureCount returns the current failure count
func (bt *BackoffTracker) getFailureCount() int {
	bt.mux.Lock()
	defer bt.mux.Unlock()
	return bt.FailureCount
}
