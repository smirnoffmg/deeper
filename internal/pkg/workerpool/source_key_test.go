package workerpool

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func failingPoolConfig(recovery time.Duration) *Config {
	return &Config{
		MaxWorkers:       2,
		QueueSize:        10,
		DefaultRateLimit: rate.Limit(1000),
		DefaultBurst:     100,
		TaskTimeout:      time.Second,
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 2,
			RecoveryTimeout:  recovery,
			HalfOpenMaxCalls: 1,
			WindowSize:       time.Minute,
		},
		TaskHandler: func(_ context.Context, task *Task) (interface{}, error) {
			if task.Payload == "ok" {
				return "ok", nil
			}
			return nil, errors.New("source down")
		},
	}
}

func submitAndWait(t *testing.T, wp *WorkerPool, task *Task) (*TaskResult, error) {
	t.Helper()
	task.ReplyTo = make(chan *TaskResult, 1)
	if err := wp.Submit(context.Background(), task); err != nil {
		return nil, err
	}
	select {
	case r := <-task.ReplyTo:
		return r, nil
	case <-time.After(2 * time.Second):
		t.Fatal("no reply")
		return nil, nil
	}
}

// Every trace yields a distinct task ID, so a breaker keyed by ID never
// accumulates failures; tasks sharing a SourceKey must trip it together.
func TestSubmit_BreakerTripsAcrossDistinctTasksSharingKey(t *testing.T) {
	wp := NewWorkerPool(failingPoolConfig(time.Minute))
	defer func() { _ = wp.Shutdown(time.Second) }()

	for i := 0; i < 2; i++ {
		_, err := submitAndWait(t, wp, &Task{ID: fmt.Sprintf("trace-%d:crtsh", i), SourceKey: "crtsh", Payload: "x"})
		require.NoError(t, err)
	}

	_, err := submitAndWait(t, wp, &Task{ID: "trace-new:crtsh", SourceKey: "crtsh", Payload: "x"})
	require.ErrorIs(t, err, ErrCircuitBreakerOpen)

	_, err = submitAndWait(t, wp, &Task{ID: "trace-new:github", SourceKey: "github", Payload: "ok"})
	require.NoError(t, err)
}

func TestSubmit_OpenBreakerLetsProbeThroughAfterRecovery(t *testing.T) {
	wp := NewWorkerPool(failingPoolConfig(50 * time.Millisecond))
	defer func() { _ = wp.Shutdown(time.Second) }()

	for i := 0; i < 2; i++ {
		_, err := submitAndWait(t, wp, &Task{ID: fmt.Sprintf("t%d", i), SourceKey: "src", Payload: "x"})
		require.NoError(t, err)
	}
	_, err := submitAndWait(t, wp, &Task{ID: "t2", SourceKey: "src", Payload: "ok"})
	require.ErrorIs(t, err, ErrCircuitBreakerOpen)

	time.Sleep(80 * time.Millisecond)

	res, err := submitAndWait(t, wp, &Task{ID: "t3", SourceKey: "src", Payload: "ok"})
	require.NoError(t, err)
	require.NoError(t, res.Error)

	_, err = submitAndWait(t, wp, &Task{ID: "t4", SourceKey: "src", Payload: "ok"})
	require.NoError(t, err, "a successful probe must close the breaker")
}

func TestSubmit_RateLimitsEachSourceKeyIndependently(t *testing.T) {
	cfg := failingPoolConfig(time.Minute)
	cfg.DefaultRateLimit = rate.Limit(1)
	cfg.DefaultBurst = 1
	wp := NewWorkerPool(cfg)
	defer func() { _ = wp.Shutdown(time.Second) }()

	submitWithin := func(id, key string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		return wp.Submit(ctx, &Task{ID: id, SourceKey: key, Payload: "ok"})
	}

	require.NoError(t, submitWithin("a1", "crtsh"))
	require.NoError(t, submitWithin("b1", "github"), "another plugin must not wait on crtsh's bucket")
	require.Error(t, submitWithin("a2", "crtsh"), "a second crtsh task within its interval must wait")
}
