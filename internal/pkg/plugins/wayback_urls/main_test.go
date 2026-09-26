package wayback_urls

import (
	"context"
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

type fakeURLFetcher struct {
	status int
	body   string
	err    error
	gotCtx context.Context
	gotURL string
}

func (f *fakeURLFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	f.gotCtx = ctx
	f.gotURL = url
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

func domainTrace() entities.Trace {
	return entities.Trace{Type: entities.Domain, Value: "example.com"}
}

func TestFollowTrace_WrongType(t *testing.T) {
	fetcher := &fakeURLFetcher{}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "alice"})
	require.NoError(t, err)
	assert.Nil(t, traces)
	assert.Empty(t, fetcher.gotURL, "must not perform I/O for a non-Domain trace")
}

func TestFollowTrace_SkipsHeaderAndParsesURLs(t *testing.T) {
	fetcher := &fakeURLFetcher{
		status: http.StatusOK,
		body:   `[["original"],["http://example.com/"],["http://example.com/about"]]`,
	}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	require.Len(t, traces, 2)
	for _, tr := range traces {
		assert.Equal(t, entities.Url, tr.Type)
	}
	assert.Equal(t, "http://example.com/", traces[0].Value)
	assert.Equal(t, "http://example.com/about", traces[1].Value)
}

func TestFollowTrace_EmptyList(t *testing.T) {
	fetcher := &fakeURLFetcher{status: http.StatusOK, body: `[["original"]]`}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFollowTrace_Deduplicates(t *testing.T) {
	fetcher := &fakeURLFetcher{
		status: http.StatusOK,
		body:   `[["original"],["http://example.com/"],["http://example.com/"],["http://example.com/x"]]`,
	}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	require.Len(t, traces, 2)
	assert.Equal(t, "http://example.com/", traces[0].Value)
	assert.Equal(t, "http://example.com/x", traces[1].Value)
}

func TestFollowTrace_SkipsEmptyStrings(t *testing.T) {
	fetcher := &fakeURLFetcher{
		status: http.StatusOK,
		body:   `[["original"],[""],["http://example.com/y"],[]]`,
	}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, "http://example.com/y", traces[0].Value)
}

func TestFollowTrace_HTTPError_NoTracesNoError(t *testing.T) {
	fetcher := &fakeURLFetcher{status: http.StatusServiceUnavailable, body: "upstream down"}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_TransportError_NoTracesNoError(t *testing.T) {
	fetcher := &fakeURLFetcher{err: assert.AnError}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_MalformedJSON_NoTracesNoError(t *testing.T) {
	fetcher := &fakeURLFetcher{status: http.StatusOK, body: `not json at all`}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_EmptyBody_NoTracesNoError(t *testing.T) {
	fetcher := &fakeURLFetcher{status: http.StatusOK, body: ""}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_CapsResults(t *testing.T) {
	var b strings.Builder
	b.WriteString(`[["original"]`)
	for i := 0; i < maxURLs+50; i++ {
		b.WriteString(`,["http://example.com/`)
		b.WriteString(strings.Repeat("a", 1))
		b.WriteString(itoa(i))
		b.WriteString(`"]`)
	}
	b.WriteString(`]`)

	fetcher := &fakeURLFetcher{status: http.StatusOK, body: b.String()}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), domainTrace())
	require.NoError(t, err)
	assert.Len(t, traces, maxURLs)
}

func TestFollowTrace_PassesContextAndBuildsURL(t *testing.T) {
	fetcher := &fakeURLFetcher{status: http.StatusOK, body: `[["original"]]`}
	p := &WaybackURLsPlugin{fetcher: fetcher}

	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("k"), "v")
	_, err := p.FollowTrace(ctx, domainTrace())
	require.NoError(t, err)

	require.NotNil(t, fetcher.gotCtx)
	assert.Equal(t, "v", fetcher.gotCtx.Value(ctxKey("k")))
	assert.Contains(t, fetcher.gotURL, "example.com")
	assert.Contains(t, fetcher.gotURL, "output=json")
}

func TestRegister_RegistersUnderDomain(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Domain] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "WaybackURLsPlugin", (&WaybackURLsPlugin{}).String())
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
