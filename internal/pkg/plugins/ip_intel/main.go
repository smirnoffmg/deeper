package ip_intel

import (
	"context"
	"net"

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

func (p *IPIntelPlugin) Register() error {
	state.RegisterPlugin(entities.IpAddr, p)
	return nil
}

func (p *IPIntelPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.IpAddr {
		return nil, nil
	}

	ctx := context.Background()

	var traces []entities.Trace
	traces = append(traces, lookupASN(ctx, trace.Value, p.txt)...)
	traces = append(traces, lookupPTR(ctx, trace.Value, p.addr)...)

	return traces, nil
}

func (p *IPIntelPlugin) String() string {
	return "IPIntelPlugin"
}
