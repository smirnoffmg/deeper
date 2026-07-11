package facebook

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

func TestSearchFacebookProfiles_Found(t *testing.T) {
	body := `<a href="https://www.facebook.com/john.doe">John Doe</a>`
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedGoogleURL(t, "john doe"): {status: http.StatusOK, body: body},
		},
	}

	profiles, err := searchFacebookProfiles(context.Background(), fetcher, "john doe")
	require.NoError(t, err)
	assert.Equal(t, []string{"https://www.facebook.com/john.doe"}, profiles)
}

func TestSearchFacebookProfiles_NonASCIIQueryIsURLEncoded(t *testing.T) {
	fetcher := &fakeSearchFetcher{responses: map[string]fakeResponse{}}

	_, err := searchFacebookProfiles(context.Background(), fetcher, "СМИРНОВ АЛЕКСЕЙ")
	require.NoError(t, err)
	require.NotNil(t, fetcher.lastURL)

	parsed, err := url.Parse(fetcher.lastURL)
	require.NoError(t, err)
	assert.Equal(t, "СМИРНОВ АЛЕКСЕЙ site:facebook.com", parsed.Query().Get("q"))
}

func TestSearchFacebookProfiles_NonOKStatusReturnsError(t *testing.T) {
	fetcher := &fakeSearchFetcher{responses: map[string]fakeResponse{}, defaultStatus: http.StatusForbidden}

	_, err := searchFacebookProfiles(context.Background(), fetcher, "john doe")
	assert.Error(t, err)
}

func TestParseGoogleResults_NoMatchIsIgnored(t *testing.T) {
	profiles := parseGoogleResults(`no facebook links here`)
	assert.Empty(t, profiles)
}

func expectedGoogleURL(t *testing.T, query string) string {
	t.Helper()
	return "https://www.google.com/search?q=" + url.QueryEscape(query+" site:facebook.com")
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
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}
