package habr_profile

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type HabrProfilePlugin struct {
	fetcher profileFetcher
}

func NewPlugin(cfg *config.Config) *HabrProfilePlugin {
	return &HabrProfilePlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *HabrProfilePlugin) Register(r plugins.Registry) error {
	r.Add(entities.Username, p)
	return nil
}

func (p *HabrProfilePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username {
		return nil, nil
	}
	return fetchProfile(ctx, p.fetcher, trace.Value)
}

func (p HabrProfilePlugin) String() string {
	return "HabrProfilePlugin"
}
