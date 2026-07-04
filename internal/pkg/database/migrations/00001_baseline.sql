-- +goose Up
CREATE TABLE IF NOT EXISTS scan_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    input TEXT NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    status TEXT DEFAULT 'running',
    total_traces INTEGER DEFAULT 0,
    unique_traces INTEGER DEFAULT 0,
    errors INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS cache_entries (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    plugin_name TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cache_expires_at ON cache_entries(expires_at);
CREATE INDEX IF NOT EXISTS idx_cache_plugin ON cache_entries(plugin_name);

-- +goose Down
DROP INDEX IF EXISTS idx_cache_plugin;
DROP INDEX IF EXISTS idx_cache_expires_at;
DROP TABLE IF EXISTS cache_entries;
DROP TABLE IF EXISTS scan_sessions;
