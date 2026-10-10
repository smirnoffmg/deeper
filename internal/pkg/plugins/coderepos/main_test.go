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

var (
	aliceUsername = entities.Trace{Type: entities.Username, Value: "alice"}
	aliceGithub   = entities.Trace{Type: entities.Github, Value: "https://github.com/alice"}
)

func follow(t *testing.T, trace entities.Trace, responses map[string]fakeResponse) ([]plugins.Finding, *fakeRepoFetcher, error) {
	t.Helper()
	fetcher := &fakeRepoFetcher{t: t, responses: responses}
	p := &CodeRepositoriesPlugin{fetcher: fetcher}
	findings, err := p.FollowEvidence(context.Background(), trace)
	return findings, fetcher, err
}

func requestedURLs(f *fakeRepoFetcher) []string {
	var urls []string
	for _, req := range f.reqs {
		urls = append(urls, req.URL.String())
	}
	return urls
}

func TestFollowEvidence_GithubAccountListsOnlyGitHubReposAsStrong(t *testing.T) {
	findings, fetcher, err := follow(t, aliceGithub, map[string]fakeResponse{
		githubURL: {status: http.StatusOK, body: `[{"html_url":"https://github.com/alice/a"}]`},
	})

	require.NoError(t, err)
	assert.Equal(t, []plugins.Finding{{
		Trace:    entities.Trace{Type: entities.Repository, Value: "https://github.com/alice/a"},
		Strength: entities.StrengthStrong,
		Basis:    "repos of github.com/alice",
	}}, findings)
	assert.Equal(t, []string{githubURL}, requestedURLs(fetcher))
}

func TestFollowEvidence_UsernameListsOnlyGitLabAndBitbucketAsWeak(t *testing.T) {
	findings, fetcher, err := follow(t, aliceUsername, map[string]fakeResponse{
		bitbucketURL: {status: http.StatusOK, body: `{"values":[{"links":{"html":{"href":"https://bitbucket.org/alice/b"}}}]}`},
		gitlabURL:    {status: http.StatusOK, body: `[{"web_url":"https://gitlab.com/alice/c"}]`},
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []plugins.Finding{
		{
			Trace:    entities.Trace{Type: entities.Repository, Value: "https://bitbucket.org/alice/b"},
			Strength: entities.StrengthWeak,
			Basis:    "namesake: bitbucket.org/alice has repos",
		},
		{
			Trace:    entities.Trace{Type: entities.Repository, Value: "https://gitlab.com/alice/c"},
			Strength: entities.StrengthWeak,
			Basis:    "namesake: gitlab.com/alice has repos",
		},
	}, findings)
	assert.NotContains(t, requestedURLs(fetcher), githubURL)
}

func TestFollowEvidence_IgnoresNonAccountTraces(t *testing.T) {
	for _, trace := range []entities.Trace{
		{Type: entities.Github, Value: "https://github.com/alice/repo"},
		{Type: entities.Domain, Value: "example.com"},
	} {
		findings, fetcher, err := follow(t, trace, nil)

		require.NoError(t, err, trace.Value)
		assert.Empty(t, findings, trace.Value)
		assert.Empty(t, fetcher.reqs, trace.Value)
	}
}

func TestFollowTrace_StripsEvidence(t *testing.T) {
	p := &CodeRepositoriesPlugin{fetcher: &fakeRepoFetcher{t: t, responses: map[string]fakeResponse{
		gitlabURL: {status: http.StatusOK, body: `[{"web_url":"https://gitlab.com/alice/c"}]`},
	}}}

	traces, err := p.FollowTrace(context.Background(), aliceUsername)

	require.NoError(t, err)
	assert.Equal(t, []entities.Trace{{Type: entities.Repository, Value: "https://gitlab.com/alice/c"}}, traces)
}

func TestFollowEvidence_NotFoundEverywhereIsNoFindingsNoError(t *testing.T) {
	for _, trace := range []entities.Trace{aliceUsername, aliceGithub} {
		findings, _, err := follow(t, trace, nil)

		require.NoError(t, err, trace.Type)
		assert.Empty(t, findings, trace.Type)
	}
}

func TestFollowEvidence_RateLimitedEverywhereIsError(t *testing.T) {
	limited := fakeResponse{status: http.StatusTooManyRequests, body: `[{"web_url":"https://gitlab.com/alice/ghost"}]`}
	findings, _, err := follow(t, aliceUsername, map[string]fakeResponse{
		bitbucketURL: limited,
		gitlabURL:    limited,
	})

	assert.Error(t, err)
	assert.Empty(t, findings)
}

func TestFollowEvidence_ForbiddenBodyNotParsedAsRepos(t *testing.T) {
	findings, _, err := follow(t, aliceGithub, map[string]fakeResponse{
		githubURL: {status: http.StatusForbidden, body: `[{"html_url":"https://github.com/alice/ghost"}]`},
	})

	assert.Error(t, err)
	assert.Empty(t, findings)
}

func TestFollowEvidence_OneSourceFailingKeepsOthersRepos(t *testing.T) {
	findings, _, err := follow(t, aliceUsername, map[string]fakeResponse{
		bitbucketURL: {status: http.StatusOK, body: `{"values":[{"links":{"html":{"href":"https://bitbucket.org/alice/b"}}}]}`},
		gitlabURL:    {status: http.StatusTooManyRequests, body: `[{"web_url":"https://gitlab.com/alice/ghost"}]`},
	})

	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, entities.Trace{Type: entities.Repository, Value: "https://bitbucket.org/alice/b"}, findings[0].Trace)
}

func authorizationByURL(f *fakeRepoFetcher) map[string]string {
	headers := map[string]string{}
	for _, req := range f.reqs {
		headers[req.URL.String()] = req.Header.Get("Authorization")
	}
	return headers
}

func TestFollowEvidence_SendsGitHubTokenOnlyToGitHub(t *testing.T) {
	fetcher := &fakeRepoFetcher{t: t}
	p := &CodeRepositoriesPlugin{fetcher: fetcher, githubToken: "ghp_secret"}

	_, err := p.FollowEvidence(context.Background(), aliceGithub)
	require.NoError(t, err)
	_, err = p.FollowEvidence(context.Background(), aliceUsername)
	require.NoError(t, err)

	headers := authorizationByURL(fetcher)
	require.Contains(t, headers, githubURL)
	require.Contains(t, headers, bitbucketURL)
	require.Contains(t, headers, gitlabURL)
	assert.Equal(t, "Bearer ghp_secret", headers[githubURL])
	assert.Empty(t, headers[bitbucketURL])
	assert.Empty(t, headers[gitlabURL])
}

func TestFollowEvidence_NoGitHubAuthorizationWhenTokenAbsent(t *testing.T) {
	_, fetcher, err := follow(t, aliceGithub, nil)

	require.NoError(t, err)
	headers := authorizationByURL(fetcher)
	require.Contains(t, headers, githubURL)
	assert.Empty(t, headers[githubURL])
}

func TestRegister_RegistersUnderUsernameAndGithub(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	assert.Contains(t, registry[entities.Username], plugins.DeeperPlugin(p))
	assert.Contains(t, registry[entities.Github], plugins.DeeperPlugin(p))
}
