package dns_resolver

import (
	"context"
	"net"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const InputTraceType = entities.Subdomain

// ipResolver is the network boundary, injectable so FollowTrace can be tested without real DNS calls.
type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type DNSResolverPlugin struct {
	resolver ipResolver
}

func NewPlugin() *DNSResolverPlugin {
	return &DNSResolverPlugin{resolver: net.DefaultResolver}
}

func (p *DNSResolverPlugin) Register(r plugins.Registry) error {
	r.Add(InputTraceType, p)
	return nil
}

func (p *DNSResolverPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	if strings.Contains(trace.Value, "*") {
		return nil, nil
	}

	addrs, err := p.resolver.LookupIPAddr(ctx, trace.Value)
	if err != nil {
		return nil, err
	}

	newTraces := make([]entities.Trace, 0, len(addrs))
	for _, addr := range addrs {
		newTraces = append(newTraces, entities.Trace{
			Value: addr.IP.String(),
			Type:  entities.IpAddr,
		})
	}

	return newTraces, nil
}

func (p *DNSResolverPlugin) String() string {
	return "DNSResolverPlugin"
}
