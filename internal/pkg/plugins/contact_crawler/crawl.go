package contact_crawler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

const (
	defaultMaxDepth  = 2
	defaultMaxPages  = 20
	maxPagesPerCall  = defaultMaxPages
	maxDepthPerCall  = defaultMaxDepth
)

type pageFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type crawlItem struct {
	url   string
	depth int
}

type domainBudget struct {
	mu      sync.Mutex
	limits  map[string]int
	maximum int
}

func newDomainBudget(maximum int) *domainBudget {
	return &domainBudget{
		limits:  make(map[string]int),
		maximum: maximum,
	}
}

func (b *domainBudget) trySpend(host string) bool {
	domain, ok := registrableDomain(host)
	if !ok {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	remaining, exists := b.limits[domain]
	if !exists {
		remaining = b.maximum
	}
	if remaining <= 0 {
		b.limits[domain] = 0
		return false
	}

	b.limits[domain] = remaining - 1
	return true
}

type crawler struct {
	fetcher      pageFetcher
	domainBudget *domainBudget
	seedHost     string
	maxDepth     int
	maxPages     int
}

func newCrawler(fetcher pageFetcher, seedHost string, domainBudget *domainBudget) *crawler {
	return &crawler{
		fetcher:      fetcher,
		domainBudget: domainBudget,
		seedHost:     seedHost,
		maxDepth:     maxDepthPerCall,
		maxPages:     maxPagesPerCall,
	}
}

func (c *crawler) crawl(ctx context.Context, seedURL string) ([]entities.Trace, error) {
	normalizedSeed := stripFragment(normalizeURL(seedURL))
	queue := []crawlItem{{url: normalizedSeed, depth: 0}}
	visited := make(map[string]bool)
	collected := make([]entities.Trace, 0)
	seenTraces := make(map[string]struct{})
	pagesFetched := 0

	for len(queue) > 0 && pagesFetched < c.maxPages {
		item := queue[0]
		queue = queue[1:]

		item.url = stripFragment(item.url)
		if visited[item.url] {
			continue
		}
		visited[item.url] = true

		parsed, err := url.Parse(item.url)
		if err != nil || parsed.Host == "" {
			continue
		}

		host := parsed.Hostname()
		if !sameSite(c.seedHost, host) {
			continue
		}
		if !c.domainBudget.trySpend(host) {
			continue
		}

		body, finalURL, err := c.fetchURL(ctx, item.url, item.depth == 0 && pagesFetched == 0)
		if err != nil {
			if item.depth == 0 && pagesFetched == 0 {
				log.Warn().Err(err).Str("host", c.seedHost).Msg("seed page unavailable, skipping crawl")
				return collected, nil
			}
			log.Debug().Err(err).Str("url", item.url).Msg("skipping page after fetch error")
			continue
		}

		pagesFetched++
		pageTraces, links := parsePage(body, finalURL)
		for _, trace := range pageTraces {
			key := string(trace.Type) + "\x00" + trace.Value
			if _, ok := seenTraces[key]; ok {
				continue
			}
			seenTraces[key] = struct{}{}
			collected = append(collected, trace)
		}

		if item.depth >= c.maxDepth {
			continue
		}

		for _, link := range links {
			link = stripFragment(link)
			if visited[link] {
				continue
			}

			linkParsed, err := url.Parse(link)
			if err != nil || linkParsed.Host == "" {
				continue
			}
			if !sameSite(c.seedHost, linkParsed.Hostname()) {
				continue
			}

			queue = append(queue, crawlItem{url: link, depth: item.depth + 1})
		}
	}

	return collected, nil
}

func (c *crawler) fetchURL(ctx context.Context, pageURL string, tryHTTPFallback bool) ([]byte, string, error) {
	urlsToTry := []string{pageURL}
	if tryHTTPFallback && strings.HasPrefix(pageURL, "https://") {
		urlsToTry = append(urlsToTry, "http://"+strings.TrimPrefix(pageURL, "https://"))
	}

	var lastErr error
	for _, u := range urlsToTry {
		resp, err := c.fetcher.Get(ctx, u)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		return body, u, nil
	}

	return nil, pageURL, lastErr
}

func parsePage(body []byte, baseURL string) ([]entities.Trace, []string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, nil
	}

	doc.Find("script, style, noscript").Remove()

	traces := make([]entities.Trace, 0)
	links := make([]string, 0)

	text := doc.Text()
	for _, email := range extractEmails(text) {
		traces = append(traces, entities.Trace{Value: email, Type: entities.Email})
	}
	for _, phone := range extractPhones(text) {
		traces = append(traces, entities.Trace{Value: phone, Type: entities.Phone})
	}

	base, _ := url.Parse(baseURL)
	doc.Find("a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, ok := sel.Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" {
			return
		}

		if trace, ok := mailtoTrace(href); ok {
			traces = append(traces, trace)
			return
		}
		if trace, ok := telTrace(href); ok {
			traces = append(traces, trace)
			return
		}
		if trace, ok := socialLinkTrace(href); ok {
			traces = append(traces, trace)
			return
		}

		absolute, ok := resolveLink(base, href)
		if !ok {
			return
		}
		links = append(links, absolute)
	})

	return traces, links
}

func resolveLink(base *url.URL, href string) (string, bool) {
	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") {
		return "", false
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return "", false
	}

	absolute := base.ResolveReference(parsed)
	if absolute.Scheme != "http" && absolute.Scheme != "https" {
		return "", false
	}

	return absolute.String(), true
}

func normalizeURL(host string) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	return "https://" + host + "/"
}

func stripFragment(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Fragment = ""
	return parsed.String()
}
