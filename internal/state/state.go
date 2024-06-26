package state

import (
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/plugins"
)

var ActivePlugins map[entities.TraceType][]plugins.DeeperPlugin = make(map[entities.TraceType][]plugins.DeeperPlugin)

func RegisterPlugin(traceType entities.TraceType, plugin plugins.DeeperPlugin) {
	ActivePlugins[traceType] = append(ActivePlugins[traceType], plugin)
}
