package domain

import (
	"net"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

func init() {
	d := DNSLookup{}
	d.Register()
}

type DNSLookup struct {
}

func (m *DNSLookup) Register() error {
	plugins := state.ActivePlugins[entities.IpAddr]
	state.ActivePlugins[entities.IpAddr] = append(plugins, m)
	return nil
}

func (m *DNSLookup) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	ip_addr := trace.Value

	// get A and AAAA records

	records, err := net.LookupAddr(ip_addr)

	if err != nil {
		log.Error().Err(err).Msg("error looking up ip_addr")
	}

	result := []entities.Trace{}

	for _, record := range records {
		result = append(result, entities.Trace{
			Value: record[:len(record)-1],
			Type:  entities.Domain,
		})
	}

	return result, nil
}

func (m DNSLookup) String() string {
	return "DNSLookup"
}
