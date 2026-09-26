package js_crawler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

// maxJSFiles caps how many linked scripts a single page fetch will pull. A
// page can reference dozens of bundles, and each is another network round
// trip; without a cap one Url trace could fan out into a runaway crawl.
const maxJSFiles = 10

var (
	// Unanchored so it can pick absolute links out of arbitrary JS source.
	// Stops at whitespace, quotes, brackets and backslashes; trailing
	// punctuation left by the match is trimmed separately.
	urlPattern   = regexp.MustCompile(`https?://[^\s"'` + "`" + `<>{}()\[\]\\]+`)
	emailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
)

type resourceFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type JSCrawlerPlugin struct {
	fetcher resourceFetcher
}

func NewPlugin(cfg *config.Config) *JSCrawlerPlugin {
	return &JSCrawlerPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *JSCrawlerPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Url, p)
	return nil
}

func (p *JSCrawlerPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Url {
		return nil, nil
	}

	pageBody, err := p.fetch(ctx, trace.Value)
	if err != nil {
		log.Warn().Err(err).Str("url", trace.Value).Msg("js_crawler: page unavailable, skipping")
		return nil, nil
	}

	scripts := scriptURLs(pageBody, trace.Value)
	if len(scripts) > maxJSFiles {
		scripts = scripts[:maxJSFiles]
	}

	collected := make([]entities.Trace, 0)
	seen := make(map[string]struct{})
	add := func(value string, traceType entities.TraceType) {
		if value == "" {
			return
		}
		key := string(traceType) + "\x00" + value
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		collected = append(collected, entities.Trace{Value: value, Type: traceType})
	}

	for _, jsURL := range scripts {
		body, err := p.fetch(ctx, jsURL)
		if err != nil {
			log.Warn().Err(err).Str("url", jsURL).Msg("js_crawler: script fetch failed, continuing")
			continue
		}

		text := string(body)
		for _, u := range extractURLs(text) {
			add(u, entities.Url)
		}
		for _, e := range emailPattern.FindAllString(text, -1) {
			if entities.IsRealEmail(e) {
				add(e, entities.Email)
			}
		}
	}

	return collected, nil
}

func (p *JSCrawlerPlugin) String() string {
	return "JSCrawlerPlugin"
}

// fetch performs a GET and returns the body, treating a non-2xx status as an
// error so a 404 script is skipped like a transport failure.
func (p *JSCrawlerPlugin) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := p.fetcher.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &statusError{code: resp.StatusCode}
	}
	return body, nil
}

type statusError struct{ code int }

func (e *statusError) Error() string {
	return "unexpected status " + http.StatusText(e.code)
}

func scriptURLs(pageBody []byte, pageURL string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(pageBody)))
	if err != nil {
		return nil
	}

	base, _ := url.Parse(pageURL)
	out := make([]string, 0)
	seen := make(map[string]struct{})

	doc.Find("script[src]").Each(func(_ int, sel *goquery.Selection) {
		src, ok := sel.Attr("src")
		if !ok {
			return
		}
		src = strings.TrimSpace(src)
		if src == "" {
			return
		}

		resolved := resolveScript(base, src)
		if resolved == "" {
			return
		}
		if _, dup := seen[resolved]; dup {
			return
		}
		seen[resolved] = struct{}{}
		out = append(out, resolved)
	})

	return out
}

func resolveScript(base *url.URL, src string) string {
	ref, err := url.Parse(src)
	if err != nil {
		return ""
	}

	var abs *url.URL
	if base != nil {
		abs = base.ResolveReference(ref)
	} else {
		abs = ref
	}
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return ""
	}
	return abs.String()
}

func extractURLs(text string) []string {
	matches := urlPattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		m = strings.TrimRight(m, ".,;:!?\"'")
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}
