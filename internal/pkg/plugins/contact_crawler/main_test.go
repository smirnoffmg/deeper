package contact_crawler

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePageFetcher struct {
	mu        sync.Mutex
	pages     map[string]string
	errors    map[string]error
	fetchLog  []string
	seedError error
}

func newFakePageFetcher() *fakePageFetcher {
	return &fakePageFetcher{
		pages:  make(map[string]string),
		errors: make(map[string]error),
	}
}

func (f *fakePageFetcher) setPage(url, html string) {
	f.pages[url] = html
}

func (f *fakePageFetcher) setError(url string, err error) {
	f.errors[url] = err
}

func (f *fakePageFetcher) setSeedError(err error) {
	f.seedError = err
}

func (f *fakePageFetcher) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	f.mu.Lock()
	f.fetchLog = append(f.fetchLog, rawURL)
	isFirst := len(f.fetchLog) == 1
	f.mu.Unlock()

	if f.seedError != nil && isFirst {
		return nil, f.seedError
	}
	if err, ok := f.errors[rawURL]; ok {
		return nil, err
	}

	html, ok := f.pages[rawURL]
	if !ok {
		return nil, &net.DNSError{Err: "not found", Name: rawURL}
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(html)),
		Header:     make(http.Header),
	}, nil
}

func (f *fakePageFetcher) fetchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.fetchLog)
}

func testResolver() *fakeIPResolver {
	ip := net.ParseIP("185.55.56.154")
	otherIP := net.ParseIP("10.0.0.1")
	return &fakeIPResolver{ips: map[string][]net.IP{
		"codescoring.ru":          {ip},
		"www.codescoring.ru":      {ip},
		"registry.codescoring.ru": {ip},
		"page":                    {ip},
		"evil.com":                {otherIP},
	}}
}

func testPlugin(fetcher pageFetcher, resolver ipResolver) *ContactCrawlerPlugin {
	return &ContactCrawlerPlugin{
		fetcher:      fetcher,
		resolver:     resolver,
		domainBudget: newDomainBudget(maxPagesPerRegistrableDomainPerProcess),
	}
}

func TestContactCrawlerPlugin_FollowTrace_WrongType(t *testing.T) {
	plugin := testPlugin(newFakePageFetcher(), testResolver())

	traces, err := plugin.FollowTrace(entities.Trace{Value: "x", Type: entities.Email})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestContactCrawlerPlugin_FollowTrace_DedupesEmail(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setPage("https://codescoring.ru/", `<html><body>
		<p>info@codescoring.ru</p>
		<a href="mailto:info@codescoring.ru">mail</a>
	</body></html>`)

	plugin := testPlugin(fetcher, testResolver())
	traces, err := plugin.FollowTrace(entities.Trace{Value: "codescoring.ru", Type: entities.Domain})

	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Email, traces[0].Type)
	assert.Equal(t, "info@codescoring.ru", traces[0].Value)
}

func TestContactCrawlerPlugin_FollowTrace_SeedFetchError(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setSeedError(errors.New("connection refused"))

	plugin := testPlugin(fetcher, testResolver())
	traces, err := plugin.FollowTrace(entities.Trace{Value: "codescoring.ru", Type: entities.Domain})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestContactCrawlerPlugin_FollowTrace_MidCrawlErrorContinues(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setPage("https://codescoring.ru/", `<html><body>
		<p>info@codescoring.ru</p>
		<a href="/about">about</a>
		<a href="/contact">contact</a>
	</body></html>`)
	fetcher.setError("https://codescoring.ru/about", errors.New("timeout"))
	fetcher.setPage("https://codescoring.ru/contact", `<html><body><a href="tel:+1-555-123-4567">call</a></body></html>`)

	plugin := testPlugin(fetcher, testResolver())
	traces, err := plugin.FollowTrace(entities.Trace{Value: "codescoring.ru", Type: entities.Domain})

	require.NoError(t, err)
	require.NotEmpty(t, traces)

	phoneFound := false
	for _, trace := range traces {
		if trace.Type == entities.Phone {
			phoneFound = true
		}
	}
	assert.True(t, phoneFound)
	assert.Equal(t, 3, fetcher.fetchCount())
}

func TestContactCrawlerPlugin_FollowTrace_ExternalHostNotFetched(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setPage("https://codescoring.ru/", `<html><body>
		<a href="https://evil.com/phish">evil</a>
	</body></html>`)
	fetcher.setPage("https://evil.com/phish", `<html><body>should not fetch</body></html>`)

	plugin := testPlugin(fetcher, testResolver())
	_, err := plugin.FollowTrace(entities.Trace{Value: "codescoring.ru", Type: entities.Domain})

	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.fetchCount())
}

func TestContactCrawlerPlugin_FollowTrace_MalformedHTML(t *testing.T) {
	fetcher := newFakePageFetcher()
	fetcher.setPage("https://codescoring.ru/", `<html><body><p>reach us at help@codescoring.ru<div`)

	plugin := testPlugin(fetcher, testResolver())
	traces, err := plugin.FollowTrace(entities.Trace{Value: "codescoring.ru", Type: entities.Domain})

	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestContactCrawlerPlugin_String(t *testing.T) {
	plugin := NewPlugin()
	assert.Equal(t, "ContactCrawlerPlugin", plugin.String())
}

func TestContactCrawlerPlugin_SharedDomainBudget(t *testing.T) {
	budget := newDomainBudget(3)
	fetcher := newFakePageFetcher()
	resolver := testResolver()

	pageHTML := `<html><body><p>user@codescoring.ru</p><a href="/next">next</a></body></html>`
	fetcher.setPage("https://registry.codescoring.ru/", pageHTML)
	fetcher.setPage("https://registry.codescoring.ru/next", pageHTML)
	fetcher.setPage("https://www.codescoring.ru/", pageHTML)
	fetcher.setPage("https://www.codescoring.ru/next", pageHTML)

	plugin := &ContactCrawlerPlugin{
		fetcher:      fetcher,
		resolver:     resolver,
		domainBudget: budget,
	}

	_, err := plugin.FollowTrace(entities.Trace{Value: "registry.codescoring.ru", Type: entities.Subdomain})
	require.NoError(t, err)
	firstCount := fetcher.fetchCount()
	require.Equal(t, 2, firstCount)

	_, err = plugin.FollowTrace(entities.Trace{Value: "www.codescoring.ru", Type: entities.Subdomain})
	require.NoError(t, err)

	assert.Equal(t, 3, fetcher.fetchCount())
}
