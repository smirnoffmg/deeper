package facebook

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type FacebookPlugin struct {
	fetcher searchFetcher
}

func NewPlugin() *FacebookPlugin {
	return &FacebookPlugin{fetcher: deeperhttp.NewClient(config.LoadConfig())}
}

func (g *FacebookPlugin) Register() error {
	state.RegisterPlugin(entities.Username, g)
	state.RegisterPlugin(entities.Name, g)
	return nil
}

func (g *FacebookPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username && trace.Type != entities.Name {
		return nil, nil
	}

	profiles, err := searchFacebookProfiles(context.Background(), g.fetcher, trace.Value)
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
