package state

import (
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/plugins"
)

var ActivePlugins map[entities.TraceType][]plugins.DeeperPlugin = make(map[entities.TraceType][]plugins.DeeperPlugin)
