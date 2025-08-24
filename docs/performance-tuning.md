# Performance Tuning Guide for Deeper OSINT Tool

## Overview

This guide provides comprehensive recommendations for tuning the performance of the Deeper OSINT tool, focusing on the new worker pool architecture that replaces the unlimited goroutine spawning with controlled concurrency.

## Key Performance Improvements

### 1. Worker Pool Architecture

The new worker pool implementation provides:
- **Controlled Concurrency**: Fixed number of worker goroutines instead of unlimited spawning
- **Request Deduplication**: Content-addressable hashing to prevent duplicate work
- **Rate Limiting**: Per-domain token bucket algorithms
- **Circuit Breaker**: Automatic failure detection and recovery
- **Resource Management**: Proper cleanup and memory management

### 2. Configuration Parameters

#### Worker Pool Configuration

| Parameter                           | Default | Description                          | Tuning Guidelines                                                |
| ----------------------------------- | ------- | ------------------------------------ | ---------------------------------------------------------------- |
| `DEEPER_WORKER_POOL_MAX_WORKERS`    | 20      | Maximum number of concurrent workers | Start with CPU cores × 2, increase based on I/O bound operations |
| `DEEPER_WORKER_POOL_QUEUE_SIZE`     | 1000    | Size of task queue                   | Should be 2-3x the number of workers for optimal throughput      |
| `DEEPER_WORKER_POOL_RATE_LIMIT`     | 10.0    | Default rate limit (requests/second) | Adjust based on external API limits                              |
| `DEEPER_WORKER_POOL_BURST`          | 5       | Burst allowance for rate limiting    | Usually 50% of rate limit for responsive behavior                |
| `DEEPER_WORKER_POOL_TASK_TIMEOUT`   | 30s     | Maximum time for task processing     | Set based on slowest plugin response time                        |
| `DEEPER_WORKER_POOL_ENABLE_DEDUP`   | true    | Enable request deduplication         | Always enable for production                                     |
| `DEEPER_WORKER_POOL_ENABLE_METRICS` | true    | Enable performance metrics           | Enable for monitoring and tuning                                 |

#### Circuit Breaker Configuration

| Parameter                                    | Default | Description                      | Tuning Guidelines                                      |
| -------------------------------------------- | ------- | -------------------------------- | ------------------------------------------------------ |
| `DEEPER_CIRCUIT_BREAKER_FAILURE_THRESHOLD`   | 5       | Failures before opening circuit  | Lower for critical services, higher for resilient ones |
| `DEEPER_CIRCUIT_BREAKER_RECOVERY_TIMEOUT`    | 60s     | Time before attempting recovery  | Should be longer than typical service recovery time    |
| `DEEPER_CIRCUIT_BREAKER_HALF_OPEN_MAX_CALLS` | 3       | Test calls in half-open state    | 2-5 calls for adequate testing                         |
| `DEEPER_CIRCUIT_BREAKER_WINDOW_SIZE`         | 60s     | Time window for failure counting | Should match typical request patterns                  |

## Performance Tuning Scenarios

### Scenario 1: High-Throughput Processing

**Goal**: Maximize traces processed per second

**Configuration**:
```bash
export DEEPER_WORKER_POOL_MAX_WORKERS=50
export DEEPER_WORKER_POOL_QUEUE_SIZE=2000
export DEEPER_WORKER_POOL_RATE_LIMIT=50.0
export DEEPER_WORKER_POOL_BURST=25
export DEEPER_WORKER_POOL_TASK_TIMEOUT=15s
```

**Considerations**:
- Monitor CPU and memory usage
- Ensure external APIs can handle increased load
- Watch for rate limiting from external services

### Scenario 2: Resource-Constrained Environment

**Goal**: Minimize resource usage while maintaining reasonable performance

**Configuration**:
```bash
export DEEPER_WORKER_POOL_MAX_WORKERS=5
export DEEPER_WORKER_POOL_QUEUE_SIZE=100
export DEEPER_WORKER_POOL_RATE_LIMIT=2.0
export DEEPER_WORKER_POOL_BURST=1
export DEEPER_WORKER_POOL_TASK_TIMEOUT=60s
```

