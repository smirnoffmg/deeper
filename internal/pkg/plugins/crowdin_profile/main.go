package crowdin_profile

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type CrowdinProfilePlugin struct {
	fetcher pageFetcher
}

func NewPlugin(cfg *config.Config) *CrowdinProfilePlugin {
	return &CrowdinProfilePlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *CrowdinProfilePlugin) Register(r plugins.Registry) error {
	r.Add(entities.SocialGeneric, p)
	return nil
}

func (p *CrowdinProfilePlugin) Matches(trace entities.Trace) bool {
	return trace.Type == entities.SocialGeneric && extractHandle(trace.Value) != ""
}

func (p *CrowdinProfilePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if !p.Matches(trace) {
		return nil, nil
	}
	return fetchProfile(ctx, p.fetcher, extractHandle(trace.Value))
}

func (p CrowdinProfilePlugin) String() string {
	return "CrowdinProfilePlugin"
}
