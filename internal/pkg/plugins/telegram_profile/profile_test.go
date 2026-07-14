package telegram_profile

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

type fakePageFetcher struct {
	responses map[string]fakeResponse
}

func (f *fakePageFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	resp, ok := f.responses[url]
	if !ok {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("<html></html>"))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

func channelURL(channel string) string {
	return "https://t.me/" + channel
}

func TestExtractChannel_ValidTelegramURL(t *testing.T) {
	assert.Equal(t, "codescoring", extractChannel("https://t.me/codescoring"))
}

func TestExtractChannel_NonTelegramURL(t *testing.T) {
	assert.Equal(t, "", extractChannel("https://keybase.io/alsmirn"))
}

func TestExtractChannel_RootPath(t *testing.T) {
	assert.Equal(t, "", extractChannel("https://t.me/"))
}

// realFixture mirrors the actual og:description captured live from t.me/codescoring.
const realFixture = `<!DOCTYPE html><html><head>
<meta property="og:title" content="CodeScoring Updates">
<meta property="og:site_name" content="Telegram">
<meta property="og:description" content="Новости о продукте CodeScoring — свежие и из первых рук.		https://codescoring.ru/">
</head></html>`

// nonexistentFixture mirrors the real markup for a channel that doesn't
// exist: HTTP 200, but an empty og:description (t.me always 200s with a
// generic "Contact @handle" shell for unknown handles).
const nonexistentFixture = `<!DOCTYPE html><html><head>
<meta property="og:title" content="Telegram: Contact @zzzznonexistent">
<meta property="og:description" content="">
</head></html>`

func TestFetchProfile_DescriptionWithEmbeddedURL(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("codescoring"): {status: http.StatusOK, body: realFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "codescoring")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["url:https://codescoring.ru/"])
}

func TestFetchProfile_DescriptionWithEmbeddedEmail(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("u"): {status: http.StatusOK, body: `<html><head>
				<meta property="og:description" content="contact us at hello@example.com">
			</head></html>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["email:hello@example.com"])
}

// Regression: found live -- the old greedy `\S+` URL pattern pulled in
// trailing punctuation adjacent to the URL in free-text descriptions.
func TestFetchProfile_URLWithTrailingParenthesisAndPeriodNotCaptured(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("u"): {status: http.StatusOK, body: `<html><head>
				<meta property="og:description" content="Follow us (https://t.me/channel).">
			</head></html>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["url:https://t.me/channel"])
	assert.False(t, got["url:https://t.me/channel)."])
}

func TestFetchProfile_URLWithTrailingCommaNotCaptured(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("u"): {status: http.StatusOK, body: `<html><head>
				<meta property="og:description" content="Contact: https://example.com, or email us">
			</head></html>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["url:https://example.com"])
	assert.False(t, got["url:https://example.com,"])
}

func TestFetchProfile_EmptyDescriptionForNonexistentChannel(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("zzzznonexistent"): {status: http.StatusOK, body: nonexistentFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "zzzznonexistent")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_PlainTextDescriptionNoTraces(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("u"): {status: http.StatusOK, body: `<html><head>
				<meta property="og:description" content="just a normal channel description">
			</head></html>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_NonOKHTTPStatusIsError(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("u"): {status: http.StatusInternalServerError, body: ""},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}
