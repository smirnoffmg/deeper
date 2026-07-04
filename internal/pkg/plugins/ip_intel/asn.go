package ip_intel

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type txtLookup interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// lookupASN performs Team Cymru IP-to-ASN DNS lookups for IPv4 addresses.
// IPv6 is not supported in v1; non-IPv4 addresses return nil.
func lookupASN(ctx context.Context, ip string, lookups txtLookup) []entities.Trace {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil
	}
	parsed = parsed.To4()
	if parsed == nil {
		// IPv6 ASN lookup deferred to a follow-up; dns_resolver output has been IPv4 so far.
		return nil
	}

	originQuery := fmt.Sprintf("%s.origin.asn.cymru.com", reverseIPv4(parsed.String()))
	txtRecords, err := lookups.LookupTXT(ctx, originQuery)
	if err != nil {
		// Every routable IP has an ASN, so a lookup error here (as opposed to a
		// clean empty response) is a real failure worth surfacing, not silent
		// absence of data — confirmed live that raw DNS TXT queries can fail in
		// environments that still resolve A/AAAA/PTR fine via the OS resolver.
		log.Warn().Err(err).Str("ip", ip).Msg("ASN origin lookup failed, skipping")
		return nil
	}
	if len(txtRecords) == 0 {
		return nil
	}

	asn, netblock, ok := parseOriginResponse(txtRecords[0])
	if !ok {
		return nil
	}

	var traces []entities.Trace
	traces = append(traces,
		entities.Trace{Type: entities.ASN, Value: "AS" + asn},
		entities.Trace{Type: entities.Netblock, Value: netblock},
	)

	// Chained lookup for human-readable AS name; failure must not block ASN/Netblock.
	asNameTraces := lookupASName(ctx, asn, lookups)
	traces = append(traces, asNameTraces...)

	return traces
}

func reverseIPv4(ip string) string {
	octets := strings.Split(ip, ".")
	if len(octets) != 4 {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s.%s", octets[3], octets[2], octets[1], octets[0])
}

// parseOriginResponse parses Team Cymru origin TXT: "ASN | BGP Prefix | Country | Registry | Date"
func parseOriginResponse(txt string) (asn, netblock string, ok bool) {
	fields := splitPipeFields(txt)
	if len(fields) < 2 {
		return "", "", false
	}
	asn = strings.TrimSpace(fields[0])
	netblock = strings.TrimSpace(fields[1])
	if asn == "" || netblock == "" {
		return "", "", false
	}
	return asn, netblock, true
}

func lookupASName(ctx context.Context, asn string, lookups txtLookup) []entities.Trace {
	query := fmt.Sprintf("AS%s.asn.cymru.com", asn)
	txtRecords, err := lookups.LookupTXT(ctx, query)
	if err != nil {
		log.Warn().Err(err).Str("asn", asn).Msg("AS name lookup failed, skipping")
		return nil
	}
	if len(txtRecords) == 0 {
		return nil
	}

	asName, ok := parseASNameResponse(txtRecords[0])
	if !ok {
		return nil
	}

	// Company is deliberately reused here for the organization that owns this IP's ASN
	// (e.g. hosting provider), not an employer name from contact crawling.
	return []entities.Trace{
		{Type: entities.Company, Value: asName},
	}
}

// parseASNameResponse parses Team Cymru AS TXT: "ASN | Country | Registry | Allocated | AS Name"
func parseASNameResponse(txt string) (string, bool) {
	fields := splitPipeFields(txt)
	if len(fields) < 5 {
		return "", false
	}
	asName := strings.TrimSpace(fields[4])
	if asName == "" {
		return "", false
	}
	return asName, true
}

func splitPipeFields(txt string) []string {
	parts := strings.Split(txt, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
