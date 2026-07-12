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

const matcherTraceType entities.TraceType = "test_matcher"

// matcherPlugin implements plugins.TraceMatcher so ProcessTrace can be
// tested against plugins that pre-declare whether they'd act on a trace.
type matcherPlugin struct {
	name    string
	matches bool
	called  bool
}

func (p *matcherPlugin) Register() error {
	state.RegisterPlugin(matcherTraceType, p)
	return nil
}

func (p *matcherPlugin) Matches(trace entities.Trace) bool {
	return p.matches
}

func (p *matcherPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	p.called = true
	return []entities.Trace{{Value: p.name, Type: trace.Type}}, nil
}

func (p *matcherPlugin) String() string {
	return p.name
}

// plainMatcherTestPlugin implements plugins.DeeperPlugin only (no
// TraceMatcher), proving plugins without the opt-in are unaffected and
// still always submitted.
type plainMatcherTestPlugin struct {
	name string
}

func (p *plainMatcherTestPlugin) Register() error {
	state.RegisterPlugin(matcherTraceType, p)
	return nil
}

func (p *plainMatcherTestPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	return []entities.Trace{{Value: p.name, Type: trace.Type}}, nil
}

func (p *plainMatcherTestPlugin) String() string {
	return p.name
}

// TestProcessor_ProcessTrace_SkipsNonMatchingPlugins is a regression test:
// registering 7 platform-specific plugins under the single broad
// entities.SocialGeneric type meant every trace fanned out to all 7, each
// paying a domain rate-limit wait in Submit() even though 6/7 would
// immediately no-op in FollowTrace. Plugins implementing TraceMatcher let
// ProcessTrace skip submission -- and that wasted rate-limit wait --
// entirely for traces they'd never act on.
func TestProcessor_ProcessTrace_SkipsNonMatchingPlugins(t *testing.T) {
	original := state.ActivePlugins[matcherTraceType]
	t.Cleanup(func() {
		if original == nil {
			delete(state.ActivePlugins, matcherTraceType)
			return
		}
		state.ActivePlugins[matcherTraceType] = original
	})

	nonMatching := &matcherPlugin{name: "non-matching", matches: false}
	matching := &matcherPlugin{name: "matching", matches: true}
	unfiltered := &plainMatcherTestPlugin{name: "unfiltered"}

	state.ActivePlugins[matcherTraceType] = nil
	require.NoError(t, nonMatching.Register())
	require.NoError(t, matching.Register())
	require.NoError(t, unfiltered.Register())

	cfg := config.DefaultConfig()
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

	trace := entities.Trace{Value: "target", Type: matcherTraceType}
	results, err := proc.ProcessTrace(context.Background(), trace)
	require.NoError(t, err)

	assert.False(t, nonMatching.called, "non-matching plugin's FollowTrace must never be invoked")
	assert.True(t, matching.called)

	names := map[string]bool{}
	for _, d := range results {
		names[d.PluginName] = true
	}
	assert.Contains(t, names, "matching")
	assert.Contains(t, names, "unfiltered")
	assert.NotContains(t, names, "non-matching")
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
