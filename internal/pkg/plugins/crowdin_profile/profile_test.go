package crowdin_profile

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
	return "https://crowdin.com/profile/" + handle
}

func TestExtractHandle_ValidCrowdinURL(t *testing.T) {
	assert.Equal(t, "alsmirn", extractHandle("https://crowdin.com/profile/alsmirn"))
}

func TestExtractHandle_NonCrowdinURL(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://keybase.io/alsmirn"))
}

func TestExtractHandle_RootPath(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://crowdin.com/"))
}

func TestFetchProfile_DistinctFullNameEmitted(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("alsmirn"): {status: http.StatusOK, body: "<html><head><title>Alexey Smirnov (alsmirn) – Crowdin</title></head></html>"},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Name, traces[0].Type)
	assert.Equal(t, "Alexey Smirnov", traces[0].Value)
}

func TestFetchProfile_NameEchoingUsernameSkipped(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("lig"): {status: http.StatusOK, body: "<html><head><title>Lig (Lig) – Crowdin</title></head></html>"},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "lig")
	require.NoError(t, err)
	assert.Empty(t, traces, "a display name that's just the username shouldn't be reported as a real name")
}

func TestFetchProfile_NotFoundReturnsNoTracesNoError(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("zzzznonexistent"): {status: http.StatusNotFound, body: "<html><head><title>Page Not Found - Crowdin</title></head></html>"},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "zzzznonexistent")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_UnparsableTitleNoTraces(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: "<html><head><title>Something unexpected</title></head></html>"},
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
