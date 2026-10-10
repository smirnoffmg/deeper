package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

func TestMigrations_EdgeEvidenceBackfillsExistingEdgesAsStrong(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")

	raw, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	goose.SetBaseFS(migrationFS)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.UpTo(raw, "migrations", 2))

	res, err := raw.Exec(`INSERT INTO scan_sessions (input, status) VALUES ('old', 'completed')`)
	require.NoError(t, err)
	scanID, err := res.LastInsertId()
	require.NoError(t, err)
	res, err = raw.Exec(`INSERT INTO traces (value, type) VALUES ('old.com', 'domain')`)
	require.NoError(t, err)
	traceID, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO trace_edges (parent_trace_id, child_trace_id, plugin_name, scan_id) VALUES (NULL, ?, 'seed', ?)`, traceID, scanID)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	db, err := NewDatabase(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, edges, err := NewRepository(db).GetScanGraph(scanID)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Equal(t, entities.StrengthStrong, edges[0].Strength)
	assert.Equal(t, "", edges[0].Basis)
}
