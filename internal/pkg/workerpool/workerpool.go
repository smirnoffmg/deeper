package workerpool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// Task represents a unit of work to be processed
type Task struct {
	ID       string
	Payload  interface{}
	Priority int
	Created  time.Time
}

// TaskResult represents the result of processing a task
type TaskResult struct {
	TaskID   string
	Result   interface{}
	Error    error
	Duration time.Duration
}

// WorkerPool manages a pool of workers for concurrent task processing
type WorkerPool struct {
	config             *Config
	workers            int
	taskQueue          chan *Task
	resultQueue        chan *TaskResult
	workerPool         sync.Pool
	domainRateLimiter  *DomainRateLimiter
	deduplicationCache *DeduplicationCache
	circuitBreakers    map[string]*CircuitBreaker
	circuitMux         sync.RWMutex
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	activeWorkers      int64
	processedTasks     int64
	failedTasks        int64
	metrics            *Metrics
}

// Config holds worker pool configuration
type Config struct {
	MaxWorkers           int
	QueueSize            int
	DefaultRateLimit     rate.Limit
	DefaultBurst         int
	CircuitBreakerConfig CircuitBreakerConfig
	TaskTimeout          time.Duration
	EnableDeduplication  bool
	EnableMetrics        bool
	DeduplicationConfig  DeduplicationConfig
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	HalfOpenMaxCalls int
	WindowSize       time.Duration
}

// Metrics holds worker pool metrics
type Metrics struct {
	ActiveWorkers        int64
	ProcessedTasks       int64
	FailedTasks          int64
	QueueSize            int
	QueueCapacity        int
	RateLimitHits        int64
	DeduplicationHits    int64
	CircuitBreakerTrips  int64
	DomainRateMetrics    map[string]DomainRateMetrics
	DeduplicationMetrics *DeduplicationMetrics
}

// NewWorkerPool creates a new worker pool with the given configuration
func NewWorkerPool(config *Config) *WorkerPool {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 10
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}
	if config.TaskTimeout <= 0 {
		config.TaskTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create default domain rate limiter configuration
	defaultDomainConfig := &DomainRateConfig{
		Domain:      "default",
		RateLimit:   float64(config.DefaultRateLimit),
		Burst:       config.DefaultBurst,
		BackoffBase: 1 * time.Second,
		BackoffMax:  60 * time.Second,
		MaxRetries:  3,
	}

	domainRateLimiter := NewDomainRateLimiter(defaultDomainConfig)

	wp := &WorkerPool{
		config:             config,
		workers:            config.MaxWorkers,
		taskQueue:          make(chan *Task, config.QueueSize),
		resultQueue:        make(chan *TaskResult, config.QueueSize),
		domainRateLimiter:  domainRateLimiter,
		deduplicationCache: nil, // Will be initialized after database cache is available
		circuitBreakers:    make(map[string]*CircuitBreaker),
		ctx:                ctx,
		cancel:             cancel,
		metrics:            &Metrics{},
	}

	// Initialize worker pool
	wp.workerPool.New = func() interface{} {
		return &Worker{
			pool: wp,
		}
	}

	// Start workers
	wp.startWorkers()

	return wp
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(ctx context.Context, task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Generate task ID if not provided
	if task.ID == "" {
		// Use a simple hash of the payload for task ID
		task.ID = fmt.Sprintf("%v", task.Payload)
	}

	// Check deduplication if enabled
	if wp.config.EnableDeduplication && wp.deduplicationCache != nil {
		isDuplicate, err := wp.deduplicationCache.IsDuplicate(ctx, task)
		if err != nil {
			log.Warn().Err(err).Str("taskID", task.ID).Msg("Failed to check deduplication")
		} else if isDuplicate {
			log.Debug().Str("taskID", task.ID).Msg("Task deduplicated")
			atomic.AddInt64(&wp.metrics.DeduplicationHits, 1)
			return nil
		}
	}

	// Check circuit breaker
	if cb := wp.getCircuitBreaker(task.ID); cb != nil && cb.IsOpen() {
		log.Warn().Str("taskID", task.ID).Msg("Circuit breaker is open, rejecting task")
		atomic.AddInt64(&wp.metrics.CircuitBreakerTrips, 1)
		return fmt.Errorf("circuit breaker is open for task %s", task.ID)
	}

	// Apply domain-specific rate limiting with backoff
	domain, err := wp.domainRateLimiter.ExtractDomainAndWait(ctx, task)
	if err != nil {
		log.Debug().Str("taskID", task.ID).Str("domain", domain).Msg("Rate limit exceeded")
		atomic.AddInt64(&wp.metrics.RateLimitHits, 1)
		return fmt.Errorf("rate limit exceeded for domain %s: %w", domain, err)
	}

	// Set creation time
	task.Created = time.Now()

	// Submit task to queue
	select {
	case wp.taskQueue <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("worker pool queue is full")
	}
}

// GetResult retrieves a result from the result queue
func (wp *WorkerPool) GetResult(ctx context.Context) (*TaskResult, error) {
	select {
	case result := <-wp.resultQueue:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetMetrics returns current worker pool metrics
func (wp *WorkerPool) GetMetrics() *Metrics {
	wp.metrics.ActiveWorkers = atomic.LoadInt64(&wp.activeWorkers)
	wp.metrics.QueueSize = len(wp.taskQueue)
	wp.metrics.QueueCapacity = cap(wp.taskQueue)
	wp.metrics.DomainRateMetrics = wp.domainRateLimiter.GetMetrics()

	// Get deduplication metrics if available
	if wp.deduplicationCache != nil {
		wp.metrics.DeduplicationMetrics = wp.deduplicationCache.GetMetrics()
	}

	return wp.metrics
}

// ConfigureDomainRateLimit configures rate limiting for a specific domain
func (wp *WorkerPool) ConfigureDomainRateLimit(config *DomainRateConfig) error {
	return wp.domainRateLimiter.AddDomainConfig(config)
}

// Shutdown gracefully shuts down the worker pool
func (wp *WorkerPool) Shutdown(timeout time.Duration) error {
	wp.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("worker pool shutdown timeout")
	}
}

// startWorkers starts the worker goroutines
func (wp *WorkerPool) startWorkers() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go func(workerID int) {
			defer wp.wg.Done()
			worker := wp.workerPool.Get().(*Worker)
			worker.workerID = workerID
			worker.run()
			wp.workerPool.Put(worker)
		}(i)
	}
}

