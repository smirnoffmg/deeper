package launchpad_profile

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

func profileURL(handle string) string {
	return "https://launchpad.net/~" + handle
}

func TestExtractHandle_ValidLaunchpadURL(t *testing.T) {
	assert.Equal(t, "alsmirn", extractHandle("https://launchpad.net/~alsmirn"))
}

func TestExtractHandle_NonLaunchpadURL(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://keybase.io/alsmirn"))
}

func TestExtractHandle_MissingTilde(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://launchpad.net/alsmirn"))
}

func TestFetchProfile_FullNamePresent(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("lig"): {status: http.StatusOK, body: `<html><head>
				<meta property="og:title" content="Serge Matveenko in Launchpad" />
			</head></html>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "lig")
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Name, traces[0].Type)
	assert.Equal(t, "Serge Matveenko", traces[0].Value)
}

func TestFetchProfile_NameEchoingHandleSkipped(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `<html><head>
				<meta property="og:title" content="u in Launchpad" />
			</head></html>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_NotFoundReturnsNoTracesNoError(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("zzzznonexistent"): {status: http.StatusNotFound, body: "<html><head><title>Error: Page not found</title></head></html>"},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "zzzznonexistent")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_MissingOGTitleNoTraces(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: "<html><head></head></html>"},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchProfile_OtherNonOKStatusIsError(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusInternalServerError, body: ""},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}
