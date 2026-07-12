package bluesky_profile

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

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

func profileURL(handle string) string {
	return "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + handle + ".bsky.social"
}

func TestExtractHandle_ValidBlueskyURL(t *testing.T) {
	assert.Equal(t, "lig", extractHandle("https://bsky.app/profile/lig.bsky.social"))
}

func TestExtractHandle_NonBlueskyURL(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://keybase.io/alsmirn"))
}

func TestExtractHandle_WrongPathShape(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://bsky.app/"))
}

// ligFixture mirrors the real API response captured live from
// public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=lig.bsky.social.
const ligFixture = `{
	"did": "did:plc:kxt4u4riehihcpe5lfd6c5l5",
	"handle": "lig.bsky.social",
	"displayName": "",
	"description": "Ligimiz",
	"createdAt": "2023-08-09T07:25:32.355Z",
	"followersCount": 8,
	"followsCount": 5,
	"postsCount": 2
}`

const notFoundFixture = `{"error":"InvalidRequest","message":"Profile not found"}`

func TestFetchProfile_PlainTextDescriptionSkipped(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("lig"): {status: http.StatusOK, body: ligFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "lig")
	require.NoError(t, err)
	assert.Empty(t, traces, "free-text bio with no embedded contact info shouldn't produce a trace")
}

func TestFetchProfile_DescriptionWithEmailExtracted(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `{"handle":"u.bsky.social","displayName":"","description":"reach me at someone@example.com for work"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["email:someone@example.com"])
}

func TestFetchProfile_DescriptionWithURLExtracted(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `{"handle":"u.bsky.social","displayName":"","description":"my site: https://example.com/blog"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["url:https://example.com/blog"])
}

func TestFetchProfile_DisplayNamePresent(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("jay"): {status: http.StatusOK, body: `{"handle":"jay.bsky.team","displayName":"Jay","description":""}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "jay")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["name:Jay"])
}

func TestFetchProfile_EmptyDisplayNameAndDescription(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `{"handle":"u.bsky.social","displayName":"","description":""}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_NotFoundReturnsNoTracesNoError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("alsmirn"): {status: http.StatusBadRequest, body: notFoundFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_OtherNonOKStatusIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusInternalServerError, body: ""},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}

func TestFetchProfile_MalformedJSONIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: "not json"},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}
