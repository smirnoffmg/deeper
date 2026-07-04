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

// NewTraceProcessingTask creates a new trace processing task
func NewTraceProcessingTask(trace entities.Trace, pluginKey string, plugin interface{}) *TraceProcessingTask {
	return &TraceProcessingTask{
		Trace:     trace,
		PluginKey: pluginKey,
		Plugin:    plugin,
	}
}

// GetID returns a unique identifier for the task
func (t *TraceProcessingTask) GetID() string {
	return t.Trace.Value + ":" + t.PluginKey
}

// GetPayload returns the task payload
func (t *TraceProcessingTask) GetPayload() interface{} {
	return t
}