**Considerations**:
- Lower concurrency reduces memory usage
- Higher timeouts accommodate slower processing
- Reduced rate limits prevent overwhelming external services

### Scenario 3: Unreliable External Services

**Goal**: Handle frequent external service failures gracefully

**Configuration**:
```bash
export DEEPER_CIRCUIT_BREAKER_FAILURE_THRESHOLD=3
export DEEPER_CIRCUIT_BREAKER_RECOVERY_TIMEOUT=120s
export DEEPER_CIRCUIT_BREAKER_HALF_OPEN_MAX_CALLS=2
export DEEPER_CIRCUIT_BREAKER_WINDOW_SIZE=30s
export DEEPER_WORKER_POOL_MAX_WORKERS=10
```

**Considerations**:
- Lower failure threshold for faster circuit opening
- Longer recovery timeout to allow service stabilization
- Reduced concurrency to minimize cascading failures

## Monitoring and Metrics

### Key Metrics to Monitor

1. **Worker Pool Metrics**:
   - Active workers count
   - Queue size and capacity
   - Rate limit hits
   - Deduplication hits
   - Circuit breaker trips

2. **Performance Metrics**:
   - Traces processed per second
   - Error rates
   - Response times
   - Memory usage

3. **External Service Metrics**:
   - API response times
   - Rate limit violations
   - Service availability

### Using the Benchmark Command

Run comprehensive performance tests:

```bash
# Basic benchmark
./deeper benchmark --traces 1000

# Test different concurrency levels
./deeper benchmark --traces 1000 --concurrency 5,10,20,50

# Test different rate limits
./deeper benchmark --traces 1000 --rate-limits 1,5,10,20

# Test circuit breaker behavior
./deeper benchmark --traces 1000 --failure-rates 0.1,0.2,0.5
```

## Best Practices

### 1. Start Conservative

Begin with conservative settings and gradually increase based on:
- System resources (CPU, memory, network)
- External API limits
- Performance requirements

### 2. Monitor Continuously

Use the built-in metrics to monitor:
- Worker pool utilization
- Error rates and types
- External service health
- Resource consumption

### 3. Test with Real Data

Benchmark with realistic data volumes and patterns:
- Use actual trace types and values
- Test with various plugin combinations
- Simulate real-world usage patterns

### 4. Gradual Scaling

When scaling up:
1. Increase worker count first
2. Adjust queue size proportionally
3. Monitor external service impact
4. Fine-tune rate limits based on API responses

### 5. Circuit Breaker Tuning

Adjust circuit breaker settings based on:
- External service reliability
- Failure patterns (intermittent vs. persistent)
- Recovery characteristics
- Business impact of failures

## Troubleshooting

### Common Issues

1. **High Memory Usage**:
   - Reduce worker count
   - Decrease queue size
   - Enable deduplication
   - Monitor for memory leaks

2. **Low Throughput**:
   - Increase worker count
   - Check external API limits
   - Verify circuit breaker settings
   - Monitor for bottlenecks

3. **High Error Rates**:
   - Check external service health
   - Adjust circuit breaker thresholds
   - Review rate limiting settings
   - Verify network connectivity

4. **Resource Exhaustion**:
   - Reduce concurrency
   - Implement proper cleanup
   - Monitor resource usage
   - Consider horizontal scaling

### Performance Profiling

Use Go's built-in profiling tools:

```bash
# CPU profiling
go tool pprof -http=:8080 cpu.prof

# Memory profiling
go tool pprof -http=:8080 mem.prof

# Goroutine profiling
go tool pprof -http=:8080 goroutine.prof
```

## Conclusion

The new worker pool architecture provides significant performance improvements while preventing resource exhaustion. Use this guide to tune the configuration for your specific use case and monitor performance continuously to ensure optimal operation.

For additional support or questions, refer to the project documentation or create an issue in the project repository.
