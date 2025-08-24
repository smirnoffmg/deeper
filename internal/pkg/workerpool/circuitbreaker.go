package workerpool

import (
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config CircuitBreakerConfig
	state  int32 // CircuitBreakerState

	// Failure tracking
	failureCount    int64
	lastFailureTime time.Time
	failureMux      sync.RWMutex

	// Half-open state tracking
	halfOpenCalls int64
	halfOpenMux   sync.RWMutex

	// Success tracking for recovery
	successCount    int64
	lastSuccessTime time.Time
	successMux      sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.RecoveryTimeout <= 0 {
		config.RecoveryTimeout = 60 * time.Second
	}
	if config.HalfOpenMaxCalls <= 0 {
		config.HalfOpenMaxCalls = 3
	}
	if config.WindowSize <= 0 {
		config.WindowSize = 60 * time.Second
	}

	return &CircuitBreaker{
		config: config,
		state:  int32(StateClosed),
	}
}

// IsOpen returns true if the circuit breaker is in the open state
func (cb *CircuitBreaker) IsOpen() bool {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateOpen:
		return true
	case StateHalfOpen:
		return cb.shouldRemainHalfOpen()
	case StateClosed:
		return cb.shouldOpen()
	default:
		return false
	}
}

// RecordResult records the result of an operation
func (cb *CircuitBreaker) RecordResult(success bool) {
	if success {
		cb.recordSuccess()
	} else {
		cb.recordFailure()
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&cb.state))
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.failureMux.RLock()
	cb.successMux.RLock()
	cb.halfOpenMux.RLock()
	defer cb.failureMux.RUnlock()
	defer cb.successMux.RUnlock()
	defer cb.halfOpenMux.RUnlock()

	return CircuitBreakerStats{
		State:           cb.GetState(),
		FailureCount:    atomic.LoadInt64(&cb.failureCount),
		SuccessCount:    atomic.LoadInt64(&cb.successCount),
		LastFailureTime: cb.lastFailureTime,
		LastSuccessTime: cb.lastSuccessTime,
		HalfOpenCalls:   atomic.LoadInt64(&cb.halfOpenCalls),
	}
}

// CircuitBreakerStats holds circuit breaker statistics
type CircuitBreakerStats struct {
	State           CircuitBreakerState
	FailureCount    int64
	SuccessCount    int64
	LastFailureTime time.Time
	LastSuccessTime time.Time
	HalfOpenCalls   int64
}

// recordSuccess records a successful operation
func (cb *CircuitBreaker) recordSuccess() {
	cb.successMux.Lock()
	defer cb.successMux.Unlock()

	cb.lastSuccessTime = time.Now()
	atomic.AddInt64(&cb.successCount, 1)

	// Reset failure count on success
	atomic.StoreInt64(&cb.failureCount, 0)

	// Transition to closed state if currently half-open
	currentState := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	if currentState == StateHalfOpen {
		atomic.StoreInt32(&cb.state, int32(StateClosed))
		atomic.StoreInt64(&cb.halfOpenCalls, 0)
	}
}

// recordFailure records a failed operation
func (cb *CircuitBreaker) recordFailure() {
	cb.failureMux.Lock()
	cb.lastFailureTime = time.Now()
	atomic.AddInt64(&cb.failureCount, 1)
	cb.failureMux.Unlock()

	// Check if we should transition to open state (outside of lock)
	if cb.shouldOpen() {
		atomic.StoreInt32(&cb.state, int32(StateOpen))
	}
}

// shouldOpen determines if the circuit breaker should transition to open state
func (cb *CircuitBreaker) shouldOpen() bool {
	failureCount := atomic.LoadInt64(&cb.failureCount)
	if failureCount >= int64(cb.config.FailureThreshold) {
		// Check if we're within the window size
		cb.failureMux.RLock()
		timeSinceLastFailure := time.Since(cb.lastFailureTime)
		cb.failureMux.RUnlock()

		if timeSinceLastFailure <= cb.config.WindowSize {
			return true
		}
	}
	return false
}

// shouldRemainHalfOpen determines if the circuit breaker should remain in half-open state
func (cb *CircuitBreaker) shouldRemainHalfOpen() bool {
	halfOpenCalls := atomic.LoadInt64(&cb.halfOpenCalls)
	return halfOpenCalls < int64(cb.config.HalfOpenMaxCalls)
}

// tryHalfOpen attempts to transition to half-open state
func (cb *CircuitBreaker) tryHalfOpen() bool {
	currentState := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	if currentState != StateOpen {
		return false
	}

	// Check if recovery timeout has passed
	cb.failureMux.RLock()
	timeSinceLastFailure := time.Since(cb.lastFailureTime)
	cb.failureMux.RUnlock()

	if timeSinceLastFailure < cb.config.RecoveryTimeout {
		return false
	}

	// Try to transition to half-open state
	if atomic.CompareAndSwapInt32(&cb.state, int32(StateOpen), int32(StateHalfOpen)) {
		atomic.StoreInt64(&cb.halfOpenCalls, 0)
		return true
	}

	return false
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Check if circuit breaker is open
	if cb.IsOpen() {
		// Try to transition to half-open state
		if !cb.tryHalfOpen() {
			return ErrCircuitBreakerOpen
		}
	}

	// Increment half-open calls if in half-open state
	if cb.GetState() == StateHalfOpen {
		atomic.AddInt64(&cb.halfOpenCalls, 1)
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.RecordResult(err == nil)

	return err
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreInt64(&cb.failureCount, 0)
	atomic.StoreInt64(&cb.successCount, 0)
	atomic.StoreInt64(&cb.halfOpenCalls, 0)
}
