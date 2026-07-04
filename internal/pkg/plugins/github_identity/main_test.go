package github_identity

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
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

type fakeCommitFetcher struct {
	status  int
	body    string
	headers http.Header
	lastReq *http.Request
}

func (f *fakeCommitFetcher) Do(req *http.Request) (*http.Response, error) {
	f.lastReq = req
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
