package js_crawler

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

type fakeResponse struct {
	status int
	body   string
	err    error
}

type fakeFetcher struct {
	responses map[string]fakeResponse
	requested []string
}

func (f *fakeFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	f.requested = append(f.requested, url)
	resp, ok := f.responses[url]
	if !ok {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	if resp.err != nil {
		return nil, resp.err
	}
	status := resp.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

func traceValues(traces []entities.Trace, t entities.TraceType) []string {
	out := make([]string, 0)
	for _, tr := range traces {
		if tr.Type == t {
			out = append(out, tr.Value)
		}
	}
	return out
}

func TestFollowTrace_DiscoversAndScrapesJS(t *testing.T) {
	page := `<html><head>
		<script src="/static/app.js"></script>
		<script src="https://cdn.example.net/lib.js"></script>
	</head><body>hi</body></html>`
	fetcher := &fakeFetcher{
		responses: map[string]fakeResponse{
			"https://target.tld/": {body: page},
			"https://target.tld/static/app.js": {body: `
				var api = "https://api.target.tld/v1/users";
				// contact: support@target.tld
			`},
			"https://cdn.example.net/lib.js": {body: `
				fetch("https://tracker.target.tld/collect");
				var owner = "dev@company.io";
			`},
		},
	}
	p := &JSCrawlerPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://target.tld/"})
	require.NoError(t, err)

	assert.Contains(t, fetcher.requested, "https://target.tld/static/app.js")
	assert.Contains(t, fetcher.requested, "https://cdn.example.net/lib.js")

	urls := traceValues(traces, entities.Url)
	assert.Contains(t, urls, "https://api.target.tld/v1/users")
	assert.Contains(t, urls, "https://tracker.target.tld/collect")

	emails := traceValues(traces, entities.Email)
	assert.Contains(t, emails, "support@target.tld")
	assert.Contains(t, emails, "dev@company.io")
}

func TestFollowTrace_RejectsReservedEmail(t *testing.T) {
	page := `<html><script src="/a.js"></script></html>`
	fetcher := &fakeFetcher{
		responses: map[string]fakeResponse{
			"https://target.tld/":     {body: page},
			"https://target.tld/a.js": {body: `contact you@example.com and real@target.tld`},
		},
	}
	p := &JSCrawlerPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://target.tld/"})
	require.NoError(t, err)

	emails := traceValues(traces, entities.Email)
	assert.NotContains(t, emails, "you@example.com")
	assert.Contains(t, emails, "real@target.tld")
}

func TestFollowTrace_Deduplicates(t *testing.T) {
	page := `<html>
		<script src="/a.js"></script>
		<script src="/b.js"></script>
	</html>`
	fetcher := &fakeFetcher{
		responses: map[string]fakeResponse{
			"https://target.tld/":     {body: page},
			"https://target.tld/a.js": {body: `x="https://dup.tld/z"; e="same@target.tld"`},
			"https://target.tld/b.js": {body: `y="https://dup.tld/z"; f="same@target.tld"`},
		},
	}
	p := &JSCrawlerPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://target.tld/"})
	require.NoError(t, err)

	assert.Len(t, traceValues(traces, entities.Url), 1)
	assert.Len(t, traceValues(traces, entities.Email), 1)
}

func TestFollowTrace_OneJSFailureDoesNotAbortOthers(t *testing.T) {
	page := `<html>
		<script src="/broken.js"></script>
		<script src="/good.js"></script>
	</html>`
	fetcher := &fakeFetcher{
		responses: map[string]fakeResponse{
			"https://target.tld/":          {body: page},
			"https://target.tld/broken.js": {status: http.StatusNotFound, body: "not found"},
			"https://target.tld/good.js":   {body: `u="https://found.tld/ok"`},
		},
	}
	p := &JSCrawlerPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://target.tld/"})
	require.NoError(t, err)

	assert.Contains(t, traceValues(traces, entities.Url), "https://found.tld/ok")
}

func TestFollowTrace_JSFetchErrorDoesNotAbortOthers(t *testing.T) {
	page := `<html>
		<script src="/err.js"></script>
		<script src="/good.js"></script>
	</html>`
	fetcher := &fakeFetcher{
		responses: map[string]fakeResponse{
			"https://target.tld/":        {body: page},
			"https://target.tld/err.js":  {err: errors.New("dial tcp: connection refused")},
			"https://target.tld/good.js": {body: `u="https://found.tld/ok"`},
		},
	}
	p := &JSCrawlerPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://target.tld/"})
	require.NoError(t, err)

	assert.Contains(t, traceValues(traces, entities.Url), "https://found.tld/ok")
}

func TestFollowTrace_PageFetchErrorReturnsNil(t *testing.T) {
	fetcher := &fakeFetcher{
		responses: map[string]fakeResponse{
			"https://target.tld/": {err: errors.New("timeout")},
		},
	}
	p := &JSCrawlerPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://target.tld/"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_WrongType(t *testing.T) {
	p := &JSCrawlerPlugin{fetcher: &fakeFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_CapsJSFetches(t *testing.T) {
	var sb strings.Builder
	responses := map[string]fakeResponse{}
	for i := 0; i < maxJSFiles+5; i++ {
		src := "/s" + string(rune('a'+i)) + ".js"
		sb.WriteString(`<script src="` + src + `"></script>`)
		responses["https://target.tld"+src] = fakeResponse{body: ``}
	}
	responses["https://target.tld/"] = fakeResponse{body: "<html>" + sb.String() + "</html>"}
	fetcher := &fakeFetcher{responses: responses}
	p := &JSCrawlerPlugin{fetcher: fetcher}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://target.tld/"})
	require.NoError(t, err)

	jsFetches := 0
	for _, u := range fetcher.requested {
		if strings.HasSuffix(u, ".js") {
			jsFetches++
		}
	}
	assert.LessOrEqual(t, jsFetches, maxJSFiles)
}

func TestRegister_RegistersUnderURL(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Url] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "JSCrawlerPlugin", (&JSCrawlerPlugin{}).String())
}
