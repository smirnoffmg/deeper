package dns_records

import (
	"context"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type DNSRecordsPlugin struct {
	doh dohFetcher
}

func NewPlugin(cfg *config.Config) *DNSRecordsPlugin {
	return &DNSRecordsPlugin{
		doh: deeperhttp.NewClient(cfg),
	}
}

func (p *DNSRecordsPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Domain, p)
	r.Add(entities.Subdomain, p)
	return nil
}

func (p *DNSRecordsPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Domain && trace.Type != entities.Subdomain {
		return nil, nil
	}

	if strings.Contains(trace.Value, "*") {
		return nil, nil
	}

	return lookupDoHRecords(ctx, trace.Value, p.doh), nil
}

func (p *DNSRecordsPlugin) String() string {
	return "DNSRecordsPlugin"
}
