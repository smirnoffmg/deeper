package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

// Repository provides data access methods for the database
type Repository struct {
	db *Database
}

// NewRepository creates a new repository instance
func NewRepository(db *Database) *Repository {
	return &Repository{db: db}
}

// TraceRepository methods

// StoreTrace stores a trace in the database
func (r *Repository) StoreTrace(trace *Trace) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	metadata, err := trace.MarshalMetadata()
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT OR IGNORE INTO traces (value, type, discovered_at, source_plugin, metadata, scan_id, depth)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.db.Exec(query,
		trace.Value,
		trace.Type,
		trace.DiscoveredAt,
		trace.SourcePlugin,
		metadata,
		trace.ScanID,
		trace.Depth,
	)
	if err != nil {
		return fmt.Errorf("failed to store trace: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil && id > 0 {
		trace.ID = id
	}

	return nil
}

// GetTraces retrieves traces based on query parameters
func (r *Repository) GetTraces(query TraceQuery) ([]Trace, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	conditions := []string{"1=1"}
	args := []interface{}{}

	if query.Type != nil {
		conditions = append(conditions, "type = ?")
		args = append(args, *query.Type)
	}

	if query.SourcePlugin != nil {
		conditions = append(conditions, "source_plugin = ?")
		args = append(args, *query.SourcePlugin)
	}

	if query.ScanID != nil {
		conditions = append(conditions, "scan_id = ?")
		args = append(args, *query.ScanID)
	}

	if query.FromDate != nil {
		conditions = append(conditions, "discovered_at >= ?")
		args = append(args, query.FromDate)
	}

	if query.ToDate != nil {
		conditions = append(conditions, "discovered_at <= ?")
		args = append(args, query.ToDate)
	}

	whereClause := strings.Join(conditions, " AND ")
	sqlQuery := fmt.Sprintf(`
		SELECT id, value, type, discovered_at, source_plugin, metadata, scan_id, depth
		FROM traces
		WHERE %s
		ORDER BY discovered_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, query.Limit, query.Offset)

	rows, err := r.db.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}
	defer rows.Close()

	var traces []Trace
	for rows.Next() {
		var trace Trace
		var metadataStr sql.NullString
		err := rows.Scan(
			&trace.ID,
			&trace.Value,
			&trace.Type,
			&trace.DiscoveredAt,
			&trace.SourcePlugin,
			&metadataStr,
			&trace.ScanID,
			&trace.Depth,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trace: %w", err)
		}

		if metadataStr.Valid {
			if err := trace.UnmarshalMetadata(metadataStr.String); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		traces = append(traces, trace)
	}

	return traces, nil
}

// GetTraceByValue retrieves a trace by its value and type
func (r *Repository) GetTraceByValue(value string, traceType entities.TraceType) (*Trace, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	query := `
		SELECT id, value, type, discovered_at, source_plugin, metadata, scan_id, depth
		FROM traces
		WHERE value = ? AND type = ?
		LIMIT 1
	`

	var trace Trace
	var metadataStr sql.NullString
	err := r.db.db.QueryRow(query, value, traceType).Scan(
		&trace.ID,
		&trace.Value,
		&trace.Type,
		&trace.DiscoveredAt,
		&trace.SourcePlugin,
		&metadataStr,
		&trace.ScanID,
		&trace.Depth,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get trace: %w", err)
	}

	if metadataStr.Valid {
		if err := trace.UnmarshalMetadata(metadataStr.String); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &trace, nil
}

// ScanSessionRepository methods

// CreateScanSession creates a new scan session
func (r *Repository) CreateScanSession(input string) (*ScanSession, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	query := `
		INSERT INTO scan_sessions (input, started_at, status)
		VALUES (?, ?, ?)
	`

	result, err := r.db.db.Exec(query, input, time.Now(), "running")
	if err != nil {
		return nil, fmt.Errorf("failed to create scan session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get session ID: %w", err)
	}

	return &ScanSession{
		ID:        id,
		Input:     input,
		StartedAt: time.Now(),
		Status:    "running",
	}, nil
}

// UpdateScanSession updates a scan session
func (r *Repository) UpdateScanSession(session *ScanSession) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	query := `
		UPDATE scan_sessions
		SET completed_at = ?, status = ?, total_traces = ?, unique_traces = ?, errors = ?
		WHERE id = ?
	`

	_, err := r.db.db.Exec(query,
		session.CompletedAt,
		session.Status,
		session.TotalTraces,
		session.UniqueTraces,
		session.Errors,
		session.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update scan session: %w", err)
	}

	return nil
}

// GetScanSession retrieves a scan session by ID
func (r *Repository) GetScanSession(id int64) (*ScanSession, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	query := `
		SELECT id, input, started_at, completed_at, status, total_traces, unique_traces, errors
		FROM scan_sessions
		WHERE id = ?
	`

	var session ScanSession
	err := r.db.db.QueryRow(query, id).Scan(
		&session.ID,
		&session.Input,
		&session.StartedAt,
		&session.CompletedAt,
		&session.Status,
		&session.TotalTraces,
		&session.UniqueTraces,
		&session.Errors,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get scan session: %w", err)
	}

	return &session, nil
}

// GetScanSessions retrieves scan sessions based on query parameters
func (r *Repository) GetScanSessions(query ScanQuery) ([]ScanSession, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	conditions := []string{"1=1"}
	args := []interface{}{}

	if query.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, *query.Status)
	}

	if query.FromDate != nil {
		conditions = append(conditions, "started_at >= ?")
		args = append(args, query.FromDate)
	}

	if query.ToDate != nil {
		conditions = append(conditions, "started_at <= ?")
		args = append(args, query.ToDate)
	}

	whereClause := strings.Join(conditions, " AND ")
	sqlQuery := fmt.Sprintf(`
		SELECT id, input, started_at, completed_at, status, total_traces, unique_traces, errors
		FROM scan_sessions
		WHERE %s
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, query.Limit, query.Offset)

	rows, err := r.db.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query scan sessions: %w", err)
	}
	defer rows.Close()

	var sessions []ScanSession
	for rows.Next() {
		var session ScanSession
		err := rows.Scan(
			&session.ID,
			&session.Input,
			&session.StartedAt,
			&session.CompletedAt,
			&session.Status,
			&session.TotalTraces,
			&session.UniqueTraces,
			&session.Errors,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// CacheRepository methods

// StoreCacheEntry stores a cache entry
func (r *Repository) StoreCacheEntry(entry *CacheEntry) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	query := `
		INSERT OR REPLACE INTO cache_entries (key, value, created_at, expires_at, plugin_name)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.db.db.Exec(query,
		entry.Key,
		entry.Value,
		entry.CreatedAt,
		entry.ExpiresAt,
		entry.PluginName,
	)
	if err != nil {
		return fmt.Errorf("failed to store cache entry: %w", err)
	}

	return nil
}

// GetCacheEntry retrieves a cache entry by key
func (r *Repository) GetCacheEntry(key string) (*CacheEntry, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	query := `
		SELECT key, value, created_at, expires_at, plugin_name
		FROM cache_entries
		WHERE key = ?
	`

	var entry CacheEntry
	err := r.db.db.QueryRow(query, key).Scan(
		&entry.Key,
		&entry.Value,
		&entry.CreatedAt,
		&entry.ExpiresAt,
		&entry.PluginName,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}

	// Check if entry is expired
	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		return nil, nil
	}

	return &entry, nil
}

// CleanExpiredCache removes expired cache entries
func (r *Repository) CleanExpiredCache() error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	query := `
		DELETE FROM cache_entries
		WHERE expires_at IS NOT NULL AND expires_at < ?
	`

	_, err := r.db.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to clean expired cache: %w", err)
	}

	return nil
}
