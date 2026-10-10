package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
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

func TestEngine_ProcessInput_SeedEdgeIsProof(t *testing.T) {
	eng, repo := setupEngine(t, plugins.Registry{})
	session, err := repo.CreateScanSession("root")
	require.NoError(t, err)

	_, err = eng.ProcessInput(context.Background(), "root", session.ID)
	require.NoError(t, err)

	_, edges, err := repo.GetScanGraph(session.ID)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Nil(t, edges[0].ParentTraceID)
	assert.Equal(t, entities.StrengthProof, edges[0].Strength)
	assert.Equal(t, "seed", edges[0].Basis)
}

type evidencePlugin struct {
	name     string
	findings []plugins.Finding
}

func (p *evidencePlugin) FollowEvidence(_ context.Context, _ entities.Trace) ([]plugins.Finding, error) {
	return p.findings, nil
}

func (p *evidencePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	findings, err := p.FollowEvidence(ctx, trace)
	if err != nil {
		return nil, err
	}
	var traces []entities.Trace
	for _, f := range findings {
		traces = append(traces, f.Trace)
	}
	return traces, nil
}

func (p *evidencePlugin) String() string {
	return p.name
}

func TestEngine_ProcessInput_WeakAccountChildrenRecordedNotExpanded(t *testing.T) {
	account := entities.Trace{Value: "https://example.org/alice", Type: entities.SocialGeneric}
	company := entities.Trace{Value: "Namesake Corp", Type: entities.Company}
	email := entities.Trace{Value: "alice@namesake-corp.com", Type: entities.Email}

	run := func(t *testing.T, accountStrength entities.Strength) (companyCalled, emailCalled bool) {
		t.Helper()
		companyRecorder := &recordingPlugin{name: "company-recorder"}
		emailRecorder := &recordingPlugin{name: "email-recorder"}
		registry := plugins.Registry{
			entities.Username: {&evidencePlugin{name: "sherlock-fake", findings: []plugins.Finding{
				{Trace: account, Strength: accountStrength, Basis: "fake check"},
			}}},
			entities.SocialGeneric: {&chainProfilePlugin{name: "profile-fake", outputs: []entities.Trace{company, email}}},
			entities.Company:       {companyRecorder},
			entities.Email:         {emailRecorder},
		}

		eng, repo := setupEngine(t, registry)
		session, err := repo.CreateScanSession("alice")
		require.NoError(t, err)

		traces, err := eng.ProcessInput(context.Background(), "alice", session.ID)
		require.NoError(t, err)
		assert.Contains(t, traces, account)
		assert.Contains(t, traces, company)
		assert.Contains(t, traces, email)

		recorded, edges, err := repo.GetScanGraph(session.ID)
		require.NoError(t, err)
		var recordedTraces []entities.Trace
		byID := map[int64]entities.Trace{}
		for _, n := range recorded {
			tr := entities.Trace{Value: n.Value, Type: n.Type}
			recordedTraces = append(recordedTraces, tr)
			byID[n.ID] = tr
		}
		assert.Contains(t, recordedTraces, company)
		assert.Contains(t, recordedTraces, email)

		var profileChildren []entities.Trace
		var accountEdgeFound bool
		for _, e := range edges {
			switch e.PluginName {
			case "profile-fake":
				profileChildren = append(profileChildren, byID[e.ChildTraceID])
			case "sherlock-fake":
				accountEdgeFound = true
				assert.Equal(t, account, byID[e.ChildTraceID])
				assert.Equal(t, accountStrength, e.Strength)
				assert.Equal(t, "fake check", e.Basis)
			}
		}
		assert.True(t, accountEdgeFound, "the account edge must be persisted")
		assert.ElementsMatch(t, []entities.Trace{company, email}, profileChildren)

		return companyRecorder.called, emailRecorder.called
	}

	t.Run("weak account", func(t *testing.T) {
		companyCalled, emailCalled := run(t, entities.StrengthWeak)
		assert.False(t, companyCalled, "a child of a weak node must not be expanded")
		assert.False(t, emailCalled, "a child of a weak node must not be expanded")
	})

	t.Run("strong account", func(t *testing.T) {
		companyCalled, emailCalled := run(t, entities.StrengthStrong)
		assert.True(t, companyCalled)
		assert.True(t, emailCalled)
	})
}

