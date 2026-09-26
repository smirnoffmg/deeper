package telegram_profile

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type TelegramProfilePlugin struct {
	fetcher pageFetcher
}

func NewPlugin(cfg *config.Config) *TelegramProfilePlugin {
	return &TelegramProfilePlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *TelegramProfilePlugin) Register(r plugins.Registry) error {
	r.Add(entities.SocialGeneric, p)
	return nil
}

func (p *TelegramProfilePlugin) Matches(trace entities.Trace) bool {
	return trace.Type == entities.SocialGeneric && extractChannel(trace.Value) != ""
}

func (p *TelegramProfilePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if !p.Matches(trace) {
		return nil, nil
	}
	return fetchProfile(ctx, p.fetcher, extractChannel(trace.Value))
}

func (p TelegramProfilePlugin) String() string {
	return "TelegramProfilePlugin"
}
