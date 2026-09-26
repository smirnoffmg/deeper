package crtsh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const InputTraceType = entities.Domain

type certFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type SubdomainPlugin struct {
	fetcher certFetcher
}

func NewPlugin(cfg *config.Config) *SubdomainPlugin {
	return &SubdomainPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (g *SubdomainPlugin) Register(r plugins.Registry) error {
	r.Add(InputTraceType, g)
	return nil
}

type CrtShEntry struct {
	NameValue string `json:"name_value"`
}

func (g *SubdomainPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	url := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", trace.Value)
	resp, err := g.fetcher.Get(ctx, url)
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
		if subdomain != "" && !strings.Contains(subdomain, "*") {
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
