package subdomains

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

const InputTraceType = entities.Domain

type hostSearchFetcher interface {
	Get(url string) (*http.Response, error)
}

type httpHostSearchFetcher struct{}

func (httpHostSearchFetcher) Get(url string) (*http.Response, error) {
	return http.Get(url)
}

type SubdomainPlugin struct {
	fetcher hostSearchFetcher
}

func init() {
	plugin := NewPlugin()
	if err := plugin.Register(); err != nil {
		panic(err)
	}
}

func NewPlugin() *SubdomainPlugin {
	return &SubdomainPlugin{fetcher: httpHostSearchFetcher{}}
}

func (p *SubdomainPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, p)
	return nil
}

func (p *SubdomainPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	url := fmt.Sprintf("https://api.hackertarget.com/hostsearch/?q=%s", trace.Value)
	resp, err := p.fetcher.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseHostSearchCSV(string(body)), nil
}

func (p *SubdomainPlugin) String() string {
	return "SubdomainPlugin"
}

// parseHostSearchCSV parses hackertarget's "subdomain,ip" CSV response.
// Both fields are trimmed and validated before becoming traces -- a blank
// IP field (hackertarget emits one for an unresolvable host) or a
// non-IP-shaped value must not become a fake IpAddr trace.
func parseHostSearchCSV(csv string) []entities.Trace {
	var traces []entities.Trace
	for _, line := range strings.Split(csv, "\n") {
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			continue
		}
		subdomain := strings.TrimSpace(parts[0])
		ipAddr := strings.TrimSpace(parts[1])
		if subdomain == "" || !entities.IsIpAddr(ipAddr) {
			continue
		}
		traces = append(traces, entities.Trace{Value: subdomain, Type: entities.Subdomain})
		traces = append(traces, entities.Trace{Value: ipAddr, Type: entities.IpAddr})
	}
	return traces
}
