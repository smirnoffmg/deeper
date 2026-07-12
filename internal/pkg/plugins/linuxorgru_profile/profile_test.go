package linuxorgru_profile

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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

func profileURL(handle string) string {
	return "https://www.linux.org.ru/people/" + handle + "/profile"
}

func TestExtractHandle_ValidURL(t *testing.T) {
	assert.Equal(t, "alsmirn", extractHandle("https://www.linux.org.ru/people/alsmirn/profile"))
}

func TestExtractHandle_NonMatchingHost(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://keybase.io/alsmirn"))
}

func TestExtractHandle_WrongPathShape(t *testing.T) {
	assert.Equal(t, "", extractHandle("https://www.linux.org.ru/people/alsmirn"))
}

// realFixture mirrors the actual profile fragment captured live from
// linux.org.ru/people/alsmirn/profile.
const realFixture = `<div class="vcard">
    <b>Nick:</b> <span class="nickname">
        alsmirn</span><br>
    <b>Имя:</b> <span class="fn">Alexey Smirnov</span><br>
    <b>ID:</b> 51237<br>
    <br>
    <b>Город:</b> Санкт-Петербург<br>
    </div>`

func TestFetchProfile_NameAndCityPresent(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("alsmirn"): {status: http.StatusOK, body: realFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tr := range traces {
		got[string(tr.Type)+":"+tr.Value] = true
	}
	assert.True(t, got["name:Alexey Smirnov"])
	assert.True(t, got["address:Санкт-Петербург"])
}

func TestFetchProfile_NameEchoingNickSkipped(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `<div class="vcard"><b>Nick:</b> <span class="nickname">u</span><br>
				<b>Имя:</b> <span class="fn">u</span><br></div>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	for _, tr := range traces {
		assert.NotEqual(t, entities.Name, tr.Type)
	}
}

func TestFetchProfile_NoCityField(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("u"): {status: http.StatusOK, body: `<div class="vcard">
				<b>Имя:</b> <span class="fn">Someone Real</span><br></div>`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Name, traces[0].Type)
}

func TestFetchProfile_NotFoundReturnsNoTracesNoError(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("zzzznonexistent"): {status: http.StatusNotFound, body: ""},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "zzzznonexistent")
	require.NoError(t, err)
	assert.Nil(t, traces)
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
