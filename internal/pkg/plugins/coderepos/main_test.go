package coderepos

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

type fakeResponse struct {
	status int
	body   string
}

type fakeRepoFetcher struct {
	t         *testing.T
	responses map[string]fakeResponse
	reqs      []*http.Request
}

func (f *fakeRepoFetcher) Do(req *http.Request) (*http.Response, error) {
	f.reqs = append(f.reqs, req)
	return f.Get(req.Context(), req.URL.String())
}

func (f *fakeRepoFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	resp, ok := f.responses[url]
	if !ok {
		if url != githubURL && url != bitbucketURL && url != gitlabURL {
			f.t.Errorf("unexpected request: %s", url)
		}
		resp = fakeResponse{status: http.StatusNotFound, body: `{"message":"Not Found"}`}
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

const (
	githubURL    = "https://api.github.com/users/alice/repos?per_page=100"
	bitbucketURL = "https://api.bitbucket.org/2.0/repositories/alice?pagelen=100"
	gitlabURL    = "https://gitlab.com/api/v4/users/alice/projects?per_page=100"
)

var aliceUsername = entities.Trace{Type: entities.Username, Value: "alice"}

func follow(t *testing.T, responses map[string]fakeResponse) ([]entities.Trace, *fakeRepoFetcher, error) {
	t.Helper()
	fetcher := &fakeRepoFetcher{t: t, responses: responses}
	p := &CodeRepositoriesPlugin{fetcher: fetcher}
	traces, err := p.FollowTrace(context.Background(), aliceUsername)
	return traces, fetcher, err
}

func TestFollowTrace_ListsReposFromAllHosts(t *testing.T) {
	traces, _, err := follow(t, map[string]fakeResponse{
		githubURL:    {status: http.StatusOK, body: `[{"html_url":"https://github.com/alice/a"}]`},
		bitbucketURL: {status: http.StatusOK, body: `{"values":[{"links":{"html":{"href":"https://bitbucket.org/alice/b"}}}]}`},
		gitlabURL:    {status: http.StatusOK, body: `[{"web_url":"https://gitlab.com/alice/c"}]`},
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []entities.Trace{
		{Type: entities.Repository, Value: "https://github.com/alice/a"},
		{Type: entities.Repository, Value: "https://bitbucket.org/alice/b"},
		{Type: entities.Repository, Value: "https://gitlab.com/alice/c"},
	}, traces)
}

func TestFollowTrace_IgnoresNonUsernameTraces(t *testing.T) {
	fetcher := &fakeRepoFetcher{t: t}
	p := &CodeRepositoriesPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})

	require.NoError(t, err)
	assert.Empty(t, traces)
	assert.Empty(t, fetcher.reqs)
}

func TestFollowTrace_NotFoundEverywhereIsNoTracesNoError(t *testing.T) {
	traces, _, err := follow(t, nil)

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFollowTrace_RateLimitedEverywhereIsError(t *testing.T) {
	limited := fakeResponse{status: http.StatusTooManyRequests, body: `[{"web_url":"https://gitlab.com/alice/ghost"}]`}
	traces, _, err := follow(t, map[string]fakeResponse{
		githubURL:    limited,
		bitbucketURL: limited,
		gitlabURL:    limited,
	})

	assert.Error(t, err)
	assert.Empty(t, traces)
}

func TestFollowTrace_ForbiddenBodyNotParsedAsRepos(t *testing.T) {
	traces, _, err := follow(t, map[string]fakeResponse{
		githubURL: {status: http.StatusForbidden, body: `[{"html_url":"https://github.com/alice/ghost"}]`},
	})

	assert.Error(t, err)
	assert.Empty(t, traces)
}

func TestFollowTrace_OneSourceFailingKeepsOthersRepos(t *testing.T) {
	traces, _, err := follow(t, map[string]fakeResponse{
		bitbucketURL: {status: http.StatusOK, body: `{"values":[{"links":{"html":{"href":"https://bitbucket.org/alice/b"}}}]}`},
		gitlabURL:    {status: http.StatusTooManyRequests, body: `[{"web_url":"https://gitlab.com/alice/ghost"}]`},
	})

	require.NoError(t, err)
	assert.Equal(t, []entities.Trace{{Type: entities.Repository, Value: "https://bitbucket.org/alice/b"}}, traces)
}

func authorizationByURL(f *fakeRepoFetcher) map[string]string {
	headers := map[string]string{}
	for _, req := range f.reqs {
		headers[req.URL.String()] = req.Header.Get("Authorization")
	}
	return headers
}

func TestFollowTrace_SendsGitHubTokenOnlyToGitHub(t *testing.T) {
	fetcher := &fakeRepoFetcher{t: t}
	p := &CodeRepositoriesPlugin{fetcher: fetcher, githubToken: "ghp_secret"}

	_, err := p.FollowTrace(context.Background(), aliceUsername)
	require.NoError(t, err)

	headers := authorizationByURL(fetcher)
	require.Contains(t, headers, githubURL)
	require.Contains(t, headers, bitbucketURL)
	require.Contains(t, headers, gitlabURL)
	assert.Equal(t, "Bearer ghp_secret", headers[githubURL])
	assert.Empty(t, headers[bitbucketURL])
	assert.Empty(t, headers[gitlabURL])
}

func TestFollowTrace_NoGitHubAuthorizationWhenTokenAbsent(t *testing.T) {
	_, fetcher, err := follow(t, nil)

	require.NoError(t, err)
	headers := authorizationByURL(fetcher)
	require.Contains(t, headers, githubURL)
	assert.Empty(t, headers[githubURL])
}

func TestRegister_RegistersUnderUsername(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	assert.Contains(t, registry[entities.Username], plugins.DeeperPlugin(p))
}
