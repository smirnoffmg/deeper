package url

import (
	"net/url"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

func init() {
	p := DomainExtractor{}
	p.Register()
}

type DomainExtractor struct{}

func (g *DomainExtractor) Register() error {
	pluginEntityType := entities.Url
	plugins := state.ActivePlugins[pluginEntityType]
	state.ActivePlugins[pluginEntityType] = append(plugins, g)

	return nil
}

// extract domain from url
// return domain as trace
func (g *DomainExtractor) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	u, err := url.Parse(trace.Value)

	if err != nil {
		log.Error().Msgf("Error parsing url %v: %v", trace.Value, err)
		return nil, nil
	}

	if u.Hostname() == "" {
		return nil, nil
	}

	return []entities.Trace{
		{
			Value: u.Hostname(),
			Type:  entities.Domain,
		},
	}, nil

}

func (g DomainExtractor) String() string {
	return "DomainExtractor"
}
