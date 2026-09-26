package plugins

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type DeeperPlugin interface {
	FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error)
	String() string
}

// Registry maps each trace type to the plugins that follow it.
type Registry map[entities.TraceType][]DeeperPlugin

func (r Registry) Add(traceType entities.TraceType, plugin DeeperPlugin) {
	r[traceType] = append(r[traceType], plugin)
}

// TraceMatcher lets a plugin declare, without doing any I/O, whether it
// would act on a given trace. Plugins implementing it let the processor
// skip task creation -- and the domain rate-limit wait bundled into
// workerpool.Submit() -- entirely for traces they'd immediately no-op on.
// This matters for plugins that register under a broad trace type (e.g.
// entities.SocialGeneric) but only actually handle a narrow slice of it.
type TraceMatcher interface {
	Matches(trace entities.Trace) bool
}
