package tech_detect

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePageFetcher struct {
	resp    *http.Response
	err     error
	gotURL  string
	gotCtx  context.Context
	callCnt int
}

func (f *fakePageFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	f.callCnt++
	f.gotURL = url
	f.gotCtx = ctx
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func response(status int, header http.Header, body string) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func techValues(traces []entities.Trace) []string {
	values := make([]string, 0, len(traces))
	for _, tr := range traces {
		values = append(values, tr.Value)
	}
	return values
}

func TestFollowTrace_DetectsFromHeader(t *testing.T) {
	header := http.Header{}
	header.Set("Server", "nginx/1.25.1")
	header.Set("X-Powered-By", "PHP/8.2.0")
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, header, "<html></html>")}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	for _, tr := range traces {
		assert.Equal(t, entities.Technology, tr.Type)
	}
	assert.Contains(t, techValues(traces), "Nginx")
	assert.Contains(t, techValues(traces), "PHP")
}

func TestFollowTrace_DetectsFromMetaGenerator(t *testing.T) {
	body := `<html><head><meta name="generator" content="WordPress 6.5.2"></head><body></body></html>`
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, http.Header{}, body)}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://blog.example.com"})
	require.NoError(t, err)
	assert.Contains(t, techValues(traces), "WordPress")
}

func TestFollowTrace_Deduplicates(t *testing.T) {
	// WordPress matches both the meta generator and the wp-content path.
	body := `<html><head><meta name="generator" content="WordPress 6.5.2"></head>` +
		`<body><img src="/wp-content/uploads/logo.png"></body></html>`
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, http.Header{}, body)}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)

	count := 0
	for _, v := range techValues(traces) {
		if v == "WordPress" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestFollowTrace_NoMatches(t *testing.T) {
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, http.Header{}, "<html><body>hi</body></html>")}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFollowTrace_HTTPErrorReturnsNil(t *testing.T) {
	fetcher := &fakePageFetcher{err: errors.New("dial tcp: connection refused")}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_BadStatusReturnsNil(t *testing.T) {
	header := http.Header{}
	header.Set("Server", "nginx")
	fetcher := &fakePageFetcher{resp: response(http.StatusInternalServerError, header, "boom")}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_WrongType(t *testing.T) {
	fetcher := &fakePageFetcher{}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Email, Value: "a@example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
	assert.Equal(t, 0, fetcher.callCnt)
}

func TestFollowTrace_DomainBuildsHTTPSURL(t *testing.T) {
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, http.Header{}, "")}
	p := &TechDetectPlugin{fetcher: fetcher}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", fetcher.gotURL)
}

func TestFollowTrace_URLUsedAsIs(t *testing.T) {
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, http.Header{}, "")}
	p := &TechDetectPlugin{fetcher: fetcher}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "http://example.com/page"})
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/page", fetcher.gotURL)
}

func TestFollowTrace_ContextFlowsToFetcher(t *testing.T) {
	type ctxKey string
	key := ctxKey("k")
	ctx := context.WithValue(context.Background(), key, "v")
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, http.Header{}, "")}
	p := &TechDetectPlugin{fetcher: fetcher}

	_, err := p.FollowTrace(ctx, entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	require.NotNil(t, fetcher.gotCtx)
	assert.Equal(t, "v", fetcher.gotCtx.Value(key))
}

func TestFollowTrace_DetectsFromCookie(t *testing.T) {
	header := http.Header{}
	header.Add("Set-Cookie", "laravel_session=abc; path=/; HttpOnly")
	fetcher := &fakePageFetcher{resp: response(http.StatusOK, header, "")}
	p := &TechDetectPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Contains(t, techValues(traces), "Laravel")
}

func TestRegister_RegistersUnderDomainAndURL(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	assert.Contains(t, registry[entities.Domain], plugins.DeeperPlugin(p))
	assert.Contains(t, registry[entities.Url], plugins.DeeperPlugin(p))
}

func TestString(t *testing.T) {
	assert.Equal(t, "TechDetectPlugin", (&TechDetectPlugin{}).String())
}
