package ip_intel

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type addrLookup interface {
	LookupAddr(ctx context.Context, addr string) ([]string, error)
}

func lookupPTR(ctx context.Context, ip string, lookups addrLookup) []entities.Trace {
	names, err := lookups.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return nil
	}

	traces := make([]entities.Trace, 0, len(names))
	for _, name := range names {
		traces = append(traces, entities.Trace{
			Type:  entities.DnsRecordPTR,
			Value: name,
		})
	}
	return traces
}
