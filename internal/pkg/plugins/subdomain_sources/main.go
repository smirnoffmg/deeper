package subdomain_sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const inputTraceType = entities.Domain

// Source: SSLMate Cert Spotter keyless issuances API. AlienVault OTX passive DNS
// (the first-choice source) now rejects anonymous access ("Anonymous access to
// this endpoint is limited. Please authenticate."), so this plugin uses Cert
// Spotter, which returns data without an API key. Each issuance object carries a
// dns_names array of hostnames covered by the certificate.
const certspotterURL = "https://api.certspotter.com/v1/issuances?domain=%s&include_subdomains=true&expand=dns_names"

type subdomainFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type SubdomainSourcesPlugin struct {
	fetcher subdomainFetcher
}

func NewPlugin(cfg *config.Config) *SubdomainSourcesPlugin {
	return &SubdomainSourcesPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *SubdomainSourcesPlugin) Register(r plugins.Registry) error {
	r.Add(inputTraceType, p)
	return nil
}

type certspotterIssuance struct {
	DNSNames []string `json:"dns_names"`
}

func (p *SubdomainSourcesPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != inputTraceType {
		return nil, nil
	}

	apex := strings.ToLower(strings.TrimSpace(trace.Value))
	url := fmt.Sprintf(certspotterURL, apex)

	resp, err := p.fetcher.Get(ctx, url)
	if err != nil {
		log.Warn().Err(err).Str("domain", apex).Msg("certspotter request failed, skipping")
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn().Err(err).Str("domain", apex).Msg("certspotter response read failed, skipping")
		return nil, nil
	}

	var issuances []certspotterIssuance
	if err := json.Unmarshal(body, &issuances); err != nil {
		log.Warn().Err(err).Str("domain", apex).Msg("certspotter returned invalid JSON, skipping")
		return nil, nil
	}

	seen := make(map[string]bool)
	var newTraces []entities.Trace
	for _, issuance := range issuances {
		for _, name := range issuance.DNSNames {
			host := strings.ToLower(strings.TrimSpace(name))
			if host == "" || host == apex || strings.Contains(host, "*") || seen[host] {
				continue
			}
			seen[host] = true
			newTraces = append(newTraces, entities.Trace{Value: host, Type: entities.Subdomain})
		}
	}

	return newTraces, nil
}

func (p *SubdomainSourcesPlugin) String() string {
	return "SubdomainSourcesPlugin"
}
