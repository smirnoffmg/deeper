package crtsh

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

const InputTraceType = entities.Domain

func init() {
	p := NewPlugin()
	p.Register()
}

type SubdomainPlugin struct{}

func NewPlugin() *SubdomainPlugin {
	return &SubdomainPlugin{}
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
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var entries []CrtShEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
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
