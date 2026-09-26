package ip_intel

import (
	"context"
	"net"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type IPIntelPlugin struct {
	txt  txtLookup
	addr addrLookup
}

func NewPlugin() *IPIntelPlugin {
	resolver := net.DefaultResolver
	return &IPIntelPlugin{
		txt:  resolver,
		addr: resolver,
	}
}

func (p *IPIntelPlugin) Register(r plugins.Registry) error {
	r.Add(entities.IpAddr, p)
	return nil
}

func (p *IPIntelPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.IpAddr {
		return nil, nil
	}

	var traces []entities.Trace
	traces = append(traces, lookupASN(ctx, trace.Value, p.txt)...)
	traces = append(traces, lookupPTR(ctx, trace.Value, p.addr)...)

	return traces, nil
}

func (p *IPIntelPlugin) String() string {
	return "IPIntelPlugin"
}
