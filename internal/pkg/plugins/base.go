package plugins

import "github.com/smirnoffmg/deeper/internal/pkg/entities"

type DeeperPlugin interface {
	Register() error
	FollowTrace(trace entities.Trace) ([]entities.Trace, error)
	String() string
}
