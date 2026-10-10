package bluesky_profile

import (
	"context"
	"fmt"
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
	t         *testing.T
	responses map[string]fakeResponse
}

func (f *fakeProfileFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	resp, ok := f.responses[url]
	if !ok {
		f.t.Errorf("unexpected request: %s", url)
		return nil, fmt.Errorf("unexpected request: %s", url)
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

func profileURL(actor string) string {
	return "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + actor
}

func TestExtractHandle_ValidBlueskyURL(t *testing.T) {
	assert.Equal(t, "lig.bsky.social", extractHandle("https://bsky.app/profile/lig.bsky.social"))
}

func TestExtractHandle_CustomDomainKeptAsGiven(t *testing.T) {
	assert.Equal(t, "alice.example.com", extractHandle("https://bsky.app/profile/alice.example.com"))
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
		t: t,
		responses: map[string]fakeResponse{
			profileURL("lig.bsky.social"): {status: http.StatusOK, body: ligFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "lig.bsky.social")
	require.NoError(t, err)
	assert.Empty(t, traces, "free-text bio with no embedded contact info shouldn't produce a trace")
}

func TestFetchProfile_DescriptionWithEmailExtracted(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("u.bsky.social"): {status: http.StatusOK, body: `{"handle":"u.bsky.social","displayName":"","description":"reach me at someone@example.com for work"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u.bsky.social")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["email:someone@example.com"])
}

func TestFetchProfile_DescriptionWithURLExtracted(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("u.bsky.social"): {status: http.StatusOK, body: `{"handle":"u.bsky.social","displayName":"","description":"my site: https://example.com/blog"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u.bsky.social")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["url:https://example.com/blog"])
}

func TestFetchProfile_DisplayNamePresent(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("jay.bsky.social"): {status: http.StatusOK, body: `{"handle":"jay.bsky.team","displayName":"Jay","description":""}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "jay.bsky.social")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["name:Jay"])
}

func TestFetchProfile_EmptyDisplayNameAndDescription(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("u.bsky.social"): {status: http.StatusOK, body: `{"handle":"u.bsky.social","displayName":"","description":""}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u.bsky.social")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_NotFoundReturnsNoTracesNoError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("alsmirn.bsky.social"): {status: http.StatusBadRequest, body: notFoundFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alsmirn.bsky.social")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_OtherNonOKStatusIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("u.bsky.social"): {status: http.StatusInternalServerError, body: ""},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u.bsky.social")
	assert.Error(t, err)
}

func TestFetchProfile_MalformedJSONIsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("u.bsky.social"): {status: http.StatusOK, body: "not json"},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u.bsky.social")
	assert.Error(t, err)
}

func TestFetchProfile_CustomDomainHandleQueriedAsIs(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("alice.example.com"): {status: http.StatusOK, body: `{"handle":"alice.example.com","displayName":"Alice"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alice.example.com")
	require.NoError(t, err)
	assert.Contains(t, traces, entities.Trace{Type: entities.Name, Value: "Alice"})
}

func TestFetchProfile_BareHandleGetsDefaultSuffix(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("jay.bsky.social"): {status: http.StatusOK, body: `{"handle":"jay.bsky.social","displayName":"Jay"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "jay")
	require.NoError(t, err)
	assert.Contains(t, traces, entities.Trace{Type: entities.Name, Value: "Jay"})
}

func TestFetchProfile_DIDQueriedWithoutSuffix(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		t: t,
		responses: map[string]fakeResponse{
			profileURL("did:plc:abc"): {status: http.StatusOK, body: `{"handle":"jay.bsky.social","displayName":"Jay"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "did:plc:abc")
	require.NoError(t, err)
	assert.Contains(t, traces, entities.Trace{Type: entities.Name, Value: "Jay"})
}
