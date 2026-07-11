package github_identity

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleCommitsJSON = `[
  {
    "author": {"login": "alsmirn"},
    "commit": {
      "author": {
        "name": "Alexey Smirnov",
        "email": "alsmirn@users.noreply.github.com"
      }
    }
  },
  {
    "commit": {
      "author": {
        "name": "Unknown Contributor",
        "email": "anon@example.com"
      }
    }
  }
]`

func TestFetchCommitAuthors_MatchedLogin(t *testing.T) {
	fetcher := &fakeCommitFetcher{
		status: http.StatusOK,
		body:   sampleCommitsJSON,
	}

	authors, err := fetchCommitAuthors(context.Background(), fetcher, "CodeScoring", "awesome-open-source-licensing", "")
	require.NoError(t, err)
	require.Len(t, authors, 2)
	assert.Equal(t, "alsmirn", authors[0].Login)
	assert.Equal(t, "Alexey Smirnov", authors[0].Name)
	assert.Empty(t, authors[1].Login)
}

func TestFetchCommitAuthors_RateLimited(t *testing.T) {
	fetcher := &fakeCommitFetcher{
		status: http.StatusForbidden,
		body:   `{"message":"API rate limit exceeded"}`,
	}
	fetcher.headers = make(http.Header)
	fetcher.headers.Set("X-RateLimit-Remaining", "0")

	_, err := fetchCommitAuthors(context.Background(), fetcher, "owner", "repo", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestFetchCommitAuthors_MalformedJSON(t *testing.T) {
	fetcher := &fakeCommitFetcher{
		status: http.StatusOK,
		body:   `{invalid`,
	}

	authors, err := fetchCommitAuthors(context.Background(), fetcher, "owner", "repo", "")
	require.NoError(t, err)
	assert.Empty(t, authors)
}

func TestFollowTrace_WrongType(t *testing.T) {
	p := testPlugin(&fakeCommitFetcher{})
	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_ValidRepo(t *testing.T) {
	fetcher := &fakeCommitFetcher{
		status: http.StatusOK,
		body:   sampleCommitsJSON,
	}
	p := testPlugin(fetcher)

	traces, err := p.FollowTrace(entities.Trace{
		Type:  entities.Github,
		Value: "https://github.com/CodeScoring/awesome-open-source-licensing",
	})
	require.NoError(t, err)
	require.NotEmpty(t, traces)

	types := make(map[entities.TraceType]bool)
	for _, tr := range traces {
		types[tr.Type] = true
	}
	assert.True(t, types[entities.Name])
	assert.True(t, types[entities.Email])
	assert.True(t, types[entities.Username])
}

func TestFollowTrace_RepositoryTraceTypeIsFollowed(t *testing.T) {
	fetcher := &fakeCommitFetcher{
		status: http.StatusOK,
		body:   sampleCommitsJSON,
	}
	p := testPlugin(fetcher)

	traces, err := p.FollowTrace(entities.Trace{
		Type:  entities.Repository,
		Value: "https://github.com/alsmirn/gyt",
	})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestFollowTrace_GitLabRepositoryTraceIsSkipped(t *testing.T) {
	p := testPlugin(&fakeCommitFetcher{})

	traces, err := p.FollowTrace(entities.Trace{
		Type:  entities.Repository,
		Value: "https://gitlab.com/owner/repo",
	})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_SharedRepoOnlyReturnsOwnerTraces(t *testing.T) {
	sharedRepoCommits := `[
		{"author": {"login": "iloncka"}, "commit": {"author": {"name": "Ilona Kovaleva", "email": "iloncka@example.com"}, "committer": {"name": "Ilona Kovaleva", "email": "iloncka@example.com"}, "message": "init"}},
		{"commit": {"author": {"name": "Erick Ramirez", "email": "erick@datastax.com"}, "committer": {"name": "Erick Ramirez", "email": "erick@datastax.com"}, "message": "a"}},
		{"commit": {"author": {"name": "Stefano Lottini", "email": "stefano@datastax.com"}, "committer": {"name": "Stefano Lottini", "email": "stefano@datastax.com"}, "message": "b"}},
		{"commit": {"author": {"name": "Cedrick Lunven", "email": "cedrick@datastax.com"}, "committer": {"name": "Cedrick Lunven", "email": "cedrick@datastax.com"}, "message": "c"}},
		{"commit": {"author": {"name": "David Jones-Gilardi", "email": "david@datastax.com"}, "committer": {"name": "David Jones-Gilardi", "email": "david@datastax.com"}, "message": "d"}},
		{"commit": {"author": {"name": "Kirsten Hunter", "email": "kirsten@datastax.com"}, "committer": {"name": "Kirsten Hunter", "email": "kirsten@datastax.com"}, "message": "e"}}
	]`
	fetcher := &fakeCommitFetcher{status: http.StatusOK, body: sharedRepoCommits}
	p := testPlugin(fetcher)

	traces, err := p.FollowTrace(entities.Trace{
		Type:  entities.Repository,
		Value: "https://github.com/iloncka/workshop-astra-tik-tok",
	})
	require.NoError(t, err)

	for _, tr := range traces {
		if tr.Type == entities.Email || tr.Type == entities.Name {
			assert.NotContains(t, tr.Value, "datastax.com", "unrelated contributor from a large shared repo must not leak through")
		}
	}
	assert.Contains(t, traces, entities.Trace{Type: entities.Name, Value: "Ilona Kovaleva"})
	assert.Contains(t, traces, entities.Trace{Type: entities.Email, Value: "iloncka@example.com"})
	assert.Contains(t, traces, entities.Trace{Type: entities.Username, Value: "iloncka"})
}

func TestFollowTrace_ForkedRepoIsSkipped(t *testing.T) {
	metadataURL := "https://api.github.com/repos/alsmirn/youtube-dl"

	fetcher := &fakeCommitFetcher{
		responses: map[string]fakeCommitResponse{
			metadataURL: {status: http.StatusOK, body: `{"fork": true}`},
			// No entry for commitsURL: if FollowTrace incorrectly proceeds to
			// fetch commits anyway, this fake's fallback (status 0, empty
			// body) will fail JSON decoding and surface as a bug, not a pass.
		},
	}
	p := testPlugin(fetcher)

	traces, err := p.FollowTrace(entities.Trace{
		Type:  entities.Repository,
		Value: "https://github.com/alsmirn/youtube-dl",
	})
	require.NoError(t, err)
	assert.Nil(t, traces)
	assert.Equal(t, metadataURL, fetcher.lastReq.URL.String(), "commits endpoint must not be reached for a fork")
}

func TestFollowTrace_OrgRootRejected(t *testing.T) {
	p := testPlugin(&fakeCommitFetcher{})
	traces, err := p.FollowTrace(entities.Trace{
		Type:  entities.Github,
		Value: "https://github.com/acme",
	})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_SetsAuthorizationWhenTokenPresent(t *testing.T) {
	fetcher := &fakeCommitFetcher{
		status: http.StatusOK,
		body:   `[]`,
	}
	p := &GitHubIdentityPlugin{fetcher: fetcher, token: "ghp_secret"}

	_, err := p.FollowTrace(entities.Trace{
		Type:  entities.Github,
		Value: "https://github.com/owner/repo",
	})
	require.NoError(t, err)
	require.NotNil(t, fetcher.lastReq)
	assert.Equal(t, "Bearer ghp_secret", fetcher.lastReq.Header.Get("Authorization"))
}

func TestFollowTrace_NoAuthorizationWhenTokenAbsent(t *testing.T) {
	fetcher := &fakeCommitFetcher{
		status: http.StatusOK,
		body:   `[]`,
	}
	p := testPlugin(fetcher)

	_, err := p.FollowTrace(entities.Trace{
		Type:  entities.Github,
		Value: "https://github.com/owner/repo",
	})
	require.NoError(t, err)
	require.NotNil(t, fetcher.lastReq)
	assert.Empty(t, fetcher.lastReq.Header.Get("Authorization"))
}

func TestString(t *testing.T) {
	p := testPlugin(&fakeCommitFetcher{})
	assert.Equal(t, "GitHubIdentityPlugin", p.String())
}

func TestRegister_RegistersUnderGithubAndRepository(t *testing.T) {
	p := NewPlugin()
	require.NoError(t, p.Register())

	for _, traceType := range []entities.TraceType{entities.Github, entities.Repository} {
		found := false
		for _, registered := range state.ActivePlugins[traceType] {
			if registered == p {
				found = true
			}
		}
		assert.Truef(t, found, "expected plugin registered for %v", traceType)
	}
}

type fakeCommitResponse struct {
	status int
	body   string
}

type fakeCommitFetcher struct {
	status  int
	body    string
	headers http.Header
	lastReq *http.Request

	// responses, if set, routes by exact request URL; falls back to the
	// single status/body above for any URL not present in the map.
	responses map[string]fakeCommitResponse
}

func (f *fakeCommitFetcher) Do(req *http.Request) (*http.Response, error) {
	f.lastReq = req

	if resp, ok := f.responses[req.URL.String()]; ok {
		return &http.Response{
			StatusCode: resp.status,
			Body:       io.NopCloser(strings.NewReader(resp.body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	headers := f.headers
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     headers,
		Request:    req,
	}, nil
}

func testPlugin(fetcher commitFetcher) *GitHubIdentityPlugin {
	return &GitHubIdentityPlugin{fetcher: fetcher}
}

var _ commitFetcher = (*fakeCommitFetcher)(nil)
