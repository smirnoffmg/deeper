package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewRepository(db)
}

func newTestScan(t *testing.T, repo *Repository) int64 {
	t.Helper()
	session, err := repo.CreateScanSession("test-input")
	require.NoError(t, err)
	return session.ID
}

func TestRepository_GetOrCreateTrace_Dedup(t *testing.T) {
	repo := newTestRepo(t)
	trace := entities.Trace{Value: "a@example.com", Type: entities.Email}

	id1, err := repo.GetOrCreateTrace(trace)
	require.NoError(t, err)
	require.NotZero(t, id1)

	id2, err := repo.GetOrCreateTrace(trace)
	require.NoError(t, err)
	assert.Equal(t, id1, id2)
}

func TestRepository_InsertEdge_Idempotent(t *testing.T) {
	repo := newTestRepo(t)
	scanID := newTestScan(t, repo)

	parentID, err := repo.GetOrCreateTrace(entities.Trace{Value: "parent.com", Type: entities.Domain})
	require.NoError(t, err)
	childID, err := repo.GetOrCreateTrace(entities.Trace{Value: "child.parent.com", Type: entities.Subdomain})
	require.NoError(t, err)

	edge := &TraceEdge{
		ParentTraceID: &parentID,
		ChildTraceID:  childID,
		PluginName:    "test-plugin",
		ScanID:        scanID,
		DiscoveredAt:  time.Now(),
	}
	require.NoError(t, repo.InsertEdge(edge))
	require.NoError(t, repo.InsertEdge(edge))

	count, err := repo.CountEdges(scanID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRepository_PersistDiscoveries_MultiParent(t *testing.T) {
	repo := newTestRepo(t)
	scanID := newTestScan(t, repo)

	parentA := entities.Trace{Value: "parent-a.com", Type: entities.Domain}
	parentB := entities.Trace{Value: "parent-b.com", Type: entities.Domain}
	child := entities.Trace{Value: "shared.child.com", Type: entities.Subdomain}

	discoveries := []entities.Discovery{
		{Parent: parentA, PluginName: "plugin-x", Child: child},
		{Parent: parentB, PluginName: "plugin-x", Child: child},
	}
	require.NoError(t, repo.PersistDiscoveries(scanID, discoveries))

	count, err := repo.CountEdges(scanID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestRepository_GetDiscoveryPath_SinglePath(t *testing.T) {
	repo := newTestRepo(t)
	scanID := newTestScan(t, repo)

	root := entities.Trace{Value: "root.com", Type: entities.Domain}
	mid := entities.Trace{Value: "mid.root.com", Type: entities.Subdomain}
	leaf := entities.Trace{Value: "1.2.3.4", Type: entities.IpAddr}

	require.NoError(t, repo.PersistDiscoveries(scanID, []entities.Discovery{
		{Parent: root, PluginName: "p1", Child: mid},
		{Parent: mid, PluginName: "p2", Child: leaf},
	}))

	leafID, err := repo.GetOrCreateTrace(leaf)
	require.NoError(t, err)

	path, err := repo.GetDiscoveryPath(scanID, leafID)
	require.NoError(t, err)
	require.Len(t, path, 2)
	assert.Equal(t, "p1", path[0].PluginName)
	assert.Equal(t, "p2", path[1].PluginName)
}

func TestRepository_GetDiscoveryPath_MultiParent(t *testing.T) {
	repo := newTestRepo(t)
	scanID := newTestScan(t, repo)

	root := entities.Trace{Value: "root.com", Type: entities.Domain}
	parentA := entities.Trace{Value: "a.root.com", Type: entities.Subdomain}
	parentB := entities.Trace{Value: "b.root.com", Type: entities.Subdomain}
	child := entities.Trace{Value: "shared.root.com", Type: entities.Subdomain}

	require.NoError(t, repo.PersistDiscoveries(scanID, []entities.Discovery{
		{Parent: root, PluginName: "p1", Child: parentA},
		{Parent: root, PluginName: "p1", Child: parentB},
		{Parent: parentA, PluginName: "p2", Child: child},
		{Parent: parentB, PluginName: "p3", Child: child},
	}))

	childID, err := repo.GetOrCreateTrace(child)
	require.NoError(t, err)

	path, err := repo.GetDiscoveryPath(scanID, childID)
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestRepository_GetReachableTraces_HopBounding(t *testing.T) {
	repo := newTestRepo(t)
	scanID := newTestScan(t, repo)

	root := entities.Trace{Value: "root.com", Type: entities.Domain}
	mid := entities.Trace{Value: "mid.root.com", Type: entities.Subdomain}
	leaf := entities.Trace{Value: "deep.root.com", Type: entities.Subdomain}

	require.NoError(t, repo.PersistDiscoveries(scanID, []entities.Discovery{
		{Parent: root, PluginName: "p1", Child: mid},
		{Parent: mid, PluginName: "p2", Child: leaf},
	}))

	rootID, err := repo.GetOrCreateTrace(root)
	require.NoError(t, err)

	reachable, err := repo.GetReachableTraces(scanID, rootID, 1)
	require.NoError(t, err)

	hopsByID := make(map[int64]int)
	for _, rt := range reachable {
		hopsByID[rt.TraceID] = rt.Hops
	}
	midID, _ := repo.GetOrCreateTrace(mid)
	leafID, _ := repo.GetOrCreateTrace(leaf)

	assert.Equal(t, 0, hopsByID[rootID])
	assert.Equal(t, 1, hopsByID[midID])
	_, hasLeaf := hopsByID[leafID]
	assert.False(t, hasLeaf)
}

func TestRepository_GetReachableTraces_HandlesCycle(t *testing.T) {
	repo := newTestRepo(t)
	scanID := newTestScan(t, repo)

	a := entities.Trace{Value: "a.com", Type: entities.Domain}
	b := entities.Trace{Value: "b.com", Type: entities.Domain}

	require.NoError(t, repo.PersistDiscoveries(scanID, []entities.Discovery{
		{Parent: a, PluginName: "p1", Child: b},
		{Parent: b, PluginName: "p2", Child: a},
	}))

	aID, err := repo.GetOrCreateTrace(a)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		_, err := repo.GetReachableTraces(scanID, aID, 10)
		assert.NoError(t, err)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetReachableTraces did not terminate on cyclic graph")
	}

	reachable, err := repo.GetReachableTraces(scanID, aID, 10)
	require.NoError(t, err)

	hopsByID := make(map[int64]int)
	for _, rt := range reachable {
		hopsByID[rt.TraceID] = rt.Hops
	}
	bID, _ := repo.GetOrCreateTrace(b)
	assert.Equal(t, 0, hopsByID[aID])
	assert.Equal(t, 1, hopsByID[bID])
}
