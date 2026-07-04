package gravatar

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

const InputTraceType = entities.Email

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type GravatarPlugin struct {
	fetcher profileFetcher
	apiKey  string
}

func NewPlugin() *GravatarPlugin {
	cfg := config.LoadConfig()
	return &GravatarPlugin{
		fetcher: deeperhttp.NewClient(cfg),
		apiKey:  cfg.GravatarAPIKey,
	}
}

func (p *GravatarPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, p)
	return nil
}

func (p *GravatarPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	hash := emailHash(trace.Value)
	profile, found, err := fetchProfile(context.Background(), p.fetcher, hash, p.apiKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	return profileToTraces(profile, trace.Value), nil
}

func (p *GravatarPlugin) String() string {
	return "GravatarPlugin"
}
