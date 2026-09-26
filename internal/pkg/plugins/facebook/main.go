package facebook

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type FacebookPlugin struct {
	fetcher searchFetcher
}

func NewPlugin(cfg *config.Config) *FacebookPlugin {
	return &FacebookPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (g *FacebookPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Username, g)
	r.Add(entities.Name, g)
	return nil
}

func (g *FacebookPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username && trace.Type != entities.Name {
		return nil, nil
	}

	profiles, err := searchFacebookProfiles(ctx, g.fetcher, trace.Value)
	if err != nil {
		return nil, err
	}

	var newTraces []entities.Trace
	for _, profile := range profiles {
		newTraces = append(newTraces, entities.Trace{Value: profile, Type: entities.Url})
	}
	return newTraces, nil
}

func (g FacebookPlugin) String() string {
	return "FacebookPlugin"
}
