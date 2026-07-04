package processor

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

const testConcurrencyTraceType entities.TraceType = "test_concurrency"

type slowPlugin struct {
	name  string
	delay time.Duration
}

func (p *slowPlugin) Register() error {
	state.RegisterPlugin(testConcurrencyTraceType, p)
	return nil
}

func (p *slowPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	time.Sleep(p.delay)
	return []entities.Trace{{Value: p.name, Type: trace.Type}}, nil
}

func (p *slowPlugin) String() string {
	return p.name
}

func TestProcessor_ProcessTrace_Concurrency(t *testing.T) {
	original := state.ActivePlugins[testConcurrencyTraceType]
	t.Cleanup(func() {
		if original == nil {
			delete(state.ActivePlugins, testConcurrencyTraceType)
			return
		}
		state.ActivePlugins[testConcurrencyTraceType] = original
	})

	const (
		pluginCount = 4
		sleepDelay  = 100 * time.Millisecond
		maxWorkers  = 2
	)

	state.ActivePlugins[testConcurrencyTraceType] = nil
	for i := 0; i < pluginCount; i++ {
		plugin := &slowPlugin{name: fmt.Sprintf("slow-%d", i), delay: sleepDelay}
		require.NoError(t, plugin.Register())
	}

	cfg := config.DefaultConfig()
	cfg.WorkerPoolConfig.MaxWorkers = maxWorkers
	cfg.WorkerPoolConfig.EnableDeduplication = false

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := database.NewRepository(db)
	cache := database.NewCache(repo)
	proc := NewProcessor(cfg, metrics.GetGlobalMetrics(), repo, cache)
	defer func() { _ = proc.Shutdown(5 * time.Second) }()

	trace := entities.Trace{Value: "target", Type: testConcurrencyTraceType}

	start := time.Now()
	results, err := proc.ProcessTrace(context.Background(), trace)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, results, pluginCount)

	pluginNames := make(map[string]bool)
	for _, d := range results {
		pluginNames[d.PluginName] = true
		assert.Equal(t, trace, d.Parent)
		assert.NotEmpty(t, d.Child.Value)
	}
	assert.Len(t, pluginNames, pluginCount)

	sequentialTime := time.Duration(pluginCount) * sleepDelay
	parallelTime := time.Duration((pluginCount+maxWorkers-1)/maxWorkers) * sleepDelay
	assert.Less(t, elapsed, sequentialTime, "expected parallel execution faster than sequential")
	assert.GreaterOrEqual(t, elapsed, parallelTime-time.Millisecond*20, "expected at least parallel lower bound")
}

const echoTraceType entities.TraceType = "test_echo_attribution"

// echoPlugin returns a child trace that encodes exactly which parent trace it
// was given, so concurrent ProcessTrace calls can be checked for cross-talk.
type echoPlugin struct{}

func (p *echoPlugin) Register() error {
	state.RegisterPlugin(echoTraceType, p)
	return nil
}

func (p *echoPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	time.Sleep(20 * time.Millisecond)
	return []entities.Trace{{Value: "resolved-for:" + trace.Value, Type: entities.IpAddr}}, nil
}

func (p *echoPlugin) String() string {
	return "EchoPlugin"
}

// TestProcessor_ConcurrentProcessTrace_NoCrossAttribution is a regression test
// for a race where GetResult()'s shared, uncorrelated result queue let one
// concurrent ProcessTrace call consume a result meant for another trace
// entirely (reproduced live against codescoring.ru: DNS results ended up
// attributed to the wrong subdomain). Each ProcessTrace call must only ever
// see results for tasks it itself submitted.
func TestProcessor_ConcurrentProcessTrace_NoCrossAttribution(t *testing.T) {
	original := state.ActivePlugins[echoTraceType]
	t.Cleanup(func() {
		if original == nil {
			delete(state.ActivePlugins, echoTraceType)
			return
		}
		state.ActivePlugins[echoTraceType] = original
	})
	state.ActivePlugins[echoTraceType] = nil
	require.NoError(t, (&echoPlugin{}).Register())

	cfg := config.DefaultConfig()
	cfg.WorkerPoolConfig.MaxWorkers = 4
	cfg.WorkerPoolConfig.EnableDeduplication = false

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := database.NewRepository(db)
	cache := database.NewCache(repo)
	proc := NewProcessor(cfg, metrics.GetGlobalMetrics(), repo, cache)
	defer func() { _ = proc.Shutdown(5 * time.Second) }()

	const hostCount = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	var mismatches []string

	for i := 0; i < hostCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hostTrace := entities.Trace{Value: fmt.Sprintf("host-%d", i), Type: echoTraceType}
			results, err := proc.ProcessTrace(context.Background(), hostTrace)
			if err != nil {
				return
			}
			for _, d := range results {
				expected := "resolved-for:" + hostTrace.Value
				if d.Child.Value != expected {
					mu.Lock()
					mismatches = append(mismatches, fmt.Sprintf(
						"submitted for %s but got child %q", hostTrace.Value, d.Child.Value))
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	assert.Empty(t, mismatches, "cross-attribution detected: %v", mismatches)
}
