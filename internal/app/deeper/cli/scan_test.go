package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/app/deeper/graphreport"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

func TestCreateEngine_ReturnsRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	eng, repo, err := createEngine()
	require.NoError(t, err)
	require.NotNil(t, eng)
	require.NotNil(t, repo)
}

func TestScanSession_Completed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, repo, err := createEngine()
	require.NoError(t, err)

	session, err := repo.CreateScanSession("test-user")
	require.NoError(t, err)
	assert.Equal(t, "running", session.Status)

	session.Status = "completed"
	require.NoError(t, repo.UpdateScanSession(session))

	updated, err := repo.GetScanSession(session.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "completed", updated.Status)
}

func TestBuildGraphReport_MapsNodesAndEdges(t *testing.T) {
	nodes := []database.Trace{
		{ID: 1, Value: "root.com", Type: entities.Domain},
		{ID: 2, Value: "leaf.root.com", Type: entities.Subdomain},
	}
	parentID := int64(1)
	edges := []database.TraceEdge{
		{ParentTraceID: &parentID, ChildTraceID: 2, PluginName: "subdomain_finder", Strength: entities.StrengthWeak, Basis: "search hit"},
	}

	reportNodes, reportEdges := buildGraphReport(nodes, edges)

	assert.Equal(t, []graphreport.Node{
		{ID: 1, Label: "root.com", Type: "domain"},
		{ID: 2, Label: "leaf.root.com", Type: "subdomain"},
	}, reportNodes)
	assert.Equal(t, []graphreport.Edge{
		{From: 1, To: 2, Label: "subdomain_finder", Strength: "weak", Basis: "search hit"},
	}, reportEdges)
}

func TestBuildGraphReport_SkipsSeedEdgeWithNilParent(t *testing.T) {
	nodes := []database.Trace{{ID: 1, Value: "root.com", Type: entities.Domain}}
	edges := []database.TraceEdge{
		{ParentTraceID: nil, ChildTraceID: 1, PluginName: database.SeedPluginName},
	}

	_, reportEdges := buildGraphReport(nodes, edges)

	assert.Empty(t, reportEdges)
}

func TestSaveGraphReport_WritesFileUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, repo, err := createEngine()
	require.NoError(t, err)

	session, err := repo.CreateScanSession("test-input")
	require.NoError(t, err)

	require.NoError(t, repo.PersistDiscoveries(session.ID, []entities.Discovery{
		{Parent: entities.Trace{Value: "root.com", Type: entities.Domain}, PluginName: "p1", Child: entities.Trace{Value: "leaf.root.com", Type: entities.Subdomain}},
	}))

	path, err := saveGraphReport(repo, session.ID, false)
	require.NoError(t, err)
	assert.Contains(t, path, filepath.Join(home, ".deeper", "reports"))

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "root.com")
	assert.Contains(t, string(contents), "leaf.root.com")
}

func TestSaveGraphReport_NoTracesReturnsEmptyPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, repo, err := createEngine()
	require.NoError(t, err)

	session, err := repo.CreateScanSession("test-input")
	require.NoError(t, err)

	path, err := saveGraphReport(repo, session.ID, false)
	require.NoError(t, err)
	assert.Empty(t, path)
}

func TestNewDatabase_UsesMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = os.Stat(dbPath)
	require.NoError(t, err)

	stats, err := db.Stats()
	require.NoError(t, err)
	assert.Contains(t, stats, "total_traces")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func newSessionWithGraph(t *testing.T) (*database.Repository, int64, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, repo, err := createEngine()
	require.NoError(t, err)

	session, err := repo.CreateScanSession("root.com")
	require.NoError(t, err)
	require.NoError(t, repo.PersistDiscoveries(session.ID, []entities.Discovery{
		{Parent: entities.Trace{Value: "root.com", Type: entities.Domain}, PluginName: "p1", Child: entities.Trace{Value: "leaf.root.com", Type: entities.Subdomain}},
	}))
	return repo, session.ID, filepath.Join(home, ".deeper", "reports", fmt.Sprintf("scan-%d.html", session.ID))
}

func TestEmitResults_NonTableFormatsStillWriteReport(t *testing.T) {
	for _, format := range []string{"json", "csv"} {
		t.Run(format, func(t *testing.T) {
			repo, sessionID, reportPath := newSessionWithGraph(t)
			traces := []entities.Trace{{Value: "leaf.root.com", Type: entities.Subdomain}}

			captureStdout(t, func() {
				require.NoError(t, emitResults(repo, sessionID, traces, format, "results.out", false))
			})

			_, err := os.Stat(reportPath)
			assert.NoError(t, err)
		})
	}
}

func TestEmitResults_NoTracesStillWritesReportAndFormattedOutput(t *testing.T) {
	for format, want := range map[string]string{
		"table": "No traces found\n",
		"json":  "[\n]\n",
		"csv":   "value,type\n",
	} {
		t.Run(format, func(t *testing.T) {
			repo, sessionID, reportPath := newSessionWithGraph(t)

			out := captureStdout(t, func() {
				require.NoError(t, emitResults(repo, sessionID, nil, format, "", false))
			})

			assert.Equal(t, want, out)
			_, err := os.Stat(reportPath)
			assert.NoError(t, err)
		})
	}
}

func TestEmitResults_UnsupportedFormatWritesNoReport(t *testing.T) {
	repo, sessionID, reportPath := newSessionWithGraph(t)

	err := emitResults(repo, sessionID, []entities.Trace{{Value: "x", Type: entities.Domain}}, "xml", "results.out", false)

	assert.Error(t, err)
	_, statErr := os.Stat(reportPath)
	assert.True(t, os.IsNotExist(statErr))
}
