package github_profile

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

type fakeResponse struct {
	status int
	body   string
}

type fakeProfileFetcher struct {
	responses map[string]fakeResponse
	lastURL   string
}

func (f *fakeProfileFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	f.lastURL = url
	resp, ok := f.responses[url]
	if !ok {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

func profileURL(username string) string {
	return "https://api.github.com/users/" + username
}

func TestFetchProfile_AllFieldsPresent(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("alsmirn"): {status: http.StatusOK, body: `{
				"name": "Alexey Smirnov",
				"company": "CodeScoring",
				"blog": "https://codescoring.com",
				"location": "Saint Petersburg",
				"email": "alsmirn@example.com",
				"twitter_username": "alsmirn_tw"
			}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	require.NoError(t, err)

	got := map[entities.TraceType]string{}
	for _, tr := range traces {
		got[tr.Type] = tr.Value
	}
	assert.Equal(t, "Alexey Smirnov", got[entities.Name])
	assert.Equal(t, "CodeScoring", got[entities.Company])
	assert.Equal(t, "https://codescoring.com", got[entities.Url])
	assert.Equal(t, "Saint Petersburg", got[entities.Address])
	assert.Equal(t, "alsmirn@example.com", got[entities.Email])
	assert.Equal(t, "alsmirn_tw", got[entities.Twitter])
}

func TestFetchProfile_AllFieldsAbsent(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("nobody"): {status: http.StatusOK, body: `{"login": "nobody"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "nobody")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_InvalidBlogURLSkipped(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `{"blog": "not a real url with spaces and : bad chars"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	for _, tr := range traces {
		assert.NotEqual(t, entities.Url, tr.Type)
	}
}

func TestFetchProfile_BlankBlogFieldSkipped(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `{"blog": ""}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_NonOKStatusReturnsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusNotFound, body: `{"message":"Not Found"}`},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}

func TestFetchProfile_MalformedJSONReturnsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `not json`},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}
