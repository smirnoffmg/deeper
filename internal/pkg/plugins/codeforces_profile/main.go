package codeforces_profile

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type CodeforcesProfilePlugin struct {
	fetcher profileFetcher
}

func NewPlugin(cfg *config.Config) *CodeforcesProfilePlugin {
	return &CodeforcesProfilePlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *CodeforcesProfilePlugin) Register(r plugins.Registry) error {
	r.Add(entities.SocialGeneric, p)
	return nil
}

func (p *CodeforcesProfilePlugin) Matches(trace entities.Trace) bool {
	return trace.Type == entities.SocialGeneric && extractHandle(trace.Value) != ""
}

func (p *CodeforcesProfilePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if !p.Matches(trace) {
		return nil, nil
	}
	return fetchProfile(ctx, p.fetcher, extractHandle(trace.Value))
}

func (p CodeforcesProfilePlugin) String() string {
	return "CodeforcesProfilePlugin"
}
