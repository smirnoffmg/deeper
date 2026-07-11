package dns_records

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

const (
	dohBaseURL   = "https://dns.google/resolve"
	dohTypeMX    = "MX"
	dohTypeTXT   = "TXT"
	dohTypeNS    = "NS"
	dohTypeCNAME = "CNAME"
	dohTypeSOA   = "SOA"
	dohTypeCAA   = "CAA"
)

type dohFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type dohResponse struct {
	Status int `json:"Status"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		Data string `json:"data"`
	} `json:"Answer"`
}

// lookupDoHRecords fetches all six record types independently over
// DNS-over-HTTPS. Each is its own HTTP request: one type failing (rate
// limit, transient network error) must not suppress the others. DoH is used
// for every type here, not just SOA/CAA (which stdlib's net.Resolver can't
// look up at all) — some environments refuse raw DNS wire queries over
// UDP/TCP port 53 entirely while outbound HTTPS works fine, so routing
// MX/TXT/NS/CNAME through DoH too makes this plugin resolver-independent.
func lookupDoHRecords(ctx context.Context, name string, fetcher dohFetcher) []entities.Trace {
	var traces []entities.Trace

	traces = append(traces, fetchDoHType(ctx, name, dohTypeMX, entities.DnsRecordMX, fetcher, mxHostValue)...)
	traces = append(traces, fetchDoHType(ctx, name, dohTypeTXT, entities.DnsRecordTXT, fetcher, identityValue)...)
	traces = append(traces, fetchDoHType(ctx, name, dohTypeNS, entities.DnsRecordNS, fetcher, identityValue)...)
	traces = append(traces, fetchDoHType(ctx, name, dohTypeCNAME, entities.DnsRecordCNAME, fetcher, cnameValue(name))...)
	traces = append(traces, fetchDoHType(ctx, name, dohTypeSOA, entities.DnsRecordSOA, fetcher, identityValue)...)
	traces = append(traces, fetchDoHType(ctx, name, dohTypeCAA, entities.DnsRecordCAA, fetcher, identityValue)...)

	return traces
}

// valueOf extracts the trace value from a DoH answer's raw data field, or
// reports ok=false to skip that answer (e.g. a self-referential CNAME).
type valueOf func(data string) (string, bool)

func identityValue(data string) (string, bool) {
	return data, true
}

// mxHostValue parses DoH's "<preference> <host>" MX data format (e.g.
// "10 ALT4.ASPMX.L.GOOGLE.COM.", verified against dns.google/resolve) and
// drops the numeric preference, matching stdlib's net.MX.Host convention.
func mxHostValue(data string) (string, bool) {
	fields := strings.Fields(data)
	if len(fields) < 2 {
		return "", false
	}
	return fields[len(fields)-1], true
}

func cnameValue(queried string) valueOf {
	return func(data string) (string, bool) {
		if isSelfReferentialCNAME(queried, data) {
			return "", false
		}
		return data, true
	}
}

func isSelfReferentialCNAME(queried, cname string) bool {
	return normalizeDNSName(queried) == normalizeDNSName(cname)
}

func normalizeDNSName(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
}

func fetchDoHType(ctx context.Context, name, recordType string, traceType entities.TraceType, fetcher dohFetcher, valueOf valueOf) []entities.Trace {
	reqURL := fmt.Sprintf("%s?name=%s&type=%s", dohBaseURL, url.QueryEscape(name), recordType)

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		log.Warn().Err(err).Str("domain", name).Str("type", recordType).Msg("DoH request failed, skipping")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn().Err(err).Str("domain", name).Str("type", recordType).Msg("DoH response read failed, skipping")
		return nil
	}

	var parsed dohResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Warn().Err(err).Str("domain", name).Str("type", recordType).Msg("DoH returned invalid JSON, skipping")
		return nil
	}

	var traces []entities.Trace
	for _, answer := range parsed.Answer {
		if answer.Data == "" {
			continue
		}
		value, ok := valueOf(answer.Data)
		if !ok {
			continue
		}
		traces = append(traces, entities.Trace{
			Type:  traceType,
			Value: value,
		})

		if traceType == entities.DnsRecordSOA {
			if email, ok := emailFromSOAData(answer.Data); ok {
				traces = append(traces, entities.Trace{
					Type:  entities.Email,
					Value: email,
				})
			}
		}
	}

	return traces
}

func emailFromSOAData(data string) (string, bool) {
	fields := strings.Fields(data)
	if len(fields) < 2 {
		return "", false
	}
	return decodeSOARnameEmail(fields[1])
}

// decodeSOARnameEmail converts an SOA RNAME to an email address.
// The first unescaped dot in the RNAME is the @ separator per RFC 1035.
func decodeSOARnameEmail(rname string) (string, bool) {
	rname = strings.TrimSuffix(strings.TrimSpace(rname), ".")
	if rname == "" {
		return "", false
	}

	var local strings.Builder
	for i := 0; i < len(rname); i++ {
		if rname[i] == '\\' && i+1 < len(rname) && rname[i+1] == '.' {
			local.WriteByte('.')
			i++
			continue
		}
		if rname[i] == '.' {
			domain := rname[i+1:]
			if domain == "" || local.Len() == 0 {
				return "", false
			}
			return local.String() + "@" + domain, true
		}
		local.WriteByte(rname[i])
	}

	return "", false
}
