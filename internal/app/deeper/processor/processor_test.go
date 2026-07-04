package processor

import (
	"context"
	"fmt"
	"path/filepath"
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

	sequentialTime := time.Duration(pluginCount) * sleepDelay
	parallelTime := time.Duration((pluginCount+maxWorkers-1)/maxWorkers) * sleepDelay
	assert.Less(t, elapsed, sequentialTime, "expected parallel execution faster than sequential")
	assert.GreaterOrEqual(t, elapsed, parallelTime-time.Millisecond*20, "expected at least parallel lower bound")
}
