package live_host

import (
	"context"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type hostProber interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type LiveHostPlugin struct {
	prober hostProber
}

func NewPlugin(cfg *config.Config) *LiveHostPlugin {
	return &LiveHostPlugin{prober: deeperhttp.NewClient(cfg)}
}

func (p *LiveHostPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Subdomain, p)
	r.Add(entities.Domain, p)
	return nil
}

// FollowTrace probes a discovered hostname over HTTPS then HTTP and, on any
// response below 500, confirms it as a live Host. An unreachable host is not a
// failure of the scan, so it returns (nil, nil) rather than an error.
func (p *LiveHostPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Subdomain && trace.Type != entities.Domain {
		return nil, nil
	}

	resp := p.probe(ctx, trace.Value)
	if resp == nil {
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	traces := []entities.Trace{{Value: trace.Value, Type: entities.Host}}

	if title := pageTitle(resp); title != "" {
		traces = append(traces, entities.Trace{Value: title, Type: entities.Name})
	}

	return traces, nil
}

func (p *LiveHostPlugin) probe(ctx context.Context, host string) *http.Response {
	for _, scheme := range []string{"https://", "http://"} {
		url := scheme + host
		resp, err := p.prober.Get(ctx, url)
		if err != nil {
			log.Debug().Err(err).Str("url", url).Msg("live host probe failed")
			continue
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			_ = resp.Body.Close()
			continue
		}
		return resp
	}
	return nil
}

func pageTitle(resp *http.Response) string {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Debug().Err(err).Msg("failed to parse live host response body")
		return ""
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}

func (p *LiveHostPlugin) String() string {
	return "LiveHostPlugin"
}
