package url_resolver

import (
	"net"
	"net/url"

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

// URLResolverPlugin re-opens the domain-based plugin chain (crtsh,
// dns_records, whois, contact_crawler, subdomains) for Url traces produced
// by facebook/academic_papers, which would otherwise be a dead end. Pure
// parsing, no network call.
type URLResolverPlugin struct{}

func NewPlugin() *URLResolverPlugin {
	return &URLResolverPlugin{}
}

func (p *URLResolverPlugin) Register() error {
	state.RegisterPlugin(entities.Url, p)
	return nil
}

func (p *URLResolverPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Url {
		return nil, nil
	}

	parsed, err := url.Parse(trace.Value)
	if err != nil || parsed.Hostname() == "" {
		return nil, nil
	}

	host := parsed.Hostname()
	if net.ParseIP(host) != nil {
		return nil, nil
	}

	return []entities.Trace{{Type: entities.Domain, Value: host}}, nil
}

func (p *URLResolverPlugin) String() string {
	return "URLResolverPlugin"
}
