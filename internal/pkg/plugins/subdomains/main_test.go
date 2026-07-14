package subdomains

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHostSearchCSV_ValidRows(t *testing.T) {
	csv := "sub1.example.com,192.168.1.1\nsub2.example.com,192.168.1.2"

	traces := parseHostSearchCSV(csv)

	require.Len(t, traces, 4)
	assert.Equal(t, entities.Trace{Value: "sub1.example.com", Type: entities.Subdomain}, traces[0])
	assert.Equal(t, entities.Trace{Value: "192.168.1.1", Type: entities.IpAddr}, traces[1])
	assert.Equal(t, entities.Trace{Value: "sub2.example.com", Type: entities.Subdomain}, traces[2])
	assert.Equal(t, entities.Trace{Value: "192.168.1.2", Type: entities.IpAddr}, traces[3])
}

// Regression: found live -- hackertarget can return a row with a blank IP
// field for an unresolvable host (e.g. "sub.example.com,"), which the old
// code emitted verbatim as an empty-valued IpAddr trace.
func TestParseHostSearchCSV_BlankIPFieldSkipped(t *testing.T) {
	csv := "sub.example.com,"

	traces := parseHostSearchCSV(csv)

	assert.Empty(t, traces)
}

func TestParseHostSearchCSV_MalformedIPFieldSkipped(t *testing.T) {
	csv := "sub.example.com,not-an-ip"

	traces := parseHostSearchCSV(csv)

	assert.Empty(t, traces)
}

func TestParseHostSearchCSV_BlankSubdomainFieldSkipped(t *testing.T) {
	csv := ",192.168.1.1"

	traces := parseHostSearchCSV(csv)

	assert.Empty(t, traces)
}

func TestParseHostSearchCSV_EmptyBody(t *testing.T) {
	assert.Empty(t, parseHostSearchCSV(""))
}

type fakeHostSearchFetcher struct {
	body    string
	err     error
	lastURL string
}

func (f *fakeHostSearchFetcher) Get(url string) (*http.Response, error) {
	f.lastURL = url
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

func TestSubdomainPlugin_FollowTrace_WrongType(t *testing.T) {
	p := &SubdomainPlugin{fetcher: &fakeHostSearchFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Value: "x", Type: entities.Email})
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestSubdomainPlugin_FollowTrace_ParsesResponse(t *testing.T) {
	fetcher := &fakeHostSearchFetcher{body: "sub.example.com,192.168.1.1"}
	p := &SubdomainPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(entities.Trace{Value: "example.com", Type: entities.Domain})
	require.NoError(t, err)
	require.Len(t, traces, 2)
	assert.Equal(t, "https://api.hackertarget.com/hostsearch/?q=example.com", fetcher.lastURL)
}

func TestSubdomainPlugin_FollowTrace_RequestError(t *testing.T) {
	p := &SubdomainPlugin{fetcher: &fakeHostSearchFetcher{err: errors.New("network error")}}

	_, err := p.FollowTrace(entities.Trace{Value: "example.com", Type: entities.Domain})
	assert.Error(t, err)
}

func TestRegister_RegistersUnderDomainOnly(t *testing.T) {
	p := NewPlugin()
	require.NoError(t, p.Register())

	found := false
	for _, registered := range state.ActivePlugins[entities.Domain] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "SubdomainPlugin", (&SubdomainPlugin{}).String())
}