// SetDeduplicationCache sets the deduplication cache for the worker pool
func (wp *WorkerPool) SetDeduplicationCache(cache *DeduplicationCache) {
	wp.deduplicationCache = cache
}

// getCircuitBreaker gets or creates a circuit breaker for the given key
func (wp *WorkerPool) getCircuitBreaker(key string) *CircuitBreaker {
	wp.circuitMux.RLock()
	if cb, exists := wp.circuitBreakers[key]; exists {
		wp.circuitMux.RUnlock()
		return cb
	}
	wp.circuitMux.RUnlock()

	// Create new circuit breaker
	wp.circuitMux.Lock()
	defer wp.circuitMux.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := wp.circuitBreakers[key]; exists {
		return cb
	}

	cb := NewCircuitBreaker(wp.config.CircuitBreakerConfig)
	wp.circuitBreakers[key] = cb
	return cb
}

// recordTaskResult records task processing results
func (wp *WorkerPool) recordTaskResult(result *TaskResult) {
	atomic.AddInt64(&wp.processedTasks, 1)
	if result.Error != nil {
		atomic.AddInt64(&wp.failedTasks, 1)
	}

	// Update circuit breaker
	if cb := wp.getCircuitBreaker(result.TaskID); cb != nil {
		cb.RecordResult(result.Error == nil)
	}
}

// Worker represents a single worker in the pool
type Worker struct {
	pool     *WorkerPool
	workerID int
}

// run runs the worker loop
func (w *Worker) run() {
	for {
		select {
		case task := <-w.pool.taskQueue:
			w.processTask(task)
		case <-w.pool.ctx.Done():
			return
		}
	}
}

// processTask processes a single task
func (w *Worker) processTask(task *Task) {
	atomic.AddInt64(&w.pool.activeWorkers, 1)
	defer atomic.AddInt64(&w.pool.activeWorkers, -1)

	startTime := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(w.pool.ctx, w.pool.config.TaskTimeout)
	defer cancel()

	// Process the task (this would be replaced with actual task processing logic)
	var err error
	if task.ID == "failing-task" {
		err = fmt.Errorf("simulated failure for testing")
	}

	result := &TaskResult{
		TaskID:   task.ID,
		Result:   task.Payload,
		Error:    err,
		Duration: time.Since(startTime),
	}

	// Record the result
	w.pool.recordTaskResult(result)

	// Send result to result queue
	select {
	case w.pool.resultQueue <- result:
	case <-ctx.Done():
		log.Warn().Str("taskID", task.ID).Msg("Failed to send task result")
	}
}
