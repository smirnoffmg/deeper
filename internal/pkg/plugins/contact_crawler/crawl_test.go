package contact_crawler

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrawler_FollowsLinksWithinBudget(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setPage("https://codescoring.ru/", `<html><body>
		<a href="/about">about</a>
	</body></html>`)
	fetcher.setPage("https://codescoring.ru/about", `<html><body><p>about@codescoring.ru</p></body></html>`)

	c := newCrawler(fetcher, testResolver(), "codescoring.ru", newDomainBudget(60))
	traces, err := c.crawl(t.Context(), "https://codescoring.ru/")

	require.NoError(t, err)
	require.NotEmpty(t, traces)
	assert.Equal(t, 2, fetcher.fetchCount())
}

func TestCrawler_StopsAtMaxPages(t *testing.T) {
	fetcher := newFakePageFetcher()
	var links strings.Builder
	links.WriteString(`<html><body>`)
	for i := 0; i < 30; i++ {
		path := fmt.Sprintf("/page%d", i)
		url := "https://codescoring.ru" + path
		fetcher.setPage(url, fmt.Sprintf(`<html><body><a href="/page%d">next</a></body></html>`, i+1))
		fmt.Fprintf(&links, `<a href="%s">p</a>`, path)
	}
	links.WriteString(`</body></html>`)
	fetcher.setPage("https://codescoring.ru/", links.String())

	c := newCrawler(fetcher, testResolver(), "codescoring.ru", newDomainBudget(60))
	_, err := c.crawl(t.Context(), "https://codescoring.ru/")

	require.NoError(t, err)
	assert.Equal(t, defaultMaxPages, fetcher.fetchCount())
}

func TestCrawler_DoesNotExceedMaxDepth(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setPage("https://codescoring.ru/", `<html><body><a href="/d1">d1</a></body></html>`)
	fetcher.setPage("https://codescoring.ru/d1", `<html><body><a href="/d2">d2</a></body></html>`)
	fetcher.setPage("https://codescoring.ru/d2", `<html><body><a href="/d3">d3</a></body></html>`)
	fetcher.setPage("https://codescoring.ru/d3", `<html><body>deep</body></html>`)

	c := newCrawler(fetcher, testResolver(), "codescoring.ru", newDomainBudget(60))
	_, err := c.crawl(t.Context(), "https://codescoring.ru/")

	require.NoError(t, err)
	assert.Equal(t, 3, fetcher.fetchCount())
	for _, url := range fetcher.fetchedURLs() {
		assert.NotContains(t, url, "/d3")
	}
}

func TestCrawler_SkipsDifferentIPSubdomain(t *testing.T) {
	ip := testResolver().ips["codescoring.ru"][0]
	otherIP := testResolver().ips["evil.com"][0]
	resolver := &fakeIPResolver{ips: map[string][]net.IP{
		"codescoring.ru":       {ip},
		"other.codescoring.ru": {otherIP},
	}}

	fetcher := newFakePageFetcher()
	fetcher.setPage("https://codescoring.ru/", `<html><body>
		<a href="https://other.codescoring.ru/secret">other</a>
	</body></html>`)
	fetcher.setPage("https://other.codescoring.ru/secret", `<html><body>secret</body></html>`)

	c := newCrawler(fetcher, resolver, "codescoring.ru", newDomainBudget(60))
	_, err := c.crawl(t.Context(), "https://codescoring.ru/")

	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.fetchCount())
}

func TestCrawler_SeedHTTPSFallbackToHTTP(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.errors["https://codescoring.ru/"] = errors.New("tls: certificate error")
	fetcher.setPage("http://codescoring.ru/", `<html><body><p>help@codescoring.ru</p></body></html>`)

	c := newCrawler(fetcher, testResolver(), "codescoring.ru", newDomainBudget(60))
	traces, err := c.crawl(t.Context(), "https://codescoring.ru/")

	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, "help@codescoring.ru", traces[0].Value)
}

func TestCrawler_SeedUnavailableReturnsNoError(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setSeedError(errors.New("connection refused"))

	c := newCrawler(fetcher, testResolver(), "codescoring.ru", newDomainBudget(60))
	traces, err := c.crawl(t.Context(), "https://codescoring.ru/")

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func (f *fakePageFetcher) fetchedURLs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.fetchLog))
	copy(out, f.fetchLog)
	return out
}

func TestParsePage_ExtractsContactsAndLinks(t *testing.T) {
	body := []byte(`<html><body>
		<p>hello@example.com</p>
		<a href="mailto:hello@example.com">mail</a>
		<a href="tel:+15551234567">call</a>
		<a href="https://github.com/acme">gh</a>
		<a href="/internal">in</a>
	</body></html>`)

	traces, links := parsePage(body, "https://example.com/")
	require.NotEmpty(t, traces)
	require.NotEmpty(t, links)

	var types []entities.TraceType
	for _, trace := range traces {
		types = append(types, trace.Type)
	}
	assert.Contains(t, types, entities.Email)
	assert.Contains(t, types, entities.Phone)
	assert.Contains(t, types, entities.Github)
	assert.Contains(t, links[0], "https://example.com/internal")
}
