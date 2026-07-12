package codeforces_profile

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
}

func (f *fakeProfileFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	resp, ok := f.responses[url]
	if !ok {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

func infoURL(handle string) string {
	return "https://codeforces.com/api/user.info?handles=" + handle
}

func TestExtractHandle_ValidCodeforcesURL(t *testing.T) {
	assert.Equal(t, "alsmirn", extractHandle("https://codeforces.com/profile/alsmirn"))
}

func TestExtractHandle_NonCodeforcesURL(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://keybase.io/alsmirn"))
}

func TestExtractHandle_WrongPathShape(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://codeforces.com/"))
}

// touristFixture mirrors the real API response for a well-known handle,
// captured live from https://codeforces.com/api/user.info?handles=tourist.
const touristFixture = `{
	"status": "OK",
	"result": [{
		"lastName": "Korotkevich",
		"country": "Belarus",
		"city": "Gomel",
		"handle": "tourist",
		"firstName": "Gennady",
		"organization": "ITMO University"
	}]
}`

const notFoundFixture = `{"status": "FAILED", "comment": "handles: User with handle zzzz not found"}`

func TestFetchProfile_FullResult(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			infoURL("tourist"): {status: http.StatusOK, body: touristFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "tourist")
	require.NoError(t, err)

	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["name:Gennady Korotkevich"])
	assert.True(t, got["company:ITMO University"])
	assert.True(t, got["address:Gomel, Belarus"])
}

func TestFetchProfile_NotFoundReturnsNoTracesNoError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			infoURL("zzzz"): {status: http.StatusOK, body: notFoundFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "zzzz")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_PartialFieldsOnlyEmitPresent(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			infoURL("lig"): {status: http.StatusOK, body: `{"status":"OK","result":[{"handle":"LIG"}]}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "lig")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_OnlyFirstNamePresent(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			infoURL("u"): {status: http.StatusOK, body: `{"status":"OK","result":[{"handle":"u","firstName":"Alex"}]}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Name, traces[0].Type)
	assert.Equal(t, "Alex", traces[0].Value)
}

func TestFetchProfile_NonOKHTTPStatusIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			infoURL("u"): {status: http.StatusInternalServerError, body: ""},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}

func TestFetchProfile_MalformedJSONIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			infoURL("u"): {status: http.StatusOK, body: "not json"},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}
