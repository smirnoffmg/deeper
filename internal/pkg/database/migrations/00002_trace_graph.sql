-- +goose Up
DROP TABLE IF EXISTS trace_edges;
DROP TABLE IF EXISTS traces;

CREATE TABLE traces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    value TEXT NOT NULL,
    type TEXT NOT NULL,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT,
    UNIQUE(value, type)
);

CREATE TABLE trace_edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_trace_id INTEGER,
    child_trace_id INTEGER NOT NULL,
    plugin_name TEXT NOT NULL,
    scan_id INTEGER NOT NULL,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_trace_id) REFERENCES traces(id),
    FOREIGN KEY (child_trace_id) REFERENCES traces(id),
    FOREIGN KEY (scan_id) REFERENCES scan_sessions(id),
    UNIQUE(parent_trace_id, child_trace_id, plugin_name, scan_id)
);

CREATE INDEX IF NOT EXISTS idx_traces_type ON traces(type);
CREATE INDEX IF NOT EXISTS idx_traces_discovered_at ON traces(discovered_at);
CREATE INDEX IF NOT EXISTS idx_trace_edges_child ON trace_edges(child_trace_id, scan_id);
CREATE INDEX IF NOT EXISTS idx_trace_edges_parent ON trace_edges(parent_trace_id, scan_id);
CREATE INDEX IF NOT EXISTS idx_trace_edges_scan ON trace_edges(scan_id);

-- +goose Down
DROP INDEX IF EXISTS idx_trace_edges_scan;
DROP INDEX IF EXISTS idx_trace_edges_parent;
DROP INDEX IF EXISTS idx_trace_edges_child;
DROP INDEX IF EXISTS idx_traces_discovered_at;
DROP INDEX IF EXISTS idx_traces_type;
DROP TABLE IF EXISTS trace_edges;
DROP TABLE IF EXISTS traces;
