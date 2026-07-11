package habr_profile

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

func cardURL(username string) string {
	return "https://habr.com/kek/v2/users/" + username + "/card"
}

// realFixture mirrors the actual JSON captured live from
// https://habr.com/kek/v2/users/alsmirn/card.
const realFixture = `{
	"alias": "alsmirn",
	"fullname": "Алексей Смирнов",
	"speciality": "Основатель CodeScoring",
	"location": null,
	"birthday": null,
	"workplace": [{"title": "CodeScoring", "alias": "codescoring"}]
}`

func TestFetchProfile_AllFieldsPresent(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("alsmirn"): {status: http.StatusOK, body: `{
				"fullname": "Alexey Smirnov",
				"location": "Saint Petersburg, Russia",
				"birthday": "1985-03-10T00:00:00+00:00",
				"workplace": [{"title": "CodeScoring", "alias": "codescoring"}]
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
	assert.Equal(t, "Saint Petersburg, Russia", got[entities.Address])
	assert.Equal(t, "1985-03-10T00:00:00+00:00", got[entities.DateOfBirth])
	assert.Equal(t, "CodeScoring", got[entities.Company])
}

func TestFetchProfile_RealFixture(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("alsmirn"): {status: http.StatusOK, body: realFixture},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "alsmirn")
	require.NoError(t, err)

	got := map[entities.TraceType]string{}
	for _, tr := range traces {
		got[tr.Type] = tr.Value
	}
	assert.Equal(t, "Алексей Смирнов", got[entities.Name])
	assert.Equal(t, "CodeScoring", got[entities.Company])
	assert.NotContains(t, got, entities.Address, "null location must not produce a trace")
	assert.NotContains(t, got, entities.DateOfBirth, "null birthday must not produce a trace")
}

func TestFetchProfile_MultipleWorkplaces(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("u"): {status: http.StatusOK, body: `{
				"fullname": "Someone",
				"workplace": [{"title": "CompanyA"}, {"title": "CompanyB"}]
			}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)

	var companies []string
	for _, tr := range traces {
		if tr.Type == entities.Company {
			companies = append(companies, tr.Value)
		}
	}
	assert.ElementsMatch(t, []string{"CompanyA", "CompanyB"}, companies)
}

func TestFetchProfile_EmptyWorkplaceArray(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("u"): {status: http.StatusOK, body: `{"fullname": "Someone", "workplace": []}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "u")
	require.NoError(t, err)
	for _, tr := range traces {
		assert.NotEqual(t, entities.Company, tr.Type)
	}
}

func TestFetchProfile_NotFoundReturnsNoTracesNoError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("nobody"): {status: http.StatusNotFound, body: `{"httpCode":404,"message":"Account with login ` + "`nobody`" + ` not found","errorCode":"NOT_FOUND"}`},
		},
	}

	traces, err := fetchProfile(context.Background(), fetcher, "nobody")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFetchProfile_OtherNonOKStatusReturnsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("u"): {status: http.StatusTooManyRequests, body: `{"message":"rate limited"}`},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}

func TestFetchProfile_MalformedJSONReturnsError(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("u"): {status: http.StatusOK, body: `not json`},
		},
	}

	_, err := fetchProfile(context.Background(), fetcher, "u")
	assert.Error(t, err)
}
