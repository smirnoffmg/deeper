# Domain-Specific Rate Limiting Implementation

## Overview

This document describes the implementation of per-domain rate limiting using token bucket algorithms with automatic backoff for the Deeper OSINT tool. The implementation follows SOLID principles and best practices for maintainable, testable code.

## Architecture

### Core Components

1. **DomainExtractor** (`internal/pkg/workerpool/domain.go`)
   - Extracts domains from different types of traces (emails, URLs, domain-only strings)
   - Follows Single Responsibility Principle (SRP)
   - Provides validation for domain formats

2. **DomainRateLimiter** (`internal/pkg/workerpool/domain_rate_limiter.go`)
   - Manages rate limiting for different domains
   - Implements token bucket algorithm using `golang.org/x/time/rate`
   - Provides automatic backoff with exponential retry
   - Thread-safe with proper mutex management

3. **BackoffTracker** (embedded in DomainRateLimiter)
   - Tracks backoff state for each domain
   - Implements exponential backoff algorithm
   - Manages failure counts and recovery timing

4. **WorkerPool Integration** (`internal/pkg/workerpool/workerpool.go`)
   - Integrates domain rate limiting into the worker pool
   - Replaces old task-based rate limiting with domain-based approach
   - Provides comprehensive metrics and monitoring

## Features Implemented

### ✅ Token Bucket Implementation
- Uses `golang.org/x/time/rate` package for robust token bucket algorithm
- Configurable rate limits and burst parameters per domain
- Thread-safe implementation with proper concurrency control

### ✅ Domain-Specific Rate Configurations
- **Domain Extraction**: Automatically extracts domains from:
  - Email addresses (e.g., `user@example.com` → `example.com`)
  - URLs (e.g., `https://api.github.com/user/repos` → `api.github.com`)
  - Domain-only strings (e.g., `google.com` → `google.com`)
  - Fallback to "default" domain for unrecognized content

- **Per-Domain Configuration**: Each domain can have its own:
  - Rate limit (requests per second)
  - Burst limit (maximum concurrent requests)
  - Backoff base duration
  - Maximum backoff duration
  - Maximum retry attempts

### ✅ Automatic Backoff on Rate Limit Hits
- **Exponential Backoff**: Implements exponential backoff algorithm
- **Failure Tracking**: Tracks failure counts per domain
- **Recovery**: Automatically resets backoff on successful requests
- **Configurable**: Backoff parameters can be set per domain

### ✅ Rate Limit Status Monitoring
- **Comprehensive Metrics**: Tracks for each domain:
  - Current rate limit and burst settings
  - Failure count
  - Current backoff duration
  - Backoff status (active/inactive)
- **Real-time Monitoring**: Metrics are updated in real-time
- **Integration**: Metrics are available through worker pool metrics

## Configuration

### Environment Variables

```bash
# Worker Pool Configuration
DEEPER_WORKER_POOL_MAX_WORKERS=20
DEEPER_WORKER_POOL_QUEUE_SIZE=1000
DEEPER_WORKER_POOL_RATE_LIMIT=10.0
DEEPER_WORKER_POOL_BURST=5
DEEPER_WORKER_POOL_TASK_TIMEOUT=30s
DEEPER_WORKER_POOL_ENABLE_DEDUP=true
DEEPER_WORKER_POOL_ENABLE_METRICS=true

# Domain-Specific Rate Limiting Configuration
# Format: DEEPER_DOMAIN_RATE_<DOMAIN>_<PARAMETER>
DEEPER_DOMAIN_RATE_GITHUB_COM_RATE_LIMIT=5.0
DEEPER_DOMAIN_RATE_GITHUB_COM_BURST=2
DEEPER_DOMAIN_RATE_GITHUB_COM_BACKOFF_BASE=2s
DEEPER_DOMAIN_RATE_GITHUB_COM_BACKOFF_MAX=120s
DEEPER_DOMAIN_RATE_GITHUB_COM_MAX_RETRIES=3

DEEPER_DOMAIN_RATE_API_GITHUB_COM_RATE_LIMIT=2.0
DEEPER_DOMAIN_RATE_API_GITHUB_COM_BURST=1
DEEPER_DOMAIN_RATE_API_GITHUB_COM_BACKOFF_BASE=5s
DEEPER_DOMAIN_RATE_API_GITHUB_COM_BACKOFF_MAX=300s
DEEPER_DOMAIN_RATE_API_GITHUB_COM_MAX_RETRIES=5
```

### Programmatic Configuration

```go
// Configure domain-specific rate limiting
err := processor.ConfigureDomainRateLimit(
    "api.github.com",    // domain
    2.0,                 // rate limit (requests per second)
    1,                   // burst limit
    5*time.Second,       // backoff base
    300*time.Second,     // backoff max
    5,                   // max retries
)
```

## CLI Commands

### Rate Limit Configuration

```bash
# Configure rate limiting for a specific domain
./deeper rate-limit --domain api.github.com --rate 2.0 --burst 1 --backoff-base 5s --backoff-max 300s --max-retries 5

# List current domain rate limit configurations
./deeper rate-limit --list

# Use predefined configurations
./deeper rate-limit github  # Configure GitHub API rate limiting
./deeper rate-limit google  # Configure Google API rate limiting
```

