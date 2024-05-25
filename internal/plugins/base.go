package plugins

import "github.com/smirnoffmg/deeper/internal/entities"

type DeeperPlugin interface {
	Register() error
	FollowTrace(trace entities.Trace) ([]entities.Trace, error)
	String() string
}
