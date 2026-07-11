package whois

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type WhoisPlugin struct {
	client whoisClient
}

func NewPlugin() *WhoisPlugin {
	return &WhoisPlugin{client: &tcpWhoisClient{timeout: 10 * time.Second}}
}

// Register only covers Domain — unlike most other plugins in this codebase,
// WHOIS is a registration-level lookup keyed to the registrable domain, not
// meaningful per-subdomain (most registries just return "not found").
func (p *WhoisPlugin) Register() error {
	state.RegisterPlugin(entities.Domain, p)
	return nil
}

func (p *WhoisPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Domain {
		return nil, nil
	}
	return lookupWhois(context.Background(), p.client, trace.Value)
}

func (p WhoisPlugin) String() string {
	return "WhoisPlugin"
}
