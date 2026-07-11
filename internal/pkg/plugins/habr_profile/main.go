package habr_profile

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

type HabrProfilePlugin struct {
	fetcher profileFetcher
}

func NewPlugin() *HabrProfilePlugin {
	return &HabrProfilePlugin{fetcher: deeperhttp.NewClient(config.LoadConfig())}
}

func (p *HabrProfilePlugin) Register() error {
	state.RegisterPlugin(entities.Username, p)
	return nil
}

func (p *HabrProfilePlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username {
		return nil, nil
	}
	return fetchProfile(context.Background(), p.fetcher, trace.Value)
}

func (p HabrProfilePlugin) String() string {
	return "HabrProfilePlugin"
}
