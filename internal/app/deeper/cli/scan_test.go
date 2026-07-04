package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/pkg/database"
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
