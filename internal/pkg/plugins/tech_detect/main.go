package tech_detect

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type pageFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type TechDetectPlugin struct {
	fetcher pageFetcher
}

func NewPlugin(cfg *config.Config) *TechDetectPlugin {
	return &TechDetectPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *TechDetectPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Domain, p)
	r.Add(entities.Url, p)
	return nil
}

func (p *TechDetectPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	target, ok := targetURL(trace)
	if !ok {
		return nil, nil
	}

	resp, err := p.fetcher.Get(ctx, target)
	if err != nil {
		log.Warn().Err(err).Str("url", target).Msg("tech detect fetch failed, skipping")
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		log.Warn().Err(err).Str("url", target).Msg("tech detect body read failed, skipping")
		return nil, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		log.Warn().Int("status", resp.StatusCode).Str("url", target).Msg("tech detect bad status, skipping")
		return nil, nil
	}

	return detect(resp.Header, string(body)), nil
}

func (p *TechDetectPlugin) String() string {
	return "TechDetectPlugin"
}

func targetURL(trace entities.Trace) (string, bool) {
	switch trace.Type {
	case entities.Domain:
		return "https://" + trace.Value, true
	case entities.Url:
		return trace.Value, true
	default:
		return "", false
	}
}

func detect(header http.Header, body string) []entities.Trace {
	var traces []entities.Trace
	seen := make(map[string]bool)

	for _, fp := range fingerprints {
		if seen[fp.name] {
			continue
		}
		if fp.match(header, body) {
			seen[fp.name] = true
			traces = append(traces, entities.Trace{Value: fp.name, Type: entities.Technology})
		}
	}

	return traces
}

type fingerprint struct {
	name  string
	match func(header http.Header, body string) bool
}

var fingerprints = []fingerprint{
	{"Nginx", headerContains("Server", "nginx")},
	{"Apache", headerContains("Server", "apache")},
	{"Microsoft-IIS", headerContains("Server", "microsoft-iis")},
	{"Cloudflare", headerPresent("Cf-Ray")},
	{"PHP", headerContains("X-Powered-By", "php")},
	{"Express", headerContains("X-Powered-By", "express")},
	{"ASP.NET", anyMatch(headerContains("X-Powered-By", "asp.net"), headerPresent("X-Aspnet-Version"))},
	{"Laravel", headerContains("Set-Cookie", "laravel_session")},
	{"Django", headerContains("Set-Cookie", "csrftoken")},
	{"Shopify", anyMatch(headerContains("Set-Cookie", "_shopify"), headerPresent("X-Shopid"))},
	{"WordPress", anyMatch(generatorMatches("wordpress"), bodyContains("wp-content"))},
	{"Drupal", anyMatch(generatorMatches("drupal"), headerContains("X-Generator", "drupal"))},
	{"Joomla", generatorMatches("joomla")},
	{"jQuery", bodyContains("jquery")},
	{"React", anyMatch(bodyContains("react-dom"), bodyContains("data-reactroot"))},
	{"Vue.js", anyMatch(bodyContains("vue.js"), bodyContains("data-v-app"))},
}

func headerContains(name, substr string) func(http.Header, string) bool {
	sub := strings.ToLower(substr)
	return func(h http.Header, _ string) bool {
		for _, v := range h.Values(name) {
			if strings.Contains(strings.ToLower(v), sub) {
				return true
			}
		}
		return false
	}
}

func headerPresent(name string) func(http.Header, string) bool {
	return func(h http.Header, _ string) bool {
		return len(h.Values(name)) > 0
	}
}

func bodyContains(substr string) func(http.Header, string) bool {
	sub := strings.ToLower(substr)
	return func(_ http.Header, body string) bool {
		return strings.Contains(strings.ToLower(body), sub)
	}
}

func generatorMatches(name string) func(http.Header, string) bool {
	re := regexp.MustCompile(`(?i)<meta[^>]+name=["']generator["'][^>]+content=["'][^"']*` + regexp.QuoteMeta(name))
	return func(_ http.Header, body string) bool {
		return re.MatchString(body)
	}
}

func anyMatch(matchers ...func(http.Header, string) bool) func(http.Header, string) bool {
	return func(h http.Header, body string) bool {
		for _, m := range matchers {
			if m(h, body) {
				return true
			}
		}
		return false
	}
}
