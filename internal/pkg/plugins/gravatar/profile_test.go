package gravatar

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

func TestEmailHash(t *testing.T) {
	// SHA256 of "test@example.com" (lowercase, trimmed)
	assert.Equal(t, "973dfe463ec85785f5f95af5ba3906eedb2d931c24e69824a89ea65dba4e813b", emailHash("test@example.com"))
	assert.Equal(t, emailHash("test@example.com"), emailHash("  TEST@EXAMPLE.COM  "))
}

func TestFetchProfile_Found(t *testing.T) {
	body := `{"entry":[{"displayName":"Jane Doe","accounts":[{"domain":"twitter.com","username":"jane","url":"https://twitter.com/jane"}]}]}`
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			"https://gravatar.com/abc123.json": {status: http.StatusOK, body: body},
		},
	}

	profile, found, err := fetchProfile(context.Background(), fetcher, "abc123", "")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Jane Doe", profile.displayName)
}

func TestFetchProfile_V3Found(t *testing.T) {
	body := `{"display_name":"Jane Doe","first_name":"Jane","last_name":"Doe","company":"Acme Corp","verified_accounts":[{"type":"twitter","url":"https://twitter.com/jane"}]}`
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			"https://api.gravatar.com/v3/profiles/abc123": {status: http.StatusOK, body: body},
		},
	}

	profile, found, err := fetchProfile(context.Background(), fetcher, "abc123", "test-key")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Jane", profile.firstName)
	assert.Equal(t, "Doe", profile.lastName)
	assert.Equal(t, "Acme Corp", profile.company)
	require.Len(t, profile.verifiedAccounts, 1)
}

func TestFetchProfile_NotFound(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			"https://gravatar.com/missing.json": {status: http.StatusNotFound, body: `"User not found"`},
		},
	}

	_, found, err := fetchProfile(context.Background(), fetcher, "missing", "")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFetchProfile_MalformedJSON(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			"https://gravatar.com/bad.json": {status: http.StatusOK, body: `{not json`},
		},
	}

	_, found, err := fetchProfile(context.Background(), fetcher, "bad", "")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestProfileToTraces_FullProfile(t *testing.T) {
	profile := &gravatarProfile{
		displayName: "Jane Doe",
		firstName:   "Jane",
		lastName:    "Doe",
		company:     "Acme Corp",
		verifiedAccounts: []verifiedAccount{
			{accountType: "twitter", url: "https://twitter.com/jane"},
		},
	}

	traces := profileToTraces(profile, "jane@example.com")
	types := traceTypes(traces)

	assert.Contains(t, types, entities.Name)
	assert.Contains(t, types, entities.Company)
	assert.Contains(t, types, entities.Twitter)
	assert.Equal(t, "Jane Doe", traceValue(traces, entities.Name))
	assert.Equal(t, "Acme Corp", traceValue(traces, entities.Company))
}

func TestProfileToTraces_SuppressesNameEchoingLocalPart(t *testing.T) {
	profile := &gravatarProfile{displayName: "jane"}

	traces := profileToTraces(profile, "jane@example.com")

	for _, tr := range traces {
		assert.NotEqual(t, entities.Name, tr.Type)
	}
}

func TestProfileToTraces_EmptyProfile(t *testing.T) {
	traces := profileToTraces(&gravatarProfile{}, "user@example.com")
	assert.Empty(t, traces)
}

type fakeResponse struct {
	status int
	body   string
}

type fakeProfileFetcher struct {
	responses map[string]fakeResponse
	lastReq   *http.Request
}

func (f *fakeProfileFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	return f.respond(ctx, url, nil)
}

func (f *fakeProfileFetcher) Do(req *http.Request) (*http.Response, error) {
	return f.respond(req.Context(), req.URL.String(), req)
}

func (f *fakeProfileFetcher) respond(ctx context.Context, url string, req *http.Request) (*http.Response, error) {
	if req != nil {
		f.lastReq = req
	}
	resp, ok := f.responses[url]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`"User not found"`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func traceTypes(traces []entities.Trace) []entities.TraceType {
	types := make([]entities.TraceType, 0, len(traces))
	for _, tr := range traces {
		types = append(types, tr.Type)
	}
	return types
}

func traceValue(traces []entities.Trace, typ entities.TraceType) string {
	for _, tr := range traces {
		if tr.Type == typ {
			return tr.Value
		}
	}
	return ""
}
