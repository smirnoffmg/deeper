package academicpapers

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchAuthorPapers_MatchesCloseName(t *testing.T) {
	body := `{"data":[{"title":"A Paper","url":"https://example.com/paper","authors":[{"name":"Jane Doe"}]}]}`
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedSemanticScholarURL(t, "Jane Doe"): {status: http.StatusOK, body: body},
		},
	}

	urls, err := searchAuthorPapers(context.Background(), fetcher, "Jane Doe")
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/paper"}, urls)
}

func TestSearchAuthorPapers_DistantNameIsExcluded(t *testing.T) {
	body := `{"data":[{"title":"A Paper","url":"https://example.com/paper","authors":[{"name":"Someone Completely Different"}]}]}`
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedSemanticScholarURL(t, "Jane Doe"): {status: http.StatusOK, body: body},
		},
	}

	urls, err := searchAuthorPapers(context.Background(), fetcher, "Jane Doe")
	require.NoError(t, err)
	assert.Empty(t, urls)
}

func TestSearchAuthorPapers_NonASCIIQueryIsURLEncoded(t *testing.T) {
	fetcher := &fakeSearchFetcher{responses: map[string]fakeResponse{}}

	_, err := searchAuthorPapers(context.Background(), fetcher, "СМИРНОВ АЛЕКСЕЙ")
	require.NoError(t, err)
	require.NotEmpty(t, fetcher.lastURL)

	parsed, err := url.Parse(fetcher.lastURL)
	require.NoError(t, err)
	assert.Equal(t, "СМИРНОВ АЛЕКСЕЙ", parsed.Query().Get("query"))
}

func TestSearchAuthorPapers_EmptyURLSkipped(t *testing.T) {
	body := `{"data":[{"title":"A Paper","url":"","authors":[{"name":"Jane Doe"}]},{"title":"Another","url":"https://example.com/real","authors":[{"name":"Jane Doe"}]}]}`
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedSemanticScholarURL(t, "Jane Doe"): {status: http.StatusOK, body: body},
		},
	}

	urls, err := searchAuthorPapers(context.Background(), fetcher, "Jane Doe")
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/real"}, urls)
}

func TestSearchAuthorPapers_NonOKStatusReturnsError(t *testing.T) {
	fetcher := &fakeSearchFetcher{responses: map[string]fakeResponse{}, defaultStatus: http.StatusTooManyRequests}

	_, err := searchAuthorPapers(context.Background(), fetcher, "Jane Doe")
	assert.Error(t, err)
}

func expectedSemanticScholarURL(t *testing.T, name string) string {
	t.Helper()
	return "https://api.semanticscholar.org/graph/v1/author/search?query=" + url.QueryEscape(name)
}

type fakeResponse struct {
	status int
	body   string
}

type fakeSearchFetcher struct {
	responses     map[string]fakeResponse
	defaultStatus int
	lastURL       string
}

func (f *fakeSearchFetcher) Get(_ context.Context, requestURL string) (*http.Response, error) {
	f.lastURL = requestURL

	resp, ok := f.responses[requestURL]
	if !ok {
		status := f.defaultStatus
		if status == 0 {
			status = http.StatusOK
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}
