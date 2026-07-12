package keybase_profile

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

func lookupURL(handle string) string {
	return "https://keybase.io/_/api/1.0/user/lookup.json?usernames=" + handle
}

func TestExtractHandle_ValidKeybaseURL(t *testing.T) {
	assert.Equal(t, "alsmirn", extractHandle("https://keybase.io/alsmirn"))
}

func TestExtractHandle_NonKeybaseURL(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://github.com/alsmirn"))
}

func TestExtractHandle_RootPath(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://keybase.io/"))
}

func TestExtractHandle_MalformedURL(t *testing.T) {
	assert.Equal(t, "", extractHandle("not a url"))
}

// ligFixture mirrors the actual JSON captured live from
// https://keybase.io/_/api/1.0/user/lookup.json?usernames=lig.
const ligFixture = `{
	"status": {"code": 0, "name": "OK"},
	"them": [{
		"profile": {
			"full_name": "Serge Matveenko",
			"location": "Saint-Petersburg, Russia",
			"bio": "about.me/lig1"
		},
		"proofs_summary": {
			"all": [
				{"proof_type": "github", "nametag": "lig", "service_url": "https://github.com/lig"},
				{"proof_type": "reddit", "nametag": "lig1", "service_url": "https://reddit.com/user/lig1"},
				{"proof_type": "some_new_unrecognized_service", "nametag": "x", "service_url": "https://weird.example/x"}
			]
		}
	}]
}`

const emptyProfileFixture = `{
	"status": {"code": 0, "name": "OK"},
	"them": [{
		"proofs_summary": {"all": []}
	}]
}`

const notFoundFixture = `{"status": {"code": 0, "name": "OK"}, "them": [null]}`

const inputErrorFixture = `{"status": {"code": 100, "desc": "bad list value", "name": "INPUT_ERROR"}}`

func TestFetchProfile_FullProfileWithProofs(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("lig"): {status: http.StatusOK, body: ligFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "lig")
	require.NoError(t, err)

	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["name:Serge Matveenko"])
	assert.True(t, got["address:Saint-Petersburg, Russia"])
	assert.True(t, got["url:https://about.me/lig1"])
	assert.True(t, got["social_generic:https://github.com/lig"])
	assert.True(t, got["social_generic:https://reddit.com/user/lig1"])
	assert.False(t, got["social_generic:https://weird.example/x"], "unrecognized proof types must not be emitted")
}

func TestFetchProfile_EmptyProfileNoProofs(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("alsmirn"): {status: http.StatusOK, body: emptyProfileFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_UserNotFound(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("zzzznonexistent"): {status: http.StatusOK, body: notFoundFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "zzzznonexistent")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_InputErrorTreatedAsNotFound(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("a"): {status: http.StatusOK, body: inputErrorFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "a")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_NonOKHTTPStatusIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("alsmirn"): {status: http.StatusInternalServerError, body: ""},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	assert.Error(t, err)
}

func TestFetchProfile_MalformedJSONIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("alsmirn"): {status: http.StatusOK, body: "not json"},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	assert.Error(t, err)
}

func TestFetchProfile_BioNotURLLikeIsSkipped(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("u"): {status: http.StatusOK, body: `{
				"status": {"code": 0, "name": "OK"},
				"them": [{"profile": {"full_name": "Someone", "bio": "just a developer, no links here"}, "proofs_summary": {"all": []}}]
			}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	for _, tr := range traces {
		assert.NotEqual(t, entities.Url, tr.Type)
	}
}
