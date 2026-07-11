package database

import (
	"encoding/json"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

const SeedPluginName = "__seed__"

// Trace represents a stored trace in the database
type Trace struct {
	ID           int64                  `json:"id" db:"id"`
	Value        string                 `json:"value" db:"value"`
	Type         entities.TraceType     `json:"type" db:"type"`
	DiscoveredAt time.Time              `json:"discovered_at" db:"discovered_at"`
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
}

// TraceEdge represents a parent→plugin→child discovery link within a scan.
type TraceEdge struct {
	ID            int64     `json:"id" db:"id"`
	ParentTraceID *int64    `json:"parent_trace_id" db:"parent_trace_id"`
	ChildTraceID  int64     `json:"child_trace_id" db:"child_trace_id"`
	PluginName    string    `json:"plugin_name" db:"plugin_name"`
	ScanID        int64     `json:"scan_id" db:"scan_id"`
	DiscoveredAt  time.Time `json:"discovered_at" db:"discovered_at"`
}

// ReachableTrace is a trace reachable from a start node within a hop budget.
type ReachableTrace struct {
	TraceID int64              `json:"trace_id"`
	Value   string             `json:"value"`
	Type    entities.TraceType `json:"type"`
	Hops    int                `json:"hops"`
}

// ScanSession represents a scan session in the database
type ScanSession struct {
	ID           int64      `json:"id" db:"id"`
	Input        string     `json:"input" db:"input"`
	StartedAt    time.Time  `json:"started_at" db:"started_at"`
	CompletedAt  *time.Time `json:"completed_at" db:"completed_at"`
	Status       string     `json:"status" db:"status"`
	TotalTraces  int        `json:"total_traces" db:"total_traces"`
	UniqueTraces int        `json:"unique_traces" db:"unique_traces"`
	Errors       int        `json:"errors" db:"errors"`
}

// CacheEntry represents a cached plugin result
type CacheEntry struct {
	Key        string     `json:"key" db:"key"`
	Value      string     `json:"value" db:"value"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at" db:"expires_at"`
	PluginName string     `json:"plugin_name" db:"plugin_name"`
}

// TraceQuery represents query parameters for searching traces
type TraceQuery struct {
	Type     *entities.TraceType `json:"type,omitempty"`
	FromDate *time.Time          `json:"from_date,omitempty"`
	ToDate   *time.Time          `json:"to_date,omitempty"`
	Limit    int                 `json:"limit"`
	Offset   int                 `json:"offset"`
}

// ScanQuery represents query parameters for searching scan sessions
type ScanQuery struct {
	Status   *string    `json:"status,omitempty"`
	FromDate *time.Time `json:"from_date,omitempty"`
	ToDate   *time.Time `json:"to_date,omitempty"`
	Limit    int        `json:"limit"`
	Offset   int        `json:"offset"`
}

// TraceStats represents statistics about traces
type TraceStats struct {
	TotalTraces    int                      `json:"total_traces"`
	TracesByType   map[string]int           `json:"traces_by_type"`
	TracesByPlugin map[string]int           `json:"traces_by_plugin"`
	RecentTraces   []Trace                  `json:"recent_traces"`
	TopSources     []map[string]interface{} `json:"top_sources"`
}

// ScanStats represents statistics about scan sessions
type ScanStats struct {
	TotalSessions     int           `json:"total_sessions"`
	CompletedSessions int           `json:"completed_sessions"`
	RunningSessions   int           `json:"running_sessions"`
	FailedSessions    int           `json:"failed_sessions"`
	AverageTraces     float64       `json:"average_traces"`
	TotalTraces       int           `json:"total_traces"`
	RecentSessions    []ScanSession `json:"recent_sessions"`
}

// DatabaseStats represents overall database statistics
type DatabaseStats struct {
	Traces     TraceStats `json:"traces"`
	Scans      ScanStats  `json:"scans"`
	Cache      CacheStats `json:"cache"`
	Size       int64      `json:"size_bytes"`
	LastUpdate time.Time  `json:"last_update"`
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries   int       `json:"total_entries"`
	ExpiredEntries int       `json:"expired_entries"`
	ValidEntries   int       `json:"valid_entries"`
	OldestEntry    time.Time `json:"oldest_entry"`
	NewestEntry    time.Time `json:"newest_entry"`
}

// ToEntity converts a database Trace to an entities.Trace
func (t *Trace) ToEntity() entities.Trace {
	return entities.Trace{
		Value: t.Value,
		Type:  t.Type,
	}
}

// MarshalMetadata marshals metadata to JSON string
func (t *Trace) MarshalMetadata() (string, error) {
	if t.Metadata == nil {
		return "", nil
	}
	data, err := json.Marshal(t.Metadata)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalMetadata unmarshals metadata from JSON string
func (t *Trace) UnmarshalMetadata(data string) error {
	if data == "" {
		t.Metadata = nil
		return nil
	}
	return json.Unmarshal([]byte(data), &t.Metadata)
}
