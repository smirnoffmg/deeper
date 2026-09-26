package live_host

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

type fakeProber struct {
	responses map[string]*http.Response
	errs      map[string]error
}

func (f *fakeProber) Get(_ context.Context, url string) (*http.Response, error) {
	if err, ok := f.errs[url]; ok {
		return nil, err
	}
	if resp, ok := f.responses[url]; ok {
		return resp, nil
	}
	return nil, errors.New("unexpected url: " + url)
}

func respWithBody(status int, html string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(html))}
}

func TestFollowTrace_HTTPSRespondsWithTitle(t *testing.T) {
	prober := &fakeProber{
		responses: map[string]*http.Response{
			"https://api.example.com": respWithBody(http.StatusOK, "<html><head><title>  Example API  </title></head></html>"),
		},
	}
	p := &LiveHostPlugin{prober: prober}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "api.example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 2)
	assert.Equal(t, entities.Trace{Value: "api.example.com", Type: entities.Host}, traces[0])
	assert.Equal(t, entities.Trace{Value: "Example API", Type: entities.Name}, traces[1])
}

func TestFollowTrace_HTTPSFailsHTTPResponds(t *testing.T) {
	prober := &fakeProber{
		responses: map[string]*http.Response{
			"http://api.example.com": respWithBody(http.StatusOK, "<html></html>"),
		},
		errs: map[string]error{
			"https://api.example.com": errors.New("tls handshake failed"),
		},
	}
	p := &LiveHostPlugin{prober: prober}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "api.example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Trace{Value: "api.example.com", Type: entities.Host}, traces[0])
}

func TestFollowTrace_BothSchemesError(t *testing.T) {
	prober := &fakeProber{
		errs: map[string]error{
			"https://dead.example.com": errors.New("no route to host"),
			"http://dead.example.com":  errors.New("connection refused"),
		},
	}
	p := &LiveHostPlugin{prober: prober}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "dead.example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_Status500NotLive(t *testing.T) {
	prober := &fakeProber{
		responses: map[string]*http.Response{
			"https://api.example.com": respWithBody(http.StatusInternalServerError, "<html><title>oops</title></html>"),
			"http://api.example.com":  respWithBody(http.StatusInternalServerError, "<html><title>oops</title></html>"),
		},
	}
	p := &LiveHostPlugin{prober: prober}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "api.example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NoTitleOnlyHost(t *testing.T) {
	prober := &fakeProber{
		responses: map[string]*http.Response{
			"https://api.example.com": respWithBody(http.StatusOK, "<html><head></head><body>hi</body></html>"),
		},
	}
	p := &LiveHostPlugin{prober: prober}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "api.example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Trace{Value: "api.example.com", Type: entities.Host}, traces[0])
}

func TestFollowTrace_EmptyTitleOnlyHost(t *testing.T) {
	prober := &fakeProber{
		responses: map[string]*http.Response{
			"https://api.example.com": respWithBody(http.StatusOK, "<html><head><title>   </title></head></html>"),
		},
	}
	p := &LiveHostPlugin{prober: prober}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "api.example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Host, traces[0].Type)
}

func TestFollowTrace_WrongType(t *testing.T) {
	p := &LiveHostPlugin{prober: &fakeProber{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "alice"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestRegister_RegistersUnderSubdomainAndDomain(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	for _, tt := range []entities.TraceType{entities.Subdomain, entities.Domain} {
		found := false
		for _, registered := range registry[tt] {
			if registered == p {
				found = true
			}
		}
		assert.Truef(t, found, "plugin not registered under %s", tt)
	}
}

func TestString(t *testing.T) {
	assert.Equal(t, "LiveHostPlugin", (&LiveHostPlugin{}).String())
}

func TestFollowTrace_ErrorPageTitleSkipped(t *testing.T) {
	// A 4xx page is still a live host, but its title ("403 Forbidden",
	// "401 Authorization Required") is an error page, not a name.
	cases := map[string]*http.Response{
		"https://api.example.com": respWithBody(http.StatusForbidden, "<html><head><title>403 Forbidden</title></head></html>"),
	}
	p := &LiveHostPlugin{prober: &fakeProber{responses: cases}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "api.example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Trace{Value: "api.example.com", Type: entities.Host}, traces[0])
}

func TestFollowTrace_401TitleSkipped(t *testing.T) {
	p := &LiveHostPlugin{prober: &fakeProber{responses: map[string]*http.Response{
		"https://api.example.com": respWithBody(http.StatusUnauthorized, "<html><title>401 Authorization Required</title></html>"),
	}}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "api.example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Host, traces[0].Type)
}

func TestFollowTrace_StatusLikeTitleOn200Skipped(t *testing.T) {
	// Some WAFs serve a block page with a 200 status; a "404 Not Found"
	// title is still noise.
	p := &LiveHostPlugin{prober: &fakeProber{responses: map[string]*http.Response{
		"https://api.example.com": respWithBody(http.StatusOK, "<html><title>404 Not Found</title></html>"),
	}}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Subdomain, Value: "api.example.com"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Host, traces[0].Type)
}
