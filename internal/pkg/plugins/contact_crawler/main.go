package contact_crawler

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const maxPagesPerRegistrableDomainPerProcess = 60

type ContactCrawlerPlugin struct {
	fetcher      pageFetcher
	domainBudget *domainBudget
}

func NewPlugin(cfg *config.Config) *ContactCrawlerPlugin {
	return &ContactCrawlerPlugin{
		fetcher:      deeperhttp.NewClient(cfg),
		domainBudget: newDomainBudget(maxPagesPerRegistrableDomainPerProcess),
	}
}

func (p *ContactCrawlerPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Domain, p)
	r.Add(entities.Subdomain, p)
	return nil
}

func (p *ContactCrawlerPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Domain && trace.Type != entities.Subdomain {
		return nil, nil
	}

	seedURL := normalizeURL(trace.Value)
	c := newCrawler(p.fetcher, trace.Value, p.domainBudget)
	return c.crawl(ctx, seedURL)
}

func (p *ContactCrawlerPlugin) String() string {
	return "ContactCrawlerPlugin"
}
