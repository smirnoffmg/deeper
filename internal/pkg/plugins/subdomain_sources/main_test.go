package subdomain_sources

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

type fakeFetcher struct {
	body    string
	status  int
	err     error
	lastURL string
	gotCtx  context.Context
}

func (f *fakeFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	f.lastURL = url
	f.gotCtx = ctx
	if f.err != nil {
		return nil, f.err
	}
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

func values(traces []entities.Trace) []string {
	out := make([]string, 0, len(traces))
	for _, t := range traces {
		out = append(out, t.Value)
	}
	return out
}

func TestFollowTrace_ParsesMultipleSubdomains(t *testing.T) {
	fetcher := &fakeFetcher{body: `[
		{"dns_names":["a.example.com","b.example.com"]},
		{"dns_names":["c.example.com"]}
	]`}
	p := &SubdomainSourcesPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 3)
	assert.ElementsMatch(t, []string{"a.example.com", "b.example.com", "c.example.com"}, values(traces))
	for _, tr := range traces {
		assert.Equal(t, entities.Subdomain, tr.Type)
	}
}

func TestFollowTrace_Deduplicates(t *testing.T) {
	fetcher := &fakeFetcher{body: `[
		{"dns_names":["a.example.com","A.EXAMPLE.COM"]},
		{"dns_names":["a.example.com"]}
	]`}
	p := &SubdomainSourcesPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, "a.example.com", traces[0].Value)
}

func TestFollowTrace_SkipsApexWildcardEmpty(t *testing.T) {
	fetcher := &fakeFetcher{body: `[
		{"dns_names":["example.com","EXAMPLE.COM","*.example.com","",""]},
		{"dns_names":["good.example.com"]}
	]`}
	p := &SubdomainSourcesPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, "good.example.com", traces[0].Value)
}

func TestFollowTrace_HTTPErrorReturnsNil(t *testing.T) {
	fetcher := &fakeFetcher{err: errors.New("boom")}
	p := &SubdomainSourcesPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_MalformedJSONReturnsNil(t *testing.T) {
	fetcher := &fakeFetcher{body: `{not json`}
	p := &SubdomainSourcesPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_WrongTypeReturnsNil(t *testing.T) {
	fetcher := &fakeFetcher{body: `[{"dns_names":["a.example.com"]}]`}
	p := &SubdomainSourcesPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
	assert.Empty(t, fetcher.lastURL)
}

func TestFollowTrace_PassesContextAndDomainToFetcher(t *testing.T) {
	fetcher := &fakeFetcher{body: `[]`}
	p := &SubdomainSourcesPlugin{fetcher: fetcher}

	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("k"), "v")
	_, err := p.FollowTrace(ctx, entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Equal(t, "v", fetcher.gotCtx.Value(ctxKey("k")))
	assert.Contains(t, fetcher.lastURL, "domain=example.com")
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
	assert.Equal(t, "SubdomainSourcesPlugin", (&SubdomainSourcesPlugin{}).String())
}
