package bluesky_profile

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

type BlueskyProfilePlugin struct {
	fetcher profileFetcher
}

func NewPlugin() *BlueskyProfilePlugin {
	return &BlueskyProfilePlugin{fetcher: deeperhttp.NewClient(config.LoadConfig())}
}

func (p *BlueskyProfilePlugin) Register() error {
	state.RegisterPlugin(entities.SocialGeneric, p)
	return nil
}

func (p *BlueskyProfilePlugin) Matches(trace entities.Trace) bool {
	return trace.Type == entities.SocialGeneric && extractHandle(trace.Value) != ""
}

func (p *BlueskyProfilePlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if !p.Matches(trace) {
		return nil, nil
	}
	return fetchProfile(context.Background(), p.fetcher, extractHandle(trace.Value))
}

func (p BlueskyProfilePlugin) String() string {
	return "BlueskyProfilePlugin"
}
