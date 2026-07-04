package dns_records

import (
	"context"
	"net"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type dnsLookups interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupNS(ctx context.Context, name string) ([]*net.NS, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
}

// lookupStdlibRecords looks up MX/TXT/NS/CNAME independently: one record
// type failing (e.g. a resolver that only supports A/AAAA/PTR lookups, not
// raw DNS wire queries) must not suppress the others, and must not block
// the separate DoH-backed SOA/CAA lookups in the caller.
func lookupStdlibRecords(ctx context.Context, name string, lookups dnsLookups) []entities.Trace {
	var traces []entities.Trace

	mxRecords, err := lookups.LookupMX(ctx, name)
	if err != nil {
		log.Warn().Err(err).Str("domain", name).Str("type", "MX").Msg("stdlib DNS lookup failed, skipping")
	}
	for _, mx := range mxRecords {
		traces = append(traces, entities.Trace{
			Type:  entities.DnsRecordMX,
			Value: mx.Host,
		})
	}

	txtRecords, err := lookups.LookupTXT(ctx, name)
	if err != nil {
		log.Warn().Err(err).Str("domain", name).Str("type", "TXT").Msg("stdlib DNS lookup failed, skipping")
	}
	for _, txt := range txtRecords {
		traces = append(traces, entities.Trace{
			Type:  entities.DnsRecordTXT,
			Value: txt,
		})
	}

	nsRecords, err := lookups.LookupNS(ctx, name)
	if err != nil {
		log.Warn().Err(err).Str("domain", name).Str("type", "NS").Msg("stdlib DNS lookup failed, skipping")
	}
	for _, ns := range nsRecords {
		traces = append(traces, entities.Trace{
			Type:  entities.DnsRecordNS,
			Value: ns.Host,
		})
	}

	cname, err := lookups.LookupCNAME(ctx, name)
	if err != nil {
		log.Warn().Err(err).Str("domain", name).Str("type", "CNAME").Msg("stdlib DNS lookup failed, skipping")
	} else if cname != "" && !isSelfReferentialCNAME(name, cname) {
		traces = append(traces, entities.Trace{
			Type:  entities.DnsRecordCNAME,
			Value: cname,
		})
	}

	return traces
}

func isSelfReferentialCNAME(queried, cname string) bool {
	return normalizeDNSName(queried) == normalizeDNSName(cname)
}

func normalizeDNSName(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
}
