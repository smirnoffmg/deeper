package contact_crawler

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

const maxPagesPerRegistrableDomainPerProcess = 60

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type ContactCrawlerPlugin struct {
	fetcher      pageFetcher
	domainBudget *domainBudget
}

func NewPlugin() *ContactCrawlerPlugin {
	return &ContactCrawlerPlugin{
		fetcher:      deeperhttp.NewClient(config.LoadConfig()),
		domainBudget: newDomainBudget(maxPagesPerRegistrableDomainPerProcess),
	}
}

func (p *ContactCrawlerPlugin) Register() error {
	state.RegisterPlugin(entities.Domain, p)
	state.RegisterPlugin(entities.Subdomain, p)
	return nil
}

func (p *ContactCrawlerPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Domain && trace.Type != entities.Subdomain {
		return nil, nil
	}

	seedURL := normalizeURL(trace.Value)
	c := newCrawler(p.fetcher, trace.Value, p.domainBudget)
	return c.crawl(context.Background(), seedURL)
}

func (p *ContactCrawlerPlugin) String() string {
	return "ContactCrawlerPlugin"
}