## Usage Examples

### Basic Usage

```go
// The domain rate limiting is automatically applied when submitting tasks
task := &Task{
    ID:      "github-user",
    Payload: "https://api.github.com/users/octocat",
}

err := workerPool.Submit(ctx, task)
// Domain "api.github.com" will be automatically extracted and rate limited
```

### Advanced Configuration

```go
// Configure multiple domains with different settings
domains := []struct {
    domain      string
    rateLimit   float64
    burst       int
    backoffBase time.Duration
    backoffMax  time.Duration
    maxRetries  int
}{
    {"api.github.com", 2.0, 1, 5*time.Second, 300*time.Second, 5},
    {"google.com", 3.0, 1, 3*time.Second, 180*time.Second, 4},
    {"example.com", 10.0, 5, 1*time.Second, 60*time.Second, 3},
}

for _, d := range domains {
    err := processor.ConfigureDomainRateLimit(
        d.domain, d.rateLimit, d.burst, 
        d.backoffBase, d.backoffMax, d.maxRetries,
    )
    if err != nil {
        log.Error().Err(err).Str("domain", d.domain).Msg("Failed to configure domain rate limit")
    }
}
```

## Monitoring and Metrics

### Metrics Structure

```go
type DomainRateMetrics struct {
    Domain         string        // Domain name
    RateLimit      float64       // Current rate limit
    Burst          int           // Current burst limit
    FailureCount   int           // Number of failures
    CurrentBackoff time.Duration // Current backoff duration
    IsInBackoff    bool          // Whether domain is in backoff
}
```

### Accessing Metrics

```go
// Get worker pool metrics including domain rate limiting
metrics := processor.GetWorkerPoolMetrics()

// Access domain-specific metrics
for domain, domainMetrics := range metrics.DomainRateMetrics {
    log.Info().
        Str("domain", domain).
        Float64("rateLimit", domainMetrics.RateLimit).
        Int("burst", domainMetrics.Burst).
        Int("failureCount", domainMetrics.FailureCount).
        Bool("inBackoff", domainMetrics.IsInBackoff).
        Msg("Domain rate limiting status")
}
```

## Testing

### Test Coverage

The implementation includes comprehensive test coverage:

- **Domain Extraction Tests**: Tests for email, URL, and domain-only extraction
- **Rate Limiting Tests**: Tests for token bucket behavior and rate limiting
- **Backoff Tests**: Tests for exponential backoff and recovery
- **Integration Tests**: Tests for worker pool integration
- **Metrics Tests**: Tests for monitoring and metrics collection

### Running Tests

```bash
# Run all worker pool tests
go test ./internal/pkg/workerpool/ -v

# Run specific test categories
go test ./internal/pkg/workerpool/ -run TestDomainExtractor
go test ./internal/pkg/workerpool/ -run TestDomainRateLimiter
go test ./internal/pkg/workerpool/ -run TestWorkerPool
```

## Best Practices Followed

### SOLID Principles

1. **Single Responsibility Principle (SRP)**
   - `DomainExtractor`: Only responsible for domain extraction
   - `DomainRateLimiter`: Only responsible for rate limiting
   - `BackoffTracker`: Only responsible for backoff management

2. **Open/Closed Principle (OCP)**
   - Rate limiting is extensible through configuration
   - New domain types can be added without modifying existing code

3. **Liskov Substitution Principle (LSP)**
   - All rate limiters follow the same interface contract
   - Default and domain-specific limiters are interchangeable

4. **Interface Segregation Principle (ISP)**
   - Focused interfaces for specific use cases
   - No unnecessary dependencies

5. **Dependency Inversion Principle (DIP)**
   - Depend on abstractions (interfaces) rather than concrete implementations
   - Easy to test and mock

### Error Handling

- **Structured Errors**: Uses custom error types for different failure scenarios
- **Graceful Degradation**: Continues operation even when rate limiting fails
- **Context Preservation**: Error messages include relevant context

### Performance

- **Efficient Algorithms**: Uses proven token bucket algorithm
- **Memory Management**: Proper cleanup and resource management
- **Concurrency**: Thread-safe implementation with minimal contention

### Security

- **Input Validation**: Validates all domain inputs
- **Rate Limiting**: Prevents abuse of external APIs
- **Error Information**: Doesn't leak sensitive information in error messages

## Benefits

1. **Prevents API Abuse**: Domain-specific rate limiting prevents hitting API limits
2. **Automatic Recovery**: Exponential backoff allows services to recover
3. **Configurable**: Flexible configuration for different domains and services
4. **Monitoring**: Comprehensive metrics for operational visibility
5. **Maintainable**: Clean, testable code following best practices
6. **Scalable**: Efficient implementation that scales with load

## Future Enhancements

1. **Dynamic Configuration**: Runtime configuration updates without restart
2. **Learning Rate Limits**: Automatically adjust rate limits based on API responses
3. **Distributed Rate Limiting**: Support for distributed rate limiting across multiple instances
4. **Advanced Metrics**: More detailed metrics and alerting capabilities
5. **Rate Limit Discovery**: Automatically discover rate limits from API responses
