package gravatar

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const InputTraceType = entities.Email

type GravatarPlugin struct {
	fetcher profileFetcher
	apiKey  string
}

func NewPlugin(cfg *config.Config) *GravatarPlugin {
	return &GravatarPlugin{
		fetcher: deeperhttp.NewClient(cfg),
		apiKey:  cfg.GravatarAPIKey,
	}
}

func (p *GravatarPlugin) Register(r plugins.Registry) error {
	r.Add(InputTraceType, p)
	return nil
}

func (p *GravatarPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	hash := emailHash(trace.Value)
	profile, found, err := fetchProfile(ctx, p.fetcher, hash, p.apiKey)
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
