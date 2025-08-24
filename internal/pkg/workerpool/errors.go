package workerpool

import (
	"errors"
)

var (
	// ErrCircuitBreakerOpen is returned when the circuit breaker is in open state
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	
	// ErrWorkerPoolShutdown is returned when the worker pool is shutting down
	ErrWorkerPoolShutdown = errors.New("worker pool is shutting down")
	
	// ErrTaskTimeout is returned when a task processing times out
	ErrTaskTimeout = errors.New("task processing timeout")
	
	// ErrQueueFull is returned when the task queue is full
	ErrQueueFull = errors.New("task queue is full")
)
