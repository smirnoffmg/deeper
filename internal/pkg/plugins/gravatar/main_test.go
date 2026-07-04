package gravatar

import (
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := testPlugin(&fakeProfileFetcher{})
	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_ProfileFound(t *testing.T) {
	hash := emailHash("jane@example.com")
	body := `{"entry":[{"displayName":"Jane Doe","accounts":[]}]}`
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			"https://gravatar.com/" + hash + ".json": {status: http.StatusOK, body: body},
		},
	}

	p := testPlugin(fetcher)
	traces, err := p.FollowTrace(entities.Trace{Type: entities.Email, Value: "jane@example.com"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
	assert.Equal(t, entities.Name, traces[0].Type)
	assert.Equal(t, "Jane Doe", traces[0].Value)
}

func TestFollowTrace_ProfileNotFound(t *testing.T) {
	hash := emailHash("nobody@example.com")
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			"https://gravatar.com/" + hash + ".json": {status: http.StatusNotFound, body: `"User not found"`},
		},
	}

	p := testPlugin(fetcher)
	traces, err := p.FollowTrace(entities.Trace{Type: entities.Email, Value: "nobody@example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_SetsAuthorizationWhenAPIKeyPresent(t *testing.T) {
	hash := emailHash("jane@example.com")
	body := `{"display_name":"Jane Doe"}`
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			"https://api.gravatar.com/v3/profiles/" + hash: {status: http.StatusOK, body: body},
		},
	}

	p := &GravatarPlugin{fetcher: fetcher, apiKey: "secret-key"}
	_, err := p.FollowTrace(entities.Trace{Type: entities.Email, Value: "jane@example.com"})
	require.NoError(t, err)
	require.NotNil(t, fetcher.lastReq)
	assert.Equal(t, "Bearer secret-key", fetcher.lastReq.Header.Get("Authorization"))
}

func TestString(t *testing.T) {
	p := testPlugin(&fakeProfileFetcher{})
	assert.Equal(t, "GravatarPlugin", p.String())
}

func testPlugin(fetcher profileFetcher) *GravatarPlugin {
	return &GravatarPlugin{fetcher: fetcher}
}

var _ profileFetcher = (*fakeProfileFetcher)(nil)
