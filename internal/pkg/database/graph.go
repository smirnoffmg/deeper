package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

func traceKey(trace entities.Trace) string {
	return string(trace.Type) + "\x00" + trace.Value
}

// GetOrCreateTrace resolves or creates a trace node by (value, type).
func (r *Repository) GetOrCreateTrace(trace entities.Trace) (int64, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	result, err := r.db.db.Exec(
		`INSERT OR IGNORE INTO traces (value, type, discovered_at) VALUES (?, ?, ?)`,
		trace.Value, trace.Type, time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert trace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to read rows affected: %w", err)
	}
	if rowsAffected > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get insert id: %w", err)
		}
		return id, nil
	}

	var id int64
	err = r.db.db.QueryRow(
		`SELECT id FROM traces WHERE value = ? AND type = ?`,
		trace.Value, trace.Type,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to lookup trace id: %w", err)
	}
	return id, nil
}

// InsertEdge inserts a discovery edge idempotently.
func (r *Repository) InsertEdge(edge *TraceEdge) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	_, err := r.db.db.Exec(
		`INSERT OR IGNORE INTO trace_edges (parent_trace_id, child_trace_id, plugin_name, scan_id, discovered_at)
		 VALUES (?, ?, ?, ?, ?)`,
		edge.ParentTraceID,
		edge.ChildTraceID,
		edge.PluginName,
		edge.ScanID,
		edge.DiscoveredAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert edge: %w", err)
	}
	return nil
}

// PersistDiscoveries resolves trace nodes and inserts all edges in one transaction.
func (r *Repository) PersistDiscoveries(scanID int64, discoveries []entities.Discovery) error {
	if len(discoveries) == 0 {
		return nil
	}

	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	tx, err := r.db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	idCache := make(map[string]int64)
	now := time.Now()

	resolveTrace := func(trace entities.Trace) (int64, error) {
		key := traceKey(trace)
		if id, ok := idCache[key]; ok {
			return id, nil
		}

		result, err := tx.Exec(
			`INSERT OR IGNORE INTO traces (value, type, discovered_at) VALUES (?, ?, ?)`,
			trace.Value, trace.Type, now,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert trace: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("failed to read rows affected: %w", err)
		}
		if rowsAffected > 0 {
			id, err := result.LastInsertId()
			if err != nil {
				return 0, fmt.Errorf("failed to get insert id: %w", err)
			}
			idCache[key] = id
			return id, nil
		}

		var id int64
		err = tx.QueryRow(
			`SELECT id FROM traces WHERE value = ? AND type = ?`,
			trace.Value, trace.Type,
		).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("failed to lookup trace id: %w", err)
		}
		idCache[key] = id
		return id, nil
	}

	for _, d := range discoveries {
		parentID, err := resolveTrace(d.Parent)
		if err != nil {
			return err
		}
		childID, err := resolveTrace(d.Child)
		if err != nil {
			return err
		}

		_, err = tx.Exec(
			`INSERT OR IGNORE INTO trace_edges (parent_trace_id, child_trace_id, plugin_name, scan_id, discovered_at)
			 VALUES (?, ?, ?, ?, ?)`,
			parentID, childID, d.PluginName, scanID, now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert edge: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// GetDiscoveryPath walks parent_trace_id backward to the root, returning root-first edges.
func (r *Repository) GetDiscoveryPath(scanID, targetTraceID int64) ([]TraceEdge, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	rows, err := r.db.db.Query(`
		WITH RECURSIVE path(child_trace_id, parent_trace_id, plugin_name, hop, visited) AS (
			SELECT child_trace_id, parent_trace_id, plugin_name, 0,
			       ',' || child_trace_id || ','
			FROM trace_edges
			WHERE scan_id = ? AND child_trace_id = ?

			UNION ALL

			SELECT e.child_trace_id, e.parent_trace_id, e.plugin_name, p.hop + 1,
			       p.visited || e.parent_trace_id || ','
			FROM trace_edges e
			JOIN path p ON e.child_trace_id = p.parent_trace_id
			WHERE e.scan_id = ?
			  AND e.parent_trace_id IS NOT NULL
			  AND p.visited NOT LIKE '%,' || e.parent_trace_id || ',%'
		)
		SELECT child_trace_id, parent_trace_id, plugin_name, hop
		FROM path
		ORDER BY hop DESC`,
		scanID, targetTraceID, scanID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query discovery path: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var edges []TraceEdge
	for rows.Next() {
		var edge TraceEdge
		var parentID sql.NullInt64
		var hop int
		if err := rows.Scan(&edge.ChildTraceID, &parentID, &edge.PluginName, &hop); err != nil {
			return nil, fmt.Errorf("failed to scan path row: %w", err)
		}
		if parentID.Valid {
			edge.ParentTraceID = &parentID.Int64
		}
		edge.ScanID = scanID
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read path rows: %w", err)
	}
	return edges, nil
}

// GetReachableTraces walks child_trace_id forward from a start node within a hop budget.
func (r *Repository) GetReachableTraces(scanID, startTraceID int64, maxHops int) ([]ReachableTrace, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	rows, err := r.db.db.Query(`
		WITH RECURSIVE reachable(trace_id, hops, visited) AS (
			SELECT ?, 0, ',' || ? || ','

			UNION ALL

			SELECT e.child_trace_id, r.hops + 1,
			       r.visited || e.child_trace_id || ','
			FROM trace_edges e
			JOIN reachable r ON e.parent_trace_id = r.trace_id
			WHERE e.scan_id = ?
			  AND r.hops < ?
			  AND r.visited NOT LIKE '%,' || e.child_trace_id || ',%'
		)
		SELECT r.trace_id, t.value, t.type, MIN(r.hops) AS hops
		FROM reachable r
		JOIN traces t ON t.id = r.trace_id
		GROUP BY r.trace_id
		ORDER BY hops, t.value`,
		startTraceID, startTraceID, scanID, maxHops,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query reachable traces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var traces []ReachableTrace
	for rows.Next() {
		var rt ReachableTrace
		if err := rows.Scan(&rt.TraceID, &rt.Value, &rt.Type, &rt.Hops); err != nil {
			return nil, fmt.Errorf("failed to scan reachable row: %w", err)
		}
		traces = append(traces, rt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read reachable rows: %w", err)
	}
	return traces, nil
}

// CountEdges returns the number of edges for a scan (test helper).
func (r *Repository) CountEdges(scanID int64) (int, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	var count int
	err := r.db.db.QueryRow(`SELECT COUNT(*) FROM trace_edges WHERE scan_id = ?`, scanID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count edges: %w", err)
	}
	return count, nil
}
