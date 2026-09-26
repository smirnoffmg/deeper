package linuxorgru_profile

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type LinuxOrgRuProfilePlugin struct {
	fetcher pageFetcher
}

func NewPlugin(cfg *config.Config) *LinuxOrgRuProfilePlugin {
	return &LinuxOrgRuProfilePlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *LinuxOrgRuProfilePlugin) Register(r plugins.Registry) error {
	r.Add(entities.SocialGeneric, p)
	return nil
}

func (p *LinuxOrgRuProfilePlugin) Matches(trace entities.Trace) bool {
	return trace.Type == entities.SocialGeneric && extractHandle(trace.Value) != ""
}

func (p *LinuxOrgRuProfilePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if !p.Matches(trace) {
		return nil, nil
	}
	return fetchProfile(ctx, p.fetcher, extractHandle(trace.Value))
}

func (p LinuxOrgRuProfilePlugin) String() string {
	return "LinuxOrgRuProfilePlugin"
}
