package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const testEngineTraceType entities.TraceType = entities.Username

type chainPlugin struct {
	name   string
	input  string
	output string
}

func (p *chainPlugin) FollowTrace(_ context.Context, trace entities.Trace) ([]entities.Trace, error) {
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

func (p *multiParentPlugin) FollowTrace(_ context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Value != "root" {
		return nil, nil
	}
	return []entities.Trace{{Value: "shared-child", Type: testEngineTraceType}}, nil
}

func (p *multiParentPlugin) String() string {
	return p.name
}

func setupEngine(t *testing.T, registry plugins.Registry) (*Engine, *database.Repository) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.WorkerPoolConfig.EnableDeduplication = false

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := database.NewRepository(db)
	cache := database.NewCache(repo)
	eng := NewEngine(cfg, registry, metrics.GetGlobalMetrics(), repo, cache)
	t.Cleanup(func() { _ = eng.Shutdown(5 * time.Second) })
	return eng, repo
}

func TestEngine_ProcessInput_BlankInputRejected(t *testing.T) {
	eng, repo := setupEngine(t, plugins.Registry{})
	session, err := repo.CreateScanSession("   ")
	require.NoError(t, err)

	traces, err := eng.ProcessInput(context.Background(), "   ", session.ID)
	assert.Error(t, err)
	assert.Nil(t, traces)
}

func TestEngine_ProcessInput_PersistsEdgeChain(t *testing.T) {
	registry := plugins.Registry{testEngineTraceType: {
		&chainPlugin{name: "step1", input: "root", output: "hop2"},
		&chainPlugin{name: "step2", input: "hop2", output: "hop3"},
	}}

	eng, repo := setupEngine(t, registry)
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
	registry := plugins.Registry{testEngineTraceType: {&chainPlugin{name: "rediscovers-seed", input: "root", output: "root"}}}

	eng, repo := setupEngine(t, registry)
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

// The trace graph holds the canonical domain, but ScanSession.Input keeps the
// seed exactly as the caller typed it.
func TestEngine_ProcessInput_DomainSeedCanonicalized_SessionInputPreservesOriginal(t *testing.T) {
	const originalSeed = "Registry-One.Example.COM."
	const canonicalSeed = "registry-one.example.com"

	registry := plugins.Registry{}
	eng, repo := setupEngine(t, registry)

	session, err := repo.CreateScanSession(originalSeed)
	require.NoError(t, err)

	traces, err := eng.ProcessInput(context.Background(), originalSeed, session.ID)
	require.NoError(t, err)
	assert.Contains(t, traces, entities.Trace{Value: canonicalSeed, Type: entities.Domain})

	stored, err := repo.GetScanSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, originalSeed, stored.Input,
		"ScanSession.Input must keep the seed as the user typed it, not NewTrace's canonical form")
}

// The length cutoff measures the trimmed value NewTrace receives, so padding
// around a 1024-byte address must not push it over the limit.
func TestEngine_ProcessInput_LengthCutoffAppliesAfterTrimSpace(t *testing.T) {
	const base = "123 Main St"
	address := base + strings.Repeat("x", 1024-len(base))
	seed := "  " + address + "  "

	registry := plugins.Registry{}
	eng, repo := setupEngine(t, registry)
	session, err := repo.CreateScanSession(seed)
	require.NoError(t, err)

	traces, err := eng.ProcessInput(context.Background(), seed, session.ID)
	require.NoError(t, err)
	assert.Contains(t, traces, entities.Trace{Value: address, Type: entities.Address})
}

type recordingPlugin struct {
	name   string
	called bool
}

func (p *recordingPlugin) FollowTrace(_ context.Context, _ entities.Trace) ([]entities.Trace, error) {
	p.called = true
	return nil, nil
}

func (p *recordingPlugin) String() string {
	return p.name
}

func TestEngine_ProcessInput_HyphenatedDomainSeedOnlyReachesDomainPlugins(t *testing.T) {
	run := func(t *testing.T, seed string) (domainCalled, usernameCalled bool) {
		t.Helper()
		domainPlugin := &recordingPlugin{name: "domain-plugin"}
		usernamePlugin := &recordingPlugin{name: "username-plugin"}
		registry := plugins.Registry{
			entities.Domain:   {domainPlugin},
			entities.Username: {usernamePlugin},
		}

		eng, repo := setupEngine(t, registry)
		session, err := repo.CreateScanSession(seed)
		require.NoError(t, err)

		_, err = eng.ProcessInput(context.Background(), seed, session.ID)
		require.NoError(t, err)

		return domainPlugin.called, usernamePlugin.called
	}

	domainCalled, usernameCalled := run(t, "registry-one.example.com")
	assert.True(t, domainCalled, "hyphenated domain seed must reach the plugin registered on entities.Domain")
	assert.False(t, usernameCalled, "hyphenated domain seed must never reach a plugin registered only on entities.Username")

	controlDomainCalled, controlUsernameCalled := run(t, "registryone.example.com")
	assert.Equal(t, domainCalled, controlDomainCalled, "hyphenated seed must dispatch to the same plugin set as its hyphen-free control")
	assert.Equal(t, usernameCalled, controlUsernameCalled, "hyphenated seed must dispatch to the same plugin set as its hyphen-free control")
}

func TestEngine_ProcessInput_MultiParentPersistence(t *testing.T) {
	registry := plugins.Registry{}
	for i := 0; i < 2; i++ {
		registry.Add(testEngineTraceType, &multiParentPlugin{name: fmt.Sprintf("parent-plugin-%d", i)})
	}

	eng, repo := setupEngine(t, registry)
	session, err := repo.CreateScanSession("root")
	require.NoError(t, err)

	_, err = eng.ProcessInput(context.Background(), "root", session.ID)
	require.NoError(t, err)

	count, err := repo.CountEdges(session.ID)
	require.NoError(t, err)
	// seed edge + 2 parent→shared-child edges
	assert.Equal(t, 3, count)
}
