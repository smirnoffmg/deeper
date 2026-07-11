package engine

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

const testEngineTraceType entities.TraceType = entities.Username

type chainPlugin struct {
	name   string
	input  string
	output string
}

func (p *chainPlugin) Register() error {
	state.RegisterPlugin(testEngineTraceType, p)
	return nil
}

func (p *chainPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Value != p.input {
		return nil, nil
	}
	if p.output == "" {
		return nil, nil
	}
	return []entities.Trace{{Value: p.output, Type: testEngineTraceType}}, nil
}

func (p *chainPlugin) String() string {
	return p.name
}

type multiParentPlugin struct {
	name string
}

func (p *multiParentPlugin) Register() error {
	state.RegisterPlugin(testEngineTraceType, p)
	return nil
}

func (p *multiParentPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Value != "root" {
		return nil, nil
	}
	return []entities.Trace{{Value: "shared-child", Type: testEngineTraceType}}, nil
}

func (p *multiParentPlugin) String() string {
	return p.name
}

func setupEngine(t *testing.T) (*Engine, *database.Repository) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.WorkerPoolConfig.EnableDeduplication = false

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := database.NewRepository(db)
	cache := database.NewCache(repo)
	eng := NewEngine(cfg, metrics.GetGlobalMetrics(), repo, cache)
	t.Cleanup(func() { _ = eng.Shutdown(5 * time.Second) })
	return eng, repo
}

func TestEngine_ProcessInput_PersistsEdgeChain(t *testing.T) {
	original := state.ActivePlugins[testEngineTraceType]
	t.Cleanup(func() {
		if original == nil {
			delete(state.ActivePlugins, testEngineTraceType)
			return
		}
		state.ActivePlugins[testEngineTraceType] = original
	})

	state.ActivePlugins[testEngineTraceType] = nil
	require.NoError(t, (&chainPlugin{name: "step1", input: "root", output: "hop2"}).Register())
	require.NoError(t, (&chainPlugin{name: "step2", input: "hop2", output: "hop3"}).Register())

	eng, repo := setupEngine(t)
	session, err := repo.CreateScanSession("root")
	require.NoError(t, err)

	traces, err := eng.ProcessInput(context.Background(), "root", session.ID)
	require.NoError(t, err)
	assert.Len(t, traces, 3, "seed trace (root) plus hop2 and hop3")
	assert.Contains(t, traces, entities.Trace{Value: "root", Type: testEngineTraceType})

	hop3ID, err := repo.GetOrCreateTrace(entities.Trace{Value: "hop3", Type: testEngineTraceType})
	require.NoError(t, err)

	path, err := repo.GetDiscoveryPath(session.ID, hop3ID)
	require.NoError(t, err)
	assert.Len(t, path, 2)

	rootID, err := repo.GetOrCreateTrace(entities.Trace{Value: "root", Type: testEngineTraceType})
	require.NoError(t, err)

	reachable, err := repo.GetReachableTraces(session.ID, rootID, 5)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(reachable), 3)
}

// TestEngine_ProcessInput_SeedNotReprocessedWhenRediscovered is a
// regression test: the seed trace must be marked as already-seen up front,
// so a plugin that happens to rediscover the exact same (value, type) pair
// as a "new" child doesn't cause it to be queued and processed a second
// time.
func TestEngine_ProcessInput_SeedNotReprocessedWhenRediscovered(t *testing.T) {
	original := state.ActivePlugins[testEngineTraceType]
	t.Cleanup(func() {
		if original == nil {
			delete(state.ActivePlugins, testEngineTraceType)
			return
		}
		state.ActivePlugins[testEngineTraceType] = original
	})

	state.ActivePlugins[testEngineTraceType] = nil
	require.NoError(t, (&chainPlugin{name: "rediscovers-seed", input: "root", output: "root"}).Register())

	eng, repo := setupEngine(t)
	session, err := repo.CreateScanSession("root")
	require.NoError(t, err)

	traces, err := eng.ProcessInput(context.Background(), "root", session.ID)
	require.NoError(t, err)

	count := 0
	for _, tr := range traces {
		if tr == (entities.Trace{Value: "root", Type: testEngineTraceType}) {
			count++
		}
	}
	assert.Equal(t, 1, count, "seed must appear exactly once even if rediscovered as a child")
}

func TestEngine_ProcessInput_MultiParentPersistence(t *testing.T) {
	original := state.ActivePlugins[testEngineTraceType]
	t.Cleanup(func() {
		if original == nil {
			delete(state.ActivePlugins, testEngineTraceType)
			return
		}
		state.ActivePlugins[testEngineTraceType] = original
	})

	state.ActivePlugins[testEngineTraceType] = nil
	for i := 0; i < 2; i++ {
		p := &multiParentPlugin{name: fmt.Sprintf("parent-plugin-%d", i)}
		require.NoError(t, p.Register())
	}

	eng, repo := setupEngine(t)
	session, err := repo.CreateScanSession("root")
	require.NoError(t, err)

	_, err = eng.ProcessInput(context.Background(), "root", session.ID)
	require.NoError(t, err)

	count, err := repo.CountEdges(session.ID)
	require.NoError(t, err)
	// seed edge + 2 parent→shared-child edges
	assert.Equal(t, 3, count)
}
