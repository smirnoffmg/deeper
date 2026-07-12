package telegram_profile

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

type TelegramProfilePlugin struct {
	fetcher pageFetcher
}

func NewPlugin() *TelegramProfilePlugin {
	return &TelegramProfilePlugin{fetcher: deeperhttp.NewClient(config.LoadConfig())}
}

func (p *TelegramProfilePlugin) Register() error {
	state.RegisterPlugin(entities.SocialGeneric, p)
	return nil
}

func (p *TelegramProfilePlugin) Matches(trace entities.Trace) bool {
	return trace.Type == entities.SocialGeneric && extractChannel(trace.Value) != ""
}

func (p *TelegramProfilePlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if !p.Matches(trace) {
		return nil, nil
	}
	return fetchProfile(context.Background(), p.fetcher, extractChannel(trace.Value))
}

func (p TelegramProfilePlugin) String() string {
	return "TelegramProfilePlugin"
}
