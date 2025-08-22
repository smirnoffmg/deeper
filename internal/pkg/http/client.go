package http

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/errors"
)

// Client interface for HTTP operations
type Client interface {
	Get(ctx context.Context, url string) (*http.Response, error)
	Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// DefaultClient implements the Client interface with retry logic and rate limiting
type DefaultClient struct {
	client      *http.Client
	config      *config.Config
	rateLimiter *time.Ticker
}

// NewClient creates a new HTTP client with the given configuration
func NewClient(cfg *config.Config) Client {
	client := &http.Client{
		Timeout: cfg.HTTPTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &DefaultClient{
		client:      client,
		config:      cfg,
		rateLimiter: time.NewTicker(time.Duration(1000/cfg.RateLimitPerSecond) * time.Millisecond),
	}
}

// Get performs a GET request with retry logic
func (c *DefaultClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.NewNetworkError("failed to create request", err)
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	return c.Do(req)
}

// Post performs a POST request with retry logic
func (c *DefaultClient) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, errors.NewNetworkError("failed to create request", err)
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// Do performs an HTTP request with retry logic and rate limiting
func (c *DefaultClient) Do(req *http.Request) (*http.Response, error) {
	// Rate limiting
	<-c.rateLimiter.C

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.config.RetryDelay)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = errors.NewNetworkError("request failed", err)
			continue
		}

		// Consider 5xx errors as retryable
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			resp.Body.Close()
			lastErr = errors.NewNetworkError("server error", nil).WithContext("status_code", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, lastErr
}

// Close cleans up resources
func (c *DefaultClient) Close() {
	if c.rateLimiter != nil {
		c.rateLimiter.Stop()
	}
}
