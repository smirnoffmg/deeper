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
	dohBaseURL    = "https://dns.google/resolve"
	dohTypeSOA    = "SOA"
	dohTypeCAA    = "CAA"
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

func lookupDoHRecords(ctx context.Context, name string, fetcher dohFetcher) []entities.Trace {
	var traces []entities.Trace

	soaTraces := fetchDoHType(ctx, name, dohTypeSOA, entities.DnsRecordSOA, fetcher)
	traces = append(traces, soaTraces...)

	caaTraces := fetchDoHType(ctx, name, dohTypeCAA, entities.DnsRecordCAA, fetcher)
	traces = append(traces, caaTraces...)

	return traces
}

func fetchDoHType(ctx context.Context, name, recordType string, traceType entities.TraceType, fetcher dohFetcher) []entities.Trace {
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
		traces = append(traces, entities.Trace{
			Type:  traceType,
			Value: answer.Data,
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
