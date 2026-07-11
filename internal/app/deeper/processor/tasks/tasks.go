package tasks

import (
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

// TraceProcessingTask represents a task for processing a trace through plugins
type TraceProcessingTask struct {
	Trace     entities.Trace
	PluginKey string
	Plugin    interface{}
}

// GetID returns a unique identifier for the task
func (t *TraceProcessingTask) GetID() string {
	return t.Trace.Value + ":" + t.PluginKey
}

// TraceValue exposes the underlying trace's value for callers (e.g. the
// workerpool's per-domain rate limiter) that need to inspect what's
// actually being processed without depending on this package's concrete type.
func (t *TraceProcessingTask) TraceValue() string {
	return t.Trace.Value
}

// GetPayload returns the task payload
func (t *TraceProcessingTask) GetPayload() interface{} {
	return t
}
