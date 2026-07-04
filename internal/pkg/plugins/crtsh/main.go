package crtsh

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

const InputTraceType = entities.Domain

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type certFetcher interface {
	Get(url string) (*http.Response, error)
}

type httpCertFetcher struct{}

func (httpCertFetcher) Get(url string) (*http.Response, error) {
	return http.Get(url)
}

type SubdomainPlugin struct {
	fetcher certFetcher
}

func NewPlugin() *SubdomainPlugin {
	return &SubdomainPlugin{fetcher: httpCertFetcher{}}
}

func (g *SubdomainPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, g)
	return nil
}

type CrtShEntry struct {
	NameValue string `json:"name_value"`
}

func (g *SubdomainPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	url := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", trace.Value)
	resp, err := g.fetcher.Get(url)
	if err != nil {
		log.Warn().Err(err).Str("domain", trace.Value).Msg("crt.sh request failed, skipping")
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn().Err(err).Str("domain", trace.Value).Msg("crt.sh response read failed, skipping")
		return nil, nil
	}

	entries, ok := decodeEntries(body, trace.Value)
	if !ok {
		return nil, nil
	}

	var newTraces []entities.Trace
	for _, entry := range entries {
		subdomains := parseSubdomains(entry.NameValue)
		for _, subdomain := range subdomains {
			newTraces = append(newTraces, entities.Trace{
				Value: subdomain,
				Type:  entities.Subdomain,
			})
		}
	}

	return newTraces, nil
}

func decodeEntries(body []byte, domain string) ([]CrtShEntry, bool) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		log.Warn().Str("domain", domain).Msg("crt.sh returned empty response, skipping")
		return nil, false
	}
	if trimmed[0] == '<' {
		log.Warn().Str("domain", domain).Msg("crt.sh returned HTML instead of JSON, skipping")
		return nil, false
	}

	var entries []CrtShEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		log.Warn().Err(err).Str("domain", domain).Msg("crt.sh returned invalid JSON, skipping")
		return nil, false
	}

	return entries, true
}

func parseSubdomains(nameValue string) []string {
	subdomains := make(map[string]bool)
	for _, subdomain := range strings.Split(nameValue, "\n") {
		subdomain = strings.TrimSpace(subdomain)
		if subdomain != "" {
			subdomains[subdomain] = true
		}
	}

	var uniqueSubdomains []string
	for subdomain := range subdomains {
		uniqueSubdomains = append(uniqueSubdomains, subdomain)
	}

	return uniqueSubdomains
}

func (g SubdomainPlugin) String() string {
	return "CrtShPlugin"
}
