package keybase_profile

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

type KeybaseProfilePlugin struct {
	fetcher profileFetcher
}

func NewPlugin() *KeybaseProfilePlugin {
	return &KeybaseProfilePlugin{fetcher: deeperhttp.NewClient(config.LoadConfig())}
}

func (p *KeybaseProfilePlugin) Register() error {
	state.RegisterPlugin(entities.SocialGeneric, p)
	return nil
}

// Matches implements plugins.TraceMatcher: it lets the processor skip
// submitting a task for this plugin entirely for traces it would just
// no-op on, rather than paying a domain rate-limit wait for nothing. All
// sherlock hits share the single entities.SocialGeneric type, so this
// plugin (like every other platform-specific one built alongside it)
// self-filters by host rather than relying on the trace type alone.
func (p *KeybaseProfilePlugin) Matches(trace entities.Trace) bool {
	return trace.Type == entities.SocialGeneric && extractHandle(trace.Value) != ""
}

func (p *KeybaseProfilePlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if !p.Matches(trace) {
		return nil, nil
	}
	return fetchProfile(context.Background(), p.fetcher, extractHandle(trace.Value))
}

func (p KeybaseProfilePlugin) String() string {
	return "KeybaseProfilePlugin"
}