type chainProfilePlugin struct {
	name    string
	outputs []entities.Trace
}

func (p *chainProfilePlugin) FollowTrace(_ context.Context, _ entities.Trace) ([]entities.Trace, error) {
	return p.outputs, nil
}

func (p *chainProfilePlugin) String() string {
	return p.name
}

type valueRecorder struct {
	name string
	mu   sync.Mutex
	seen []string
}

func (p *valueRecorder) FollowTrace(_ context.Context, trace entities.Trace) ([]entities.Trace, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, trace.Value)
	return nil, nil
}

func (p *valueRecorder) String() string {
	return p.name
}

func TestEngine_ProcessInput_GithubAccountSeedExpandsItsRepos(t *testing.T) {
	repo := entities.Trace{Type: entities.Repository, Value: "https://github.com/alice/a"}
	repoRecorder := &recordingPlugin{name: "repo-recorder"}
	usernamePlugin := &recordingPlugin{name: "username-plugin"}
	registry := plugins.Registry{
		entities.Github:     {&evidencePlugin{name: "repos-fake", findings: []plugins.Finding{{Trace: repo, Strength: entities.StrengthStrong}}}},
		entities.Repository: {repoRecorder},
		entities.Username:   {usernamePlugin},
	}

	eng, repoDB := setupEngine(t, registry)
	session, err := repoDB.CreateScanSession("https://github.com/alice")
	require.NoError(t, err)

	traces, err := eng.ProcessInput(context.Background(), "https://github.com/alice", session.ID)
	require.NoError(t, err)

	assert.Contains(t, traces, entities.Trace{Type: entities.Github, Value: "https://github.com/alice"})
	assert.True(t, repoRecorder.called, "a proof Github seed must expand its strong repos")
	assert.False(t, usernamePlugin.called, "a Github seed must not fall back to bare-username plugins")
}

func TestEngine_ProcessInput_DomainChainReachesCommitLoginAccount(t *testing.T) {
	repoURL := entities.Trace{Type: entities.Github, Value: "https://github.com/acme/tool"}
	login := entities.Trace{Type: entities.Username, Value: "bob"}
	loginAccount := entities.Trace{Type: entities.Github, Value: "https://github.com/bob"}
	accountRecorder := &valueRecorder{name: "github-profile-fake"}
	registry := plugins.Registry{
		entities.Domain: {&evidencePlugin{name: "crawler-fake", findings: []plugins.Finding{{Trace: repoURL, Strength: entities.StrengthStrong}}}},
		entities.Github: {
			&chainOnValuePlugin{name: "identity-fake", input: repoURL.Value, outputs: []entities.Trace{login, loginAccount}},
			accountRecorder,
		},
	}

	eng, repoDB := setupEngine(t, registry)
	session, err := repoDB.CreateScanSession("acme.example")
	require.NoError(t, err)

	_, err = eng.ProcessInput(context.Background(), "acme.example", session.ID)
	require.NoError(t, err)

	assert.Contains(t, accountRecorder.seen, loginAccount.Value)
}

type chainOnValuePlugin struct {
	name    string
	input   string
	outputs []entities.Trace
}

func (p *chainOnValuePlugin) FollowTrace(_ context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Value != p.input {
		return nil, nil
	}
	return p.outputs, nil
}

func (p *chainOnValuePlugin) String() string {
	return p.name
}

type hangingPlugin struct{}

func (hangingPlugin) FollowTrace(ctx context.Context, _ entities.Trace) ([]entities.Trace, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingPlugin) String() string { return "hanging" }

func TestEngine_ProcessInput_StopsWhenContextCancelled(t *testing.T) {
	eng, repo := setupEngine(t, plugins.Registry{testEngineTraceType: {hangingPlugin{}}})
	session, err := repo.CreateScanSession("root")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := eng.ProcessInput(ctx, "root", session.ID)
		done <- err
	}()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(3 * time.Second):
		t.Fatal("ProcessInput did not return after its context was cancelled")
	}
}
