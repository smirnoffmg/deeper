package dns_records

import (
	"context"
	"net"
	"strings"

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

type DNSRecordsPlugin struct {
	lookups dnsLookups
	doh     dohFetcher
}

func NewPlugin() *DNSRecordsPlugin {
	return &DNSRecordsPlugin{
		lookups: net.DefaultResolver,
		doh:     deeperhttp.NewClient(config.LoadConfig()),
	}
}

func (p *DNSRecordsPlugin) Register() error {
	state.RegisterPlugin(entities.Domain, p)
	state.RegisterPlugin(entities.Subdomain, p)
	return nil
}

func (p *DNSRecordsPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Domain && trace.Type != entities.Subdomain {
		return nil, nil
	}

	if strings.Contains(trace.Value, "*") {
		return nil, nil
	}

	ctx := context.Background()

	var traces []entities.Trace
	traces = append(traces, lookupStdlibRecords(ctx, trace.Value, p.lookups)...)
	traces = append(traces, lookupDoHRecords(ctx, trace.Value, p.doh)...)

	return traces, nil
}

func (p *DNSRecordsPlugin) String() string {
	return "DNSRecordsPlugin"
}
